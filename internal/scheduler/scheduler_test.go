package scheduler

import (
	"strings"
	"testing"
	"time"

	"github.com/vsangava/sentinel/internal/config"
)

// helpers ────────────────────────────────────────────────────────────────────

// singleGroup builds a config with exactly one group containing one domain
// and one rule that references it. Most tests want this shape.
func singleGroup(domain string, isActive bool, schedules map[string][]config.TimeSlot) config.Config {
	groupName := domain
	return config.Config{
		Groups: map[string][]string{groupName: {domain}},
		Rules: []config.Rule{
			{Group: groupName, IsActive: isActive, Schedules: schedules},
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────

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
	cfg := singleGroup("youtube.com", false, map[string][]config.TimeSlot{
		"Monday": {{Start: "09:00", End: "17:00"}},
	})

	// Monday 10:30 (should be blocked if rule was active)
	testTime := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if len(result) != 0 {
		t.Errorf("expected inactive rule to not block, got %v", result)
	}
}

func TestEvaluateRulesAtTime_DomainBlockedDuringSchedule(t *testing.T) {
	cfg := singleGroup("youtube.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "09:00", End: "17:00"}},
	})

	// Monday 10:30 (within block time)
	testTime := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["youtube.com"] {
		t.Errorf("expected youtube.com to be blocked at 10:30 on Monday, got %v", result)
	}
}

func TestEvaluateRulesAtTime_DomainNotBlockedOutsideSchedule(t *testing.T) {
	cfg := singleGroup("youtube.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "09:00", End: "17:00"}},
	})

	// Monday 18:30 (after block time)
	testTime := time.Date(2024, time.April, 1, 18, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["youtube.com"] {
		t.Errorf("expected youtube.com to NOT be blocked at 18:30, got %v", result)
	}
}

func TestEvaluateRulesAtTime_DomainNotBlockedWrongDay(t *testing.T) {
	cfg := singleGroup("youtube.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "09:00", End: "17:00"}},
	})

	// Tuesday 10:30 (different day, no schedule)
	testTime := time.Date(2024, time.April, 2, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["youtube.com"] {
		t.Errorf("expected youtube.com to NOT be blocked on Tuesday, got %v", result)
	}
}

func TestEvaluateRulesAtTime_BlockedAtExactStartTime(t *testing.T) {
	cfg := singleGroup("reddit.com", true, map[string][]config.TimeSlot{
		"Wednesday": {{Start: "14:00", End: "15:00"}},
	})

	// Wednesday 14:00 (exact start time)
	testTime := time.Date(2024, time.April, 3, 14, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["reddit.com"] {
		t.Errorf("expected reddit.com to be blocked at exact start time, got %v", result)
	}
}

func TestEvaluateRulesAtTime_NotBlockedAtExactEndTime(t *testing.T) {
	cfg := singleGroup("twitter.com", true, map[string][]config.TimeSlot{
		"Friday": {{Start: "09:00", End: "17:00"}},
	})

	// Friday 17:00 (exact end time, should NOT be blocked)
	testTime := time.Date(2024, time.April, 5, 17, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["twitter.com"] {
		t.Errorf("expected twitter.com to NOT be blocked at exact end time, got %v", result)
	}
}

func TestEvaluateRulesAtTime_MultipleDomainsMultipleSchedules(t *testing.T) {
	cfg := config.Config{
		Groups: map[string][]string{
			"youtube.com": {"youtube.com"},
			"reddit.com":  {"reddit.com"},
			"twitter.com": {"twitter.com"},
		},
		Rules: []config.Rule{
			{
				Group:    "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "09:00", End: "17:00"}},
				},
			},
			{
				Group:    "reddit.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "09:00", End: "12:00"}},
				},
			},
			{
				Group:    "twitter.com",
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
	cfg := singleGroup("youtube.com", true, map[string][]config.TimeSlot{
		"Monday": {
			{Start: "09:00", End: "12:00"},
			{Start: "14:00", End: "17:00"},
		},
	})

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

func TestEvaluateRulesAtTime_GroupExpandsToAllDomains(t *testing.T) {
	cfg := config.Config{
		Groups: map[string][]string{
			"games": {"roblox.com", "fortnite.com", "minecraft.net"},
		},
		Rules: []config.Rule{
			{
				Group:    "games",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "09:00", End: "15:00"}},
				},
			},
		},
	}

	testTime := time.Date(2024, time.April, 1, 10, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	for _, d := range []string{"roblox.com", "fortnite.com", "minecraft.net"} {
		if !result[d] {
			t.Errorf("expected %s to be blocked (group expansion)", d)
		}
	}
	if len(result) != 3 {
		t.Errorf("expected exactly 3 blocked domains, got %d: %v", len(result), result)
	}
}

func TestEvaluateRulesAtTime_RuleWithMissingGroupIsSkipped(t *testing.T) {
	cfg := config.Config{
		Groups: map[string][]string{}, // intentionally empty
		Rules: []config.Rule{
			{
				Group:    "phantom",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "09:00", End: "17:00"}},
				},
			},
		},
	}

	testTime := time.Date(2024, time.April, 1, 10, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if len(result) != 0 {
		t.Errorf("expected rule with missing group to be skipped, got %v", result)
	}
}

func TestCheckWarningDomainsAtTime_WarningTriggersAt3MinBefore(t *testing.T) {
	cfg := singleGroup("youtube.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "10:00", End: "12:00"}},
	})

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
	cfg := singleGroup("reddit.com", true, map[string][]config.TimeSlot{
		"Tuesday": {{Start: "14:00", End: "16:00"}},
	})

	// Tuesday 13:54 (more than 3 minutes before)
	testTime := time.Date(2024, time.April, 2, 13, 54, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) != 0 {
		t.Errorf("expected no warning at 13:54, got %v", warnings)
	}
}

