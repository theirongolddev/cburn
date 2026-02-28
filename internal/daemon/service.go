// Package daemon provides the long-running background usage monitor service.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/theirongolddev/cburn/internal/model"
	"github.com/theirongolddev/cburn/internal/pipeline"
	"github.com/theirongolddev/cburn/internal/store"
)

// Config controls the daemon runtime behavior.
type Config struct {
	DataDir          string
	Days             int
	ProjectFilter    string
	ModelFilter      string
	IncludeSubagents bool
	UseCache         bool
	Interval         time.Duration
	Addr             string
	EventsBuffer     int
}

// Snapshot is a compact usage state for status/event payloads.
type Snapshot struct {
	At               time.Time `json:"at"`
	Sessions         int       `json:"sessions"`
	Prompts          int       `json:"prompts"`
	APICalls         int       `json:"api_calls"`
	Tokens           int64     `json:"tokens"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	CacheHitRate     float64   `json:"cache_hit_rate"`
	CostPerDayUSD    float64   `json:"cost_per_day_usd"`
	TokensPerDay     int64     `json:"tokens_per_day"`
	SessionsPerDay   float64   `json:"sessions_per_day"`
}

// Delta captures snapshot deltas between polls.
type Delta struct {
	Sessions         int     `json:"sessions"`
	Prompts          int     `json:"prompts"`
	APICalls         int     `json:"api_calls"`
	Tokens           int64   `json:"tokens"`
	EstimatedCostUSD float64 `json:"estimated_cost_usd"`
}

func (d Delta) isZero() bool {
	return d.Sessions == 0 &&
		d.Prompts == 0 &&
		d.APICalls == 0 &&
		d.Tokens == 0 &&
		d.EstimatedCostUSD == 0
}

// Event is emitted whenever usage snapshot updates.
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Snapshot  Snapshot  `json:"snapshot"`
	Delta     Delta     `json:"delta"`
}

// Status is served at /v1/status.
type Status struct {
	StartedAt       time.Time `json:"started_at"`
	LastPollAt      time.Time `json:"last_poll_at"`
	PollIntervalSec int       `json:"poll_interval_sec"`
	PollCount       int64     `json:"poll_count"`
	DataDir         string    `json:"data_dir"`
	Days            int       `json:"days"`
	ProjectFilter   string    `json:"project_filter,omitempty"`
	ModelFilter     string    `json:"model_filter,omitempty"`
	Summary         Snapshot  `json:"summary"`
	LastError       string    `json:"last_error,omitempty"`
	EventCount      int       `json:"event_count"`
	SubscriberCount int       `json:"subscriber_count"`
}

// Service provides the daemon runtime and HTTP API.
type Service struct {
	cfg Config

	mu          sync.RWMutex
	startedAt   time.Time
	lastPollAt  time.Time
	pollCount   int64
	lastError   string
	hasSnapshot bool
	snapshot    Snapshot
	nextEventID int64
	events      []Event

	nextSubID int
	subs      map[int]chan Event
}

// New returns a new daemon service with the provided config.
func New(cfg Config) *Service {
	if cfg.Interval < 2*time.Second {
		cfg.Interval = 10 * time.Second
	}
	if cfg.EventsBuffer < 1 {
		cfg.EventsBuffer = 200
	}
	if cfg.Addr == "" {
		cfg.Addr = "127.0.0.1:8787"
	}

	return &Service{
		cfg:       cfg,
		startedAt: time.Now(),
		subs:      make(map[int]chan Event),
	}
}

// Run starts HTTP endpoints and polling until ctx is canceled.
func (s *Service) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/v1/status", s.handleStatus)
	mux.HandleFunc("/v1/events", s.handleEvents)
	mux.HandleFunc("/v1/stream", s.handleStream)

	server := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Seed initial snapshot so status is useful immediately.
	s.pollOnce()

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdownCtx)
		case <-ticker.C:
			s.pollOnce()
		case err := <-errCh:
			return fmt.Errorf("daemon http server: %w", err)
		}
	}
}

func (s *Service) pollOnce() {
	start := time.Now()
	sessions, err := s.loadSessions()
	if err != nil {
		s.mu.Lock()
		s.lastError = err.Error()
		s.lastPollAt = time.Now()
		s.pollCount++
		s.mu.Unlock()
		log.Printf("cburn daemon poll error: %v", err)
		return
	}

	now := time.Now()
	since := now.AddDate(0, 0, -s.cfg.Days)

	filtered := sessions
	if s.cfg.ProjectFilter != "" {
		filtered = pipeline.FilterByProject(filtered, s.cfg.ProjectFilter)
	}
	if s.cfg.ModelFilter != "" {
		filtered = pipeline.FilterByModel(filtered, s.cfg.ModelFilter)
	}

	stats := pipeline.Aggregate(filtered, since, now)
	snap := snapshotFromSummary(stats, now)

	var (
		ev      Event
		publish bool
	)

	s.mu.Lock()
	prev := s.snapshot
	prevExists := s.hasSnapshot

	s.hasSnapshot = true
	s.snapshot = snap
	s.lastPollAt = now
	s.pollCount++
	s.lastError = ""

	if !prevExists {
		s.nextEventID++
		ev = Event{
			ID:        s.nextEventID,
			Type:      "snapshot",
			Timestamp: now,
			Snapshot:  snap,
			Delta:     Delta{},
		}
		publish = true
	} else {
		delta := diffSnapshots(prev, snap)
		if !delta.isZero() {
			s.nextEventID++
			ev = Event{
				ID:        s.nextEventID,
				Type:      "usage_delta",
				Timestamp: now,
				Snapshot:  snap,
				Delta:     delta,
			}
			publish = true
		}
	}
	s.mu.Unlock()

	if publish {
		s.publishEvent(ev)
	}

	_ = start
}

func (s *Service) loadSessions() ([]model.SessionStats, error) {
	if s.cfg.UseCache {
		cache, err := store.Open(pipeline.CachePath())
		if err == nil {
			defer func() { _ = cache.Close() }()
			cr, loadErr := pipeline.LoadWithCache(s.cfg.DataDir, s.cfg.IncludeSubagents, cache, nil)
			if loadErr == nil {
				return cr.Sessions, nil
			}
		}
	}

	result, err := pipeline.Load(s.cfg.DataDir, s.cfg.IncludeSubagents, nil)
	if err != nil {
		return nil, err
	}
	return result.Sessions, nil
}

func snapshotFromSummary(stats model.SummaryStats, at time.Time) Snapshot {
	return Snapshot{
		At:               at,
		Sessions:         stats.TotalSessions,
		Prompts:          stats.TotalPrompts,
		APICalls:         stats.TotalAPICalls,
		Tokens:           stats.TotalBilledTokens,
		EstimatedCostUSD: stats.EstimatedCost,
		CacheHitRate:     stats.CacheHitRate,
		CostPerDayUSD:    stats.CostPerDay,
		TokensPerDay:     stats.TokensPerDay,
		SessionsPerDay:   stats.SessionsPerDay,
	}
}

func diffSnapshots(prev, curr Snapshot) Delta {
	return Delta{
		Sessions:         curr.Sessions - prev.Sessions,
		Prompts:          curr.Prompts - prev.Prompts,
		APICalls:         curr.APICalls - prev.APICalls,
		Tokens:           curr.Tokens - prev.Tokens,
		EstimatedCostUSD: curr.EstimatedCostUSD - prev.EstimatedCostUSD,
	}
}

func (s *Service) publishEvent(ev Event) {
	s.mu.Lock()
	s.events = append(s.events, ev)
	if len(s.events) > s.cfg.EventsBuffer {
		s.events = s.events[len(s.events)-s.cfg.EventsBuffer:]
	}

	for _, ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	s.mu.Unlock()
}

func (s *Service) snapshotStatus() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Status{
		StartedAt:       s.startedAt,
		LastPollAt:      s.lastPollAt,
		PollIntervalSec: int(s.cfg.Interval.Seconds()),
		PollCount:       s.pollCount,
		DataDir:         s.cfg.DataDir,
		Days:            s.cfg.Days,
		ProjectFilter:   s.cfg.ProjectFilter,
		ModelFilter:     s.cfg.ModelFilter,
		Summary:         s.snapshot,
		LastError:       s.lastError,
		EventCount:      len(s.events),
		SubscriberCount: len(s.subs),
	}
}

func (s *Service) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Service) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.snapshotStatus())
}

func (s *Service) handleEvents(w http.ResponseWriter, _ *http.Request) {
	s.mu.RLock()
	events := make([]Event, len(s.events))
	copy(events, s.events)
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(events)
}

func (s *Service) handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan Event, 16)
	id := s.addSubscriber(ch)
	defer s.removeSubscriber(id)

	// Send current snapshot immediately.
	current := Event{
		Type:      "snapshot",
		Timestamp: time.Now(),
		Snapshot:  s.snapshotStatus().Summary,
	}
	writeSSE(w, current)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			writeSSE(w, ev)
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, ev Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\n", ev.Type)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

func (s *Service) addSubscriber(ch chan Event) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSubID++
	id := s.nextSubID
	s.subs[id] = ch
	return id
}

func (s *Service) removeSubscriber(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, id)
}
