package pf

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateAnchorContent_empty(t *testing.T) {
	got := GenerateAnchorContent(nil)
	if !strings.Contains(got, "no IPs") {
		t.Errorf("expected no-IPs comment, got %q", got)
	}
}

func TestGenerateAnchorContent_withIPs(t *testing.T) {
	ips := []string{"1.2.3.4", "5.6.7.8", "2001:db8::1"}
	got := GenerateAnchorContent(ips)

	for _, ip := range ips {
		if !strings.Contains(got, ip) {
			t.Errorf("missing IP %s in anchor content", ip)
		}
	}
	if !strings.Contains(got, "block drop out quick inet proto {tcp udp}") {
		t.Error("missing inet block rule")
	}
	if !strings.Contains(got, "block drop out quick inet6 proto {tcp udp}") {
		t.Error("missing inet6 block rule")
	}
	if strings.Contains(got, "table") {
		t.Error("anchor content must not use table declarations (unsupported on macOS)")
	}
}

func TestGeneratePreview_structure(t *testing.T) {
	p := GeneratePreview([]string{"google.com"}, "8.8.8.8:53")

	if len(p.Domains) != 1 || p.Domains[0] != "google.com" {
		t.Errorf("unexpected domains: %v", p.Domains)
	}
	if p.AnchorContent == "" {
		t.Error("anchor content should not be empty")
	}
	ips, ok := p.ResolvedIPs["google.com"]
	if !ok {
		t.Fatal("no resolved IPs for google.com")
	}
	if len(ips) == 0 {
		t.Skip("no IPs resolved — possible offline environment")
	}
}

func TestResolveDomainIPs_returnsIPs(t *testing.T) {
	ips := ResolveDomainIPs("google.com", "8.8.8.8:53")
	if len(ips) == 0 {
		t.Skip("no IPs resolved — possible offline/CI environment")
	}
	for _, ip := range ips {
		if ip == "" {
			t.Error("empty IP in result")
		}
	}
}

func TestResolveDomainIPs_deduplication(t *testing.T) {
	ips := ResolveDomainIPs("youtube.com", "8.8.8.8:53")
	seen := make(map[string]bool)
	for _, ip := range ips {
		if seen[ip] {
			t.Errorf("duplicate IP %s in result", ip)
		}
		seen[ip] = true
	}
}

func TestPFConfInjectAndStrip(t *testing.T) {
	original := "# pf.conf\nset skip on lo\n"
	tmp := t.TempDir() + "/pf.conf"

	if err := os.WriteFile(tmp, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Inject — replicate injectPFConf logic against tmp path.
	content, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), markerBeg) {
		t.Fatal("marker already present before inject")
	}

	injection := "\n" + markerBeg + "\n" + anchorLine + "\n" + loadLine + "\n" + markerEnd + "\n"
	injected := string(content) + injection
	if err := os.WriteFile(tmp, []byte(injected), 0644); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(tmp)
	if !strings.Contains(string(got), markerBeg) {
		t.Error("marker not found after inject")
	}
	if !strings.Contains(string(got), anchorLine) {
		t.Error("anchor line missing after inject")
	}

	// Strip — replicate stripPFConf logic.
	var outLines []string
	skip := false
	for _, line := range strings.Split(injected, "\n") {
		if line == markerBeg {
			skip = true
			continue
		}
		if line == markerEnd {
			skip = false
			continue
		}
		if !skip {
			outLines = append(outLines, line)
		}
	}
	stripped := strings.TrimRight(strings.Join(outLines, "\n"), "\n") + "\n"

	if strings.Contains(stripped, markerBeg) {
		t.Error("marker still present after strip")
	}
	if !strings.Contains(stripped, "set skip on lo") {
		t.Error("original content lost after strip")
	}
}
