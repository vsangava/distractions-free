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
)

var activeBlocks = make(map[string]bool)
var activeBlocksMu sync.RWMutex
var lastEvalTime time.Time

var activeEnforcer enforcer.Enforcer

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
		log.Printf("AppleScript execution failed: %v", err)
		return err
	}
	return nil
}

func (e *MacOSScriptExecutor) LogScript(script string) {
	log.Printf("AppleScript generated:\n%s", script)
}

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
	for _, d := range domains {
		quotedDomains = append(quotedDomains, fmt.Sprintf(`"%s"`, strings.TrimPrefix(d, "www.")))
	}
	domainListStr := "{" + strings.Join(quotedDomains, ", ") + "}"

	return fmt.Sprintf(`
		set domainsToBlock to %s
		
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
				repeat with t in tabsToClose
					close t
				end repeat
			end tell
		end if
	`, domainListStr)
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
// This is the testable function that doesn't depend on time.Now().
func EvaluateRulesAtTime(t time.Time, cfg config.Config) map[string]bool {
	if cfg.IsPaused(t) {
		return make(map[string]bool)
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

	return newBlocked
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

	newBlocked := EvaluateRulesAtTime(now, cfg)
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
			closeMacOSTabs(newlyBlocked)
		}
		if len(newlyUnblocked) > 0 {
			if err := activeEnforcer.Deactivate(newlyUnblocked); err != nil {
				log.Printf("scheduler: deactivate failed: %v", err)
			}
		}
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

