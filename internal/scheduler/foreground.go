package scheduler

import (
	"fmt"
	neturl "net/url"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/enforcer"
	"github.com/vsangava/sentinel/internal/proxy"
)

// foregroundIdleCutoffSeconds is the threshold above which the user is treated
// as idle (away from keyboard). Set to 60 because the scheduler ticks every
// minute, so any idle period >= 60s means the user has been away across at
// least one full sample interval — attributing that minute to the last
// foreground tab would inflate the metric. Anything under is "in front of the
// machine doing something."
const foregroundIdleCutoffSeconds = 60

// supportedForegroundBrowsers mirrors the set the per-tick close path supports.
// AppleScript URL access uses a slightly different idiom for Safari ("current
// tab") vs the others ("active tab"), handled inside the probe script.
var supportedForegroundBrowsers = []string{
	"Google Chrome",
	"Safari",
	"Arc",
	"Brave Browser",
}

// ForegroundProbeResult is the parsed output of one probe invocation.
//
//   - App is the macOS frontmost application name (e.g. "Google Chrome").
//   - URL is the active tab URL when App is a supported browser; "" otherwise.
//   - IdleSeconds is how long the user has been idle (no keyboard/mouse input).
type ForegroundProbeResult struct {
	App         string
	URL         string
	IdleSeconds int
}

// ForegroundProbeGenerator interface so tests can swap in a deterministic
// script body without forking osascript.
type ForegroundProbeGenerator interface {
	GenerateForegroundProbeScript() string
}

// MacOSForegroundProbeGenerator emits AppleScript that prints
// `frontmost_app<TAB>active_url<TAB>idle_seconds` on stdout.
type MacOSForegroundProbeGenerator struct{}

func (g *MacOSForegroundProbeGenerator) GenerateForegroundProbeScript() string {
	// Per-browser URL fetches are wrapped individually so a single
	// "Application isn't running" (-600) fails the inner block, not the whole
	// probe — same defensive shape used by GenerateCloseTabsScript.
	//
	// Idle time comes from IOKit (HIDIdleTime in nanoseconds). It needs no
	// special permission and is the standard idiom; AppleScript has no
	// first-class idle-time accessor.
	return `
		set frontApp to ""
		set activeURL to ""
		set idleSeconds to 0

		try
			tell application "System Events"
				set frontApp to name of first application process whose frontmost is true
			end tell
		end try

		try
			set idleNanos to do shell script "/usr/sbin/ioreg -c IOHIDSystem | /usr/bin/awk '/HIDIdleTime/ {print int($NF/1000000000); exit}'"
			set idleSeconds to idleNanos as integer
		end try

		if frontApp is "Google Chrome" then
			try
				tell application "Google Chrome" to set activeURL to URL of active tab of front window
			end try
		else if frontApp is "Safari" then
			try
				tell application "Safari" to set activeURL to URL of current tab of front window
			end try
		else if frontApp is "Arc" then
			try
				tell application "Arc" to set activeURL to URL of active tab of front window
			end try
		else if frontApp is "Brave Browser" then
			try
				tell application "Brave Browser" to set activeURL to URL of active tab of front window
			end try
		end if

		return frontApp & tab & activeURL & tab & (idleSeconds as text)
	`
}

var foregroundProbeGenerator ForegroundProbeGenerator = &MacOSForegroundProbeGenerator{}

// SetForegroundProbeGenerator replaces the probe-script generator. Tests use
// this to substitute a stub.
func SetForegroundProbeGenerator(g ForegroundProbeGenerator) {
	foregroundProbeGenerator = g
}

// runForegroundProbe is the production probe runner — executes the generated
// AppleScript and returns its stdout. Replaceable in tests.
var runForegroundProbe = func() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", nil
	}
	return runOsaScriptCapture(foregroundProbeGenerator.GenerateForegroundProbeScript())
}

