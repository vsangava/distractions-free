package proxy

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/vsangava/sentinel/internal/config"
)

// UsageEvent records a single observation that a domain in a known group was
// touched by the user. Two flavours coexist in the same JSONL log:
//
//   - Kind == "" (legacy) or "dns": one entry per DNS lookup. Aggregated into
//     5-minute buckets — feeds the existing used_minutes / quota signal.
//   - Kind == "foreground": one entry per scheduler tick where the active
//     browser tab matched a configured group. Aggregated into 1-minute buckets
//     — feeds foreground_minutes. macOS-only.
//
// The empty-string default for Kind is intentional — it keeps every entry that
// existed before foreground tracking was introduced parsing as DNS without a
// migration step.
type UsageEvent struct {
	TS     time.Time `json:"ts"`
	Domain string    `json:"domain"`
	Group  string    `json:"group"`
	Kind   string    `json:"kind,omitempty"`
}

// Usage event Kind constants. Empty Kind on an existing event is equivalent to
// KindDNS for backwards compatibility.
const (
	KindDNS        = "dns"
	KindForeground = "foreground"
)

// IsDNSKind reports whether a UsageEvent should count toward DNS-bucket usage.
// Treats the legacy empty Kind as DNS so pre-feature events keep aggregating.
func (e UsageEvent) IsDNSKind() bool {
	return e.Kind == "" || e.Kind == KindDNS
}

func usageFilePath() string {
	return filepath.Join(config.ConfigDir(), "usage.jsonl")
}

// AppendUsageEvent appends a single usage event to the JSONL log. Best-effort.
func AppendUsageEvent(e UsageEvent) error {
	f, err := os.OpenFile(usageFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(e)
}

// ReadUsageEventsSince returns all usage events with TS strictly after since.
// A zero since means no lower bound.
func ReadUsageEventsSince(since time.Time) ([]UsageEvent, error) {
	path := usageFilePath()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []UsageEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e UsageEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if !since.IsZero() && !e.TS.After(since) {
			continue
		}
		events = append(events, e)
	}
	return events, scanner.Err()
}

// PruneOldUsageEvents rewrites the usage log keeping only entries within maxAge.
func PruneOldUsageEvents(maxAge time.Duration) error {
	path := usageFilePath()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	var kept []UsageEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e UsageEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if e.TS.After(cutoff) {
			kept = append(kept, e)
		}
	}
	f.Close()
	if err := scanner.Err(); err != nil {
		return err
	}

	tmp := path + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	for _, e := range kept {
		enc.Encode(e)
	}
	out.Close()
	return os.Rename(tmp, path)
}

// bucketKey returns the 5-minute bucket key for a timestamp.
// Queries within the same 5-minute window collapse to one bucket.
func bucketKey(t time.Time) int64 {
	return t.Unix() / 300
}

// ComputeGroupUsageMinutes returns the number of minutes a group was actively
// used on the calendar day containing t, derived from the provided events.
// Usage is measured in distinct 5-minute buckets × 5 minutes. Only DNS-kind
// events count — foreground events live alongside in the same log but are
// aggregated separately via ComputeGroupForegroundMinutes.
func ComputeGroupUsageMinutes(events []UsageEvent, group string, t time.Time) int {
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	buckets := make(map[int64]struct{})
	for _, e := range events {
		if !e.IsDNSKind() {
			continue
		}
		if e.Group != group {
			continue
		}
		if e.TS.Before(dayStart) || !e.TS.Before(dayEnd) {
			continue
		}
		buckets[bucketKey(e.TS)] = struct{}{}
	}
	return len(buckets) * 5
}

// ComputeAllGroupUsageMinutes returns ComputeGroupUsageMinutes for every group
// in groups, for the calendar day containing t.
func ComputeAllGroupUsageMinutes(events []UsageEvent, groups []string, t time.Time) map[string]int {
	result := make(map[string]int, len(groups))
	for _, g := range groups {
		result[g] = ComputeGroupUsageMinutes(events, g, t)
	}
	return result
}

// minuteBucketKey returns the 1-minute bucket for a timestamp. Foreground
// observations are minute-granular by construction (one per scheduler tick),
// so 5-minute bucketing would only obscure the signal.
func minuteBucketKey(t time.Time) int64 {
	return t.Unix() / 60
}

// ComputeGroupForegroundMinutes returns minutes spent with the foreground
// browser tab on a domain in `group`, for the calendar day containing t.
// Counts distinct 1-minute buckets — duplicate observations within the same
// minute (shouldn't happen, but harmless) collapse to a single tick.
func ComputeGroupForegroundMinutes(events []UsageEvent, group string, t time.Time) int {
	dayStart := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	buckets := make(map[int64]struct{})
	for _, e := range events {
		if e.Kind != KindForeground {
			continue
		}
		if e.Group != group {
			continue
		}
		if e.TS.Before(dayStart) || !e.TS.Before(dayEnd) {
			continue
		}
		buckets[minuteBucketKey(e.TS)] = struct{}{}
	}
	return len(buckets)
}
