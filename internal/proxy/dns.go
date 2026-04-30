package proxy

import (
	"log"
	"strings"
	"sync"

	"github.com/miekg/dns"
	"github.com/vsangava/sentinel/internal/config"
)

var (
	blockedDomains map[string]bool
	blockMu        sync.RWMutex
	dnsServer      *dns.Server
)

func UpdateBlockedDomains(newBlocked map[string]bool) {
	blockMu.Lock()
	defer blockMu.Unlock()
	blockedDomains = newBlocked
}

// IsDomainBlocked reports whether domain matches any entry in blocked, including subdomains.
func IsDomainBlocked(domain string, blocked map[string]bool) bool {
	if blocked[domain] {
		return true
	}
	for d := range blocked {
		if strings.HasSuffix(domain, "."+d) {
			return true
		}
	}
	return false
}

// GetDNSResponse is a testable function that processes DNS requests without binding to a port.
// It returns the appropriate DNS response based on blocking rules and upstream DNS queries.
func GetDNSResponse(r *dns.Msg, blockedDomainsList map[string]bool, primaryDNS, backupDNS string) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	if len(r.Question) == 0 {
		return m, nil
	}

	q := r.Question[0]
	domain := strings.TrimSuffix(q.Name, ".")

	if IsDomainBlocked(domain, blockedDomainsList) {
		switch q.Qtype {
		case dns.TypeA:
			rr, _ := dns.NewRR(q.Name + " 60 IN A 0.0.0.0")
			m.Answer = append(m.Answer, rr)
			return m, nil
		case dns.TypeAAAA:
			rr, _ := dns.NewRR(q.Name + " 60 IN AAAA ::")
			m.Answer = append(m.Answer, rr)
			return m, nil
		}
	}

	// Forward to upstream DNS
	c := new(dns.Client)

	in, _, err := c.Exchange(r, primaryDNS)
	if err != nil {
		in, _, err = c.Exchange(r, backupDNS)
		if err != nil {
			return nil, err
		}
	}
	return in, nil
}

func handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	blockMu.RLock()
	blockedCopy := blockedDomains
	blockMu.RUnlock()

	cfg := config.GetConfig()

	m, err := GetDNSResponse(r, blockedCopy, cfg.Settings.PrimaryDNS, cfg.Settings.BackupDNS)
	if err != nil {
		dns.HandleFailed(w, r)
		return
	}

	w.WriteMsg(m)
}

func StartDNSServer() {
	UpdateBlockedDomains(make(map[string]bool))
	dns.HandleFunc(".", handleDNSRequest)

	dnsServer = &dns.Server{Addr: "127.0.0.1:53", Net: "udp"}
	log.Printf("Starting local DNS proxy on 127.0.0.1:53...")
	if err := dnsServer.ListenAndServe(); err != nil {
		log.Fatalf("Failed to start DNS server: %s", err.Error())
	}
}

func StopDNSServer() {
	if dnsServer != nil {
		if err := dnsServer.Shutdown(); err != nil {
			log.Printf("DNS server shutdown error: %v", err)
		}
	}
}
