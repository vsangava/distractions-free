package scheduler

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/enforcer"
	"github.com/vsangava/sentinel/internal/proxy"
)

var activeBlocks = make(map[string]bool)
var activeBlocksMu sync.RWMutex
var lastEvalTime time.Time

var activeEnforcer enforcer.Enforcer

var lastPruneDay time.Time

// SetEnforcer wires the enforcement backend used by the scheduler.
// Must be called before Start().
func SetEnforcer(e enforcer.Enforcer) {
	activeEnforcer = e
}

var lastWarningTime = make(map[string]time.Time)
var lastWarningMu sync.Mutex

// ScriptExecutor interface for testing AppleScript execution
type ScriptExecutor interface {
	ExecuteScript(script string) error
	LogScript(script string)
}

// MacOSScriptExecutor executes scripts on macOS
type MacOSScriptExecutor struct{}

func (e *MacOSScriptExecutor) ExecuteScript(script string) error {
	if err := runAsMacUser(script); err != nil {
		// "Application isn't running" (-600) is expected when Safari/Chrome isn't open — not worth logging.
		if !strings.Contains(err.Error(), "isn't running") && !strings.Contains(err.Error(), "(-600)") {
			log.Printf("AppleScript execution failed: %v", err)
		}
		return err
	}
	return nil
}

func (e *MacOSScriptExecutor) LogScript(script string) {}

// TestScriptExecutor logs scripts instead of executing them
type TestScriptExecutor struct {
	executedScripts []string
}

func (e *TestScriptExecutor) ExecuteScript(script string) error {
	e.executedScripts = append(e.executedScripts, script)
	e.LogScript(script)
	return nil
}

func (e *TestScriptExecutor) LogScript(script string) {
	log.Printf("TEST MODE - AppleScript would execute:\n%s", script)
}

// Global executor - can be replaced for testing
var scriptExecutor ScriptExecutor = &MacOSScriptExecutor{}

// AppleScriptGenerator interface for testing script generation
type AppleScriptGenerator interface {
	GenerateWarningScript(domains []string) string
	GenerateCloseTabsScript(domains []string) string
}

// MacOSAppleScriptGenerator generates macOS AppleScripts
type MacOSAppleScriptGenerator struct{}

func (g *MacOSAppleScriptGenerator) GenerateWarningScript(domains []string) string {
	msg := fmt.Sprintf("Tabs for %s will close in 3 minutes.", strings.Join(domains, ", "))
	return fmt.Sprintf(`display notification "%s" with title "Sentinel" subtitle "Upcoming Block" sound name "Basso"`, msg)
}

func (g *MacOSAppleScriptGenerator) GenerateCloseTabsScript(domains []string) string {
	var quotedDomains []string
	var displayDomains []string
	for _, d := range domains {
		stripped := strings.TrimPrefix(d, "www.")
		quotedDomains = append(quotedDomains, fmt.Sprintf(`"%s"`, stripped))
		displayDomains = append(displayDomains, stripped)
	}
	domainListStr := "{" + strings.Join(quotedDomains, ", ") + "}"

	// Build the notification body. Include the domain list when short, fall back to
	// a count of distinct sites for longer lists so the notification stays readable.
	// The actual closed-tab count is computed in AppleScript (`closedCount`) and
	// interpolated at runtime.
	var notifyMsg string
	if len(displayDomains) <= 3 {
		notifyMsg = fmt.Sprintf(`"Closed " & closedCount & " tab(s) on %s"`, strings.Join(displayDomains, ", "))
	} else {
		notifyMsg = fmt.Sprintf(`"Closed " & closedCount & " distracting tab(s) across %d sites"`, len(displayDomains))
	}

	return fmt.Sprintf(`
		set domainsToBlock to %s
		set closedCount to 0

		if application "Google Chrome" is running then
			tell application "Google Chrome"
				set tabsToClose to {}
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToBlock
							if tabURL contains d then
								set end of tabsToClose to t
								exit repeat
							end if
						end repeat
					end repeat
				end repeat
				set closedCount to closedCount + (count of tabsToClose)
				repeat with t in tabsToClose
					close t
				end repeat
			end tell
		end if

		if application "Safari" is running then
			tell application "Safari"
				set tabsToClose to {}
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToBlock
							if tabURL contains d then
								set end of tabsToClose to t
								exit repeat
							end if
						end repeat
					end repeat
				end repeat
				set closedCount to closedCount + (count of tabsToClose)
				repeat with t in tabsToClose
					close t
				end repeat
			end tell
		end if

		if application "Arc" is running then
			tell application "Arc"
				set tabsToClose to {}
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToBlock
							if tabURL contains d then
								set end of tabsToClose to t
								exit repeat
							end if
						end repeat
					end repeat
				end repeat
				set closedCount to closedCount + (count of tabsToClose)
				repeat with t in tabsToClose
					close t
				end repeat
			end tell
		end if

		if application "Brave Browser" is running then
			tell application "Brave Browser"
				set tabsToClose to {}
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToBlock
							if tabURL contains d then
								set end of tabsToClose to t
								exit repeat
							end if
						end repeat
					end repeat
				end repeat
				set closedCount to closedCount + (count of tabsToClose)
				repeat with t in tabsToClose
					close t
				end repeat
			end tell
		end if

		if closedCount > 0 then
			display notification %s with title "Sentinel" subtitle "Tab Closed"
		end if
	`, domainListStr, notifyMsg)
}

