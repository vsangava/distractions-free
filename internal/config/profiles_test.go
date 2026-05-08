package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// withTempDir routes config I/O at a fresh tempdir for the lifetime of the
// test, restoring the prior override on cleanup. Tests that exercise on-disk
// state should call this so they don't race against the package-level
// AppConfig set up by other tests.
func withTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prevOverride := ConfigDirOverride
	prevApp := AppConfig
	prevActive := activeProfile
	ConfigDirOverride = dir
	t.Cleanup(func() {
		ConfigDirOverride = prevOverride
		AppConfig = prevApp
		activeProfile = prevActive
	})
	return dir
}

func TestValidateProfileName(t *testing.T) {
	cases := []struct {
		name string
		want bool // true = valid
	}{
		{"work", true},
		{"work-mode", true},
		{"study_2025", true},
		{"a", true},
		{"123work", true},
		{"", false},
		{"-leading-dash", false},
		{"_leading-underscore", false},
		{"UPPER", false},
		{"has space", false},
		{"has/slash", false},
		{"sentinel", false},  // reserved
		{"bootstrap", false}, // reserved
		{"config", false},    // reserved
	}
	for _, c := range cases {
		err := ValidateProfileName(c.name)
		if (err == nil) != c.want {
			t.Errorf("ValidateProfileName(%q) error=%v, want valid=%v", c.name, err, c.want)
		}
	}
}

func TestLoadConfig_FreshInstall_SeedsBootstrapAndDefaultProfile(t *testing.T) {
	dir := withTempDir(t)

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Bootstrap and profile files should exist on disk.
	if _, err := os.Stat(filepath.Join(dir, "sentinel.json")); err != nil {
		t.Fatalf("sentinel.json not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "profiles", "default.json")); err != nil {
		t.Fatalf("profiles/default.json not created: %v", err)
	}

	// Auth token should be generated.
	cfg := GetConfig()
	if cfg.Settings.AuthToken == "" {
		t.Error("expected auth token to be generated on fresh install")
	}
	// Embedded default has rules — assert they were carried into AppConfig.
	if len(cfg.Rules) == 0 {
		t.Error("expected embedded default rules to populate Config.Rules")
	}
	// Active profile should be "default".
	if got := ActiveProfile(); got != DefaultProfileName {
		t.Errorf("ActiveProfile=%q, want %q", got, DefaultProfileName)
	}
}

func TestLoadConfig_RoundTrip_PreservesAuthToken(t *testing.T) {
	withTempDir(t)

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig 1: %v", err)
	}
	originalToken := GetConfig().Settings.AuthToken

	// Reset in-memory state, force a re-read from disk.
	AppConfig = Config{}
	activeProfile = ""

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig 2: %v", err)
	}
	if GetConfig().Settings.AuthToken != originalToken {
		t.Errorf("auth token changed across LoadConfig calls: was %q, now %q",
			originalToken, GetConfig().Settings.AuthToken)
	}
}

