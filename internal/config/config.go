package config

import (
	_ "embed"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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
	// EnableForegroundTracking turns on the per-tick AppleScript probe that
	// records how long each blocked-list domain is actually the foreground
	// browser tab. macOS-only; safe under any enforcement_mode (hosts/dns/strict)
	// because the probe runs in the scheduler, independent of the enforcer. Off
	// by default — flip to true to start populating foreground_minutes in
	// /api/usage.
	EnableForegroundTracking bool `json:"enable_foreground_tracking,omitempty"`
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
)

func GetConfigFilePath() (string, error) {
	dir := configDir()
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "config.json"), nil
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

func LoadConfig() error {
	path, err := GetConfigFilePath()
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return saveDefaultConfig(path)
		}
		return err
	}
	if err := json.Unmarshal(data, &AppConfig); err != nil {
		return err
	}
	if AppConfig.Settings.AuthToken == "" {
		token, err := generateToken()
		if err != nil {
			return err
		}
		AppConfig.Settings.AuthToken = token
		updated, _ := json.MarshalIndent(AppConfig, "", "  ")
		os.WriteFile(path, updated, 0644)
	}
	return nil
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

// SaveConfig writes the current in-memory config to disk.
func SaveConfig() error {
	path, err := GetConfigFilePath()
	if err != nil {
		return err
	}
	mu.RLock()
	data, err := json.MarshalIndent(AppConfig, "", "  ")
	mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func saveDefaultConfig(path string) error {
	if err := json.Unmarshal(defaultConfigBytes, &AppConfig); err != nil {
		return err
	}
	token, err := generateToken()
	if err != nil {
		return err
	}
	AppConfig.Settings.AuthToken = token
	data, _ := json.MarshalIndent(AppConfig, "", "  ")
	return os.WriteFile(path, data, 0644)
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
func AutoSetPrimaryDNS(server string) {
	path, err := GetConfigFilePath()
	if err != nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if AppConfig.Settings.PrimaryDNS != factoryPrimaryDNS {
		return
	}
	AppConfig.Settings.PrimaryDNS = server
	data, err := json.MarshalIndent(AppConfig, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("config: could not save auto-detected upstream DNS: %v", err)
		return
	}
	log.Printf("config: auto-detected upstream DNS %s → saved as primary_dns", server)
}

// ConfigDir returns the OS-specific config directory path without creating it.
func ConfigDir() string {
	return configDir()
}
