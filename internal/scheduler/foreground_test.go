package scheduler

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/proxy"
)

// withForegroundProbeStub swaps runForegroundProbe with a stub that returns the
// given output (and optional error). Restores the original on test cleanup so
// parallel tests don't leak state into each other.
func withForegroundProbeStub(t *testing.T, out string, runErr error) {
	t.Helper()
	orig := runForegroundProbe
	runForegroundProbe = func() (string, error) { return out, runErr }
	t.Cleanup(func() { runForegroundProbe = orig })
}

// trackedCfg builds a minimal config covering the tracked-domain branches.
func trackedCfg() config.Config {
	return config.Config{
		Groups: map[string][]string{
			"social": {"youtube.com", "reddit.com"},
			"_doh":   {"dns.google", "cloudflare-dns.com"},
		},
	}
}

func TestParseForegroundProbeOutput_HappyPath(t *testing.T) {
	res, err := parseForegroundProbeOutput("Google Chrome\thttps://www.youtube.com/watch?v=abc\t3\n")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.App != "Google Chrome" {
		t.Errorf("App = %q, want Google Chrome", res.App)
	}
	if res.URL != "https://www.youtube.com/watch?v=abc" {
		t.Errorf("URL = %q", res.URL)
	}
	if res.IdleSeconds != 3 {
		t.Errorf("IdleSeconds = %d, want 3", res.IdleSeconds)
	}
}

func TestParseForegroundProbeOutput_EmptyURL(t *testing.T) {
	// Frontmost is a non-browser app — URL field is empty by design.
	res, err := parseForegroundProbeOutput("Slack\t\t12")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.App != "Slack" || res.URL != "" || res.IdleSeconds != 12 {
		t.Errorf("got %+v", res)
	}
}

func TestParseForegroundProbeOutput_Malformed(t *testing.T) {
	cases := []string{
		"",
		"only one field",
		"two\tfields",
		"app\turl\tnotanint",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := parseForegroundProbeOutput(in); err == nil {
				t.Errorf("expected error for %q, got nil", in)
			}
		})
	}
}

func TestTrackedDomainSet_ExcludesDoH(t *testing.T) {
	cfg := trackedCfg()
	got := trackedDomainSet(cfg)
	if !got["youtube.com"] || !got["reddit.com"] {
		t.Errorf("missing tracked domain: %v", got)
	}
	if got["dns.google"] || got["cloudflare-dns.com"] {
		t.Errorf("trackedDomainSet leaked _doh entry: %v", got)
	}
}

func TestMatchTrackedDomain(t *testing.T) {
	tracked := map[string]bool{"youtube.com": true, "reddit.com": true}
	cases := []struct {
		host, want string
	}{
		{"youtube.com", "youtube.com"},
		{"music.youtube.com", "youtube.com"},
		{"old.reddit.com", "reddit.com"},
		{"notyoutube.com", ""},
		{"", ""},
		{"example.com", ""},
		// "youtubex.com" must not match "youtube.com" — partial-string false positive guard.
		{"youtubex.com", ""},
	}
	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			got := matchTrackedDomain(c.host, tracked)
			if got != c.want {
				t.Errorf("matchTrackedDomain(%q) = %q, want %q", c.host, got, c.want)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	cases := []struct {
		raw, want string
	}{
		{"https://www.youtube.com/watch?v=abc", "youtube.com"},
		{"https://music.YOUTUBE.com/", "music.youtube.com"},
		{"http://reddit.com", "reddit.com"},
		{"about:blank", ""},
		{"chrome://newtab/", ""},
		{"", ""},
		{"::not a url::", ""},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			if got := extractHost(c.raw); got != c.want {
				t.Errorf("extractHost(%q) = %q, want %q", c.raw, got, c.want)
			}
		})
	}
}

func TestGenerateForegroundProbeScript_IncludesAllBrowsers(t *testing.T) {
	g := &MacOSForegroundProbeGenerator{}
	script := g.GenerateForegroundProbeScript()
	for _, browser := range []string{"Google Chrome", "Safari", "Arc", "Brave Browser"} {
		if !strings.Contains(script, browser) {
			t.Errorf("probe script missing browser %q", browser)
		}
	}
	// Different idiom for Safari than the others — important.
	if !strings.Contains(script, "current tab of front window") {
		t.Error("probe script missing Safari 'current tab of front window' idiom")
	}
	if !strings.Contains(script, "active tab of front window") {
		t.Error("probe script missing Chromium 'active tab of front window' idiom")
	}
	// HIDIdleTime is the only standard idle source available without entitlements.
	if !strings.Contains(script, "HIDIdleTime") {
		t.Error("probe script missing HIDIdleTime idle-source idiom")
	}
	// Must emit tab-separated output for parseForegroundProbeOutput.
	if !strings.Contains(script, "frontApp & tab & activeURL & tab") {
		t.Error("probe script missing tab-separated stdout return")
	}
}