func TestCheckWarningDomainsAtTime_NoWarningForInactiveRule(t *testing.T) {
	cfg := singleGroup("twitter.com", false, map[string][]config.TimeSlot{
		"Wednesday": {{Start: "11:00", End: "13:00"}},
	})

	// Wednesday 10:57 (3 minutes before, but rule inactive)
	testTime := time.Date(2024, time.April, 3, 10, 57, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) != 0 {
		t.Errorf("expected no warning for inactive rule, got %v", warnings)
	}
}

func TestCheckWarningDomainsAtTime_MultipleWarnings(t *testing.T) {
	cfg := config.Config{
		Groups: map[string][]string{
			"youtube.com": {"youtube.com"},
			"reddit.com":  {"reddit.com"},
		},
		Rules: []config.Rule{
			{
				Group:    "youtube.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Thursday": {{Start: "09:00", End: "10:00"}},
				},
			},
			{
				Group:    "reddit.com",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Thursday": {{Start: "09:00", End: "10:00"}},
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

func TestCheckWarningDomainsAtTime_GroupExpandsToAllDomains(t *testing.T) {
	cfg := config.Config{
		Groups: map[string][]string{
			"games": {"roblox.com", "fortnite.com"},
		},
		Rules: []config.Rule{
			{
				Group:    "games",
				IsActive: true,
				Schedules: map[string][]config.TimeSlot{
					"Monday": {{Start: "10:00", End: "12:00"}},
				},
			},
		},
	}

	testTime := time.Date(2024, time.April, 1, 9, 57, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) != 2 {
		t.Errorf("expected 2 warning domains from group, got %d: %v", len(warnings), warnings)
	}
}

func TestCheckWarningDomainsAtTime_WarningTriggersAtEveryMinute(t *testing.T) {
	cfg := singleGroup("facebook.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "10:00", End: "12:00"}},
	})

	testCases := []struct {
		name        string
		minute      int
		shouldWarn  bool
		description string
	}{
		{
			name:        "3 minutes before",
			minute:      57,
			shouldWarn:  true,
			description: "Monday 09:57 should warn (3 min before 10:00)",
		},
		{
			name:        "2 minutes before",
			minute:      58,
			shouldWarn:  true,
			description: "Monday 09:58 should warn (2 min before 10:00)",
		},
		{
			name:        "1 minute before",
			minute:      59,
			shouldWarn:  true,
			description: "Monday 09:59 should warn (1 min before 10:00)",
		},
		{
			name:        "at block start",
			minute:      0,
			shouldWarn:  false,
			description: "Monday 10:00 should NOT warn (block is active, not warning window)",
		},
		{
			name:        "after block starts",
			minute:      1,
			shouldWarn:  false,
			description: "Monday 10:01 should NOT warn (block already started)",
		},
		{
			name:        "4 minutes before",
			minute:      56,
			shouldWarn:  false,
			description: "Monday 09:56 should NOT warn (outside 3-min window)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testTime := time.Date(2024, time.April, 1, 9, tc.minute, 0, 0, time.UTC)
			if tc.minute == 0 {
				testTime = time.Date(2024, time.April, 1, 10, 0, 0, 0, time.UTC)
			}
			if tc.minute == 1 {
				testTime = time.Date(2024, time.April, 1, 10, 1, 0, 0, time.UTC)
			}

			warnings := CheckWarningDomainsAtTime(testTime, cfg)

			if tc.shouldWarn && len(warnings) == 0 {
				t.Errorf("%s: expected warning, got none", tc.description)
			}
			if !tc.shouldWarn && len(warnings) > 0 {
				t.Errorf("%s: expected no warning, got %v", tc.description, warnings)
			}
		})
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
		cfg := singleGroup("youtube.com", true, map[string][]config.TimeSlot{
			wd.day: {{Start: "09:00", End: "17:00"}},
		})

		testTime := time.Date(2024, time.April, wd.dayOfWeek, 10, 0, 0, 0, time.UTC)
		result := EvaluateRulesAtTime(testTime, cfg)

		if !result["youtube.com"] {
			t.Errorf("expected youtube.com to be blocked on %s at 10:00", wd.day)
		}
	}
}

func TestEvaluateRulesAtTime_EdgeCaseMinuteBefore(t *testing.T) {
	cfg := singleGroup("youtube.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "10:00", End: "11:00"}},
	})

	// Monday 09:59 (1 minute before start)
	testTime := time.Date(2024, time.April, 1, 9, 59, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["youtube.com"] {
		t.Errorf("expected youtube.com to NOT be blocked at 09:59, got %v", result)
	}
}

func TestEvaluateRulesAtTime_EdgeCaseMinuteAfterEnd(t *testing.T) {
	cfg := singleGroup("reddit.com", true, map[string][]config.TimeSlot{
		"Friday": {{Start: "14:00", End: "15:00"}},
	})

	// Friday 15:01 (1 minute after end)
	testTime := time.Date(2024, time.April, 5, 15, 1, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["reddit.com"] {
		t.Errorf("expected reddit.com to NOT be blocked at 15:01, got %v", result)
	}
}

// Overnight slot tests — April 1 2024 is Monday, April 2 is Tuesday, March 31 is Sunday.

func TestEvaluateRulesAtTime_OvernightSlot_EveningBlocked(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	testTime := time.Date(2024, time.April, 1, 22, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["example.com"] {
		t.Errorf("expected example.com blocked at 22:00 Monday (evening side of overnight slot)")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_MorningBlocked(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	testTime := time.Date(2024, time.April, 2, 7, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["example.com"] {
		t.Errorf("expected example.com blocked at 07:00 Tuesday (morning side of Monday overnight slot)")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_ExactStart_Blocked(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	testTime := time.Date(2024, time.April, 1, 21, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["example.com"] {
		t.Errorf("expected example.com blocked at exact start 21:30 Monday")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_JustBeforeEnd_Blocked(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	testTime := time.Date(2024, time.April, 2, 7, 59, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if !result["example.com"] {
		t.Errorf("expected example.com blocked at 07:59 Tuesday (one minute before overnight End)")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_ExactEnd_NotBlocked(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	testTime := time.Date(2024, time.April, 2, 8, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["example.com"] {
		t.Errorf("expected example.com NOT blocked at exact end 08:00 Tuesday")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_AfterEnd_NotBlocked(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	testTime := time.Date(2024, time.April, 2, 8, 1, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["example.com"] {
		t.Errorf("expected example.com NOT blocked at 08:01 Tuesday (after overnight End)")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_BeforeStart_NotBlocked(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	testTime := time.Date(2024, time.April, 1, 21, 29, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["example.com"] {
		t.Errorf("expected example.com NOT blocked at 21:29 Monday (one minute before overnight Start)")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_SundayToMonday(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Sunday": {{Start: "22:00", End: "07:00"}},
	})

	// Monday 06:30 — morning continuation of Sunday's overnight slot
	testTime := time.Date(2024, time.April, 1, 6, 30, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)
	if !result["example.com"] {
		t.Errorf("expected example.com blocked at 06:30 Monday (morning side of Sunday→Monday overnight slot)")
	}

	// Monday 07:00 — exact End, must not be blocked
	testTimeEnd := time.Date(2024, time.April, 1, 7, 0, 0, 0, time.UTC)
	resultEnd := EvaluateRulesAtTime(testTimeEnd, cfg)
	if resultEnd["example.com"] {
		t.Errorf("expected example.com NOT blocked at exact end 07:00 Monday (Sunday→Monday slot)")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_MorningNotBlockedWithoutYesterdaySchedule(t *testing.T) {
	// Monday has a normal daytime slot only — no overnight slot
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "09:00", End: "17:00"}},
	})

	// Tuesday 07:00 — yesterday's slot is same-day and must not trigger the overnight path
	testTime := time.Date(2024, time.April, 2, 7, 0, 0, 0, time.UTC)
	result := EvaluateRulesAtTime(testTime, cfg)

	if result["example.com"] {
		t.Errorf("expected example.com NOT blocked at 07:00 Tuesday when Monday only has a daytime slot")
	}
}

func TestEvaluateRulesAtTime_OvernightSlot_CoexistsWithSameDaySlot(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {
			{Start: "09:00", End: "12:00"},
			{Start: "21:30", End: "08:00"},
		},
	})

	cases := []struct {
		hour, min, day int
		blocked        bool
		label          string
	}{
		{10, 0, 1, true, "10:00 Monday (daytime slot)"},
		{13, 0, 1, false, "13:00 Monday (gap between slots)"},
		{22, 0, 1, true, "22:00 Monday (evening overnight)"},
		{7, 0, 2, true, "07:00 Tuesday (morning overnight)"},
		{8, 1, 2, false, "08:01 Tuesday (after overnight end)"},
	}

	for _, c := range cases {
		testTime := time.Date(2024, time.April, c.day, c.hour, c.min, 0, 0, time.UTC)
		result := EvaluateRulesAtTime(testTime, cfg)
		if c.blocked && !result["example.com"] {
			t.Errorf("expected example.com BLOCKED at %s", c.label)
		}
		if !c.blocked && result["example.com"] {
			t.Errorf("expected example.com NOT BLOCKED at %s", c.label)
		}
	}
}

func TestCheckWarningDomainsAtTime_OvernightSlot_WarningFires(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	// Monday 21:27 — 3 minutes before slot Start
	testTime := time.Date(2024, time.April, 1, 21, 27, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) == 0 {
		t.Errorf("expected warning for example.com at 21:27 Monday (3 min before overnight slot Start)")
	}
}

func TestCheckWarningDomainsAtTime_OvernightSlot_NoWarningInMorning(t *testing.T) {
	cfg := singleGroup("example.com", true, map[string][]config.TimeSlot{
		"Monday": {{Start: "21:30", End: "08:00"}},
	})

	// Tuesday 07:57 — 3 min before 08:00, but 08:00 is an End time, not a Start
	testTime := time.Date(2024, time.April, 2, 7, 57, 0, 0, time.UTC)
	warnings := CheckWarningDomainsAtTime(testTime, cfg)

	if len(warnings) != 0 {
		t.Errorf("expected no warning at 07:57 Tuesday (morning-end of overnight slot has no warning); got %v", warnings)
	}
}

func TestGenerateCloseTabsScript_IncludesArcAndBrave(t *testing.T) {
	g := &MacOSAppleScriptGenerator{}
	script := g.GenerateCloseTabsScript([]string{"youtube.com"})

	for _, app := range []string{"Google Chrome", "Safari", "Arc", "Brave Browser"} {
		if !strings.Contains(script, app) {
			t.Errorf("GenerateCloseTabsScript: expected block for %q but not found in script", app)
		}
	}
}
