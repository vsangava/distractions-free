package cleanup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestRemoveBinary_NotInstalled verifies that the no-op branch returns a
// "skipped" step instead of an error when the binary is not on disk.
func TestRemoveBinary_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "sentinel")

	step := removeBinary(missing)
	if step.Status != StatusSkipped {
		t.Errorf("expected Skipped for missing binary, got %s (detail=%q)", step.Status, step.Detail)
	}
}

// TestRemoveBinary_Installed verifies that an existing file is unlinked and
// the step is reported as done.
func TestRemoveBinary_Installed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sentinel")
	if err := os.WriteFile(path, []byte("dummy"), 0755); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := removeBinary(path)
	if step.Status != StatusDone {
		t.Errorf("expected Done, got %s (detail=%q)", step.Status, step.Detail)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected file to be removed, stat err=%v", err)
	}
}

// TestDefaultInstalledBinaryPath_PerOS pins the canonical install path per
// platform — change-detector for the macOS↔Windows parity contract.
func TestDefaultInstalledBinaryPath_PerOS(t *testing.T) {
	got := defaultInstalledBinaryPath()
	switch runtime.GOOS {
	case "darwin":
		if got != "/usr/local/bin/sentinel" {
			t.Errorf("darwin install path: got %q, want /usr/local/bin/sentinel", got)
		}
	case "windows":
		if !strings.HasSuffix(got, `Sentinel\sentinel.exe`) {
			t.Errorf("windows install path: got %q, want suffix Sentinel\\sentinel.exe", got)
		}
	default:
		if got != "" {
			t.Errorf("unsupported OS install path: got %q, want \"\"", got)
		}
	}
}

// TestRemoveInstalledBinary_OverridePath confirms that RemoveInstalledBinary
// honours the package-level installedBinaryPath (the test-override hook used
// by anyone wiring it against a tempdir) and that removal works on supported
// OSes; on unsupported OSes the step is skipped regardless.
func TestRemoveInstalledBinary_OverridePath(t *testing.T) {
	if installedBinaryPath == "" {
		// Unsupported OS: RemoveInstalledBinary should always skip, no override.
		step := RemoveInstalledBinary()
		if step.Status != StatusSkipped {
			t.Errorf("unsupported OS: expected Skipped, got %s", step.Status)
		}
		return
	}

	orig := installedBinaryPath
	t.Cleanup(func() { installedBinaryPath = orig })

	dir := t.TempDir()
	fake := filepath.Join(dir, "sentinel")
	if err := os.WriteFile(fake, []byte("dummy"), 0755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	installedBinaryPath = fake

	step := RemoveInstalledBinary()
	if step.Status != StatusDone {
		t.Errorf("expected Done, got %s (detail=%q)", step.Status, step.Detail)
	}
	if _, err := os.Stat(fake); !os.IsNotExist(err) {
		t.Errorf("expected fake binary removed, stat err=%v", err)
	}
}