// Global generator - can be replaced for testing
var scriptGenerator AppleScriptGenerator = &MacOSAppleScriptGenerator{}

// GetScriptGenerator returns the current script generator (for testing)
func GetScriptGenerator() AppleScriptGenerator {
	return scriptGenerator
}

// SetScriptGenerator sets the script generator (for testing)
func SetScriptGenerator(generator AppleScriptGenerator) {
	scriptGenerator = generator
}

// GetScriptExecutor returns the current script executor (for testing)
func GetScriptExecutor() ScriptExecutor {
	return scriptExecutor
}

// SetScriptExecutor sets the script executor (for testing)
func SetScriptExecutor(executor ScriptExecutor) {
	scriptExecutor = executor
}

// GetStatus returns the currently blocked domains and the last evaluation time.
func GetStatus() (blocked map[string]bool, lastEval time.Time) {
	activeBlocksMu.RLock()
	defer activeBlocksMu.RUnlock()
	cp := make(map[string]bool, len(activeBlocks))
	for k, v := range activeBlocks {
		cp[k] = v
	}
	return cp, lastEvalTime
}

// isOvernightSlot reports whether a slot crosses midnight (End is before Start).
func isOvernightSlot(slotStart, slotEnd time.Time) bool {
	return slotEnd.Before(slotStart)
}

