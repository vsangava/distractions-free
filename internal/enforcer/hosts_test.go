package enforcer

import (
	"os"
	"strings"
	"testing"

	"github.com/vsangava/distractions-free/internal/config"
)

func testHostsConfig() config.Config {
	return config.Config{
		Settings: config.Settings{PrimaryDNS: "8.8.8.8:53", BackupDNS: "1.1.1.1:53"},
	}
}

func newTestEnforcer(t *testing.T) *HostsEnforcer {
	t.Helper()
	tmp := t.TempDir() + "/hosts"
	if err := os.WriteFile(tmp, []byte("127.0.0.1 localhost\n::1 localhost\n"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return &HostsEnforcer{hostsPath: tmp, cfg: testHostsConfig()}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFile: %v", err)
	}
	return string(b)
}

func TestHostsEnforcer_ActivateAddsEntries(t *testing.T) {
	e := newTestEnforcer(t)
	if err := e.Activate([]string{"youtube.com"}); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	content := readFile(t, e.hostsPath)
	for _, want := range []string{
		"0.0.0.0 youtube.com",
		"0.0.0.0 www.youtube.com",
		"0.0.0.0 m.youtube.com",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in hosts file\n%s", want, content)
		}
	}
	if !strings.Contains(content, "127.0.0.1 localhost") {
		t.Error("original entries must be preserved")
	}
}

func TestHostsEnforcer_ActivateIsIdempotent(t *testing.T) {
	e := newTestEnforcer(t)
	e.Activate([]string{"youtube.com"})
	e.Activate([]string{"youtube.com"}) // second call must be a no-op

	content := readFile(t, e.hostsPath)
	if got := strings.Count(content, "0.0.0.0 youtube.com"); got != 1 {
		t.Errorf("expected exactly 1 entry for youtube.com, got %d\n%s", got, content)
	}
}

func TestHostsEnforcer_DeactivateRemovesEntries(t *testing.T) {
	e := newTestEnforcer(t)
	e.Activate([]string{"youtube.com"})

	if err := e.Deactivate([]string{"youtube.com"}); err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	content := readFile(t, e.hostsPath)
	if strings.Contains(content, "0.0.0.0 youtube.com") {
		t.Errorf("youtube.com should be removed after Deactivate\n%s", content)
	}
	if !strings.Contains(content, "127.0.0.1 localhost") {
		t.Error("original entries must be preserved")
	}
}

func TestHostsEnforcer_DeactivatePartialPreservesOthers(t *testing.T) {
	e := newTestEnforcer(t)
	e.Activate([]string{"youtube.com"})
	e.Activate([]string{"facebook.com"})
	e.Deactivate([]string{"youtube.com"})

	content := readFile(t, e.hostsPath)
	if strings.Contains(content, "0.0.0.0 youtube.com") {
		t.Error("youtube.com should be removed")
	}
	if !strings.Contains(content, "0.0.0.0 facebook.com") {
		t.Error("facebook.com should still be blocked")
	}
}

func TestHostsEnforcer_DeactivateAllClearsBlock(t *testing.T) {
	e := newTestEnforcer(t)
	e.Activate([]string{"youtube.com", "facebook.com"})

	if err := e.DeactivateAll(); err != nil {
		t.Fatalf("DeactivateAll: %v", err)
	}

	content := readFile(t, e.hostsPath)
	if strings.Contains(content, "0.0.0.0") {
		t.Errorf("all block entries should be removed\n%s", content)
	}
	if strings.Contains(content, blockBegin) || strings.Contains(content, blockEnd) {
		t.Errorf("block markers should be removed\n%s", content)
	}
	if !strings.Contains(content, "127.0.0.1 localhost") {
		t.Error("original entries must survive activate+deactivate-all cycle")
	}
}

func TestHostsEnforcer_ActivateDeactivateRoundTrip(t *testing.T) {
	original := "127.0.0.1 localhost\n::1 localhost\n"
	tmp := t.TempDir() + "/hosts"
	os.WriteFile(tmp, []byte(original), 0644)
	e := &HostsEnforcer{hostsPath: tmp, cfg: testHostsConfig()}

	e.Activate([]string{"reddit.com"})
	e.DeactivateAll()

	content := readFile(t, tmp)
	// Pre-existing entries must survive a full round-trip.
	if !strings.Contains(content, "127.0.0.1 localhost") || !strings.Contains(content, "::1 localhost") {
		t.Errorf("pre-existing entries lost after round-trip:\n%s", content)
	}
	if strings.Contains(content, blockBegin) {
		t.Errorf("block markers should not remain after DeactivateAll:\n%s", content)
	}
}

func TestHostsEnforcer_DeactivateAllOnEmptyFile(t *testing.T) {
	tmp := t.TempDir() + "/hosts"
	os.WriteFile(tmp, []byte("127.0.0.1 localhost\n"), 0644)
	e := &HostsEnforcer{hostsPath: tmp, cfg: testHostsConfig()}

	// DeactivateAll on a file with no managed block should be a no-op.
	if err := e.DeactivateAll(); err != nil {
		t.Fatalf("DeactivateAll on clean file: %v", err)
	}
}
