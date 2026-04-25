// Package pf wraps pfctl for firewall-level domain blocking on macOS.
// Requires root; all exported functions degrade gracefully (log + no-op) on non-darwin or non-root.
package pf

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const (
	anchorName = "distractions-free"
	anchorFile = "/etc/pf.anchors/distractions-free"
	pfConf     = "/etc/pf.conf"
	markerBeg  = "# distractions-free:begin"
	markerEnd  = "# distractions-free:end"
	anchorLine = "anchor \"distractions-free\""
	loadLine   = "load anchor \"distractions-free\" from \"/etc/pf.anchors/distractions-free\""
	dnsTimeout = 3 * time.Second
)

// Preview is returned by GeneratePreview — the data the web UI renders without root.
type Preview struct {
	Domains      []string            `json:"domains"`
	ResolvedIPs  map[string][]string `json:"resolved_ips"`
	AnchorContent string             `json:"anchor_content"`
}

// ResolveDomainIPs returns deduplicated A and AAAA addresses for domain using dnsServer (host:port).
// Falls back to net.LookupHost if DNS query fails.
func ResolveDomainIPs(domain, dnsServer string) []string {
	seen := make(map[string]bool)
	var ips []string

	add := func(addr string) {
		if !seen[addr] {
			seen[addr] = true
			ips = append(ips, addr)
		}
	}

	c := new(dns.Client)
	c.Timeout = dnsTimeout

	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA} {
		m := new(dns.Msg)
		m.SetQuestion(dns.Fqdn(domain), qtype)
		m.RecursionDesired = true

		r, _, err := c.Exchange(m, dnsServer)
		if err != nil || r == nil {
			continue
		}
		for _, ans := range r.Answer {
			switch rr := ans.(type) {
			case *dns.A:
				add(rr.A.String())
			case *dns.AAAA:
				add(rr.AAAA.String())
			}
		}
	}

	if len(ips) == 0 {
		addrs, err := net.LookupHost(domain)
		if err == nil {
			for _, a := range addrs {
				add(a)
			}
		}
	}

	return ips
}

// GenerateAnchorContent builds the pfctl anchor rules for the given IP list.
// Pure function — no I/O, safe to call without root.
func GenerateAnchorContent(ips []string) string {
	if len(ips) == 0 {
		return "# no IPs to block\n"
	}

	var sb strings.Builder
	sb.WriteString("table <blocked_ips> persist {\n")
	for _, ip := range ips {
		sb.WriteString("  " + ip + "\n")
	}
	sb.WriteString("}\n")
	sb.WriteString("block drop out quick proto {tcp udp} from any to <blocked_ips>\n")
	return sb.String()
}

// GeneratePreview resolves IPs for each domain and builds anchor content without touching the system.
func GeneratePreview(domains []string, dnsServer string) Preview {
	resolved := make(map[string][]string, len(domains))
	var allIPs []string
	seen := make(map[string]bool)

	for _, d := range domains {
		ips := ResolveDomainIPs(d, dnsServer)
		resolved[d] = ips
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				allIPs = append(allIPs, ip)
			}
		}
	}

	return Preview{
		Domains:       domains,
		ResolvedIPs:   resolved,
		AnchorContent: GenerateAnchorContent(allIPs),
	}
}

// InstallAnchor writes the anchor stub file and injects the load directive into /etc/pf.conf.
// Safe to call if anchor is already installed (idempotent).
func InstallAnchor() error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	// Write a placeholder anchor file so pfctl can validate the config.
	if err := os.WriteFile(anchorFile, []byte("# managed by distractions-free\n"), 0644); err != nil {
		return fmt.Errorf("pf: write anchor file: %w", err)
	}

	if err := injectPFConf(); err != nil {
		return fmt.Errorf("pf: update pf.conf: %w", err)
	}

	// Dry-run validation before reloading.
	if out, err := exec.Command("pfctl", "-n", "-f", pfConf).CombinedOutput(); err != nil {
		return fmt.Errorf("pf: config validation failed: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if out, err := exec.Command("pfctl", "-f", pfConf).CombinedOutput(); err != nil {
		return fmt.Errorf("pf: reload pf.conf: %w: %s", err, strings.TrimSpace(string(out)))
	}

	if out, err := exec.Command("pfctl", "-e").CombinedOutput(); err != nil {
		// pfctl -e returns exit 1 if pf is already enabled; that's fine.
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "already enabled") {
			log.Printf("pf: enable warning: %v: %s", err, msg)
		}
	}

	log.Printf("pf: anchor installed")
	return nil
}

