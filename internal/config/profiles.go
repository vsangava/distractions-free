package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// DefaultProfileName is the profile created on a fresh install and the fallback
// when active_profile points at a missing file.
const DefaultProfileName = "default"

// profileNameRegex enforces a small, filesystem-safe name shape. The rules file
// will be saved as <name>.json in profilesDir, so we keep names ASCII-lowercase,
// short, and free of separator characters that would surprise users on case-
// preserving-but-insensitive filesystems (APFS, NTFS).
var profileNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// reservedProfileNames cannot be used because they would collide with bootstrap
// or legacy filenames in the same directory.
var reservedProfileNames = map[string]bool{
	"sentinel":  true,
	"bootstrap": true,
	"config":    true,
}

// BootstrapFile is the on-disk shape of sentinel.json. It carries everything
// that is *not* per-profile: the system settings (DNS, enforcement_mode, auth
// token, foreground tracking), the shared groups dictionary, runtime state
// (pause/pomodoro), and the name of the active profile.
//
// Groups are deliberately bootstrap-level so users can reuse the same domain
// dictionary across profiles — a profile only differs in which groups are on
// the schedule and when.
type BootstrapFile struct {
	Settings      Settings            `json:"settings"`
	Groups        map[string][]string `json:"groups"`
	Pause         *PauseWindow        `json:"pause,omitempty"`
	Pomodoro      *PomodoroSession    `json:"pomodoro,omitempty"`
	ActiveProfile string              `json:"active_profile"`
}

// ProfileFile is the on-disk shape of profiles/<name>.json. It only carries
// the rules; settings and groups come from the bootstrap.
type ProfileFile struct {
	Rules []Rule `json:"rules"`
}

func bootstrapPath() string {
	return filepath.Join(configDir(), "sentinel.json")
}

func profilesDir() string {
	return filepath.Join(configDir(), "profiles")
}

func profilePath(name string) string {
	return filepath.Join(profilesDir(), name+".json")
}

func legacyConfigPath() string {
	return filepath.Join(configDir(), "config.json")
}

func legacyBackupPath() string {
	return filepath.Join(configDir(), "config.json.bak")
}

// ValidateProfileName returns nil if name is a usable profile name.
func ValidateProfileName(name string) error {
	if name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}
	if reservedProfileNames[name] {
		return fmt.Errorf("profile name %q is reserved", name)
	}
	if !profileNameRegex.MatchString(name) {
		return fmt.Errorf("profile name %q invalid: must match %s", name, profileNameRegex.String())
	}
	return nil
}

// ensureProfilesDir creates the profiles/ subdirectory if it does not already exist.
func ensureProfilesDir() error {
	dir := profilesDir()
	return os.MkdirAll(dir, 0755)
}

// loadBootstrap reads sentinel.json from disk. Caller must hold mu if it cares
// about consistency with AppConfig — these helpers do not lock.
func loadBootstrap() (BootstrapFile, error) {
	var b BootstrapFile
	data, err := os.ReadFile(bootstrapPath())
	if err != nil {
		return b, err
	}
	if err := json.Unmarshal(data, &b); err != nil {
		return b, fmt.Errorf("parse sentinel.json: %w", err)
	}
	return b, nil
}

// saveBootstrap writes sentinel.json atomically (write-rename) so a partial
// write cannot leave the file unparseable.
func saveBootstrap(b BootstrapFile) error {
	if err := os.MkdirAll(configDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(bootstrapPath(), data)
}

// loadProfile reads profiles/<name>.json. Returns os.ErrNotExist if missing.
func loadProfile(name string) (ProfileFile, error) {
	var p ProfileFile
	data, err := os.ReadFile(profilePath(name))
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("parse profile %q: %w", name, err)
	}
	return p, nil
}

func saveProfile(name string, p ProfileFile) error {
	if err := ensureProfilesDir(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(profilePath(name), data)
}

// listProfiles returns all profile names found on disk, sorted.
func listProfiles() ([]string, error) {
	entries, err := os.ReadDir(profilesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".json"))
	}
	sort.Strings(names)
	return names, nil
}

// atomicWrite writes data to path via a temp file + rename so concurrent
// readers (e.g. the scheduler tick reading mid-write) never see a truncated
// file. Permissions match the original 0644 of os.WriteFile.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// migrateLegacyConfigIfNeeded converts an old single-file config.json into the
// new bootstrap+profile layout. Idempotent: a no-op once sentinel.json exists,
// and a no-op when the legacy file is absent.
//
// On migration the legacy file is renamed to config.json.bak so a botched
// migration is recoverable by a human operator.
func migrateLegacyConfigIfNeeded() (bool, error) {
	if _, err := os.Stat(bootstrapPath()); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	legacy := legacyConfigPath()
	data, err := os.ReadFile(legacy)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var legacyCfg Config
	if err := json.Unmarshal(data, &legacyCfg); err != nil {
		return false, fmt.Errorf("parse legacy config.json: %w", err)
	}

	boot := BootstrapFile{
		Settings:      legacyCfg.Settings,
		Groups:        legacyCfg.Groups,
		Pause:         legacyCfg.Pause,
		Pomodoro:      legacyCfg.Pomodoro,
		ActiveProfile: DefaultProfileName,
	}
	prof := ProfileFile{Rules: legacyCfg.Rules}

	if err := saveBootstrap(boot); err != nil {
		return false, err
	}
	if err := saveProfile(DefaultProfileName, prof); err != nil {
		return false, err
	}
	if err := os.Rename(legacy, legacyBackupPath()); err != nil {
		return false, fmt.Errorf("rename legacy config: %w", err)
	}
	return true, nil
}
