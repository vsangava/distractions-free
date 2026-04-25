package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/kardianos/service"
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

func main() {
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

	svcConfig := &service.Config{
		Name:        "DistractionsFree",
		DisplayName: "Distractions Free DNS Proxy",
		Description: "Local DNS proxy for blocking distractions.",
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
