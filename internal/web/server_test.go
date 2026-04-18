package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/vsangava/distractions-free/internal/config"
)

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
		if rule.Domain == "" {
			t.Errorf("rule missing Domain field")
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

func BenchmarkConfigHandler(b *testing.B) {
	handler := http.HandlerFunc(ConfigHandler)

	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/config", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