// EvaluateRulesAtTime evaluates blocking rules at a specific time and returns blocked domains.
// Returns an empty map immediately if the config has an active pause window.
// quotaUsage maps group name → minutes used today; pass nil for no quota enforcement.
// This is the testable function that doesn't depend on time.Now().
func EvaluateRulesAtTime(t time.Time, cfg config.Config, quotaUsage map[string]int) map[string]bool {
	if cfg.IsPaused(t) {
		return make(map[string]bool)
	}

	// During a Pomodoro work phase, activate ALL rules with IsActive=true,
	// bypassing their schedule windows entirely for stricter blocking.
	if cfg.IsLockedByPomodoro(t) {
		forced := make(map[string]bool)
		for _, rule := range cfg.Rules {
			if !rule.IsActive {
				continue
			}
			for _, domain := range cfg.ResolveGroup(rule.Group) {
				forced[domain] = true
			}
		}
		return forced
	}

	currentDay := t.Weekday().String()
	yesterdayDay := t.AddDate(0, 0, -1).Weekday().String()
	now := time.Date(0, 1, 1, t.Hour(), t.Minute(), 0, 0, time.UTC)

	newBlocked := make(map[string]bool)

	for _, rule := range cfg.Rules {
		if !rule.IsActive {
			continue
		}
		domains := cfg.ResolveGroup(rule.Group)
		if len(domains) == 0 {
			continue
		}

		// Today's slots
		if slots, exists := rule.Schedules[currentDay]; exists {
			for _, slot := range slots {
				slotStart, errS := time.Parse("15:04", slot.Start)
				slotEnd, errE := time.Parse("15:04", slot.End)
				if errS != nil || errE != nil {
					continue
				}
				if isOvernightSlot(slotStart, slotEnd) {
					// Evening portion of an overnight slot: blocks from Start until midnight
					if now.Equal(slotStart) || now.After(slotStart) {
						for _, d := range domains {
							newBlocked[d] = true
						}
						break
					}
				} else {
					if (now.Equal(slotStart) || now.After(slotStart)) && now.Before(slotEnd) {
						for _, d := range domains {
							newBlocked[d] = true
						}
						break
					}
				}
			}
		}

		// Yesterday's overnight slots — morning continuation past midnight
		if slots, exists := rule.Schedules[yesterdayDay]; exists {
			for _, slot := range slots {
				slotStart, errS := time.Parse("15:04", slot.Start)
				slotEnd, errE := time.Parse("15:04", slot.End)
				if errS != nil || errE != nil {
					continue
				}
				if isOvernightSlot(slotStart, slotEnd) && now.Before(slotEnd) {
					for _, d := range domains {
						newBlocked[d] = true
					}
					break
				}
			}
		}
	}

	// Quota enforcement: block any group whose daily allowance is exhausted.
	if len(quotaUsage) > 0 {
		for _, rule := range cfg.Rules {
			if !rule.IsActive || rule.DailyQuotaMinutes <= 0 {
				continue
			}
			used := quotaUsage[rule.Group]
			if used >= rule.DailyQuotaMinutes {
				for _, d := range cfg.ResolveGroup(rule.Group) {
					newBlocked[d] = true
				}
			}
		}
	}

	return newBlocked
}

// BuildGroupLookup builds a domain→group map covering all domains in all configured
// groups, regardless of whether they are currently blocked.
func BuildGroupLookup(cfg config.Config) map[string]string {
	lookup := make(map[string]string)
	for group, domains := range cfg.Groups {
		for _, d := range domains {
			lookup[d] = group
		}
	}
	return lookup
}

// CheckWarningDomainsAtTime checks if any domains should trigger warnings within 3 minutes of block start.
// Returns nil immediately if the config has an active pause window.
// This is the testable function that doesn't depend on time.Now().
func CheckWarningDomainsAtTime(t time.Time, cfg config.Config) []string {
	if cfg.IsPaused(t) {
		return nil
	}

	currentDay := t.Weekday().String()

	var warningDomains []string

	// Check for warnings within 3-minute window before block start
	seen := make(map[string]bool)
	for _, rule := range cfg.Rules {
		if !rule.IsActive {
			continue
		}
		domains := cfg.ResolveGroup(rule.Group)
		if len(domains) == 0 {
			continue
		}
		if slots, exists := rule.Schedules[currentDay]; exists {
			for _, slot := range slots {
				// Parse block start time
				parts := strings.Split(slot.Start, ":")
				if len(parts) != 2 {
					continue
				}
				blockHour, errH := strconv.Atoi(parts[0])
				blockMin, errM := strconv.Atoi(parts[1])
				if errH != nil || errM != nil {
					continue
				}

				// Create block start time for today
				blockTime := time.Date(t.Year(), t.Month(), t.Day(), blockHour, blockMin, 0, 0, t.Location())

				// Calculate warning window: 3 minutes before block start
				warningStart := blockTime.Add(-3 * time.Minute)

				// Warn if current time is within [warningStart, blockTime)
				if (t.After(warningStart) || t.Equal(warningStart)) && t.Before(blockTime) {
					for _, d := range domains {
						if !seen[d] {
							seen[d] = true
							warningDomains = append(warningDomains, d)
						}
					}
				}
			}
		}
	}

	return warningDomains
}

func Start() {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		evaluateRules() // Run immediately
		for range ticker.C {
			evaluateRules()
		}
	}()
}

