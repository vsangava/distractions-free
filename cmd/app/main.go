package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kardianos/service"
	"github.com/vsangava/distractions-free/internal/cleanup"
	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/enforcer"
	"github.com/vsangava/distractions-free/internal/proxy"
	"github.com/vsangava/distractions-free/internal/scheduler"
	"github.com/vsangava/distractions-free/internal/testcli"
	"github.com/vsangava/distractions-free/internal/web"
)

func testAppleScript() {
	log.Println("Testing AppleScript generation and execution...")

	// Test domains - facebook for closing tabs, reddit and roblox for warning
	closeTabsDomains := []string{"facebook.com"}
	warningDomains := []string{"reddit.com", "roblox.com"}

	closeTabsScript := scheduler.GetScriptGenerator().GenerateCloseTabsScript(closeTabsDomains)

	// Display close script
	log.Println("=== CLOSE TABS SCRIPT (facebook.com) ===")
	log.Println(closeTabsScript)
	log.Println()

	log.Println("Execute scripts? (y/N): ")
	var response string
	fmt.Scanln(&response)

	if response == "y" || response == "Y" {
		warningScript := ""
		openWarningDomains := scheduler.GetOpenBrowserDomains(warningDomains)
		if len(openWarningDomains) > 0 {
			warningScript = fmt.Sprintf(`display alert "Distractions-Free" message "Tabs for %s will close in 3 minutes." buttons {"OK"} giving up after 15`, strings.Join(openWarningDomains, ", "))
			log.Println("Executing warning script...")
			if err := scheduler.GetScriptExecutor().ExecuteScript(warningScript); err != nil {
				log.Printf("Warning script execution failed: %v", err)
			}
		} else {
			log.Println("No matching open warning domains found; warning not executed.")
		}

		log.Println("Sleeping 2 seconds before close tabs script...")
		time.Sleep(2 * time.Second)

		log.Println("Executing close tabs script...")
		if err := scheduler.GetScriptExecutor().ExecuteScript(closeTabsScript); err != nil {
			log.Printf("Close tabs script execution failed: %v", err)
		}
	} else {
		log.Println("Scripts not executed (use 'y' to execute)")
	}
}

type program struct {
	enforcer enforcer.Enforcer
}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *program) run() {
	if err := config.LoadConfig(); err != nil {
		log.Printf("Config warning: %v", err)
	}

	cfg := config.GetConfig()
	mode := cfg.Settings.GetEnforcementMode()
	log.Printf("Enforcement mode: %s", mode)

	e := enforcer.New(cfg)
	if err := e.Setup(); err != nil {
		log.Fatalf("Enforcer setup failed: %v", err)
	}
	p.enforcer = e

	scheduler.SetEnforcer(e)
	scheduler.Start()
	go web.StartWebServer()

	if mode == "dns" || mode == "strict" {
		// DNS server blocks until stopped; keep this as the last call.
		proxy.StartDNSServer()
	} else {
		// Hosts mode: no port binding needed; park the goroutine.
		select {}
	}
}

func (p *program) Stop(s service.Service) error {
	log.Println("Stopping service...")
	proxy.StopDNSServer()
	if p.enforcer != nil {
		p.enforcer.Teardown()
	}
	return nil
}

var svcConfig = &service.Config{
	Name:        "DistractionsFree",
	DisplayName: "Distractions Free DNS Proxy",
	Description: "Local DNS proxy for blocking distractions.",
}

