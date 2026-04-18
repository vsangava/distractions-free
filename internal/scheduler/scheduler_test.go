package scheduler

import (
	"testing"
	"time"

	"github.com/vsangava/distractions-free/internal/config"
)

func TestEvaluateRulesAtTime_NoActiveRules(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{},
	}

	testTime := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if len(result) != 0 {
		t.Errorf("expected empty blocked domains, got %v", result)
	}
}

func TestEvaluateRulesAtTime_InactiveRule(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: false,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {
						{Start: "09:00", End: "17:00"},
					},
				},
			},
		},
	}

	// Monday 10:30 (should be blocked if rule was active)
	testTime := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if len(result) != 0 {
		t.Errorf("expected inactive rule to not block, got %v", result)
	}
}

func TestEvaluateRulesAtTime_DomainBlockedDuringSchedule(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {
						{Start: "09:00", End: "17:00"},
					},
				},
			},
		},
	}

	// Monday 10:30 (within block time)
	testTime := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["youtube.com"] {
		t.Errorf("expected youtube.com to be blocked at 10:30 on Monday, got %v", result)
	}
}

func TestEvaluateRulesAtTime_DomainNotBlockedOutsideSchedule(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {
						{Start: "09:00", End: "17:00"},
					},
				},
			},
		},
	}

	// Monday 18:30 (after block time)
	testTime := time.Date(2024, time.April, 1, 18, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["youtube.com"] {
		t.Errorf("expected youtube.com to NOT be blocked at 18:30, got %v", result)
	}
}

func TestEvaluateRulesAtTime_DomainNotBlockedWrongDay(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {
						{Start: "09:00", End: "17:00"},
					},
				},
			},
		},
	}

	// Tuesday 10:30 (different day, no schedule)
	testTime := time.Date(2024, time.April, 2, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["youtube.com"] {
		t.Errorf("expected youtube.com to NOT be blocked on Tuesday, got %v", result)
	}
}

func TestEvaluateRulesAtTime_BlockedAtExactStartTime(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "reddit.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Wednesday": {
						{Start: "14:00", End: "15:00"},
					},
				},
			},
		},
	}

	// Wednesday 14:00 (exact start time)
	testTime := time.Date(2024, time.April, 3, 14, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["reddit.com"] {
		t.Errorf("expected reddit.com to be blocked at exact start time, got %v", result)
	}
}

func TestEvaluateRulesAtTime_NotBlockedAtExactEndTime(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "twitter.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Friday": {
						{Start: "09:00", End: "17:00"},
					},
				},
			},
		},
	}

	// Friday 17:00 (exact end time, should NOT be blocked)
	testTime := time.Date(2024, time.April, 5, 17, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["twitter.com"] {
		t.Errorf("expected twitter.com to NOT be blocked at exact end time, got %v", result)
	}
}

func TestEvaluateRulesAtTime_MultipleDomainsMultipleSchedules(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "09:00", End: "17:00"}},
				},
			},
			{
				Domain:   "reddit.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "09:00", End: "12:00"}},
				},
			},
			{
				Domain:   "twitter.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "14:00", End: "17:00"}},
				},
			},
		},
	}

	// Monday 10:30
	testTime := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["youtube.com"] {
		t.Errorf("expected youtube.com to be blocked")
	}
	if !result["reddit.com"] {
		t.Errorf("expected reddit.com to be blocked")
	}
	if result["twitter.com"] {
		t.Errorf("expected twitter.com to NOT be blocked at 10:30")
	}
}

func TestEvaluateRulesAtTime_MultipleTimeSlotsPerDay(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {
						{Start: "09:00", End: "12:00"},
						{Start: "14:00", End: "17:00"},
					},
				},
			},
		},
	}

	// Monday 10:30 (first slot)
	testTime1 := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result1 := EvaluateRulesAtTime(testTime1, cfg)
	if !result1["youtube.com"] {
		t.Errorf("expected youtube.com to be blocked in first slot")
	}

	// Monday 13:00 (between slots)
	testTime2 := time.Date(2024, time.April, 1, 13, 0, 0, 0, time.UTC)
	result2 := EvaluateRulesAtTime(testTime2, cfg)
	if result2["youtube.com"] {
		t.Errorf("expected youtube.com to NOT be blocked between slots")
	}

	// Monday 15:00 (second slot)
	testTime3 := time.Date(2024, time.April, 1, 15, 0, 0, 0, time.UTC)
	result3 := EvaluateRulesAtTime(testTime3, cfg)
	if !result3["youtube.com"] {
		t.Errorf("expected youtube.com to be blocked in second slot")
	}
}

