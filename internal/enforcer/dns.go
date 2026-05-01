package enforcer

import (
	"bufio"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/proxy"
)

// DNSEnforcer preserves the original DNS-proxy behaviour: it maintains a
// running blocked-domains map and pushes updates to the proxy on each change.
// The DNS server itself is started by main.go, not here.
type DNSEnforcer struct {
	cfg     config.Config
	blocked map[string]bool
	mu      sync.Mutex
}

func NewDNSEnforcer(cfg config.Config) *DNSEnforcer {
	return &DNSEnforcer{cfg: cfg, blocked: make(map[string]bool)}
}

func (e *DNSEnforcer) Setup() error {
	proxy.UpdateBlockedDomains(make(map[string]bool))
	if runtime.GOOS == "darwin" {
		e.configureSystemDNS()
	}
	return nil
}

// configureSystemDNS detects the current upstream DNS before taking over port 53,
// saves it as primary_dns if the config is still at factory default, then points
// every active network interface at the local DNS proxy (127.0.0.1).
func (e *DNSEnforcer) configureSystemDNS() {
	if upstream := detectSystemDNS(); upstream != "" {
		config.AutoSetPrimaryDNS(upstream)
	}
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		exec.Command("networksetup", "-setdnsservers", "Wi-Fi", "127.0.0.1").Run()
		flushDNSCache()
		return
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "An asterisk") || strings.TrimSpace(line) == "" {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		exec.Command("networksetup", "-setdnsservers", name, "127.0.0.1").Run()
	}
	flushDNSCache()
}

// detectSystemDNS returns the first non-loopback DNS server currently configured
// on the system, in host:port form. Returns "" if none can be determined.
// It first tries manually configured servers; if none, falls back to DHCP-assigned
// DNS from scutil.
func detectSystemDNS() string {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err == nil {
		sc := bufio.NewScanner(strings.NewReader(string(out)))
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "An asterisk") || strings.TrimSpace(line) == "" {
				continue
			}
			name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
			dnsOut, err := exec.Command("networksetup", "-getdnsservers", name).Output()
			if err != nil {
				continue
			}
			for _, srv := range strings.Split(strings.TrimSpace(string(dnsOut)), "\n") {
				srv = strings.TrimSpace(srv)
				if net.ParseIP(srv) != nil && !strings.HasPrefix(srv, "127.") {
					return srv + ":53"
				}
			}
		}
	}
	// Fall back to DHCP-assigned DNS via scutil.
	scOut, err := exec.Command("scutil", "--dns").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(scOut), "\n") {
		if strings.Contains(line, "nameserver[0]") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				srv := strings.TrimSpace(parts[1])
				if net.ParseIP(srv) != nil && !strings.HasPrefix(srv, "127.") {
					return srv + ":53"
				}
			}
		}
	}
	return ""
}

func (e *DNSEnforcer) Teardown() error {
	if err := e.DeactivateAll(); err != nil {
		log.Printf("dns teardown: %v", err)
	}
	// Restore system DNS on every interface that points at 127.0.0.1.
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
		if err == nil {
			sc := bufio.NewScanner(strings.NewReader(string(out)))
			for sc.Scan() {
				line := sc.Text()
				if strings.HasPrefix(line, "An asterisk") || strings.TrimSpace(line) == "" {
					continue
				}
				name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
				dnsOut, _ := exec.Command("networksetup", "-getdnsservers", name).Output()
				if strings.Contains(string(dnsOut), "127.0.0.1") {
					exec.Command("networksetup", "-setdnsservers", name, "Empty").Run()
				}
			}
		} else {
			exec.Command("networksetup", "-setdnsservers", "Wi-Fi", "Empty").Run()
		}
		exec.Command("dscacheutil", "-flushcache").Run()
		exec.Command("killall", "-HUP", "mDNSResponder").Run()
	case "windows":
		exec.Command("powershell", "-Command",
			`Get-DnsClientServerAddress | Where-Object { $_.ServerAddresses -contains "127.0.0.1" } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses }`).Run()
	}
	return nil
}

func (e *DNSEnforcer) Activate(domains []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range domains {
		e.blocked[d] = true
	}
	proxy.UpdateBlockedDomains(e.blocked)
	flushDNSCache()
	log.Printf("dns: activated %v", domains)
	return nil
}

func (e *DNSEnforcer) Deactivate(domains []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, d := range domains {
		delete(e.blocked, d)
	}
	proxy.UpdateBlockedDomains(e.blocked)
	flushDNSCache()
	log.Printf("dns: deactivated %v", domains)
	return nil
}

func (e *DNSEnforcer) DeactivateAll() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.blocked = make(map[string]bool)
	proxy.UpdateBlockedDomains(e.blocked)
	return nil
}