func runClean(yes bool) {
	if !cleanup.IsPrivileged() {
		log.Fatal("--clean requires root/admin privileges. Run with: sudo ./distractions-free --clean")
	}

	fmt.Println("Cleaning all distractions-free system changes...")
	fmt.Println()

	var steps []cleanup.Step
	criticalFailed := false

	addStep := func(s cleanup.Step) {
		steps = append(steps, s)
		if s.Status == cleanup.StatusError && s.Critical {
			criticalFailed = true
		}
	}
	addSteps := func(ss []cleanup.Step) {
		for _, s := range ss {
			addStep(s)
		}
	}

	// Step 1 — Stop the running service.
	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		addStep(cleanup.Step{Label: "Service stopped", Status: cleanup.StatusWarn, Detail: "could not create service handle: " + err.Error()})
	} else if err := service.Control(s, "stop"); err != nil {
		// Not running is not a failure.
		addStep(cleanup.Step{Label: "Service stopped", Status: cleanup.StatusWarn, Detail: err.Error()})
	} else {
		addStep(cleanup.Step{Label: "Service stopped", Status: cleanup.StatusDone})
	}

	// Step 2 — Reset DNS on all network interfaces that point at 127.0.0.1.
	addSteps(cleanup.ResetAllDNSInterfaces())

	// Step 3 — Remove distractions-free managed block from /etc/hosts.
	addStep(cleanup.CleanHostsFile())

	// Step 4 — Remove pf anchor from /etc/pf.conf (macOS strict mode).
	addStep(cleanup.CleanPFAnchor())

	// Step 5 — Flush DNS cache.
	addStep(cleanup.FlushDNSCache())

	// Step 6 — Uninstall the system service.
	if s != nil {
		if err := service.Control(s, "uninstall"); err != nil {
			addStep(cleanup.Step{Label: "Service uninstalled", Status: cleanup.StatusWarn, Detail: err.Error(), Critical: true})
		} else {
			addStep(cleanup.Step{Label: "Service uninstalled", Status: cleanup.StatusDone, Critical: true})
		}
	}

	// Step 7 — Remove config directory.
	addStep(cleanup.RemoveConfigDir(yes))

	// Step 8 — Remove temp files.
	addStep(cleanup.RemoveTempFiles())

	// Step 9 — Verify port 53 is free.
	addStep(cleanup.CheckPort53())

	// Print summary.
	fmt.Println()
	for _, step := range steps {
		fmt.Println(step.String())
	}
	fmt.Println()
	if criticalFailed {
		fmt.Println("Some critical steps failed. Review the output above.")
		os.Exit(1)
	}
	fmt.Println("System is clean. distractions-free has been fully removed.")
}

func main() {
	// --clean safely removes all system-level changes made by distractions-free.
	// Usage: sudo ./distractions-free --clean [--yes]
	if len(os.Args) > 1 && os.Args[1] == "--clean" {
		yes := len(os.Args) > 2 && os.Args[2] == "--yes"
		runClean(yes)
		return
	}

	// Test web UI mode: interactive web interface for testing blocking status
	if len(os.Args) > 1 && os.Args[1] == "--test-web" {
		// Use local config for test mode (no need for system paths)
		config.UseLocalConfig = true

		log.Println("Starting test web UI on http://localhost:8040")
		log.Println("Open your browser to http://localhost:8040 to test queries")
		web.StartTestWebServer()
		return
	}

	// Test mode: query blocking status for a domain at a specific time
	if len(os.Args) > 1 && os.Args[1] == "--test-query" {
		if len(os.Args) < 4 {
			log.Fatalf("Usage: %s --test-query <time> <domain>\n", os.Args[0])
			log.Fatalf("Time format: 2006-01-02 15:04 (example: 2024-04-01 10:30)\n")
		}
		// Use local config for test mode (no need for system paths)
		config.UseLocalConfig = true

		timeStr := os.Args[2]
		domain := os.Args[3]

		if err := testcli.QueryBlocking(timeStr, domain); err != nil {
			log.Fatalf("Test query failed: %v", err)
		}
		return
	}

	// Test AppleScript generation and execution
	if len(os.Args) > 1 && os.Args[1] == "--test-applescript" {
		testAppleScript()
		return
	}

	// Run as regular program without service
	if len(os.Args) > 1 && os.Args[1] == "--no-service" {
		config.UseLocalConfig = true
		prg := &program{}
		prg.run()
		return
	}

	// --strict sets enforcement_mode to "strict" in config and exits.
	// Usage: sudo ./distractions-free --strict
	//        sudo ./distractions-free install && sudo ./distractions-free start
	if len(os.Args) > 1 && os.Args[1] == "--strict" {
		if err := config.LoadConfig(); err != nil {
			log.Printf("Config warning (will use defaults): %v", err)
		}
		config.SetEnforcementMode("strict")
		if err := config.SaveConfig(); err != nil {
			log.Fatalf("Failed to save config: %v", err)
		}
		log.Println("Enforcement mode set to 'strict'. Restart the service to apply.")
		return
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) > 1 {
		err = service.Control(s, os.Args[1])
		if err != nil {
			log.Fatalf("Failed to %s: %v", os.Args[1], err)
		}
		log.Printf("Successfully performed: %s", os.Args[1])
		return
	}

	if err = s.Run(); err != nil {
		log.Fatal(err)
	}
}
