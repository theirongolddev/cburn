package daemon

import (
	"math"
	"testing"
	"time"
)

func TestDiffSnapshots(t *testing.T) {
	prev := Snapshot{
		Sessions:         10,
		Prompts:          100,
		APICalls:         120,
		Tokens:           1_000_000,
		EstimatedCostUSD: 10.5,
	}
	curr := Snapshot{
		Sessions:         12,
		Prompts:          112,
		APICalls:         136,
		Tokens:           1_250_000,
		EstimatedCostUSD: 13.1,
	}

	delta := diffSnapshots(prev, curr)
	if delta.Sessions != 2 {
		t.Fatalf("Sessions delta = %d, want 2", delta.Sessions)
	}
	if delta.Prompts != 12 {
		t.Fatalf("Prompts delta = %d, want 12", delta.Prompts)
	}
	if delta.APICalls != 16 {
		t.Fatalf("APICalls delta = %d, want 16", delta.APICalls)
	}
	if delta.Tokens != 250_000 {
		t.Fatalf("Tokens delta = %d, want 250000", delta.Tokens)
	}
	if math.Abs(delta.EstimatedCostUSD-2.6) > 1e-9 {
		t.Fatalf("Cost delta = %.2f, want 2.60", delta.EstimatedCostUSD)
	}
	if delta.isZero() {
		t.Fatal("delta unexpectedly reported as zero")
	}
}

func TestPublishEventRingBuffer(t *testing.T) {
	s := New(Config{
		DataDir:      ".",
		Interval:     10 * time.Second,
		EventsBuffer: 2,
	})

	s.publishEvent(Event{ID: 1})
	s.publishEvent(Event{ID: 2})
	s.publishEvent(Event{ID: 3})

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.events) != 2 {
		t.Fatalf("events len = %d, want 2", len(s.events))
	}
	if s.events[0].ID != 2 || s.events[1].ID != 3 {
		t.Fatalf("events ring contains IDs [%d, %d], want [2, 3]", s.events[0].ID, s.events[1].ID)
	}
}
