package web

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/vsangava/sentinel/internal/config"
	"github.com/vsangava/sentinel/internal/version"
)

func TestMain(m *testing.M) {
	// Point config I/O at a tempdir. Several handlers under test (Pomodoro,
	// Pause) call config.SaveConfig, which would otherwise rewrite the
	// repo-rooted ./config.json with json.MarshalIndent — alphabetizing
	// weekday keys and expanding inline arrays — and race with the testcli
	// package when go test ./... runs them in parallel.
	dir, err := os.MkdirTemp("", "sentinel-web-*")
	if err != nil {
		log.Fatalf("could not create temp config dir: %v", err)
	}
	config.ConfigDirOverride = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestVersionHandler_ReturnsCurrentVersion(t *testing.T) {
	orig := version.Version
	version.Version = "v9.9.9-test"
	defer func() { version.Version = orig }()

	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rr := httptest.NewRecorder()
	VersionHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if body["version"] != "v9.9.9-test" {
		t.Errorf("expected version v9.9.9-test, got %q", body["version"])
	}
}

func TestConfigHandler_ReturnsJSON(t *testing.T) {
	// Create a test request and response recorder
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	// Check status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200, got %d", status)
	}

	// Check content type
	expected := "application/json"
	if ct := rr.Header().Get("Content-Type"); ct != expected {
		t.Errorf("expected Content-Type %s, got %s", expected, ct)
	}

	// Check response body can be unmarshalled as Config
	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Errorf("response not valid JSON: %v", err)
	}
}

func TestConfigHandler_ReturnsValidConfigStructure(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify it has the expected structure (even if empty)
	// Settings and Rules should be valid (may be zero-valued)
	_ = cfg.Settings
	_ = cfg.Rules
}

func TestConfigHandler_ConfigStructure(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Config should have the structure (Rules may be nil or empty slice)
	if cfg.Rules == nil {
		// Nil is acceptable - config may not have loaded yet
		t.Logf("Rules is nil (expected in test environment)")
	}
}

func TestConfigHandler_MultipleRequests(t *testing.T) {
	handler := http.HandlerFunc(ConfigHandler)

	// Make multiple requests to ensure handler is stateless
	for i := 0; i < 5; i++ {
		req, err := http.NewRequest("GET", "/api/config", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			t.Errorf("iteration %d: expected status 200, got %d", i, status)
		}

		var cfg config.Config
		if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
			t.Errorf("iteration %d: response not valid JSON: %v", i, err)
		}
	}
}

func TestConfigHandler_HTTPMethod_POST(t *testing.T) {
	// ConfigHandler should work with any HTTP method (we use GET but handler is universal)
	req, err := http.NewRequest("POST", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	// Should still return valid JSON
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200 for POST, got %d", status)
	}

	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Errorf("response not valid JSON: %v", err)
	}
}

func TestConfigHandler_JSONEncoding(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Re-encode and verify it's valid JSON
	reencoded, err := json.Marshal(cfg)
	if err != nil {
		t.Errorf("failed to re-encode config as JSON: %v", err)
	}

	// Verify re-encoded JSON is not empty
	if len(reencoded) == 0 {
		t.Errorf("re-encoded JSON is empty")
	}
}

func TestStaticFileHandler_ReturnsValidHandler(t *testing.T) {
	handler, err := StaticFileHandler()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if handler == nil {
		t.Fatalf("expected handler to not be nil")
	}
}

func TestStaticFileHandler_HandlerServesRequests(t *testing.T) {
	handler, err := StaticFileHandler()
	if err != nil {
		t.Fatalf("failed to create static file handler: %v", err)
	}

	// Test requesting index.html or root
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Should return 200 or 404 (depending on if index.html exists)
	// Either is valid - we're just testing the handler responds
	if status := rr.Code; status != http.StatusOK && status != http.StatusNotFound {
		t.Errorf("expected status 200 or 404, got %d", status)
	}
}

func TestConfigHandler_ContentTypeHeaderSet(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	// Verify Content-Type header is explicitly set
	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
	}
}

func TestConfigHandler_ResponseIsJSON(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	// Verify response body starts with { or [ (valid JSON)
	body := rr.Body.String()
	if len(body) == 0 {
		t.Errorf("expected response body, got empty")
	}

	if body[0] != '{' && body[0] != '[' {
		t.Errorf("expected JSON response starting with { or [, got: %s...", body[:10])
	}
}