// RemoveAnchor flushes the block table, removes anchor directives from pf.conf, removes the anchor file, and reloads.
func RemoveAnchor() {
	if runtime.GOOS != "darwin" {
		return
	}

	// Flush table first so existing connections are released.
	DeactivateBlock()

	if err := stripPFConf(); err != nil {
		log.Printf("pf: strip pf.conf: %v", err)
	}

	if out, err := exec.Command("pfctl", "-f", pfConf).CombinedOutput(); err != nil {
		log.Printf("pf: reload after remove: %v: %s", err, strings.TrimSpace(string(out)))
	}

	if err := os.Remove(anchorFile); err != nil && !os.IsNotExist(err) {
		log.Printf("pf: remove anchor file: %v", err)
	}

	log.Printf("pf: anchor removed")
}

// ActivateBlock resolves IPs for the given domains, writes the anchor file, and loads it into pf.
func ActivateBlock(domains []string, primaryDNS string) {
	if runtime.GOOS != "darwin" || len(domains) == 0 {
		return
	}

	resolved := make(map[string][]string, len(domains))
	seen := make(map[string]bool)
	var allIPs []string

	for _, d := range domains {
		ips := ResolveDomainIPs(d, primaryDNS)
		resolved[d] = ips
		for _, ip := range ips {
			if !seen[ip] {
				seen[ip] = true
				allIPs = append(allIPs, ip)
			}
		}
	}

	if len(allIPs) == 0 {
		log.Printf("pf: no IPs resolved for domains %v — skipping activation", domains)
		return
	}

	content := GenerateAnchorContent(allIPs)
	if err := os.WriteFile(anchorFile, []byte(content), 0644); err != nil {
		log.Printf("pf: write anchor file: %v", err)
		return
	}

	// Load the anchor into the running pf.
	if out, err := exec.Command("pfctl", "-a", anchorName, "-f", anchorFile).CombinedOutput(); err != nil {
		log.Printf("pf: load anchor: %v: %s", err, strings.TrimSpace(string(out)))
		return
	}

	// Kill existing outbound states to those IPs so cached connections are severed.
	for _, ip := range allIPs {
		killStateToIP(ip)
	}

	log.Printf("pf: activated block for %d domains (%d IPs)", len(domains), len(allIPs))
}

// DeactivateBlock flushes all IPs from the pf block table.
func DeactivateBlock() {
	if runtime.GOOS != "darwin" {
		return
	}

	out, err := exec.Command("pfctl", "-a", anchorName, "-t", "blocked_ips", "-T", "flush").CombinedOutput()
	if err != nil {
		// Table may not exist yet — that's fine on first run.
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "No such table") && !strings.Contains(msg, "No ALTQ support") {
			log.Printf("pf: flush table: %v: %s", err, msg)
		}
	}
}

// killStateToIP kills outbound pf states targeting the given IP.
func killStateToIP(ip string) {
	// IPv6 addresses use a different source wildcard.
	src := "0.0.0.0/0"
	if strings.Contains(ip, ":") {
		src = "::/0"
	}
	if out, err := exec.Command("pfctl", "-k", src, "-k", ip).CombinedOutput(); err != nil {
		log.Printf("pf: kill state for %s: %v: %s", ip, err, strings.TrimSpace(string(out)))
	}
}

// injectPFConf adds the anchor and load lines to /etc/pf.conf if not already present.
func injectPFConf() error {
	data, err := os.ReadFile(pfConf)
	if err != nil {
		return err
	}

	content := string(data)
	if strings.Contains(content, markerBeg) {
		return nil // already present
	}

	injection := "\n" + markerBeg + "\n" + anchorLine + "\n" + loadLine + "\n" + markerEnd + "\n"
	return atomicWrite(pfConf, []byte(content+injection), 0644)
}

// stripPFConf removes the marker-delimited block from /etc/pf.conf.
func stripPFConf() error {
	data, err := os.ReadFile(pfConf)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var out []byte
	skip := false
	for scanner.Scan() {
		line := scanner.Text()
		if line == markerBeg {
			skip = true
			continue
		}
		if line == markerEnd {
			skip = false
			continue
		}
		if !skip {
			out = append(out, []byte(line+"\n")...)
		}
	}

	// Trim trailing blank lines added by injection.
	out = bytes.TrimRight(out, "\n")
	out = append(out, '\n')

	return atomicWrite(pfConf, out, 0644)
}

// atomicWrite writes data to path via a temp file + rename.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := "/"
	tmp, err := os.CreateTemp(dir, ".pf-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
