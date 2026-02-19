package model

import "time"

// SummaryStats holds the top-level aggregate across all sessions.
type SummaryStats struct {
	TotalSessions   int
	TotalPrompts    int
	TotalAPICalls   int
	TotalDurationSecs int64
	ActiveDays      int

	InputTokens           int64
	OutputTokens          int64
	CacheCreation5mTokens int64
	CacheCreation1hTokens int64
	CacheReadTokens       int64
	TotalBilledTokens     int64

	EstimatedCost float64
	ActualCost    *float64
	CacheSavings  float64
	CacheHitRate  float64

	CostPerDay     float64
	TokensPerDay   int64
	SessionsPerDay float64
	PromptsPerDay  float64
	MinutesPerDay  float64
}

// DailyStats holds metrics for a single calendar day.
type DailyStats struct {
	Date            time.Time
	Sessions        int
	Prompts         int
	APICalls        int
	DurationSecs    int64
	InputTokens     int64
	OutputTokens    int64
	CacheCreation5m int64
	CacheCreation1h int64
	CacheReadTokens int64
	EstimatedCost   float64
	ActualCost      *float64
}

// ModelStats holds aggregated metrics for a single model.
type ModelStats struct {
	Model           string
	APICalls        int
	InputTokens     int64
	OutputTokens    int64
	CacheCreation5m int64
	CacheCreation1h int64
	CacheReadTokens int64
	EstimatedCost   float64
	SharePercent    float64
	TrendDirection  int // -1, 0, +1 vs previous period
}

// ProjectStats holds aggregated metrics for a single project.
type ProjectStats struct {
	Project        string
	Sessions       int
	Prompts        int
	TotalTokens    int64
	EstimatedCost  float64
	TrendDirection int
}

// HourlyStats holds prompt/session counts for one hour of the day.
type HourlyStats struct {
	Hour     int
	Prompts  int
	Sessions int
	Tokens   int64
}

// WeeklyStats holds metrics for one calendar week.
type WeeklyStats struct {
	WeekStart    time.Time
	Sessions     int
	Prompts      int
	TotalTokens  int64
	DurationSecs int64
	EstimatedCost float64
}

// PeriodComparison holds current and previous period data for delta computation.
type PeriodComparison struct {
	Current  SummaryStats
	Previous SummaryStats
}