func evaluateRules() {
	if err := config.LoadConfig(); err != nil {
		log.Printf("Config reload warning: %v", err)
	}
	cfg := config.GetConfig()
	now := time.Now()

	// Remove an expired pause so config.json stays clean.
	if cfg.Pause != nil && !now.Before(cfg.Pause.Until) {
		config.ClearPause()
		if err := config.SaveConfig(); err != nil {
			log.Printf("scheduler: clear expired pause: %v", err)
		}
		cfg = config.GetConfig()
	}

	// Handle Pomodoro phase transitions.
	if cfg.Pomodoro != nil && !now.Before(cfg.Pomodoro.PhaseEndsAt) {
		switch cfg.Pomodoro.Phase {
		case "work":
			breakMin := cfg.Pomodoro.BreakMinutes
			config.AdvancePomodoroPhase()
			if err := config.SaveConfig(); err != nil {
				log.Printf("scheduler: save pomodoro break phase: %v", err)
			}
			cfg = config.GetConfig()
			sendPomodoroNotification("Sentinel — Break time!",
				fmt.Sprintf("Take a %d-minute break.", breakMin))
		case "break":
			config.ClearPomodoro()
			if err := config.SaveConfig(); err != nil {
				log.Printf("scheduler: clear pomodoro session: %v", err)
			}
			cfg = config.GetConfig()
			sendPomodoroNotification("Sentinel — Ready?",
				"Break over. Start a new focus session when ready.")
		}
	}

	// Build domain→group lookup and push to proxy for usage tracking.
	gl := BuildGroupLookup(cfg)
	proxy.UpdateGroupLookup(gl)

	// Compute today's quota usage per group (best-effort; nil on error → no quota enforcement).
	var quotaUsage map[string]int
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if usageEvents, err := proxy.ReadUsageEventsSince(dayStart.Add(-time.Second)); err == nil {
		var quotaGroups []string
		for _, rule := range cfg.Rules {
			if rule.IsActive && rule.DailyQuotaMinutes > 0 {
				quotaGroups = append(quotaGroups, rule.Group)
			}
		}
		if len(quotaGroups) > 0 {
			quotaUsage = proxy.ComputeAllGroupUsageMinutes(usageEvents, quotaGroups, now)
		}
	} else {
		log.Printf("scheduler: read usage events: %v", err)
	}

	newBlocked := EvaluateRulesAtTime(now, cfg, quotaUsage)
	warningDomains := CheckWarningDomainsAtTime(now, cfg)

	// Compute diffs relative to the previous tick.
	var newlyBlocked, newlyUnblocked []string
	activeBlocksMu.RLock()
	for domain := range newBlocked {
		if !activeBlocks[domain] {
			newlyBlocked = append(newlyBlocked, domain)
		}
	}
	for domain := range activeBlocks {
		if !newBlocked[domain] {
			newlyUnblocked = append(newlyUnblocked, domain)
		}
	}
	activeBlocksMu.RUnlock()

	activeBlocksMu.Lock()
	activeBlocks = newBlocked
	lastEvalTime = now
	activeBlocksMu.Unlock()

	// Push changes through the enforcement backend.
	if activeEnforcer != nil {
		if len(newlyBlocked) > 0 {
			if err := activeEnforcer.Activate(newlyBlocked); err != nil {
				log.Printf("scheduler: activate failed: %v", err)
			}
		}
		if len(newlyUnblocked) > 0 {
			if err := activeEnforcer.Deactivate(newlyUnblocked); err != nil {
				log.Printf("scheduler: deactivate failed: %v", err)
			}
		}
		// Refresh pf IPs every tick even when the domain set is unchanged.
		// CDN-backed sites rotate IPs between Activate calls; Refresh re-resolves
		// them so the firewall rules stay current. No-op for dns/hosts mode.
		if len(newlyBlocked) == 0 && len(activeBlocks) > 0 {
			activeEnforcer.Refresh()
		}
	}

	// Per-tick browser tab closer. Runs every tick (not just on transitions) so
	// tabs that survive the initial block — opened mid-window via DoH bypass,
	// iCloud Private Relay, memorized IPs, or stale reloads — get closed within
	// ~60s. Filters out the _doh group: those endpoints aren't sites users visit
	// with browsers, and probing for them wastes osascript work.
	runPerTickCloseTabs(newBlocked, cfg, browserTabProbe)

	// Log block/unblock events grouped by config group.
	if len(newlyBlocked) > 0 || len(newlyUnblocked) > 0 {
		blockedSet := make(map[string]bool, len(newlyBlocked))
		for _, d := range newlyBlocked {
			blockedSet[d] = true
		}
		unblockedSet := make(map[string]bool, len(newlyUnblocked))
		for _, d := range newlyUnblocked {
			unblockedSet[d] = true
		}
		var events []BlockEvent
		for group, domains := range cfg.Groups {
			var bd, ubd []string
			for _, d := range domains {
				if blockedSet[d] {
					bd = append(bd, d)
				}
				if unblockedSet[d] {
					ubd = append(ubd, d)
				}
			}
			if len(bd) > 0 {
				events = append(events, BlockEvent{TS: now, Event: "blocked", Group: group, Domains: bd})
			}
			if len(ubd) > 0 {
				events = append(events, BlockEvent{TS: now, Event: "unblocked", Group: group, Domains: ubd})
			}
		}
		if err := AppendEvents(events); err != nil {
			log.Printf("scheduler: append events: %v", err)
		}
	}

	// Prune log files once per calendar day.
	today := now.Truncate(24 * time.Hour)
	if today.After(lastPruneDay) {
		if err := PruneOldEvents(30 * 24 * time.Hour); err != nil {
			log.Printf("scheduler: prune events: %v", err)
		}
		if err := proxy.PruneOldUsageEvents(60 * 24 * time.Hour); err != nil {
			log.Printf("scheduler: prune usage: %v", err)
		}
		lastPruneDay = today
	}

	if len(warningDomains) > 0 {
		var domainsToWarn []string
		lastWarningMu.Lock()
		for _, domain := range warningDomains {
			lastTime, exists := lastWarningTime[domain]
			if !exists || now.Sub(lastTime) >= 1*time.Minute {
				domainsToWarn = append(domainsToWarn, domain)
				lastWarningTime[domain] = now
			}
		}
		lastWarningMu.Unlock()
		if len(domainsToWarn) > 0 {
			runMacOSWarning(domainsToWarn)
		}
	}
}

