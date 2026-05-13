package config

import (
	_ "embed"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

//go:embed default_config.json
var defaultConfigBytes []byte

type TimeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// Rule binds a named group of domains to a weekly schedule.
// Group must reference a key in Config.Groups.
type Rule struct {
	Group             string                `json:"group"`
	IsActive          bool                  `json:"is_active"`
	DailyQuotaMinutes int                   `json:"daily_quota_minutes,omitempty"`
	Schedules         map[string][]TimeSlot `json:"schedules"`
}

type Settings struct {
	PrimaryDNS      string `json:"primary_dns"`
	BackupDNS       string `json:"backup_dns"`
	AuthToken       string `json:"auth_token"`
	EnforcementMode string `json:"enforcement_mode,omitempty"`
	DNSFailureMode  string `json:"dns_failure_mode,omitempty"`
	// EnableForegroundTracking turns on the per-tick foreground-tab probe that
	// records how long each blocked-list domain is actually the foreground
	// browser tab. Safe under any enforcement_mode (hosts/dns/strict) because the
	// probe runs in the scheduler, independent of the enforcer. Off by default —
	// flip to true to start populating foreground_minutes in /api/usage. macOS
	// reads the real active-tab URL via AppleScript; Windows uses a window-title
	// heuristic (Chrome/Edge), optionally upgraded by WindowsForegroundUseUIA;
	// no-op on other platforms.
	EnableForegroundTracking bool `json:"enable_foreground_tracking,omitempty"`
	// WindowsForegroundUseUIA, when true, makes the Windows foreground probe try
	// to read the active tab's real URL out of the browser's address bar via UI
	// Automation, falling back to the window-title heuristic on any failure.
	// Windows-only; ignored elsewhere. Off by default — it's the more accurate
	// path but exercises a fair amount of COM plumbing, so it's opt-in until it
	// has more real-world Windows mileage. Requires EnableForegroundTracking.
	WindowsForegroundUseUIA bool `json:"windows_foreground_use_uia,omitempty"`
}

// GetEnforcementMode returns the validated enforcement mode, defaulting to "hosts"
// when the field is absent or unrecognised. This means existing configs that predate
// this field automatically get the new default without any migration step.
func (s Settings) GetEnforcementMode() string {
	switch s.EnforcementMode {
	case "hosts", "dns", "strict":
		return s.EnforcementMode
	default:
		return "hosts"
	}
}

// GetDNSFailureMode returns the validated DNS failure mode, defaulting to "open"
// when the field is absent or unrecognised. "open" means the OS DNS config
// includes backup_dns as a system-level fallback so the machine stays online if
// Sentinel crashes; "closed" means only 127.0.0.1 is set and DNS fails entirely
// if Sentinel is not running.
func (s Settings) GetDNSFailureMode() string {
	switch s.DNSFailureMode {
	case "open", "closed":
		return s.DNSFailureMode
	default:
		return "open"
	}
}

// PauseWindow represents a temporary suspension of all blocking rules.
type PauseWindow struct {
	Until time.Time `json:"until"`
}

// PomodoroSession tracks an active Pomodoro cycle.
// Phase is "work" or "break". There is no auto-restart; each work session
// must be started explicitly by the user.
type PomodoroSession struct {
	Phase        string    `json:"phase"`
	PhaseEndsAt  time.Time `json:"phase_ends_at"`
	WorkMinutes  int       `json:"work_minutes"`
	BreakMinutes int       `json:"break_minutes"`
}

type Config struct {
	Settings Settings            `json:"settings"`
	Groups   map[string][]string `json:"groups"`
	Rules    []Rule              `json:"rules"`
	Pause    *PauseWindow        `json:"pause,omitempty"`
	Pomodoro *PomodoroSession    `json:"pomodoro,omitempty"`
}

// IsPaused reports whether all blocking rules are suspended at time t.
func (c Config) IsPaused(t time.Time) bool {
	return c.Pause != nil && t.Before(c.Pause.Until)
}

// ResolveGroup returns the domains in the named group, or nil if the group does not exist.
func (c Config) ResolveGroup(name string) []string {
	if c.Groups == nil {
		return nil
	}
	return c.Groups[name]
}

var (
	AppConfig      Config
	mu             sync.RWMutex
	UseLocalConfig bool
	// ConfigDirOverride, if non-empty, takes precedence over UseLocalConfig and
	// the OS-specific defaults. Used by tests to point config I/O at a tempdir
	// so parallel test packages don't race on the repo-rooted ./config.json
	// (and don't reformat it as a side effect).
	ConfigDirOverride string
	// activeProfile is the name of the profile whose rules currently populate
	// AppConfig.Rules. Held under mu like AppConfig.
	activeProfile string
)

// ActiveProfile returns the name of the profile whose rules currently populate
// the in-memory Config.
func ActiveProfile() string {
	mu.RLock()
	defer mu.RUnlock()
	return activeProfile
}

// EnsureConfigDir creates the OS-specific config directory if it does not
// already exist. Callers do not need this for normal Load/Save flows — the
// helpers in this package create directories on demand — but it is exposed
// for tests and one-shot CLI tools that want to fail fast on a permissions
// problem.
func EnsureConfigDir() error {
	dir := configDir()
	if dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// configDir resolves the directory that should hold config.json.
// Precedence: explicit override > local mode > OS default.
func configDir() string {
	if ConfigDirOverride != "" {
		return ConfigDirOverride
	}
	if UseLocalConfig {
		return "."
	}
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/Sentinel"
	case "windows":
		return filepath.Join(os.Getenv("PROGRAMDATA"), "Sentinel")
	default:
		return "/etc/sentinel"
	}
}

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// LoadConfig reads the on-disk bootstrap + active profile and merges them
// into the in-memory AppConfig. On a fresh install (neither file present) it
// seeds the defaults. Legacy single-file config.json deployments are migrated
// transparently on first call; the legacy file is renamed to config.json.bak.
//
// Auth tokens are generated and persisted on first run so the dashboard has a
// stable bearer token from boot.
func LoadConfig() error {
	if err := EnsureConfigDir(); err != nil {
		return err
	}
	if _, err := migrateLegacyConfigIfNeeded(); err != nil {
		return fmt.Errorf("legacy config migration: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()

	boot, err := loadBootstrap()
	if err != nil {
		if os.IsNotExist(err) {
			return seedDefaultsLocked()
		}
		return err
	}

	tokenWasGenerated := false
	if boot.Settings.AuthToken == "" {
		token, err := generateToken()
		if err != nil {
			return err
		}
		boot.Settings.AuthToken = token
		tokenWasGenerated = true
	}
	if boot.ActiveProfile == "" {
		boot.ActiveProfile = DefaultProfileName
	}

	prof, err := loadProfile(boot.ActiveProfile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// Active profile points at a missing file. Fall back to "default";
		// if that's also missing, write an empty profile so the daemon stays
		// up and the user can recover via the dashboard.
		log.Printf("config: active profile %q missing on disk, falling back to %q",
			boot.ActiveProfile, DefaultProfileName)
		boot.ActiveProfile = DefaultProfileName
		prof, err = loadProfile(DefaultProfileName)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			prof = ProfileFile{Rules: nil}
			if err := saveProfile(DefaultProfileName, prof); err != nil {
				return err
			}
		}
		if err := saveBootstrap(boot); err != nil {
			return err
		}
	} else if tokenWasGenerated {
		if err := saveBootstrap(boot); err != nil {
			return err
		}
	}

	AppConfig = mergeBootstrapAndProfile(boot, prof)
	activeProfile = boot.ActiveProfile
	return nil
}

// mergeBootstrapAndProfile produces the in-memory Config consumed by the
// scheduler/proxy/enforcer. Keeping this view identical to the pre-profiles
// shape lets every downstream package stay untouched.
func mergeBootstrapAndProfile(b BootstrapFile, p ProfileFile) Config {
	return Config{
		Settings: b.Settings,
		Groups:   b.Groups,
		Rules:    p.Rules,
		Pause:    b.Pause,
		Pomodoro: b.Pomodoro,
	}
}

// splitConfigForDisk is the inverse of mergeBootstrapAndProfile. It also
// records which profile name to write to.
func splitConfigForDisk(c Config, activeProfile string) (BootstrapFile, ProfileFile) {
	if activeProfile == "" {
		activeProfile = DefaultProfileName
	}
	return BootstrapFile{
			Settings:      c.Settings,
			Groups:        c.Groups,
			Pause:         c.Pause,
			Pomodoro:      c.Pomodoro,
			ActiveProfile: activeProfile,
		}, ProfileFile{
			Rules: c.Rules,
		}
}

// ListProfiles returns the names of every profile on disk, sorted.
func ListProfiles() ([]string, error) {
	return listProfiles()
}

// ProfileExists reports whether profiles/<name>.json exists.
func ProfileExists(name string) (bool, error) {
	if err := ValidateProfileName(name); err != nil {
		return false, err
	}
	_, err := os.Stat(profilePath(name))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CreateProfile writes profiles/<name>.json. If cloneFrom is non-empty, the
// new profile copies its rules from the named profile; otherwise it starts
// with an empty rule set. Returns an error if the name is invalid, the
// profile already exists, or cloneFrom is set but missing.
func CreateProfile(name, cloneFrom string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	exists, err := ProfileExists(name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("profile %q already exists", name)
	}

	var prof ProfileFile
	if cloneFrom != "" {
		if err := ValidateProfileName(cloneFrom); err != nil {
			return fmt.Errorf("clone_from: %w", err)
		}
		src, err := loadProfile(cloneFrom)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("clone_from profile %q does not exist", cloneFrom)
			}
			return err
		}
		prof = src
	}
	return saveProfile(name, prof)
}

// DeleteProfile removes profiles/<name>.json. Returns an error if name is
// invalid, the profile does not exist, or the profile is currently active.
func DeleteProfile(name string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	if name == ActiveProfile() {
		return fmt.Errorf("cannot delete active profile %q", name)
	}
	if name == DefaultProfileName {
		return fmt.Errorf("cannot delete the default profile")
	}
	if err := os.Remove(profilePath(name)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("profile %q does not exist", name)
		}
		return err
	}
	return nil
}

