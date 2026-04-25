// Package cleanup implements the --clean command: a forensic recovery path that
// removes every system-level change distractions-free may have made, regardless
// of whether the service was stopped gracefully.
package cleanup

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/enforcer"
	"github.com/vsangava/distractions-free/internal/pf"
)

const (
	StatusDone    = "done"
	StatusSkipped = "skipped"
	StatusWarn    = "warn"
	StatusError   = "error"
)

// Step records the outcome of a single cleanup action.
type Step struct {
	Label  string
	Status string // StatusDone | StatusSkipped | StatusWarn | StatusError
	Detail string
	// Critical marks steps whose failure should cause a non-zero exit code.
	Critical bool
}

func (s Step) String() string {
	var icon string
	switch s.Status {
	case StatusDone:
		icon = "[✓]"
	case StatusSkipped:
		icon = "[—]"
	case StatusWarn:
		icon = "[!]"
	default:
		icon = "[✗]"
	}
	if s.Detail != "" {
		return icon + " " + s.Label + ": " + s.Detail
	}
	return icon + " " + s.Label
}

// logCmd prints a shell command to stdout before executing it so the user can
// see exactly what is happening.
func logCmd(name string, args ...string) {
	parts := append([]string{name}, args...)
	fmt.Println(" $", strings.Join(parts, " "))
}

// ResetAllDNSInterfaces enumerates all network services and resets DNS on any
// that currently point at 127.0.0.1. Returns one Step per interface checked.
// Only resets interfaces that we set — does not touch interfaces with other DNS.
func ResetAllDNSInterfaces() []Step {
	switch runtime.GOOS {
	case "darwin":
		return resetDNSInterfacesDarwin()
	case "windows":
		return resetDNSInterfacesWindows()
	default:
		return []Step{{Label: "Reset DNS interfaces", Status: StatusSkipped, Detail: "not applicable on " + runtime.GOOS}}
	}
}

func resetDNSInterfacesDarwin() []Step {
	logCmd("networksetup", "-listallnetworkservices")
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return []Step{{
			Label:    "Reset DNS interfaces",
			Status:   StatusError,
			Detail:   fmt.Sprintf("list services: %v", err),
			Critical: true,
		}}
	}

	var steps []Step
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		// First line is a header about asterisks; blank lines are separators.
		if strings.HasPrefix(line, "An asterisk") || strings.TrimSpace(line) == "" {
			continue
		}
		// Disabled interfaces are prefixed with "*" — strip it to get the name.
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))

		logCmd("networksetup", "-getdnsservers", name)
		dnsOut, err := exec.Command("networksetup", "-getdnsservers", name).Output()
		if err != nil {
			steps = append(steps, Step{
				Label:  "DNS on " + name,
				Status: StatusWarn,
				Detail: fmt.Sprintf("get dns: %v", err),
			})
			continue
		}

		if !strings.Contains(string(dnsOut), "127.0.0.1") {
			steps = append(steps, Step{
				Label:  "DNS on " + name,
				Status: StatusSkipped,
				Detail: "already Empty",
			})
			continue
		}

		logCmd("networksetup", "-setdnsservers", name, "Empty")
		if resetOut, err := exec.Command("networksetup", "-setdnsservers", name, "Empty").CombinedOutput(); err != nil {
			steps = append(steps, Step{
				Label:    "DNS on " + name,
				Status:   StatusError,
				Detail:   fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(resetOut))),
				Critical: true,
			})
		} else {
			steps = append(steps, Step{Label: "DNS on " + name, Status: StatusDone, Critical: true})
		}
	}

	if len(steps) == 0 {
		steps = append(steps, Step{Label: "Reset DNS interfaces", Status: StatusSkipped, Detail: "no interfaces found"})
	}
	return steps
}

func resetDNSInterfacesWindows() []Step {
	script := `Get-DnsClientServerAddress | Where-Object { $_.ServerAddresses -contains "127.0.0.1" } | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.InterfaceIndex -ResetServerAddresses }`
	logCmd("powershell", "-Command", script)
	out, err := exec.Command("powershell", "-Command", script).CombinedOutput()
	if err != nil {
		return []Step{{
			Label:    "Reset DNS interfaces",
			Status:   StatusError,
			Detail:   fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out))),
			Critical: true,
		}}
	}
	return []Step{{Label: "Reset DNS interfaces", Status: StatusDone, Critical: true}}
}