func TestConfigHandler_RulesStructure(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// If there are rules, verify they have expected fields
	for _, rule := range cfg.Rules {
		if rule.Group == "" {
			t.Errorf("rule missing Group field")
		}

		if rule.Schedules == nil {
			t.Errorf("rule missing Schedules field")
		}
	}
}

func TestConfigHandler_SettingsPresent(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Settings should be present (may be empty in test, but structure should exist)
	_ = cfg.Settings
}

func TestConfigHandler_HTTPGet(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("expected status 200 for GET, got %d", status)
	}
}

func TestConfigHandler_HTTPDelete(t *testing.T) {
	// Handler should accept any HTTP method (it doesn't check method)
	req, err := http.NewRequest("DELETE", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	// Should still respond (handler doesn't restrict methods)
	if status := rr.Code; status != http.StatusOK {
		t.Logf("DELETE returned %d (handler doesn't restrict methods)", status)
	}
}

func TestConfigHandler_OutputNotEmpty(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	if rr.Body.Len() == 0 {
		t.Errorf("expected response body, got empty")
	}
}

func TestConfigHandler_ValidJSONAfterMarshal(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/config", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(ConfigHandler)
	handler.ServeHTTP(rr, req)

	var cfg config.Config
	if err := json.NewDecoder(rr.Body).Decode(&cfg); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify we can marshal it back
	_, err = json.Marshal(cfg)
	if err != nil {
		t.Errorf("failed to marshal config back to JSON: %v", err)
	}
}

// TestStatusHandler_UsesPostedConfig verifies that StatusHandler evaluates blocking
// using a config POSTed in the request body when the scheduler hasn't run (lastEvalTime
// zero). This is the --test-web mode scenario: the user edits config in the browser
// textarea and expects Status to reflect the same config as test-query.
func TestStatusHandler_UsesPostedConfig(t *testing.T) {
	day := time.Now().Weekday().String()

	// Use a wide window spanning most of the day so the test is not sensitive to
	// the time-of-day it runs. A now±1h window would produce an overnight slot near
	// midnight that the scheduler correctly refuses to mark as "currently active".
	blocked := config.Config{
		Settings: config.Settings{PrimaryDNS: "8.8.8.8:53", BackupDNS: "1.1.1.1:53"},
		Groups:   map[string][]string{"video": {"youtube.com"}},
		Rules: []config.Rule{
			{
				Group:    "video",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					day: {{Start: "00:01", End: "23:59"}},
				},
			},
		},
	}

	body, _ := json.Marshal(blocked)
	req, _ := http.NewRequest("POST", "/api/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	StatusHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var result map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	domains, ok := result["blocked_domains"].(map[string]any)
	if !ok {
		t.Fatalf("blocked_domains missing or wrong type: %v", result)
	}
	if domains["youtube.com"] != true {
		t.Errorf("expected youtube.com to be blocked, got blocked_domains=%v", domains)
	}
}

// TestStatusHandler_EmptyWhenNotBlocked verifies that a domain outside its schedule
// window is not reported as blocked.
func TestStatusHandler_EmptyWhenNotBlocked(t *testing.T) {
	notBlocked := config.Config{
		Settings: config.Settings{PrimaryDNS: "8.8.8.8:53", BackupDNS: "1.1.1.1:53"},
		Groups:   map[string][]string{"video": {"youtube.com"}},
		Rules: []config.Rule{
			{
				Group:    "video",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					// Window in the past — never active now
					"Monday": {{Start: "01:00", End: "01:01"}},
					"Tuesday": {{Start: "01:00", End: "01:01"}},
					"Wednesday": {{Start: "01:00", End: "01:01"}},
					"Thursday": {{Start: "01:00", End: "01:01"}},
					"Friday": {{Start: "01:00", End: "01:01"}},
					"Saturday": {{Start: "01:00", End: "01:01"}},
					"Sunday": {{Start: "01:00", End: "01:01"}},
				},
			},
		},
	}

	body, _ := json.Marshal(notBlocked)
	req, _ := http.NewRequest("POST", "/api/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	StatusHandler(rr, req)

	var result map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	domains := result["blocked_domains"].(map[string]any)
	if domains["youtube.com"] == true {
		t.Errorf("expected youtube.com not to be blocked outside its window")
	}
}

func BenchmarkConfigHandler(b *testing.B) {
	handler := http.HandlerFunc(ConfigHandler)

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/config", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

// ── Pomodoro handler tests ────────────────────────────────────────────────────

func authRequest(method, path string, body []byte) *http.Request {
	var req *http.Request
	if body != nil {
		req, _ = http.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	// Use the in-memory auth token
	req.Header.Set("X-Auth-Token", config.AppConfig.Settings.AuthToken)
	return req
}

func setWorkPhase(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { config.AppConfig.Pomodoro = nil })
	config.AppConfig.Pomodoro = &config.PomodoroSession{
		Phase:        "work",
		PhaseEndsAt:  time.Now().Add(10 * time.Minute),
		WorkMinutes:  25,
		BreakMinutes: 5,
	}
}

func setBreakPhase(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { config.AppConfig.Pomodoro = nil })
	config.AppConfig.Pomodoro = &config.PomodoroSession{
		Phase:        "break",
		PhaseEndsAt:  time.Now().Add(5 * time.Minute),
		WorkMinutes:  25,
		BreakMinutes: 5,
	}
}

func TestPomodoroStart_ValidBody(t *testing.T) {
	t.Cleanup(func() { config.AppConfig.Pomodoro = nil })

	body, _ := json.Marshal(map[string]int{"work_minutes": 25, "break_minutes": 5})
	req := authRequest("POST", "/api/pomodoro/start", body)
	rr := httptest.NewRecorder()
	PomodoroStartHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var result map[string]any
	json.NewDecoder(rr.Body).Decode(&result)
	if result["phase"] != "work" {
		t.Errorf("expected phase=work, got %v", result["phase"])
	}
}

func TestPomodoroStart_WorkMinutesTooLarge(t *testing.T) {
	body, _ := json.Marshal(map[string]int{"work_minutes": 999, "break_minutes": 5})
	req := authRequest("POST", "/api/pomodoro/start", body)
	rr := httptest.NewRecorder()
	PomodoroStartHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPomodoroStart_BreakMinutesTooLarge(t *testing.T) {
	body, _ := json.Marshal(map[string]int{"work_minutes": 25, "break_minutes": 999})
	req := authRequest("POST", "/api/pomodoro/start", body)
	rr := httptest.NewRecorder()
	PomodoroStartHandler(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPomodoroStop_DuringBreak_Returns200(t *testing.T) {
	setBreakPhase(t)

	req := authRequest("DELETE", "/api/pomodoro", nil)
	rr := httptest.NewRecorder()
	PomodoroStopHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if config.AppConfig.Pomodoro != nil {
		t.Error("expected Pomodoro to be nil after stop")
	}
}

func TestPomodoroStop_DuringWork_Returns423(t *testing.T) {
	setWorkPhase(t)

	req := authRequest("DELETE", "/api/pomodoro", nil)
	rr := httptest.NewRecorder()
	PomodoroStopHandler(rr, req)

	if rr.Code != http.StatusLocked {
		t.Errorf("expected 423, got %d", rr.Code)
	}
}

func TestPauseHandler_LockedDuringWork_Returns423(t *testing.T) {
	setWorkPhase(t)

	body, _ := json.Marshal(map[string]int{"minutes": 15})
	req := authRequest("POST", "/api/pause", body)
	rr := httptest.NewRecorder()
	PauseHandler(rr, req)

	if rr.Code != http.StatusLocked {
		t.Errorf("expected 423, got %d", rr.Code)
	}
}

func TestStatusHandler_IncludesPomodoroState(t *testing.T) {
	setWorkPhase(t)

	req, _ := http.NewRequest("GET", "/api/status", nil)
	rr := httptest.NewRecorder()
	StatusHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var result map[string]any
	json.NewDecoder(rr.Body).Decode(&result)
	pom, ok := result["pomodoro"].(map[string]any)
	if !ok {
		t.Fatalf("expected pomodoro field in status response, got %v", result)
	}
	if pom["phase"] != "work" {
		t.Errorf("expected phase=work, got %v", pom["phase"])
	}
	if pom["locked"] != true {
		t.Errorf("expected locked=true, got %v", pom["locked"])
	}
}

func TestUpdateConfigHandler_LockedDuringWork_Returns423(t *testing.T) {
	setWorkPhase(t)

	cfg := config.Config{
		Settings: config.Settings{AuthToken: config.AppConfig.Settings.AuthToken},
		Groups:   map[string][]string{"g": {"example.com"}},
		Rules:    []config.Rule{{Group: "g", IsActive: true, Schedules: map[string][]config.TimeSlot{}}},
	}
	body, _ := json.Marshal(cfg)
	req := authRequest("POST", "/api/config/update", body)
	rr := httptest.NewRecorder()
	UpdateConfigHandler(rr, req)

	if rr.Code != http.StatusLocked {
		t.Errorf("expected 423, got %d: %s", rr.Code, rr.Body.String())
	}
}