// SwitchProfile changes the active profile in the bootstrap file, then
// reloads the in-memory config so the next scheduler tick (or any reader)
// sees the new rules. Returns an error if name is invalid or the profile
// file does not exist.
func SwitchProfile(name string) error {
	if err := ValidateProfileName(name); err != nil {
		return err
	}
	exists, err := ProfileExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("profile %q does not exist", name)
	}

	boot, err := loadBootstrap()
	if err != nil {
		return err
	}
	boot.ActiveProfile = name
	if err := saveBootstrap(boot); err != nil {
		return err
	}
	return LoadConfig()
}

// ReplaceFullConfig accepts a merged Config (the legacy on-the-wire shape used
// by /api/config/update) and writes it across the bootstrap + active profile
// files. The auth token from the existing bootstrap is preserved — callers
// cannot rotate it through this path. The active profile name is unchanged.
func ReplaceFullConfig(cfg Config) error {
	mu.Lock()
	prof := activeProfile
	if prof == "" {
		prof = DefaultProfileName
	}
	cfg.Settings.AuthToken = AppConfig.Settings.AuthToken
	mu.Unlock()

	boot, profFile := splitConfigForDisk(cfg, prof)
	if err := saveBootstrap(boot); err != nil {
		return err
	}
	if err := saveProfile(prof, profFile); err != nil {
		return err
	}
	return LoadConfig()
}

