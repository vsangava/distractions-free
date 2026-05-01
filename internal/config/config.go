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
	Group     string                `json:"group"`
	IsActive  bool                  `json:"is_active"`
	Schedules map[string][]TimeSlot `json:"schedules"`
}

type Settings struct {
	PrimaryDNS      string `json:"primary_dns"`
	BackupDNS       string `json:"backup_dns"`
	AuthToken       string `json:"auth_token"`
	EnforcementMode string `json:"enforcement_mode,omitempty"`
	DNSFailureMode  string `json:"dns_failure_mode,omitempty"`
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

type Config struct {
	Settings Settings            `json:"settings"`
	Groups   map[string][]string `json:"groups"`
	Rules    []Rule              `json:"rules"`
	Pause    *PauseWindow        `json:"pause,omitempty"`
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
)

func GetConfigFilePath() (string, error) {
	var dir string
	if UseLocalConfig {
		dir = "."
	} else if runtime.GOOS == "darwin" {
		dir = "/Library/Application Support/Sentinel"
	} else if runtime.GOOS == "windows" {
		dir = filepath.Join(os.Getenv("PROGRAMDATA"), "Sentinel")
	} else {
		dir = "/etc/sentinel"
	}
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, "config.json"), nil
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
