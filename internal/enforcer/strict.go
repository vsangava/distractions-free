package enforcer

import (
	"log"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/pf"
)

// StrictEnforcer composes DNS-proxy blocking with pf firewall blocking.
// pf enforcement is currently a stub; the enforcer degrades gracefully to
// DNS-only until the pf package is fully implemented (see issue #12).
type StrictEnforcer struct {
	dns *DNSEnforcer
	cfg config.Config
}

func NewStrictEnforcer(cfg config.Config) *StrictEnforcer {
	return &StrictEnforcer{dns: NewDNSEnforcer(cfg), cfg: cfg}
}

func (e *StrictEnforcer) Setup() error {
	if err := e.dns.Setup(); err != nil {
		return err
	}
	if err := pf.InstallAnchor(); err != nil {
		log.Printf("strict: pf anchor setup failed, continuing with DNS-only: %v", err)
	}
	return nil
}

func (e *StrictEnforcer) Teardown() error {
	e.dns.Teardown()
	pf.RemoveAnchor()
	return nil
}

func (e *StrictEnforcer) Activate(domains []string) error {
	if err := e.dns.Activate(domains); err != nil {
		return err
	}
	// Re-resolve ALL currently blocked domains on every activation, not just the
	// newly added ones. CDN-backed sites rotate IPs frequently; refreshing from
	// scratch each tick evicts stale addresses that would otherwise linger in the
	// pf table until the block ends.
	e.dns.mu.Lock()
	allDomains := make([]string, 0, len(e.dns.blocked))
	for d := range e.dns.blocked {
		allDomains = append(allDomains, d)
	}
	e.dns.mu.Unlock()
	pf.DeactivateBlock()
	pf.ActivateBlock(allDomains, e.cfg.Settings.PrimaryDNS, e.cfg.Settings.BackupDNS)
	return nil
}

func (e *StrictEnforcer) Deactivate(domains []string) error {
	if err := e.dns.Deactivate(domains); err != nil {
		return err
	}
	// Rebuild pf table from the remaining DNS-blocked set.
	pf.DeactivateBlock()
	e.dns.mu.Lock()
	remaining := make([]string, 0, len(e.dns.blocked))
	for d := range e.dns.blocked {
		remaining = append(remaining, d)
	}
	e.dns.mu.Unlock()
	if len(remaining) > 0 {
		pf.ActivateBlock(remaining, e.cfg.Settings.PrimaryDNS, e.cfg.Settings.BackupDNS)
	}
	return nil
}

func (e *StrictEnforcer) DeactivateAll() error {
	e.dns.DeactivateAll()
	pf.DeactivateBlock()
	return nil
}

// Refresh re-resolves all currently blocked domains and reloads the pf anchor.
// Called every scheduler tick so CDN IP rotations are picked up without waiting
// for a domain set change to trigger Activate.
func (e *StrictEnforcer) Refresh() {
	e.dns.mu.Lock()
	domains := make([]string, 0, len(e.dns.blocked))
	for d := range e.dns.blocked {
		domains = append(domains, d)
	}
	e.dns.mu.Unlock()

	if len(domains) == 0 {
		return
	}
	pf.DeactivateBlock()
	pf.ActivateBlock(domains, e.cfg.Settings.PrimaryDNS, e.cfg.Settings.BackupDNS)
}
