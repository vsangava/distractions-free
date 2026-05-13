package scheduler

import (
	"fmt"
	neturl "net/url"
	"regexp"
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

// supportedForegroundBrowsers is the set of frontmost-app names recordForegroundTick
// will accept a URL/host from. On macOS the AppleScript probe reads tab URLs from
// Chrome/Safari/Arc/Brave (Safari uses "current tab", the others "active tab").
// "Microsoft Edge" is here for the Windows probe (Edge is the default browser
// there); the macOS AppleScript probe doesn't query Edge, so the entry is a
// harmless no-op on macOS.
var supportedForegroundBrowsers = []string{
	"Google Chrome",
	"Microsoft Edge",
	"Safari",
	"Arc",
	"Brave Browser",
}

// ForegroundProbeResult is the parsed output of one probe invocation.
//
//   - App is the frontmost application name (e.g. "Google Chrome"). On
//     Windows this is the supported-browser name we mapped the foreground
//     process to, or "" if the foreground app isn't a tracked browser.
//   - URL is the active tab URL when App is a supported browser; "" otherwise.
//     On Windows (window-title heuristic) it's a synthesised "https://<host>"
//     for whatever host was extractable from the title, or "" if none —
//     extractHost handles it either way.
//   - IdleSeconds is how long the user has been idle (no keyboard/mouse input).
//
// The zero value (App == "") means "nothing to record this tick" — no GUI
// session, non-browser frontmost, or an unsupported platform.
type ForegroundProbeResult struct {
	App         string
	URL         string
	IdleSeconds int
}

// ForegroundProbe is the per-tick foreground sampler. Each OS provides its own
// implementation; tests substitute a stub. Probe returns the zero
// ForegroundProbeResult (not an error) when there's nothing to record.
type ForegroundProbe interface {
	Probe() (ForegroundProbeResult, error)
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
	// Per-browser URL fetches are dispatched as nested `osascript -e` calls via
	// `do shell script` rather than written as inline `tell application "X" to
	// ...` blocks. Reason: AppleScript compiles the whole script up front, and
	// `URL of active tab of front window` requires the target app's scripting
	// dictionary at compile time. If the app isn't installed (e.g. Brave/Arc on
	// most machines), compilation fails with -2741 "Expected end of line but
	// found property" — and a `try` block can't catch a compile-time error, so
	// the whole probe dies before the `if frontApp is "X"` runtime guard runs.
	// Nesting via `do shell script "osascript -e ..."` isolates each app's
	// terminology resolution into its own osascript process: only the branch
	// that actually fires gets compiled, and a missing app errors only if it's
	// somehow the frontmost (which can't happen if it isn't installed).
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
				set activeURL to do shell script "osascript -e 'tell application \"Google Chrome\" to get URL of active tab of front window'"
			end try
		else if frontApp is "Safari" then
			try
				set activeURL to do shell script "osascript -e 'tell application \"Safari\" to get URL of current tab of front window'"
			end try
		else if frontApp is "Arc" then
			try
				set activeURL to do shell script "osascript -e 'tell application \"Arc\" to get URL of active tab of front window'"
			end try
		else if frontApp is "Brave Browser" then
			try
				set activeURL to do shell script "osascript -e 'tell application \"Brave Browser\" to get URL of active tab of front window'"
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

// macOSForegroundProbe runs the generated AppleScript via osascript and parses
// its stdout. On non-darwin it's a no-op (returns the zero result) so this
// stays the package default until a platform-specific probe replaces it.
type macOSForegroundProbe struct{}

func (macOSForegroundProbe) Probe() (ForegroundProbeResult, error) {
	if runtime.GOOS != "darwin" {
		return ForegroundProbeResult{}, nil
	}
	out, err := runOsaScriptCapture(foregroundProbeGenerator.GenerateForegroundProbeScript())
	if err != nil {
		return ForegroundProbeResult{}, err
	}
	if strings.TrimSpace(out) == "" {
		// No GUI session (e.g. running headless): nothing to record.
		return ForegroundProbeResult{}, nil
	}
	return parseForegroundProbeOutput(out)
}

// foregroundProbe is the active per-tick sampler. Replaceable in tests; a
// later change wires in a Windows implementation behind build tags.
var foregroundProbe ForegroundProbe = macOSForegroundProbe{}

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

// browserTitleSuffixes are the trailing " - <Browser>" markers Chromium appends
// to every window title. Stripped before scanning for a host. Small on purpose:
// issue #94 scopes the first Windows iteration to Chrome + Edge (English UI);
// other browsers and locale variants come with the UI Automation probe. The
// zero-width-space variant has been observed in some Edge builds.
var browserTitleSuffixes = []string{
	" - Google Chrome",
	" - Microsoft Edge",
	" - Microsoft​Edge",
}

// hostTokenRe matches a hostname-shaped token: one or more dot-separated DNS
// labels followed by an alphabetic TLD (2-63 letters). Deliberately strict so
// arbitrary words in a page title aren't mistaken for hosts — and IP literals
// (no alpha TLD) are excluded.
var hostTokenRe = regexp.MustCompile(`(?i)\b((?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63})\b`)

// hostFromBrowserWindowTitle does a best-effort extraction of a hostname from a
// Chromium browser window title. A window title is the *page* title, which only
// sometimes contains the domain — so this returns "" for most titles, and a
// synthesised "https://<host>" (for extractHost) when a host-shaped token is
// present.
//
// Known limitation: if a page title *mentions* a domain other than the one
// being viewed (e.g. a forum thread titled "...youtube.com..."), this will
// attribute the minute to the mentioned domain. The UI Automation probe (issue
// #94 follow-up) reads the real address bar and doesn't have this problem; the
// window-title path is the "something rather than nothing" first cut.
func hostFromBrowserWindowTitle(title string) string {
	t := strings.TrimSpace(title)
	for _, sfx := range browserTitleSuffixes {
		if strings.HasSuffix(t, sfx) {
			t = strings.TrimSpace(strings.TrimSuffix(t, sfx))
			break
		}
	}
	if t == "" {
		return ""
	}
	host := hostTokenRe.FindString(strings.ToLower(t))
	if host == "" {
		return ""
	}
	return "https://" + host
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

// recordForegroundTick is the per-tick gate: probe → filter → emit one usage
// event when every condition passes. Returns ok=false (no event to append) on
// any miss, with no error logged — these are normal cases (no browser
// frontmost, idle, off-list domain).
//
// groupLookup is the shared domain→group map already built each tick by the
// scheduler; reusing it avoids walking cfg.Groups again here.
func recordForegroundTick(t time.Time, cfg config.Config, probe ForegroundProbe, groupLookup map[string]string) (proxy.UsageEvent, bool, error) {
	res, err := probe.Probe()
	if err != nil {
		return proxy.UsageEvent{}, false, err
	}
	if res.IdleSeconds >= foregroundIdleCutoffSeconds {
		return proxy.UsageEvent{}, false, nil
	}
	// Zero-value result (App == "") and non-browser frontmost both land here:
	// nothing to record.
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
