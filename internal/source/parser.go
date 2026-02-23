// Package source discovers and parses Claude Code JSONL session files.
package source

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"time"

	"github.com/theirongolddev/cburn/internal/config"
	"github.com/theirongolddev/cburn/internal/model"
)

// Byte patterns for field extraction.
var (
	patTurnDuration = []byte(`"turn_duration"`)
	patDurationMs   = []byte(`"durationMs":`)
	patTimestamp1   = []byte(`"timestamp":"`)
	patTimestamp2   = []byte(`"timestamp": "`)
	patCwd1         = []byte(`"cwd":"`)
	patCwd2         = []byte(`"cwd": "`)
)

// ParseResult holds the output of parsing a single JSONL file.
type ParseResult struct {
	Stats       model.SessionStats
	ParseErrors int
	Err         error
}

// ParseFile reads a JSONL session file and produces deduplicated session statistics.
// It deduplicates by message.id, keeping only the last entry per ID (final billed usage).
//
// Entry routing by top-level "type" field:
//   - "user"      → byte-level extraction (timestamp, cwd, count)
//   - "system"    → byte-level extraction (timestamp, cwd, durationMs)
//   - "assistant" → full JSON parse (token usage, model, costs)
//   - everything else → skip
func ParseFile(df DiscoveredFile) ParseResult {
	f, err := os.Open(df.Path)
	if err != nil {
		return ParseResult{Err: err}
	}
	defer func() { _ = f.Close() }()

	calls := make(map[string]*model.APICall)

	var (
		userMessages  int
		parseErrors   int
		totalDuration int64
		minTime       time.Time
		maxTime       time.Time
		cwd           string
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()

		entryType := extractTopLevelType(line)
		if entryType == "" {
			continue
		}

		switch entryType {
		case "user":
			userMessages++
			if ts, ok := extractTimestampBytes(line); ok {
				updateTimeRange(&minTime, &maxTime, ts)
			}
			if cwd == "" {
				if c := extractCwdBytes(line); c != "" {
					cwd = c
				}
			}

		case "system":
			if ts, ok := extractTimestampBytes(line); ok {
				updateTimeRange(&minTime, &maxTime, ts)
			}
			if cwd == "" {
				if c := extractCwdBytes(line); c != "" {
					cwd = c
				}
			}
			if bytes.Contains(line, patTurnDuration) {
				if ms, ok := extractDurationMs(line); ok {
					totalDuration += ms
				}
			}

		case "assistant":
			var entry RawEntry
			if err := json.Unmarshal(line, &entry); err != nil {
				parseErrors++
				continue
			}

			if entry.Timestamp != "" {
				ts, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
				if err == nil {
					updateTimeRange(&minTime, &maxTime, ts)
				}
			}
			if cwd == "" && entry.Cwd != "" {
				cwd = entry.Cwd
			}
			if entry.DurationMs > 0 {
				totalDuration += entry.DurationMs
			} else if entry.Data != nil && entry.Data.DurationMs > 0 {
				totalDuration += entry.Data.DurationMs
			}

			if entry.Message == nil || entry.Message.ID == "" {
				continue
			}
			msg := entry.Message
			if msg.Usage == nil {
				continue
			}

			u := msg.Usage
			var cache5m, cache1h int64
			if u.CacheCreation != nil {
				cache5m = u.CacheCreation.Ephemeral5mInputTokens
				cache1h = u.CacheCreation.Ephemeral1hInputTokens
			} else if u.CacheCreationInputTokens > 0 {
				cache5m = u.CacheCreationInputTokens
			}

			ts, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

			calls[msg.ID] = &model.APICall{
				MessageID:             msg.ID,
				Model:                 msg.Model,
				Timestamp:             ts,
				InputTokens:           u.InputTokens,
				OutputTokens:          u.OutputTokens,
				CacheCreation5mTokens: cache5m,
				CacheCreation1hTokens: cache1h,
				CacheReadTokens:       u.CacheReadInputTokens,
				ServiceTier:           u.ServiceTier,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return ParseResult{Err: err}
	}

	stats := model.SessionStats{
		SessionID:     df.SessionID,
		Project:       df.Project,
		ProjectPath:   cwd,
		FilePath:      df.Path,
		IsSubagent:    df.IsSubagent,
		ParentSession: df.ParentSession,
		StartTime:     minTime,
		EndTime:       maxTime,
		UserMessages:  userMessages,
		APICalls:      len(calls),
		Models:        make(map[string]*model.ModelUsage),
	}

	if totalDuration > 0 {
		stats.DurationSecs = totalDuration / 1000
	} else if !minTime.IsZero() && !maxTime.IsZero() {
		stats.DurationSecs = int64(maxTime.Sub(minTime).Seconds())
	}

	for _, call := range calls {
		call.EstimatedCost = config.CalculateCostAt(
			call.Model,
			call.Timestamp,
			call.InputTokens,
			call.OutputTokens,
			call.CacheCreation5mTokens,
			call.CacheCreation1hTokens,
			call.CacheReadTokens,
		)

		stats.InputTokens += call.InputTokens
		stats.OutputTokens += call.OutputTokens
		stats.CacheCreation5mTokens += call.CacheCreation5mTokens
		stats.CacheCreation1hTokens += call.CacheCreation1hTokens
		stats.CacheReadTokens += call.CacheReadTokens
		stats.EstimatedCost += call.EstimatedCost

		normalized := config.NormalizeModelName(call.Model)
		mu, ok := stats.Models[normalized]
		if !ok {
			mu = &model.ModelUsage{}
			stats.Models[normalized] = mu
		}
		mu.APICalls++
		mu.InputTokens += call.InputTokens
		mu.OutputTokens += call.OutputTokens
		mu.CacheCreation5mTokens += call.CacheCreation5mTokens
		mu.CacheCreation1hTokens += call.CacheCreation1hTokens
		mu.CacheReadTokens += call.CacheReadTokens
		mu.EstimatedCost += call.EstimatedCost
	}

	totalCacheInput := stats.CacheReadTokens + stats.CacheCreation5mTokens +
		stats.CacheCreation1hTokens + stats.InputTokens
	if totalCacheInput > 0 {
		stats.CacheHitRate = float64(stats.CacheReadTokens) / float64(totalCacheInput)
	}

	return ParseResult{
		Stats:       stats,
		ParseErrors: parseErrors,
	}
}

// typeKey is the byte sequence for a JSON key named "type" (with quotes).
var typeKey = []byte(`"type"`)

// extractTopLevelType finds the top-level "type" field in a JSONL line.
// Tracks brace depth and string boundaries so nested "type" keys are ignored.
// Early-exits once found (~400 bytes in), making cost O(1) vs line length.
func extractTopLevelType(line []byte) string {
	depth := 0
	for i := 0; i < len(line); {
		switch line[i] {
		case '"':
			if depth == 1 && bytes.HasPrefix(line[i:], typeKey) {
				val, isKey := classifyType(line, i+len(typeKey))
				if isKey {
					return val // found the "type" key — done regardless of value
				}
				// "type" appeared as a value, not a key. Continue scanning.
			}
			i = skipJSONString(line, i)
		case '{':
			depth++
			i++
		case '}':
			depth--
			i++
		default:
			i++
		}
	}
	return ""
}

// classifyType checks whether pos follows a JSON key (expects : then value).
// Returns the type value and whether this was a valid key:value pair.
// isKey=false means "type" appeared as a value, not a key — caller should continue.
func classifyType(line []byte, pos int) (val string, isKey bool) {
	i := skipSpaces(line, pos)
	if i >= len(line) || line[i] != ':' {
		return "", false // no colon — this was a value, not a key
	}
	i = skipSpaces(line, i+1)
	if i >= len(line) || line[i] != '"' {
		return "", true // key with non-string value (null, number, etc.)
	}
	i++ // past opening quote

	end := bytes.IndexByte(line[i:], '"')
	if end < 0 || end > 20 {
		return "", true
	}
	v := string(line[i : i+end])
	switch v {
	case "assistant", "user", "system":
		return v, true
	}
	return "", true // valid key but irrelevant type (e.g., "progress")
}

// skipJSONString advances past a JSON string starting at the opening quote.
//
//nolint:gosec // manual bounds checking throughout
func skipJSONString(line []byte, i int) int {
	i++ // skip opening quote
	for i < len(line) {
		switch line[i] {
		case '\\':
			i += 2
		case '"':
			return i + 1
		default:
			i++
		}
	}
	return i
}

func skipSpaces(line []byte, i int) int {
	for i < len(line) && line[i] == ' ' {
		i++
	}
	return i
}

// extractTimestampBytes extracts the timestamp field via byte scanning.
func extractTimestampBytes(line []byte) (time.Time, bool) {
	for _, pat := range [][]byte{patTimestamp1, patTimestamp2} {
		idx := bytes.Index(line, pat)
		if idx < 0 {
			continue
		}
		start := idx + len(pat)
		end := bytes.IndexByte(line[start:], '"')
		if end < 0 || end > 40 {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, string(line[start:start+end]))
		if err != nil {
			return time.Time{}, false
		}
		return ts, true
	}
	return time.Time{}, false
}

// extractCwdBytes extracts the cwd field via byte scanning.
func extractCwdBytes(line []byte) string {
	for _, pat := range [][]byte{patCwd1, patCwd2} {
		idx := bytes.Index(line, pat)
		if idx < 0 {
			continue
		}
		start := idx + len(pat)
		end := bytes.IndexByte(line[start:], '"')
		if end < 0 || end > 1024 {
			continue
		}
		return string(line[start : start+end])
	}
	return ""
}

// extractDurationMs extracts the durationMs integer via byte scanning.
func extractDurationMs(line []byte) (int64, bool) {
	idx := bytes.Index(line, patDurationMs)
	if idx < 0 {
		return 0, false
	}
	start := idx + len(patDurationMs)
	for start < len(line) && line[start] == ' ' {
		start++
	}
	end := start
	for end < len(line) && line[end] >= '0' && line[end] <= '9' {
		end++
	}
	if end == start {
		return 0, false
	}
	var n int64
	for i := start; i < end; i++ {
		n = n*10 + int64(line[i]-'0')
	}
	return n, true
}

func updateTimeRange(minTime, maxTime *time.Time, ts time.Time) {
	if minTime.IsZero() || ts.Before(*minTime) {
		*minTime = ts
	}
	if maxTime.IsZero() || ts.After(*maxTime) {
		*maxTime = ts
	}
}