// parseForegroundProbeOutput parses the tab-separated probe output.
// Trailing whitespace from AppleScript is tolerated; an empty/missing URL
// field is normal (frontmost app isn't a supported browser).
func parseForegroundProbeOutput(out string) (ForegroundProbeResult, error) {
	line := strings.TrimSpace(out)
	if line == "" {
		return ForegroundProbeResult{}, fmt.Errorf("probe produced no output")
	}
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 {
		return ForegroundProbeResult{}, fmt.Errorf("probe output malformed (expected 3 tab-separated fields, got %d): %q", len(parts), line)
	}
	idle, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		return ForegroundProbeResult{}, fmt.Errorf("probe output: idle seconds not an integer: %w", err)
	}
	return ForegroundProbeResult{
		App:         strings.TrimSpace(parts[0]),
		URL:         strings.TrimSpace(parts[1]),
		IdleSeconds: idle,
	}, nil
}

// trackedDomainSet returns every domain across every group in cfg, except the
// _doh group. Foreground tracking is opt-in to "domains the user explicitly
// configured for blocking" — _doh is infrastructure, not user-tracked sites,
// and its endpoints aren't visited via browsers anyway.
func trackedDomainSet(cfg config.Config) map[string]bool {
	out := make(map[string]bool)
	for group, domains := range cfg.Groups {
		if group == enforcer.DohGroupName {
			continue
		}
		for _, d := range domains {
			out[d] = true
		}
	}
	return out
}

// matchTrackedDomain returns the configured domain that host belongs to, or ""
// if none. host should already have a leading "www." stripped. Subdomain-aware:
// "music.youtube.com" matches a configured "youtube.com".
func matchTrackedDomain(host string, tracked map[string]bool) string {
	if host == "" {
		return ""
	}
	if tracked[host] {
		return host
	}
	for d := range tracked {
		if strings.HasSuffix(host, "."+d) {
			return d
		}
	}
	return ""
}

// extractHost pulls the lowercase, www-stripped hostname from a browser-tab
// URL. Returns "" on any parse failure, missing host, or non-http(s) scheme.
// The scheme gate is deliberate: chrome://newtab/, brave://settings/, file://,
// about:blank etc. are internal browser URLs, not web destinations — they must
// not feed the tracked-domain matcher (otherwise "newtab" could be parsed out
// and a configured "newtab.example" would falsely attribute time).
func extractHost(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := neturl.Parse(rawURL)
	if err != nil {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	return strings.TrimPrefix(host, "www.")
}

// isSupportedBrowser reports whether name is one of the browsers the probe
// can read tab URLs from.
func isSupportedBrowser(name string) bool {
	for _, b := range supportedForegroundBrowsers {
		if b == name {
			return true
		}
	}
	return false
}

// recordForegroundTick is the per-tick gate: probe → parse → filter → emit one
// usage event when every condition passes. Returns ok=false (no event to
// append) on any miss, with no error logged — these are normal cases (no
// browser frontmost, idle, off-list domain).
//
// groupLookup is the shared domain→group map already built each tick by the
// scheduler; reusing it avoids walking cfg.Groups again here.
func recordForegroundTick(t time.Time, cfg config.Config, runProbe func() (string, error), groupLookup map[string]string) (proxy.UsageEvent, bool, error) {
	out, err := runProbe()
	if err != nil {
		return proxy.UsageEvent{}, false, err
	}
	if strings.TrimSpace(out) == "" {
		// Non-darwin / no GUI session: probe is a no-op, no event to record.
		return proxy.UsageEvent{}, false, nil
	}

	res, err := parseForegroundProbeOutput(out)
	if err != nil {
		return proxy.UsageEvent{}, false, err
	}
	if res.IdleSeconds >= foregroundIdleCutoffSeconds {
		return proxy.UsageEvent{}, false, nil
	}
	if !isSupportedBrowser(res.App) {
		return proxy.UsageEvent{}, false, nil
	}
	host := extractHost(res.URL)
	if host == "" {
		return proxy.UsageEvent{}, false, nil
	}

	tracked := trackedDomainSet(cfg)
	matched := matchTrackedDomain(host, tracked)
	if matched == "" {
		return proxy.UsageEvent{}, false, nil
	}
	group := groupLookup[matched]
	if group == "" {
		// Defensive: matched came from cfg.Groups, so a missing lookup entry
		// should be impossible. Skip rather than emit a group-less event.
		return proxy.UsageEvent{}, false, nil
	}
	return proxy.UsageEvent{
		TS:     t,
		Domain: matched,
		Group:  group,
		Kind:   proxy.KindForeground,
	}, true, nil
}
