package claudeai

import (
	"encoding/json"
	"time"
)

// Organization represents a claude.ai organization.
type Organization struct {
	UUID         string   `json:"uuid"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
}

// UsageResponse is the raw API response from the usage endpoint.
type UsageResponse struct {
	FiveHour       *UsageWindow `json:"five_hour"`
	SevenDay       *UsageWindow `json:"seven_day"`
	SevenDayOpus   *UsageWindow `json:"seven_day_opus"`
	SevenDaySonnet *UsageWindow `json:"seven_day_sonnet"`
}

// UsageWindow is a single rate-limit window from the API.
// Utilization can be int, float, or string — kept as raw JSON for defensive parsing.
type UsageWindow struct {
	Utilization json.RawMessage `json:"utilization"`
	ResetsAt    *string         `json:"resets_at"`
}

// OverageLimit is the raw API response from the overage spend limit endpoint.
type OverageLimit struct {
	IsEnabled          bool    `json:"isEnabled"`
	UsedCredits        float64 `json:"usedCredits"`
	MonthlyCreditLimit float64 `json:"monthlyCreditLimit"`
	Currency           string  `json:"currency"`
}

// SubscriptionData is the parsed, TUI-ready aggregate of all claude.ai API data.
type SubscriptionData struct {
	Org       Organization
	Usage     *ParsedUsage
	Overage   *OverageLimit
	FetchedAt time.Time
	Error     error
}

// ParsedUsage holds normalized usage windows.
type ParsedUsage struct {
	FiveHour       *ParsedWindow
	SevenDay       *ParsedWindow
	SevenDayOpus   *ParsedWindow
	SevenDaySonnet *ParsedWindow
}

// ParsedWindow is a single rate-limit window, normalized for display.
type ParsedWindow struct {
	Pct      float64 // 0.0-1.0
	ResetsAt time.Time
}