func TestRecordForegroundTick_HappyPath(t *testing.T) {
	withForegroundProbeStub(t, "Google Chrome\thttps://music.youtube.com/playlist\t0", nil)

	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	gl := BuildGroupLookup(cfg)

	event, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, gl)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !ok {
		t.Fatal("expected event to be emitted")
	}
	if event.Domain != "youtube.com" {
		t.Errorf("Domain = %q, want youtube.com (subdomain attribution)", event.Domain)
	}
	if event.Group != "social" {
		t.Errorf("Group = %q, want social", event.Group)
	}
	if event.Kind != proxy.KindForeground {
		t.Errorf("Kind = %q, want foreground", event.Kind)
	}
	if !event.TS.Equal(now) {
		t.Errorf("TS = %v, want %v", event.TS, now)
	}
}

func TestRecordForegroundTick_IdleAboveCutoff(t *testing.T) {
	withForegroundProbeStub(t, "Google Chrome\thttps://youtube.com\t120", nil)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	_, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Error("idle >= cutoff should suppress the event")
	}
}

func TestRecordForegroundTick_NonBrowserApp(t *testing.T) {
	withForegroundProbeStub(t, "Slack\t\t0", nil)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	_, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Error("non-browser frontmost app should suppress the event")
	}
}

func TestRecordForegroundTick_DomainNotInGroups(t *testing.T) {
	withForegroundProbeStub(t, "Safari\thttps://example.com/article\t0", nil)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	_, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Error("untracked domain should suppress the event (privacy floor)")
	}
}

func TestRecordForegroundTick_DohDomainExcluded(t *testing.T) {
	// Even if a user pulls up a DoH provider's marketing page, _doh entries
	// must not be tracked as foreground time.
	withForegroundProbeStub(t, "Google Chrome\thttps://dns.google/\t0", nil)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	_, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Error("_doh domain visited in browser must not produce a foreground event")
	}
}

func TestRecordForegroundTick_EmptyProbeOutputIsNoOp(t *testing.T) {
	// Non-darwin (or no GUI session) → empty stdout. Must NOT be treated as an
	// error and must NOT emit an event.
	withForegroundProbeStub(t, "", nil)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	_, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err != nil {
		t.Errorf("expected no error on empty probe output, got %v", err)
	}
	if ok {
		t.Error("empty probe output must not produce an event")
	}
}

func TestRecordForegroundTick_ProbeError(t *testing.T) {
	stubErr := errors.New("osascript exited 1")
	withForegroundProbeStub(t, "", stubErr)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	_, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err == nil {
		t.Error("expected probe error to propagate")
	}
	if ok {
		t.Error("probe error must not produce an event")
	}
}

func TestRecordForegroundTick_StripsWWW(t *testing.T) {
	// www.youtube.com must collapse to youtube.com (matches the configured group entry).
	withForegroundProbeStub(t, "Google Chrome\thttps://www.youtube.com/\t0", nil)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	event, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err != nil || !ok {
		t.Fatalf("expected event, got ok=%v err=%v", ok, err)
	}
	if event.Domain != "youtube.com" {
		t.Errorf("Domain = %q, want youtube.com after www strip", event.Domain)
	}
}

func TestRecordForegroundTick_BlankURLForBrowser(t *testing.T) {
	// User has Chrome frontmost on a chrome://newtab/ tab — URL field is the
	// blank/internal page, no host. No event should fire.
	withForegroundProbeStub(t, "Google Chrome\tchrome://newtab/\t0", nil)
	now := time.Date(2026, 5, 6, 14, 30, 0, 0, time.UTC)
	cfg := trackedCfg()
	_, ok, err := recordForegroundTick(now, cfg, runForegroundProbe, BuildGroupLookup(cfg))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ok {
		t.Error("internal browser URL with no host must not produce an event")
	}
}
