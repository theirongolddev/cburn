// Package model defines domain types for cburn metrics and sessions.
package model

import "time"

// APICall represents one deduplicated API request (final state of a message.id).
type APICall struct {
	MessageID             string
	Model                 string
	Timestamp             time.Time
	InputTokens           int64
	OutputTokens          int64
	CacheCreation5mTokens int64
	CacheCreation1hTokens int64
	CacheReadTokens       int64
	ServiceTier           string
	EstimatedCost         float64
}

// ModelUsage tracks per-model token usage within a session.
type ModelUsage struct { //nolint:revive // renaming would break many call sites
	APICalls              int
	InputTokens           int64
	OutputTokens          int64
	CacheCreation5mTokens int64
	CacheCreation1hTokens int64
	CacheReadTokens       int64
	EstimatedCost         float64
}

// SessionStats holds aggregated metrics for a single session file.
type SessionStats struct {
	SessionID     string
	Project       string
	ProjectPath   string
	FilePath      string
	IsSubagent    bool
	ParentSession string
	StartTime     time.Time
	EndTime       time.Time
	DurationSecs  int64

	UserMessages int
	APICalls     int

	InputTokens           int64
	OutputTokens          int64
	CacheCreation5mTokens int64
	CacheCreation1hTokens int64
	CacheReadTokens       int64

	Models map[string]*ModelUsage

	EstimatedCost float64
	CacheHitRate  float64
}
