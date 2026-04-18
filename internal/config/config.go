package config

import (
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
	PrimaryDNS string `json:"primary_dns"`
	BackupDNS  string `json:"backup_dns"`
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
	return json.Unmarshal(data, &AppConfig)
}

func saveDefaultConfig(path string) error {
	AppConfig = Config{
		Settings: Settings{
			PrimaryDNS: "8.8.8.8:53",
			BackupDNS:  "1.1.1.1:53",
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
