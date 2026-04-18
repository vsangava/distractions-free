package scheduler

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/proxy"
)

var activeBlocks = make(map[string]bool)

// EvaluateRulesAtTime evaluates blocking rules at a specific time and returns blocked domains.
// This is the testable function that doesn't depend on time.Now().
func EvaluateRulesAtTime(t time.Time, cfg config.Config) map[string]bool {
	currentDay := t.Weekday().String()
	currentTime := t.Format("15:04")

	newBlocked := make(map[string]bool)

	// Evaluate times
	for _, rule := range cfg.Rules {
		if !rule.IsActive {
			continue
		}
		if slots, exists := rule.Schedules[currentDay]; exists {
			for _, slot := range slots {
				// Check active blocks
				if currentTime >= slot.Start && currentTime < slot.End {
					newBlocked[rule.Domain] = true
					break
				}
			}
		}
	}

	return newBlocked
}

// CheckWarningDomainsAtTime checks if any domains should trigger 3-minute warnings at a specific time.
// This is the testable function that doesn't depend on time.Now().
func CheckWarningDomainsAtTime(t time.Time, cfg config.Config) []string {
	currentDay := t.Weekday().String()
	futureTime := t.Add(3 * time.Minute).Format("15:04")

	var warningDomains []string

	// Check for 3-minute warnings
	for _, rule := range cfg.Rules {
		if !rule.IsActive {
			continue
		}
		if slots, exists := rule.Schedules[currentDay]; exists {
			for _, slot := range slots {
				if futureTime == slot.Start {
					warningDomains = append(warningDomains, rule.Domain)
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
	cfg := config.GetConfig()
	now := time.Now()

	newBlocked := EvaluateRulesAtTime(now, cfg)
	warningDomains := CheckWarningDomainsAtTime(now, cfg)

	var newlyBlockedDomains []string
	requiresFlush := false

	// Check if state changed (domains added or removed)
	if len(newBlocked) != len(activeBlocks) || len(newlyBlockedDomains) > 0 {
		for domain := range newBlocked {
			if !activeBlocks[domain] {
				newlyBlockedDomains = append(newlyBlockedDomains, domain)
			}
		}
		if len(newlyBlockedDomains) > 0 {
			requiresFlush = true
		}
	}

	// Apply states
	activeBlocks = newBlocked
	proxy.UpdateBlockedDomains(newBlocked)

	if len(warningDomains) > 0 {
		runMacOSWarning(warningDomains)
	}

	if requiresFlush {
		flushDNS()
		if len(newlyBlockedDomains) > 0 {
			closeMacOSTabs(newlyBlockedDomains)
		}
	}
}

func getMacUser() string {
	out, err := exec.Command("stat", "-f", "%Su", "/dev/console").Output()
	if err != nil {
		return ""
	}
	user := strings.TrimSpace(string(out))
	if user == "root" {
		return ""
	}
	return user
}

func runAsMacUser(scriptContent string) {
	if runtime.GOOS != "darwin" {
		return
	}
	user := getMacUser()
	if user == "" {
		return
	}

	scriptPath := "/tmp/df_script.scpt"
	os.WriteFile(scriptPath, []byte(scriptContent), 0644)
	exec.Command("su", "-", user, "-c", "osascript "+scriptPath).Run()
}

func runMacOSWarning(domains []string) {
	msg := fmt.Sprintf("Tabs for %s will close in 3 minutes.", strings.Join(domains, ", "))
	script := fmt.Sprintf(`display notification "%s" with title "Distractions-Free" subtitle "Upcoming Block" sound name "Basso"`, msg)
	runAsMacUser(script)
}

func closeMacOSTabs(domains []string) {
	var quotedDomains []string
	for _, d := range domains {
		quotedDomains = append(quotedDomains, fmt.Sprintf(`"%s"`, strings.TrimPrefix(d, "www.")))
	}
	domainListStr := "{" + strings.Join(quotedDomains, ", ") + "}"

	script := fmt.Sprintf(`
		set domainsToBlock to %s
		
		if application "Google Chrome" is running then
			tell application "Google Chrome"
				repeat with w in windows
					repeat with t in tabs of w
						set tabURL to URL of t
						repeat with d in domainsToBlock
							if tabURL contains d then close t
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
						repeat with d in domainsToBlock
							if tabURL contains d then close t
						end repeat
					end repeat
				end repeat
			end tell
		end if
	`, domainListStr)

	runAsMacUser(script)
}

func flushDNS() {
	if runtime.GOOS == "darwin" {
		exec.Command("dscacheutil", "-flushcache").Run()
		exec.Command("killall", "-HUP", "mDNSResponder").Run()
		log.Println("macOS DNS Cache Flushed.")
	} else if runtime.GOOS == "windows" {
		exec.Command("ipconfig", "/flushdns").Run()
	}
}
