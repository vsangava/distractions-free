package scheduler

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/vsangava/sentinel/internal/config"
)

// BlockEvent records a domain block or unblock transition.
type BlockEvent struct {
	TS      time.Time `json:"ts"`
	Event   string    `json:"event"` // "blocked" or "unblocked"
	Group   string    `json:"group"`
	Domains []string  `json:"domains"`
}

func eventsFilePath() string {
	return filepath.Join(config.ConfigDir(), "events.jsonl")
}

// AppendEvents appends block/unblock events to the JSONL log.
func AppendEvents(events []BlockEvent) error {
	if len(events) == 0 {
		return nil
	}
	f, err := os.OpenFile(eventsFilePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range events {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// PruneOldEvents rewrites the event log keeping only entries within maxAge.
// The rewrite is atomic (write to .tmp then rename).
func PruneOldEvents(maxAge time.Duration) error {
	path := eventsFilePath()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	var kept []BlockEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e BlockEvent
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

// ReadEvents returns events after `since`, capped to the last `limit` entries.
// A zero `since` means no lower bound. A zero `limit` means no cap.
func ReadEvents(since time.Time, limit int) ([]BlockEvent, error) {
	path := eventsFilePath()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []BlockEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []BlockEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e BlockEvent
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		if !since.IsZero() && !e.TS.After(since) {
			continue
		}
		events = append(events, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}
	return events, nil
}