// SetEnforcementMode updates the in-memory enforcement mode.
// Call SaveConfig to persist the change to disk.
func SetEnforcementMode(mode string) {
	mu.Lock()
	defer mu.Unlock()
	AppConfig.Settings.EnforcementMode = mode
}

// SetPause sets a pause window that suppresses all blocking until until.
// Call SaveConfig to persist the change to disk.
func SetPause(until time.Time) {
	mu.Lock()
	defer mu.Unlock()
	AppConfig.Pause = &PauseWindow{Until: until}
}

// ClearPause removes any active pause window.
// Call SaveConfig to persist the change to disk.
func ClearPause() {
	mu.Lock()
	defer mu.Unlock()
	AppConfig.Pause = nil
}

// IsLockedByPomodoro reports whether the Pomodoro work phase is active at t.
func (c Config) IsLockedByPomodoro(t time.Time) bool {
	return c.Pomodoro != nil && c.Pomodoro.Phase == "work" && t.Before(c.Pomodoro.PhaseEndsAt)
}

// StartPomodoro begins a new Pomodoro work session.
// Call SaveConfig to persist.
func StartPomodoro(workMin, breakMin int) {
	mu.Lock()
	defer mu.Unlock()
	AppConfig.Pomodoro = &PomodoroSession{
		Phase:        "work",
		PhaseEndsAt:  time.Now().Add(time.Duration(workMin) * time.Minute),
		WorkMinutes:  workMin,
		BreakMinutes: breakMin,
	}
}

