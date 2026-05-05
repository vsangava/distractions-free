package testcli

import (
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vsangava/sentinel/internal/config"
)

func TestMain(m *testing.M) {
	// Point config I/O at a tempdir so the test run doesn't read or write
	// the repo-rooted ./config.json. Without this, LoadConfig + SaveConfig
	// (e.g. via the Pomodoro/Pause handlers under test) would re-marshal
	// the on-disk file — alphabetizing weekday keys, expanding inline
	// arrays — and the testcli + web packages would race when go test ./...
	// runs them in parallel.
	dir, err := os.MkdirTemp("", "sentinel-testcli-*")
	if err != nil {
		log.Fatalf("could not create temp config dir: %v", err)
	}
	config.ConfigDirOverride = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestQueryBlocking_ValidTimeFormat(t *testing.T) {
	config.UseLocalConfig = true
	err := QueryBlocking("2024-04-01 10:30", "google.com")
	if err != nil {
		t.Errorf("Expected success for valid time format, got error: %v", err)
	}
}

func TestQueryBlocking_InvalidTimeFormat(t *testing.T) {
	err := QueryBlocking("2024-04-01", "google.com")
	if err == nil {
		t.Error("Expected error for invalid time format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid time format") {
		t.Errorf("Expected 'invalid time format' error, got: %v", err)
	}
}

func TestQueryBlocking_EmptyDomain(t *testing.T) {
	err := QueryBlocking("2024-04-01 10:30", "")
	if err == nil {
		t.Error("Expected error for empty domain, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Expected 'empty' error, got: %v", err)
	}
}

func TestQueryBlocking_DomainNormalization(t *testing.T) {
	config.UseLocalConfig = true
	// Should not error on domain with trailing dot
	err := QueryBlocking("2024-04-01 10:30", "google.com.")
	if err != nil {
		t.Errorf("Expected success for domain with trailing dot, got error: %v", err)
	}
}

func TestQueryBlocking_InvalidTimeValidity(t *testing.T) {
	// Test that it parses even though April 32 doesn't exist
	// time.Parse should handle this
	err := QueryBlocking("2024-04-32 10:30", "google.com")
	// time.Parse will error on invalid day
	if err == nil {
		t.Error("Expected error for invalid day in date")
	}
}

func TestQueryBlocking_Midnight(t *testing.T) {
	config.UseLocalConfig = true
	err := QueryBlocking("2024-04-01 00:00", "google.com")
	if err != nil {
		t.Errorf("Expected success for midnight time, got error: %v", err)
	}
}

func TestQueryBlocking_EndOfDay(t *testing.T) {
	config.UseLocalConfig = true
	err := QueryBlocking("2024-04-01 23:59", "google.com")
	if err != nil {
		t.Errorf("Expected success for end-of-day time, got error: %v", err)
	}
}

func TestQueryBlocking_AllWeekdays(t *testing.T) {
	config.UseLocalConfig = true
	// Test all 7 days of the week (April 1-7, 2024)
	days := []string{
		"2024-04-01", // Monday
		"2024-04-02", // Tuesday
		"2024-04-03", // Wednesday
		"2024-04-04", // Thursday
		"2024-04-05", // Friday
		"2024-04-06", // Saturday
		"2024-04-07", // Sunday
	}

	for i, day := range days {
		err := QueryBlocking(day+" 10:30", "google.com")
		if err != nil {
			t.Errorf("Expected success for day %d, got error: %v", i, err)
		}
	}
}

func TestQueryBlocking_TimeFormatValidation(t *testing.T) {
	// Test incorrect format variations
	tests := []struct {
		name    string
		timeStr string
		domain  string
	}{
		{"Missing minute", "2024-04-01 10", "google.com"},
		{"Wrong separator", "2024-04-01T10:30", "google.com"},
		{"Missing date", "10:30", "google.com"},
		{"Reversed order", "10:30 2024-04-01", "google.com"},
		{"Text month", "2024-April-01 10:30", "google.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := QueryBlocking(tt.timeStr, tt.domain)
			if err == nil {
				t.Error("Expected error for invalid format")
			}
		})
	}
}

func TestQueryBlocking_DomainVariants(t *testing.T) {
	config.UseLocalConfig = true
	domains := []string{
		"google.com",
		"www.google.com",
		"mail.google.com",
		"localhost",
		"example.org",
	}

	for _, domain := range domains {
		err := QueryBlocking("2024-04-01 10:30", domain)
		if err != nil {
			t.Errorf("Expected success for domain %s, got error: %v", domain, err)
		}
	}
}

func TestTimeFormatConstant(t *testing.T) {
	// Verify the timeFormat constant matches what we document
	if timeFormat != "2006-01-02 15:04" {
		t.Errorf("Expected timeFormat to be '2006-01-02 15:04', got '%s'", timeFormat)
	}

	// Verify parsing works with the format
	_, err := time.Parse(timeFormat, "2024-04-01 10:30")
	if err != nil {
		t.Errorf("Expected timeFormat to parse '2024-04-01 10:30', got error: %v", err)
	}
}

func TestContainsHelper(t *testing.T) {
	// Test the contains helper function
	slice := []string{"youtube.com", "facebook.com", "twitter.com"}

	tests := []struct {
		name     string
		slice    []string
		element  string
		expected bool
	}{
		{"Found first", slice, "youtube.com", true},
		{"Found middle", slice, "facebook.com", true},
		{"Found last", slice, "twitter.com", true},
		{"Not found", slice, "google.com", false},
		{"Empty slice", []string{}, "youtube.com", false},
		{"Empty element", slice, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.element)
			if result != tt.expected {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.element, result, tt.expected)
			}
		})
	}
}
