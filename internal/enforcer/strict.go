package enforcer

import (
	"log"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/pf"
)

// DohGroupName names the special group of DNS-over-HTTPS / DNS-over-TLS endpoints.
// Its domains get a different pf treatment from the regular blocked set: instead of
// the all-port block used for site IPs, the resolved DoH/DoT IPs get port-restricted
// rules (TCP/443 for DoH, TCP+UDP/853 for DoT). Plain DNS on UDP/53 is left reachable
// so the daemon's own backup_dns (often 1.1.1.1:53 — same IP as a DoH endpoint) keeps
// resolving real domains. See pf.GenerateAnchorContentMixed.
//
// Exported so the scheduler can identify and skip these domains in browser-tab
// matching paths — DoH endpoints aren't sites users visit with browsers.
const DohGroupName = "_doh"

// StrictEnforcer composes DNS-proxy blocking with pf firewall blocking.
// Domains in the _doh group flow into a port-restricted pf section instead of the
// all-port one — see reloadPF.
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
	e.reloadPF()
	return nil
}

func (e *StrictEnforcer) Deactivate(domains []string) error {
	if err := e.dns.Deactivate(domains); err != nil {
		return err
	}
	e.reloadPF()
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
	e.reloadPF()
}

// reloadPF rewrites the pf anchor from the current DNS-blocked set, splitting it
// into a regular set (all-port pf rules) and a _doh set (port-restricted pf rules
// for TCP/443 and TCP+UDP/853 only). Both sections live in one anchor file —
// see pf.GenerateAnchorContentMixed.
//
// pf.ActivateBlockMixed is an atomic anchor overwrite, so no DeactivateBlock call
// is needed before it — adding one would briefly leave pf with no rules between
// the two reloads.
//
// Resolves a fresh config every call so a toggle of _doh.is_active or a domain
// addition takes effect on the next tick without restarting the enforcer.
func (e *StrictEnforcer) reloadPF() {
	cfg := config.GetConfig()
	dohSet := make(map[string]bool)
	for _, d := range cfg.ResolveGroup(DohGroupName) {
		dohSet[d] = true
	}

	e.dns.mu.Lock()
	var blockDomains, dohDomains []string
	for d := range e.dns.blocked {
		if dohSet[d] {
			dohDomains = append(dohDomains, d)
		} else {
			blockDomains = append(blockDomains, d)
		}
	}
	e.dns.mu.Unlock()

	if len(blockDomains) == 0 && len(dohDomains) == 0 {
		pf.DeactivateBlock()
		return
	}
	pf.ActivateBlockMixed(blockDomains, dohDomains, cfg.Settings.PrimaryDNS, cfg.Settings.BackupDNS)
}
