// Package pf wraps pfctl for firewall-level domain blocking on macOS.
// The implementation is currently a stub — full pf integration is tracked
// separately. StrictEnforcer degrades gracefully to DNS-only until this is complete.
package pf

import "log"

// InstallAnchor installs the distractions-free pf anchor.
func InstallAnchor() error {
	log.Println("pf: anchor installation not yet implemented; strict mode will use DNS-only blocking")
	return nil
}

// RemoveAnchor removes the distractions-free pf anchor.
func RemoveAnchor() {
	// no-op until implemented
}

// ActivateBlock resolves IPs for the given domains and adds them to the pf block table.
func ActivateBlock(domains []string, primaryDNS string) {
	// no-op until implemented
}

// DeactivateBlock clears all entries from the pf block table.
func DeactivateBlock() {
	// no-op until implemented
}