func TestCheckWarningDomainsAtTime_WarningTriggersAt3MinBefore(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {
						{Start: "10:00", End: "12:00"},
					},
				},
			},
		},
	}

	// Monday 09:57 (3 minutes before 10:00)
	testTime := time.Date(2024, time.April, 1, 9, 57, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) == 0 {
		t.Errorf("expected warning for youtube.com at 09:57, got none")
	}
	if len(warnings) > 0 && warnings[0] != "youtube.com" {
		t.Errorf("expected youtube.com in warnings, got %v", warnings)
	}
}

func TestCheckWarningDomainsAtTime_NoWarningOutsideWindow(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "reddit.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Tuesday": {
						{Start: "14:00", End: "16:00"},
					},
				},
			},
		},
	}

	// Tuesday 13:54 (more than 3 minutes before)
	testTime := time.Date(2024, time.April, 2, 13, 54, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) != 0 {
		t.Errorf("expected no warning at 13:54, got %v", warnings)
	}
}

func TestCheckWarningDomainsAtTime_NoWarningForInactiveRule(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "twitter.com",
				IsActive: false,
				Schedules: map[string][]config.TimeSlot{
					"Wednesday": {
						{Start: "11:00", End: "13:00"},
					},
				},
			},
		},
	}

	// Wednesday 10:57 (3 minutes before, but rule inactive)
	testTime := time.Date(2024, time.April, 3, 10, 57, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) != 0 {
		t.Errorf("expected no warning for inactive rule, got %v", warnings)
	}
}

func TestCheckWarningDomainsAtTime_MultipleWarnings(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Thursday": {
						{Start: "09:00", End: "10:00"},
					},
				},
			},
			{
				Domain:   "reddit.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Thursday": {
						{Start: "09:00", End: "10:00"},
					},
				},
			},
		},
	}

	// Thursday 08:57 (3 minutes before 09:00)
	testTime := time.Date(2024, time.April, 4, 8, 57, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}
}

func TestEvaluateRulesAtTime_AllWeekdaySchedules(t *testing.T) {
	weekdays := []struct {
		day       string
		dayOfWeek int
	}{
		{"Monday", 1},
		{"Tuesday", 2},
		{"Wednesday", 3},
		{"Thursday", 4},
		{"Friday", 5},
		{"Saturday", 6},
		{"Sunday", 7},
	}

	for _, wd := range weekdays {
		cfg := config.Config{
			Rules: []config.Rule{
				{
					Domain:   "youtube.com",
					IsActive: true,
					Schedules: map[string][]config.TimeSlot{
						wd.day: {
							{Start: "09:00", End: "17:00"},
						},
					},
				},
			},
		}

		testTime := time.Date(2024, time.April, wd.dayOfWeek, 10, 0, 0, 0, time.UTC)
		result := EvaluateRulesAtTime(testTime, cfg)

		if !result["youtube.com"] {
			t.Errorf("expected youtube.com to be blocked on %s at 10:00", wd.day)
		}
	}
}

func TestEvaluateRulesAtTime_EdgeCaseMinuteBefore(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {
						{Start: "10:00", End: "11:00"},
					},
				},
			},
		},
	}

	// Monday 09:59 (1 minute before start)
	testTime := time.Date(2024, time.April, 1, 9, 59, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["youtube.com"] {
		t.Errorf("expected youtube.com to NOT be blocked at 09:59, got %v", result)
	}
}

func TestEvaluateRulesAtTime_EdgeCaseMinuteAfterEnd(t *testing.T) {
	cfg := config.Config{
		Rules: []config.Rule{
			{
				Domain:   "reddit.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Friday": {
						{Start: "14:00", End: "15:00"},
					},
				},
			},
		},
	}

	// Friday 15:01 (1 minute after end)
	testTime := time.Date(2024, time.April, 5, 15, 1, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["reddit.com"] {
		t.Errorf("expected reddit.com to NOT be blocked at 15:01, got %v", result)
	}
}
