// Package version exposes the build-injected version string.
//
// The Version variable is overridden at build time via:
//
//	go build -ldflags "-X github.com/vsangava/sentinel/internal/version.Version=v1.2.3"
//
// In unreleased builds it stays at the default "dev".
package version

var Version = "dev"
