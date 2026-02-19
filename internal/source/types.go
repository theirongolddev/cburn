package source

// RawEntry represents a single line in a Claude Code JSONL session file.
type RawEntry struct {
	Type      string      `json:"type"`
	Subtype   string      `json:"subtype,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
	SessionID string      `json:"sessionId,omitempty"`
	Cwd       string      `json:"cwd,omitempty"`
	Version   string      `json:"version,omitempty"`
	Message   *RawMessage `json:"message,omitempty"`

	// For system entries with subtype "turn_duration"
	DurationMs int64 `json:"durationMs,omitempty"`

	// For progress entries with turn_duration data
	Data *RawProgressData `json:"data,omitempty"`
}

// RawProgressData holds typed progress data from system/progress entries.
type RawProgressData struct {
	Type       string `json:"type"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

// RawMessage represents the assistant's message envelope.
type RawMessage struct {
	ID    string    `json:"id"`
	Role  string    `json:"role"`
	Model string    `json:"model"`
	Usage *RawUsage `json:"usage,omitempty"`
}

// RawUsage holds token counts from the API response.
type RawUsage struct {
	InputTokens              int64          `json:"input_tokens"`
	OutputTokens             int64          `json:"output_tokens"`
	CacheCreationInputTokens int64          `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64          `json:"cache_read_input_tokens"`
	CacheCreation            *CacheCreation `json:"cache_creation,omitempty"`
	ServiceTier              string         `json:"service_tier"`
}

// CacheCreation holds the breakdown of cache write tokens by TTL bucket.
type CacheCreation struct {
	Ephemeral5mInputTokens int64 `json:"ephemeral_5m_input_tokens"`
	Ephemeral1hInputTokens int64 `json:"ephemeral_1h_input_tokens"`
}

// DiscoveredFile represents a JSONL file found during directory scanning.
type DiscoveredFile struct {
	Path          string
	Project       string // decoded display name (e.g., "gitlore")
	ProjectDir    string // raw directory name
	SessionID     string // extracted from filename
	IsSubagent    bool
	ParentSession string // for subagents: parent session UUID
}
