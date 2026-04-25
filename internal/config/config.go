package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type TimeSlot struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type Rule struct {
	Domain    string                `json:"domain"`
	IsActive  bool                  `json:"is_active"`
	Schedules map[string][]TimeSlot `json:"schedules"`
}

type Settings struct {
	PrimaryDNS      string `json:"primary_dns"`
	BackupDNS       string `json:"backup_dns"`
	AuthToken       string `json:"auth_token"`
	EnforcementMode string `json:"enforcement_mode,omitempty"`
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

type Config struct {
	Settings Settings `json:"settings"`
	Rules    []Rule   `json:"rules"`
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
		dir = "/Library/Application Support/DistractionsFree"
	} else if runtime.GOOS == "windows" {
		dir = filepath.Join(os.Getenv("PROGRAMDATA"), "DistractionsFree")
	} else {
		dir = "/etc/distractionsfree"
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
	token, err := generateToken()
	if err != nil {
		return err
	}
	AppConfig = Config{
		Settings: Settings{
			PrimaryDNS:      "8.8.8.8:53",
			BackupDNS:       "1.1.1.1:53",
			AuthToken:       token,
			EnforcementMode: "hosts",
		},
		Rules: []Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]TimeSlot{
					"Monday":    {{Start: "09:00", End: "17:00"}},
					"Tuesday":   {{Start: "09:00", End: "17:00"}},
					"Wednesday": {{Start: "09:00", End: "17:00"}},
					"Thursday":  {{Start: "09:00", End: "17:00"}},
					"Friday":    {{Start: "09:00", End: "17:00"}},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(AppConfig, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return AppConfig
}
