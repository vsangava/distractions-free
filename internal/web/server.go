package web

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/enforcer"
	"github.com/vsangava/sentinel/internal/pf"
	"github.com/vsangava/sentinel/internal/proxy"
	"github.com/vsangava/sentinel/internal/scheduler"
	"github.com/vsangava/sentinel/internal/testcli"
)

const maxPauseMinutes = 240 // 4 hours

const (
	minPomodoroWork  = 1
	maxPomodoroWork  = 120
	minPomodoroBreak = 1
	maxPomodoroBreak = 60
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
	if existing.IsLockedByPomodoro(time.Now()) {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(map[string]string{"error": "config changes are locked during a Pomodoro work session"})
		return
	}
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
		blocked = scheduler.EvaluateRulesAtTime(time.Now(), cfg, nil)
		lastEval = time.Now()
	}
	cfg := config.GetConfig()
	now := time.Now()
	resp := map[string]any{
		"blocked_domains":  blocked,
		"last_evaluated":   lastEval,
		"enforcement_mode": cfg.Settings.GetEnforcementMode(),
		"paused":           cfg.IsPaused(now),
	}
	if cfg.Pause != nil {
		resp["paused_until"] = cfg.Pause.Until.Format(time.RFC3339)
	}
	if cfg.Pomodoro != nil {
		resp["pomodoro"] = map[string]any{
			"phase":         cfg.Pomodoro.Phase,
			"phase_ends_at": cfg.Pomodoro.PhaseEndsAt.Format(time.RFC3339),
			"work_minutes":  cfg.Pomodoro.WorkMinutes,
			"break_minutes": cfg.Pomodoro.BreakMinutes,
			"locked":        cfg.IsLockedByPomodoro(now),
		}
	}

	// Include quota usage for any rule that has daily_quota_minutes set.
	quotaRules := quotaRulesFromConfig(cfg)
	if len(quotaRules) > 0 {
		dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		events, _ := proxy.ReadUsageEventsSince(dayStart.Add(-time.Second))
		quotas := make([]map[string]any, 0, len(quotaRules))
		for _, rule := range quotaRules {
			used := proxy.ComputeGroupUsageMinutes(events, rule.Group, now)
			quotas = append(quotas, map[string]any{
				"group":            rule.Group,
				"quota_minutes":    rule.DailyQuotaMinutes,
				"used_minutes":     used,
				"quota_exceeded":   used >= rule.DailyQuotaMinutes,
				"mode_compatible":  cfg.Settings.GetEnforcementMode() != "hosts",
			})
		}
		resp["quotas"] = quotas
	}

	json.NewEncoder(w).Encode(resp)
}

// quotaRulesFromConfig returns rules with a positive daily_quota_minutes.
func quotaRulesFromConfig(cfg config.Config) []config.Rule {
	var out []config.Rule
	for _, r := range cfg.Rules {
		if r.IsActive && r.DailyQuotaMinutes > 0 {
			out = append(out, r)
		}
	}
	return out
}

// PauseHandler handles POST /api/pause.
// Body: {"minutes": N} where N must be between 1 and maxPauseMinutes.
func PauseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if cfg := config.GetConfig(); cfg.IsLockedByPomodoro(time.Now()) {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(map[string]string{"error": "blocking is locked during a Pomodoro work session"})
		return
	}

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

