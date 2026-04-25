package web

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/scheduler"
	"github.com/vsangava/distractions-free/internal/testcli"
)

//go:embed static/*
var webFiles embed.FS

// authMiddleware rejects requests to /api/* (except GET /api/config) without a valid X-Auth-Token.
// GET /api/config is intentionally public so the web UI can bootstrap the token on first load.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := config.GetConfig()
		if r.Header.Get("X-Auth-Token") != cfg.Settings.AuthToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// ConfigHandler is a testable handler that returns the current config as JSON.
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := config.GetConfig()
	json.NewEncoder(w).Encode(cfg)
}

// ValidatePostedConfig validates a posted config (exported for testing).
func ValidatePostedConfig(cfg config.Config) error {
	validDays := map[string]bool{
		"Monday":    true,
		"Tuesday":   true,
		"Wednesday": true,
		"Thursday":  true,
		"Friday":    true,
		"Saturday":  true,
		"Sunday":    true,
	}

	for _, rule := range cfg.Rules {
		if rule.Domain == "" {
			return errors.New("rule domain cannot be empty")
		}
		for day, slots := range rule.Schedules {
			if !validDays[day] {
				return errors.New("invalid schedule day: " + day)
			}
			if len(slots) == 0 {
				return errors.New("schedule for " + day + " must contain at least one timeslot")
			}
			for _, slot := range slots {
				if slot.Start == "" || slot.End == "" {
					return errors.New("schedule timeslot must include start and end")
				}
				if _, err := time.Parse("15:04", slot.Start); err != nil {
					return errors.New("invalid start time (use HH:MM, zero-padded): " + slot.Start)
				}
				if _, err := time.Parse("15:04", slot.End); err != nil {
					return errors.New("invalid end time (use HH:MM, zero-padded): " + slot.End)
				}
				if slot.Start >= slot.End {
					return errors.New("schedule timeslot start time must be before end time")
				}
			}
		}
	}
	return nil
}

// UpdateConfigHandler accepts a full config payload, validates it, writes it to disk, and reloads.
func UpdateConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var cfg config.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if err := ValidatePostedConfig(cfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	path, err := config.GetConfigFilePath()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "cannot resolve config path: " + err.Error()})
		return
	}

	// Preserve the existing auth token — callers cannot replace it via this endpoint.
	existing := config.GetConfig()
	cfg.Settings.AuthToken = existing.Settings.AuthToken

	data, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to write config: " + err.Error()})
		return
	}
	if err := config.LoadConfig(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "config written but reload failed: " + err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// StatusHandler returns the currently blocked domains and the last evaluation timestamp.
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	blocked, lastEval := scheduler.GetStatus()
	json.NewEncoder(w).Encode(map[string]any{
		"blocked_domains": blocked,
		"last_evaluated":  lastEval,
	})
}

// TestQueryHandler handles test queries for the web UI.
func TestQueryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Try to get config from POST body
	var cfg config.Config
	var cfgFromBody bool
	if r.Method == "POST" {
		if err := r.ParseMultipartForm(10 << 20); err != nil && err != http.ErrNotMultipart {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to parse form data: " + err.Error(),
			})
			return
		}

		configJSON := r.FormValue("config")
		if configJSON == "" && r.MultipartForm != nil {
			if values, ok := r.MultipartForm.Value["config"]; ok && len(values) > 0 {
				configJSON = values[0]
			}
		}

		if configJSON != "" {
			// Parse config from form
			if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Invalid config JSON: " + err.Error(),
				})
				return
			}
			if err := ValidatePostedConfig(cfg); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Invalid config: " + err.Error(),
				})
				return
			}
			cfgFromBody = true
		}
	}

	// Fallback to loaded config if not provided
	if !cfgFromBody {
		config.LoadConfig()
		cfg = config.GetConfig()
	}

	// Get query parameters
	timeStr := r.URL.Query().Get("time")
	domain := r.URL.Query().Get("domain")

	if timeStr == "" || domain == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Missing time or domain parameter",
		})
		return
	}

	// Get query result with provided config
	result := testcli.GetQueryResultWithConfig(timeStr, domain, cfg)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// TestPageHandler serves the test UI HTML page.
func TestPageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	page, err := webFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Failed to load test page: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(page)
}

// StaticFileHandler returns a handler for serving embedded static files.
func StaticFileHandler() (http.Handler, error) {
	fsys, err := fs.Sub(webFiles, "static")
	if err != nil {
		return nil, err
	}
	return http.FileServer(http.FS(fsys)), nil
}

func StartWebServer() {
	staticHandler, err := StaticFileHandler()
	if err != nil {
		log.Fatalf("Failed to load embedded web files: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", staticHandler)
	mux.HandleFunc("/api/config", ConfigHandler)
	mux.HandleFunc("/api/status", authMiddleware(StatusHandler))
	mux.HandleFunc("/api/test-query", authMiddleware(TestQueryHandler))
	mux.HandleFunc("/api/config/update", authMiddleware(UpdateConfigHandler))

	log.Println("Web server starting on http://localhost:8040")
	if err := http.ListenAndServe("127.0.0.1:8040", mux); err != nil {
		log.Fatalf("Web server failed: %v", err)
	}
}

// StartTestWebServer starts a web server dedicated to test queries.
func StartTestWebServer() {
	if err := config.LoadConfig(); err != nil {
		log.Printf("Config warning: %v", err)
	}

	staticHandler, err := StaticFileHandler()
	if err != nil {
		log.Fatalf("Failed to load embedded web files: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", staticHandler)
	mux.HandleFunc("/api/config", ConfigHandler)
	mux.HandleFunc("/api/status", authMiddleware(StatusHandler))
	mux.HandleFunc("/api/test-query", authMiddleware(TestQueryHandler))
	mux.HandleFunc("/api/config/update", authMiddleware(UpdateConfigHandler))

	log.Println("Test web server starting on http://localhost:8040")
	if err := http.ListenAndServe("127.0.0.1:8040", mux); err != nil {
		log.Fatalf("Test web server failed: %v", err)
	}
}
