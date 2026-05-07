package proxy

import (
	"testing"
	"time"
)

// usageEventsForKindFiltering builds a fixture with both DNS and foreground
// events on the same day, same group, same domain — used to verify the two
// aggregations stay independent.
func usageEventsForKindFiltering(day time.Time) []UsageEvent {
	at := func(h, m int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, day.Location())
	}
	return []UsageEvent{
		// Three DNS events in two distinct 5-min buckets → 10 minutes used.
		{TS: at(9, 1), Domain: "youtube.com", Group: "social"},                      // legacy empty Kind
		{TS: at(9, 4), Domain: "youtube.com", Group: "social", Kind: KindDNS},       // explicit dns
		{TS: at(9, 8), Domain: "youtube.com", Group: "social", Kind: KindDNS},       // distinct bucket
		// Four foreground events in three distinct minute buckets → 3 minutes.
		{TS: at(10, 0), Domain: "youtube.com", Group: "social", Kind: KindForeground},
		{TS: at(10, 0), Domain: "youtube.com", Group: "social", Kind: KindForeground}, // dup minute → collapse
		{TS: at(10, 1), Domain: "youtube.com", Group: "social", Kind: KindForeground},
		{TS: at(10, 2), Domain: "youtube.com", Group: "social", Kind: KindForeground},
		// Foreground for a different group on the same day.
		{TS: at(11, 30), Domain: "reddit.com", Group: "discussion", Kind: KindForeground},
	}
}

func TestComputeGroupUsageMinutes_IgnoresForegroundEvents(t *testing.T) {
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	events := usageEventsForKindFiltering(day)

	got := ComputeGroupUsageMinutes(events, "social", day.Add(15*time.Hour))
	if got != 10 {
		t.Errorf("ComputeGroupUsageMinutes(social) = %d, want 10 (DNS-only buckets, foreground excluded)", got)
	}
}

func TestComputeGroupUsageMinutes_LegacyEmptyKindStillCounts(t *testing.T) {
	// Pre-feature usage.jsonl entries have no Kind field. They must continue to
	// count toward used_minutes — this is the backwards-compat contract.
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	at := func(h, m int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, day.Location())
	}
	events := []UsageEvent{
		{TS: at(8, 0), Domain: "youtube.com", Group: "social"}, // empty Kind
		{TS: at(8, 6), Domain: "youtube.com", Group: "social"}, // distinct 5-min bucket
	}
	if got := ComputeGroupUsageMinutes(events, "social", day.Add(12*time.Hour)); got != 10 {
		t.Errorf("legacy events: got %d minutes, want 10", got)
	}
}

func TestComputeGroupForegroundMinutes_DistinctMinuteBuckets(t *testing.T) {
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	events := usageEventsForKindFiltering(day)

	got := ComputeGroupForegroundMinutes(events, "social", day.Add(15*time.Hour))
	if got != 3 {
		t.Errorf("ComputeGroupForegroundMinutes(social) = %d, want 3 (distinct minute buckets)", got)
	}

	// Other group only has one foreground event.
	if got := ComputeGroupForegroundMinutes(events, "discussion", day.Add(15*time.Hour)); got != 1 {
		t.Errorf("ComputeGroupForegroundMinutes(discussion) = %d, want 1", got)
	}
}

func TestComputeGroupForegroundMinutes_IgnoresDNSEvents(t *testing.T) {
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	at := func(h, m int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, day.Location())
	}
	events := []UsageEvent{
		{TS: at(8, 0), Domain: "youtube.com", Group: "social", Kind: KindDNS},
		{TS: at(8, 1), Domain: "youtube.com", Group: "social"}, // legacy empty
	}
	if got := ComputeGroupForegroundMinutes(events, "social", day.Add(12*time.Hour)); got != 0 {
		t.Errorf("foreground aggregator must ignore DNS-kind events; got %d", got)
	}
}

func TestComputeGroupForegroundMinutes_DayBoundary(t *testing.T) {
	// Events outside the calendar day of `t` must not count.
	day := time.Date(2026, 5, 6, 0, 0, 0, 0, time.UTC)
	yesterday := day.Add(-1 * time.Hour) // previous day, late evening
	tomorrow := day.Add(25 * time.Hour)
	events := []UsageEvent{
		{TS: yesterday, Domain: "youtube.com", Group: "social", Kind: KindForeground},
		{TS: tomorrow, Domain: "youtube.com", Group: "social", Kind: KindForeground},
		{TS: day.Add(10 * time.Hour), Domain: "youtube.com", Group: "social", Kind: KindForeground},
	}
	if got := ComputeGroupForegroundMinutes(events, "social", day.Add(15*time.Hour)); got != 1 {
		t.Errorf("expected 1 minute (only same-day event), got %d", got)
	}
}

func TestUsageEvent_IsDNSKind(t *testing.T) {
	cases := []struct {
		kind string
		want bool
	}{
		{"", true},
		{KindDNS, true},
		{KindForeground, false},
		{"unknown", false},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			if got := (UsageEvent{Kind: c.kind}).IsDNSKind(); got != c.want {
				t.Errorf("IsDNSKind(%q) = %v, want %v", c.kind, got, c.want)
			}
		})
	}
}