func TestLoadConfig_LegacyMigration_SplitsAndBacksUp(t *testing.T) {
	dir := withTempDir(t)

	// Write a legacy single-file config.json.
	legacy := Config{
		Settings: Settings{
			PrimaryDNS:      "8.8.8.8:53",
			BackupDNS:       "1.1.1.1:53",
			AuthToken:       "preserved-token-xyz",
			EnforcementMode: "strict",
		},
		Groups: map[string][]string{
			"social": {"twitter.com", "instagram.com"},
		},
		Rules: []Rule{
			{
				Group:    "social",
				IsActive: true,
				Schedules: map[string][]TimeSlot{
					"Monday": {{Start: "09:00", End: "12:00"}},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0644); err != nil {
		t.Fatalf("seeding legacy file: %v", err)
	}

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Legacy file should have been renamed to .bak.
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Error("expected legacy config.json to have been renamed away")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.json.bak")); err != nil {
		t.Errorf("expected config.json.bak after migration: %v", err)
	}

	// In-memory config should match the legacy content.
	cfg := GetConfig()
	if cfg.Settings.AuthToken != "preserved-token-xyz" {
		t.Errorf("auth_token not preserved: got %q", cfg.Settings.AuthToken)
	}
	if cfg.Settings.EnforcementMode != "strict" {
		t.Errorf("enforcement_mode not preserved: got %q", cfg.Settings.EnforcementMode)
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Group != "social" {
		t.Errorf("rules not migrated correctly: %+v", cfg.Rules)
	}
	if got := ActiveProfile(); got != DefaultProfileName {
		t.Errorf("ActiveProfile after migration=%q, want %q", got, DefaultProfileName)
	}

	// Re-running LoadConfig should be idempotent — no error, no state churn.
	if err := LoadConfig(); err != nil {
		t.Fatalf("second LoadConfig (idempotency): %v", err)
	}
}

func TestSwitchProfile_RoundTrip(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Create a "work" profile that overrides the default rules.
	if err := CreateProfile("work", ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	// Write distinguishable rules into the new profile.
	if err := saveProfile("work", ProfileFile{
		Rules: []Rule{{Group: "videos", IsActive: true, Schedules: map[string][]TimeSlot{
			"Monday": {{Start: "10:00", End: "11:00"}},
		}}},
	}); err != nil {
		t.Fatalf("saveProfile: %v", err)
	}

	if err := SwitchProfile("work"); err != nil {
		t.Fatalf("SwitchProfile: %v", err)
	}

	cfg := GetConfig()
	if got := ActiveProfile(); got != "work" {
		t.Errorf("ActiveProfile=%q after switch, want %q", got, "work")
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].Group != "videos" {
		t.Errorf("expected work rules after switch, got %+v", cfg.Rules)
	}

	// Auth token should still be intact (lives in bootstrap).
	if cfg.Settings.AuthToken == "" {
		t.Error("auth token wiped by profile switch")
	}
}

func TestSwitchProfile_MissingProfile_Errors(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if err := SwitchProfile("nope"); err == nil {
		t.Error("expected error switching to nonexistent profile")
	}
}

func TestSwitchProfile_InvalidName_Errors(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if err := SwitchProfile("BAD NAME"); err == nil {
		t.Error("expected validation error for invalid profile name")
	}
}

func TestCreateProfile_Clone(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if err := CreateProfile("study", DefaultProfileName); err != nil {
		t.Fatalf("CreateProfile clone: %v", err)
	}
	src, err := loadProfile(DefaultProfileName)
	if err != nil {
		t.Fatalf("loadProfile default: %v", err)
	}
	clone, err := loadProfile("study")
	if err != nil {
		t.Fatalf("loadProfile study: %v", err)
	}
	if len(src.Rules) != len(clone.Rules) {
		t.Errorf("clone rule count mismatch: src=%d clone=%d",
			len(src.Rules), len(clone.Rules))
	}
}

func TestCreateProfile_Duplicate_Errors(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if err := CreateProfile("dup", ""); err != nil {
		t.Fatalf("first CreateProfile: %v", err)
	}
	if err := CreateProfile("dup", ""); err == nil {
		t.Error("expected error on duplicate CreateProfile")
	}
}

func TestDeleteProfile_Active_Errors(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if err := DeleteProfile(DefaultProfileName); err == nil {
		t.Error("expected error deleting default/active profile")
	}
}

func TestDeleteProfile_Inactive_OK(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if err := CreateProfile("temp", ""); err != nil {
		t.Fatalf("CreateProfile: %v", err)
	}
	if err := DeleteProfile("temp"); err != nil {
		t.Fatalf("DeleteProfile: %v", err)
	}
	exists, err := ProfileExists("temp")
	if err != nil {
		t.Fatalf("ProfileExists: %v", err)
	}
	if exists {
		t.Error("expected profile file removed after DeleteProfile")
	}
}

func TestLoadConfig_MissingActiveProfile_FallsBackToDefault(t *testing.T) {
	dir := withTempDir(t)

	// Hand-author a bootstrap that points at a profile that won't exist.
	boot := BootstrapFile{
		Settings: Settings{
			PrimaryDNS: "8.8.8.8:53",
			AuthToken:  "test-token",
		},
		Groups:        map[string][]string{"social": {"twitter.com"}},
		ActiveProfile: "ghost",
	}
	bootData, _ := json.MarshalIndent(boot, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "sentinel.json"), bootData, 0644); err != nil {
		t.Fatalf("write bootstrap: %v", err)
	}

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := ActiveProfile(); got != DefaultProfileName {
		t.Errorf("ActiveProfile=%q, want fallback %q", got, DefaultProfileName)
	}
	// Auth token should be preserved across the fallback.
	if cfg := GetConfig(); cfg.Settings.AuthToken != "test-token" {
		t.Errorf("auth token lost on fallback: got %q", cfg.Settings.AuthToken)
	}
}

func TestListProfiles_Sorted(t *testing.T) {
	withTempDir(t)
	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, n := range []string{"zebra", "alpha", "mango"} {
		if err := CreateProfile(n, ""); err != nil {
			t.Fatalf("CreateProfile %q: %v", n, err)
		}
	}
	got, err := ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	want := []string{"alpha", "default", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("ListProfiles len=%d, want %d (got %v)", len(got), len(want), got)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("ListProfiles[%d]=%q, want %q", i, got[i], name)
		}
	}
}