// PomodoroStartHandler handles POST /api/pomodoro/start.
// Body: {"work_minutes": N, "break_minutes": M}
func PomodoroStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		WorkMinutes  int `json:"work_minutes"`
		BreakMinutes int `json:"break_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if body.WorkMinutes < minPomodoroWork || body.WorkMinutes > maxPomodoroWork {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("work_minutes must be between %d and %d", minPomodoroWork, maxPomodoroWork),
		})
		return
	}
	if body.BreakMinutes < minPomodoroBreak || body.BreakMinutes > maxPomodoroBreak {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("break_minutes must be between %d and %d", minPomodoroBreak, maxPomodoroBreak),
		})
		return
	}

	config.StartPomodoro(body.WorkMinutes, body.BreakMinutes)
	if err := config.SaveConfig(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to save config: " + err.Error()})
		return
	}

	cfg := config.GetConfig()
	json.NewEncoder(w).Encode(map[string]any{
		"status":        "ok",
		"phase":         cfg.Pomodoro.Phase,
		"phase_ends_at": cfg.Pomodoro.PhaseEndsAt.Format(time.RFC3339),
	})
}

// PomodoroStopHandler handles DELETE /api/pomodoro.
// Returns 423 if currently in work phase (lock-down active).
func PomodoroStopHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	cfg := config.GetConfig()
	if cfg.IsLockedByPomodoro(time.Now()) {
		w.WriteHeader(http.StatusLocked)
		json.NewEncoder(w).Encode(map[string]string{"error": "cannot stop session during a work phase"})
		return
	}

	config.ClearPomodoro()
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

	blocked := scheduler.EvaluateRulesAtTime(time.Now(), cfg, nil)

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

	blocked := scheduler.EvaluateRulesAtTime(time.Now(), cfg, nil)
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

// UsageHandler handles GET /api/usage.
// Query params:
//   - range: "today" (default) | "7d" | "30d" | "60d"
//
// Returns per-group and per-domain usage minutes for the requested range,
// bucketed in 5-minute windows to avoid counting idle DNS re-resolution.
func UsageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	now := time.Now()
	var since time.Time
	rangeParam := r.URL.Query().Get("range")
	switch rangeParam {
	case "7d":
		since = now.AddDate(0, 0, -7)
	case "30d":
		since = now.AddDate(0, 0, -30)
	case "60d":
		since = now.AddDate(0, 0, -60)
	default:
		// "today" — from start of calendar day
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-time.Second)
	}

	events, err := proxy.ReadUsageEventsSince(since)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	cfg := config.GetConfig()

	// Per-group totals.
	type groupRow struct {
		Group        string `json:"group"`
		UsedMinutes  int    `json:"used_minutes"`
		QuotaMinutes int    `json:"quota_minutes,omitempty"`
	}
	groupMap := make(map[string]*groupRow)
	for _, rule := range cfg.Rules {
		if _, ok := groupMap[rule.Group]; !ok {
			groupMap[rule.Group] = &groupRow{Group: rule.Group, QuotaMinutes: rule.DailyQuotaMinutes}
		}
	}

	// Per-domain totals using 5-min bucket deduplication.
	type domainRow struct {
		Domain      string `json:"domain"`
		Group       string `json:"group"`
		UsedMinutes int    `json:"used_minutes"`
	}
	domainBuckets := make(map[string]map[int64]struct{}) // domain → set of bucket keys
	groupBuckets := make(map[string]map[int64]struct{})  // group → set of bucket keys

	for _, e := range events {
		bk := e.TS.Unix() / 300
		if domainBuckets[e.Domain] == nil {
			domainBuckets[e.Domain] = make(map[int64]struct{})
		}
		domainBuckets[e.Domain][bk] = struct{}{}
		if groupBuckets[e.Group] == nil {
			groupBuckets[e.Group] = make(map[int64]struct{})
		}
		groupBuckets[e.Group][bk] = struct{}{}
	}

	// Populate group rows.
	for group, buckets := range groupBuckets {
		if row, ok := groupMap[group]; ok {
			row.UsedMinutes = len(buckets) * 5
		} else {
			groupMap[group] = &groupRow{Group: group, UsedMinutes: len(buckets) * 5}
		}
	}
	groups := make([]groupRow, 0, len(groupMap))
	for _, row := range groupMap {
		if row.UsedMinutes > 0 || row.QuotaMinutes > 0 {
			groups = append(groups, *row)
		}
	}

	// Build per-domain rows.
	domainRows := make([]domainRow, 0, len(domainBuckets))
	// Build domain→group from config for labelling.
	gl := scheduler.BuildGroupLookup(cfg)
	for domain, buckets := range domainBuckets {
		domainRows = append(domainRows, domainRow{
			Domain:      domain,
			Group:       gl[domain],
			UsedMinutes: len(buckets) * 5,
		})
	}

	json.NewEncoder(w).Encode(map[string]any{
		"range":   rangeParam,
		"groups":  groups,
		"domains": domainRows,
	})
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
	mux.HandleFunc("/api/pomodoro/start", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		PomodoroStartHandler(w, r)
	}))
	mux.HandleFunc("/api/pomodoro", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		PomodoroStopHandler(w, r)
	}))
	mux.HandleFunc("/api/events", authMiddleware(EventsHandler))
	mux.HandleFunc("/api/usage", authMiddleware(UsageHandler))

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
	mux.HandleFunc("/api/pomodoro/start", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		PomodoroStartHandler(w, r)
	}))
	mux.HandleFunc("/api/pomodoro", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		PomodoroStopHandler(w, r)
	}))
	mux.HandleFunc("/api/events", authMiddleware(EventsHandler))
	mux.HandleFunc("/api/usage", authMiddleware(UsageHandler))

	log.Println("Test web server starting on http://localhost:8040")
	if err := http.ListenAndServe("127.0.0.1:8040", mux); err != nil {
		log.Fatalf("Test web server failed: %v", err)
	}
}