// CleanHostsFile removes the distractions-free managed block from /etc/hosts.
// Safe to call even if no block is present (idempotent).
func CleanHostsFile() Step {
	// NewHostsEnforcer needs only the hosts path, which it derives from runtime.GOOS.
	e := enforcer.NewHostsEnforcer(config.Config{})
	if err := e.DeactivateAll(); err != nil {
		return Step{Label: "Clean /etc/hosts", Status: StatusError, Detail: err.Error()}
	}
	return Step{Label: "Clean /etc/hosts", Status: StatusDone}
}

// CleanPFAnchor removes the pf anchor from /etc/pf.conf and deletes the anchor
// file. No-op on non-darwin or if the anchor was never installed.
func CleanPFAnchor() Step {
	if runtime.GOOS != "darwin" {
		return Step{Label: "Remove pf anchor", Status: StatusSkipped, Detail: "not applicable on " + runtime.GOOS}
	}
	if _, err := os.Stat("/etc/pf.anchors/distractions-free"); os.IsNotExist(err) {
		return Step{Label: "Remove pf anchor", Status: StatusSkipped, Detail: "was not installed"}
	}
	pf.RemoveAnchor()
	return Step{Label: "Remove pf anchor", Status: StatusDone}
}

// FlushDNSCache flushes the OS resolver cache.
func FlushDNSCache() Step {
	switch runtime.GOOS {
	case "darwin":
		logCmd("dscacheutil", "-flushcache")
		exec.Command("dscacheutil", "-flushcache").Run()
		logCmd("killall", "-HUP", "mDNSResponder")
		exec.Command("killall", "-HUP", "mDNSResponder").Run()
	case "windows":
		logCmd("ipconfig", "/flushdns")
		exec.Command("ipconfig", "/flushdns").Run()
	default:
		return Step{Label: "Flush DNS cache", Status: StatusSkipped, Detail: "not applicable on " + runtime.GOOS}
	}
	return Step{Label: "Flush DNS cache", Status: StatusDone}
}

// RemoveConfigDir deletes the application config directory. If yes is false the
// user is prompted for confirmation before deletion.
func RemoveConfigDir(yes bool) Step {
	dir := config.ConfigDir()
	if dir == "." {
		return Step{Label: "Remove config directory", Status: StatusSkipped, Detail: "local config in use"}
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return Step{Label: "Remove config directory", Status: StatusSkipped, Detail: "does not exist"}
	}

	if !yes {
		fmt.Printf("Remove config directory %q? [y/N] ", dir)
		var resp string
		fmt.Scanln(&resp)
		if strings.ToLower(strings.TrimSpace(resp)) != "y" {
			return Step{Label: "Remove config directory", Status: StatusSkipped, Detail: "user declined"}
		}
	}

	fmt.Printf(" $ rm -rf %q\n", dir)
	if err := os.RemoveAll(dir); err != nil {
		return Step{Label: "Remove config directory", Status: StatusWarn, Detail: err.Error()}
	}
	return Step{Label: "Remove config directory", Status: StatusDone}
}

// RemoveTempFiles removes known temporary files created by the daemon.
func RemoveTempFiles() Step {
	candidates := []string{"/tmp/df_script.scpt"}
	var removed []string
	for _, p := range candidates {
		if err := os.Remove(p); err == nil {
			removed = append(removed, p)
		}
	}
	if len(removed) == 0 {
		return Step{Label: "Remove temp files", Status: StatusSkipped, Detail: "none found"}
	}
	return Step{Label: "Remove temp files", Status: StatusDone, Detail: strings.Join(removed, ", ")}
}

// CheckPort53 verifies that 127.0.0.1:53 UDP is no longer in use.
func CheckPort53() Step {
	conn, err := net.ListenPacket("udp", "127.0.0.1:53")
	if err != nil {
		return Step{
			Label:  "Port 53 is free",
			Status: StatusWarn,
			Detail: "something may still be holding port 53 — reboot or check with 'lsof -i :53'",
		}
	}
	conn.Close()
	return Step{Label: "Port 53 is free", Status: StatusDone}
}
