package cleanup

import (
	"os"
	"path/filepath"
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
