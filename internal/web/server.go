package web

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"

	"github.com/vsangava/distractions-free/internal/config"
	"github.com/vsangava/distractions-free/internal/testcli"
)

//go:embed static/*
var webFiles embed.FS

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
				if slot.Start >= slot.End {
					return errors.New("schedule timeslot start time must be before end time")
				}
			}
		}
	}
	return nil
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

	http.Handle("/", staticHandler)
	http.HandleFunc("/api/config", ConfigHandler)
	http.HandleFunc("/api/test-query", TestQueryHandler)

	log.Println("Web server starting on http://localhost:8040")
	if err := http.ListenAndServe("127.0.0.1:8040", nil); err != nil {
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

	http.Handle("/", staticHandler)
	http.HandleFunc("/api/config", ConfigHandler)
	http.HandleFunc("/api/test-query", TestQueryHandler)

	log.Println("Test web server starting on http://localhost:8040")
	if err := http.ListenAndServe("127.0.0.1:8040", nil); err != nil {
		log.Fatalf("Test web server failed: %v", err)
	}
}