func getMacUser() string {
	out, err := exec.Command("stat", "-f", "%Su", "/dev/console").Output()
	if err != nil {
		return ""
	}
	user := strings.TrimSpace(string(out))
	if user == "root" || user == "" {
		return ""
	}
	return user
}

func runAsMacUser(scriptContent string) error {
	if runtime.GOOS != "darwin" {
		return nil
	}

	scriptPath := "/tmp/df_script.scpt"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		return fmt.Errorf("write script: %w", err)
	}

	user := getMacUser()
	var cmd *exec.Cmd
	if user == "" || os.Getuid() != 0 {
		cmd = exec.Command("osascript", scriptPath)
	} else {
		cmd = exec.Command("su", "-", user, "-c", "osascript "+scriptPath)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func getOpenBrowserDomains(domains []string) []string {
	if runtime.GOOS != "darwin" || len(domains) == 0 {
		return nil
	}

	var quotedDomains []string
	for _, d := range domains {
		quotedDomains = append(quotedDomains, fmt.Sprintf(`"%s"`, strings.TrimSpace(d)))
	}
	domainListStr := "{" + strings.Join(quotedDomains, ", ") + "}"

	script := fmt.Sprintf(`
		set domainsToCheck to %s
		set matchedDomains to {}

		if application "Google Chrome" is running then
			tell application "Google Chrome"
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToCheck
							if tabURL contains d then
								if matchedDomains does not contain d then
									set end of matchedDomains to d
								end if
							end if
						end repeat
					end repeat
				end repeat
			end tell
		end if

		if application "Safari" is running then
			tell application "Safari"
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToCheck
							if tabURL contains d then
								if matchedDomains does not contain d then
									set end of matchedDomains to d
								end if
							end if
						end repeat
					end repeat
				end repeat
			end tell
		end if

		if application "Arc" is running then
			tell application "Arc"
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToCheck
							if tabURL contains d then
								if matchedDomains does not contain d then
									set end of matchedDomains to d
								end if
							end if
						end repeat
					end repeat
				end repeat
			end tell
		end if

		if application "Brave Browser" is running then
			tell application "Brave Browser"
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToCheck
							if tabURL contains d then
								if matchedDomains does not contain d then
									set end of matchedDomains to d
								end if
							end if
						end repeat
					end repeat
				end repeat
			end tell
		end if

		if matchedDomains is {} then
			return ""
		else
			set matchedString to ""
			repeat with i from 1 to count of matchedDomains
				if i > 1 then
					set matchedString to matchedString & ", "
				end if
				set matchedString to matchedString & item i of matchedDomains
			end repeat
			return matchedString
		end if
	`, domainListStr)

	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		log.Printf("Error checking open browser domains: %v", err)
		return nil
	}

	result := strings.TrimSpace(string(out))
	if result == "" {
		return nil
	}

	parts := strings.Split(result, ", ")
	seen := make(map[string]bool, len(parts))
	var unique []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if !seen[part] {
			seen[part] = true
			unique = append(unique, part)
		}
	}

	if len(unique) == 0 {
		return nil
	}
	return unique
}

