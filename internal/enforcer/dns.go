package enforcer

import (
	"bufio"
	"log"
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
	return nil
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
