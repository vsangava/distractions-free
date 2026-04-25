package enforcer

import (
	"log"
	"os/exec"
	"runtime"
	"sync"

	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/proxy"
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
	// Restore system DNS — currently resets Wi-Fi only.
	// Full multi-interface cleanup is tracked in issue #12.
	if runtime.GOOS == "darwin" {
		exec.Command("networksetup", "-setdnsservers", "Wi-Fi", "Empty").Run()
		exec.Command("dscacheutil", "-flushcache").Run()
		exec.Command("killall", "-HUP", "mDNSResponder").Run()
	} else if runtime.GOOS == "windows" {
		exec.Command("powershell", "-Command",
			"Set-DnsClientServerAddress -InterfaceAlias 'Wi-Fi' -ResetServerAddresses").Run()
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