// AdvancePomodoroPhase transitions the work phase to break.
// Must only be called when the current phase is "work" and has just expired.
// Call SaveConfig to persist.
func AdvancePomodoroPhase() {
	mu.Lock()
	defer mu.Unlock()
	if AppConfig.Pomodoro == nil || AppConfig.Pomodoro.Phase != "work" {
		return
	}
	AppConfig.Pomodoro.Phase = "break"
	AppConfig.Pomodoro.PhaseEndsAt = time.Now().Add(
		time.Duration(AppConfig.Pomodoro.BreakMinutes) * time.Minute,
	)
}

// ClearPomodoro removes any active Pomodoro session.
// Call SaveConfig to persist.
func ClearPomodoro() {
	mu.Lock()
	defer mu.Unlock()
	AppConfig.Pomodoro = nil
}

// SaveConfig persists the in-memory AppConfig to disk by splitting it across
// the bootstrap (sentinel.json) and the active profile file. Both writes are
// atomic individually; the pair is not. A crash between the two leaves the
// active profile pointing at the most recent rules — acceptable because the
// scheduler reloads from disk every minute and re-converges.
func SaveConfig() error {
	mu.RLock()
	cfg := AppConfig
	prof := activeProfile
	mu.RUnlock()

	if prof == "" {
		prof = DefaultProfileName
	}
	boot, profFile := splitConfigForDisk(cfg, prof)
	if err := saveBootstrap(boot); err != nil {
		return err
	}
	return saveProfile(prof, profFile)
}

// seedDefaultsLocked populates AppConfig from the embedded default config and
// writes the bootstrap + default profile to disk. Caller must hold mu.
func seedDefaultsLocked() error {
	var seed Config
	if err := json.Unmarshal(defaultConfigBytes, &seed); err != nil {
		return err
	}
	token, err := generateToken()
	if err != nil {
		return err
	}
	seed.Settings.AuthToken = token

	boot, prof := splitConfigForDisk(seed, DefaultProfileName)
	if err := saveBootstrap(boot); err != nil {
		return err
	}
	if err := saveProfile(DefaultProfileName, prof); err != nil {
		return err
	}
	AppConfig = seed
	activeProfile = DefaultProfileName
	return nil
}

func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return AppConfig
}

const factoryPrimaryDNS = "8.8.8.8:53"

// AutoSetPrimaryDNS updates primary_dns only when it still holds the factory
// default. This lets the first dns/strict mode startup capture whatever DNS
// the user had before Sentinel, without overwriting deliberate user settings.
//
// Settings live in the bootstrap, so this writes only sentinel.json.
func AutoSetPrimaryDNS(server string) {
	mu.Lock()
	if AppConfig.Settings.PrimaryDNS != factoryPrimaryDNS {
		mu.Unlock()
		return
	}
	AppConfig.Settings.PrimaryDNS = server
	cfg := AppConfig
	prof := activeProfile
	mu.Unlock()

	if prof == "" {
		prof = DefaultProfileName
	}
	boot, _ := splitConfigForDisk(cfg, prof)
	if err := saveBootstrap(boot); err != nil {
		log.Printf("config: could not save auto-detected upstream DNS: %v", err)
		return
	}
	log.Printf("config: auto-detected upstream DNS %s → saved as primary_dns", server)
}

// ConfigDir returns the OS-specific config directory path without creating it.
func ConfigDir() string {
	return configDir()
}