// GetOpenBrowserDomains returns currently open browser domains from the provided list.
func GetOpenBrowserDomains(domains []string) []string {
	return getOpenBrowserDomains(domains)
}

func runMacOSWarning(domains []string) {
	openDomains := getOpenBrowserDomains(domains)
	if len(openDomains) == 0 {
		return
	}

	script := scriptGenerator.GenerateWarningScript(openDomains)
	scriptExecutor.LogScript(script)
	scriptExecutor.ExecuteScript(script)
}

func closeMacOSTabs(domains []string) {
	script := scriptGenerator.GenerateCloseTabsScript(domains)
	scriptExecutor.LogScript(script)
	scriptExecutor.ExecuteScript(script)
}

// browserTabProbe identifies which of the given blocked domains are open in
// supported macOS browsers. Replaceable in tests so the per-tick close path can
// be exercised without forking osascript.
var browserTabProbe func(domains []string) []string = getOpenBrowserDomains

// browserTargetableDomains returns the subset of `blocked` that should be
// searched for in browser tabs — i.e., everything except the _doh group.
// DoH/DoT endpoints are blocked at the IP layer (pf in strict mode); they are
// not sites users visit with browsers, so probing for them in tabs is wasted
// osascript work and risks false positives if a user legitimately visits a
// DoH provider's marketing page.
func browserTargetableDomains(blocked map[string]bool, cfg config.Config) []string {
	dohSet := make(map[string]bool)
	for _, d := range cfg.ResolveGroup(enforcer.DohGroupName) {
		dohSet[d] = true
	}
	out := make([]string, 0, len(blocked))
	for d := range blocked {
		if dohSet[d] {
			continue
		}
		out = append(out, d)
	}
	return out
}

// runPerTickCloseTabs is invoked every scheduler tick. It probes for blocked
// domains that have open tabs (via probe) and, when any match, runs a single
// osascript that closes the tabs and emits a bundled "Closed N tab(s)…"
// notification. Silent no-op when nothing is blocked, no targetable domains
// remain after the _doh filter, or no matching tabs are open.
func runPerTickCloseTabs(blocked map[string]bool, cfg config.Config, probe func([]string) []string) {
	if len(blocked) == 0 {
		return
	}
	targets := browserTargetableDomains(blocked, cfg)
	if len(targets) == 0 {
		return
	}
	openDomains := probe(targets)
	if len(openDomains) == 0 {
		return
	}
	closeMacOSTabs(openDomains)
}

func sendPomodoroNotification(title, message string) {
	script := fmt.Sprintf(`display notification %q with title %q sound name "Ping"`, message, title)
	if err := scriptExecutor.ExecuteScript(script); err != nil {
		log.Printf("scheduler: pomodoro notification: %v", err)
	}
}

