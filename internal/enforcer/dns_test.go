package enforcer

import (
	"strings"
	"testing"
)

func TestWinSetDNSServersScript_SingleServer(t *testing.T) {
	got := winSetDNSServersScript([]string{"127.0.0.1"})
	if !strings.Contains(got, "@('127.0.0.1')") {
		t.Errorf("expected single-quoted server array, got: %s", got)
	}
	if !strings.Contains(got, "Get-NetAdapter -Physical") {
		t.Errorf("expected physical-adapter enumeration, got: %s", got)
	}
	if !strings.Contains(got, "Status -eq 'Up'") {
		t.Errorf("expected filter on Up adapters, got: %s", got)
	}
	if !strings.Contains(got, "Set-DnsClientServerAddress -InterfaceIndex $_.ifIndex") {
		t.Errorf("expected per-adapter Set call, got: %s", got)
	}
}

func TestWinSetDNSServersScript_WithBackup(t *testing.T) {
	got := winSetDNSServersScript([]string{"127.0.0.1", "1.1.1.1"})
	if !strings.Contains(got, "@('127.0.0.1','1.1.1.1')") {
		t.Errorf("expected both servers in quoted comma-separated array, got: %s", got)
	}
}
