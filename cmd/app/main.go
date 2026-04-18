package main

import (
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/kardianos/service"
	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/proxy"
	"github.com/vsangava/distractions-free/internal/scheduler"
	"github.com/vsangava/distractions-free/internal/web"
)

type program struct{}

func (p *program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *program) run() {
	if err := config.LoadConfig(); err != nil {
		log.Printf("Config warning: %v", err)
	}
	scheduler.Start()
	go web.StartWebServer()
	proxy.StartDNSServer()
}

func (p *program) Stop(s service.Service) error {
	log.Println("Stopping service... restoring default OS DNS.")
	if runtime.GOOS == "darwin" {
		exec.Command("networksetup", "-setdnsservers", "Wi-Fi", "Empty").Run()
		exec.Command("dscacheutil", "-flushcache").Run()
		exec.Command("killall", "-HUP", "mDNSResponder").Run()
	} else if runtime.GOOS == "windows" {
		exec.Command("powershell", "-Command", "Set-DnsClientServerAddress -InterfaceAlias 'Wi-Fi' -ResetServerAddresses").Run()
	}
	return nil
}

func main() {
	// Run as regular program without service
	if len(os.Args) > 1 && os.Args[1] == "--no-service" {
		config.UseLocalConfig = true
		prg := &program{}
		prg.run()
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
