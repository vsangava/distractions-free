package web

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/enforcer"
	"github.com/vsangava/sentinel/internal/pf"
	"github.com/vsangava/sentinel/internal/scheduler"
	"github.com/vsangava/sentinel/internal/testcli"
)

const maxPauseMinutes = 240 // 4 hours

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

// ConfigHandler returns the current config as JSON, reloading from disk first so the
// browser always sees the latest file state rather than the startup snapshot.
func ConfigHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := config.LoadConfig(); err != nil {
		log.Printf("config reload warning: %v", err)
	}
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

	if mode := cfg.Settings.EnforcementMode; mode != "" {
		switch mode {
		case "hosts", "dns", "strict":
		default:
			return errors.New("invalid enforcement_mode: must be 'hosts', 'dns', or 'strict'")
		}
	}

	for groupName, domains := range cfg.Groups {
		if groupName == "" {
			return errors.New("group name cannot be empty")
		}
		if len(domains) == 0 {
			return errors.New("group '" + groupName + "' must contain at least one domain")
		}
		for _, d := range domains {
			if d == "" {
				return errors.New("group '" + groupName + "' contains an empty domain")
			}
		}
	}

	for _, rule := range cfg.Rules {
		if rule.Group == "" {
			return errors.New("rule group cannot be empty")
		}
		if _, ok := cfg.Groups[rule.Group]; !ok {
			return errors.New("rule references unknown group: " + rule.Group)
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
// When the scheduler is not running (e.g. --test-web mode), it evaluates rules at
// time.Now() using whichever config the caller POSTs (same as test-query), falling
// back to a fresh disk reload if no body is provided.
func StatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	blocked, lastEval := scheduler.GetStatus()
	if lastEval.IsZero() {
		cfg := resolveConfig(r)
		blocked = scheduler.EvaluateRulesAtTime(time.Now(), cfg)
		lastEval = time.Now()
	}
	cfg := config.GetConfig()
	resp := map[string]any{
		"blocked_domains":  blocked,
		"last_evaluated":   lastEval,
		"enforcement_mode": cfg.Settings.GetEnforcementMode(),
		"paused":           cfg.IsPaused(time.Now()),
	}
	if cfg.Pause != nil {
		resp["paused_until"] = cfg.Pause.Until.Format(time.RFC3339)
	}
	json.NewEncoder(w).Encode(resp)
}

// PauseHandler handles POST /api/pause.
// Body: {"minutes": N} where N must be between 1 and maxPauseMinutes.
func PauseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Minutes int `json:"minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if body.Minutes <= 0 || body.Minutes > maxPauseMinutes {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "minutes must be between 1 and 240"})
		return
	}

	until := time.Now().Add(time.Duration(body.Minutes) * time.Minute)
	config.SetPause(until)
	if err := config.SaveConfig(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"status":      "ok",
		"paused_until": until.Format(time.RFC3339),
	})
}

// ResumeHandler handles DELETE /api/pause, clearing any active pause immediately.
func ResumeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	config.ClearPause()
	if err := config.SaveConfig(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// resolveConfig extracts a config from the request body if present, otherwise reloads
// from disk. Used by endpoints that need the "current" config in --test-web mode.
func resolveConfig(r *http.Request) config.Config {
	if r.Body != nil && r.Header.Get("Content-Type") == "application/json" {
		var posted config.Config
		if err := json.NewDecoder(r.Body).Decode(&posted); err == nil {
			return posted
		}
	}
	if err := config.LoadConfig(); err != nil {
		log.Printf("config reload warning: %v", err)
	}
	return config.GetConfig()
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

// HostsPreviewHandler evaluates which domains are currently blocked under the
// posted config (or the disk config if no body is provided) and returns the
// /etc/hosts entries that would be written by HostsEnforcer — without touching
// any file on the system. Useful for testing enforcement logic in --test-web mode.
func HostsPreviewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := resolveConfig(r)
	mode := cfg.Settings.GetEnforcementMode()

	blocked := scheduler.EvaluateRulesAtTime(time.Now(), cfg)

	var domains []string
	for d := range blocked {
		domains = append(domains, d)
	}

	entries := enforcer.GenerateHostsEntries(domains)

	json.NewEncoder(w).Encode(map[string]any{
		"enforcement_mode": mode,
		"blocked_domains":  domains,
		"hosts_entries":    entries,
		"evaluated_at":     time.Now(),
	})
}

// PFPreviewHandler evaluates blocked domains under the posted (or disk) config and returns
// resolved IPs + anchor content that strict mode would load into pf — without touching pf.
func PFPreviewHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	cfg := resolveConfig(r)

	if cfg.Settings.GetEnforcementMode() != "strict" {
		json.NewEncoder(w).Encode(map[string]string{"error": "pf-preview is only relevant for strict mode"})
		return
	}

	blocked := scheduler.EvaluateRulesAtTime(time.Now(), cfg)
	var domains []string
	for d := range blocked {
		domains = append(domains, d)
	}

	dnsServer := cfg.Settings.PrimaryDNS
	if dnsServer == "" {
		dnsServer = "8.8.8.8:53"
	}

	preview := pf.GeneratePreview(domains, dnsServer)
	json.NewEncoder(w).Encode(preview)
}

func EventsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var since time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		var err error
		since, err = time.Parse(time.RFC3339, s)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid since: " + err.Error()})
			return
		}
	}

	limit := 200
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	events, err := scheduler.ReadEvents(since, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(events)
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
	mux.HandleFunc("/api/hosts-preview", authMiddleware(HostsPreviewHandler))
	mux.HandleFunc("/api/pf-preview", authMiddleware(PFPreviewHandler))
	mux.HandleFunc("/api/pause", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			PauseHandler(w, r)
		case http.MethodDelete:
			ResumeHandler(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/events", authMiddleware(EventsHandler))

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
	mux.HandleFunc("/api/hosts-preview", authMiddleware(HostsPreviewHandler))
	mux.HandleFunc("/api/pf-preview", authMiddleware(PFPreviewHandler))
	mux.HandleFunc("/api/pause", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			PauseHandler(w, r)
		case http.MethodDelete:
			ResumeHandler(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	mux.HandleFunc("/api/events", authMiddleware(EventsHandler))

	log.Println("Test web server starting on http://localhost:8040")
	if err := http.ListenAndServe("127.0.0.1:8040", mux); err != nil {
		log.Fatalf("Test web server failed: %v", err)
	}
}
