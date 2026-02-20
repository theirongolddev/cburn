package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeSession creates a temp JSONL file and returns a DiscoveredFile for it.
func writeSession(t *testing.T, lines ...string) DiscoveredFile {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return DiscoveredFile{
		Path:      path,
		SessionID: "test-session",
		Project:   "test-project",
	}
}

func TestParseFile_UserMessages(t *testing.T) {
	df := writeSession(t,
		`{"type":"user","timestamp":"2025-06-01T10:00:00Z","cwd":"/tmp/proj"}`,
		`{"type":"user","timestamp":"2025-06-01T10:05:00Z"}`,
		`{"type":"user","timestamp":"2025-06-01T10:10:00Z"}`,
	)

	result := ParseFile(df)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if result.Stats.UserMessages != 3 {
		t.Errorf("UserMessages = %d, want 3", result.Stats.UserMessages)
	}
	if result.Stats.ProjectPath != "/tmp/proj" {
		t.Errorf("ProjectPath = %q, want /tmp/proj", result.Stats.ProjectPath)
	}
}

func TestParseFile_AssistantDedup(t *testing.T) {
	// Two entries with same message ID — second should win (deduplication).
	df := writeSession(t,
		`{"type":"assistant","timestamp":"2025-06-01T10:00:00Z","message":{"id":"msg1","model":"claude-sonnet-4-6-20250514","usage":{"input_tokens":100,"output_tokens":50}}}`,
		`{"type":"assistant","timestamp":"2025-06-01T10:00:01Z","message":{"id":"msg1","model":"claude-sonnet-4-6-20250514","usage":{"input_tokens":200,"output_tokens":80}}}`,
	)

	result := ParseFile(df)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if result.Stats.APICalls != 1 {
		t.Errorf("APICalls = %d, want 1 (dedup)", result.Stats.APICalls)
	}
	if result.Stats.InputTokens != 200 {
		t.Errorf("InputTokens = %d, want 200 (last wins)", result.Stats.InputTokens)
	}
	if result.Stats.OutputTokens != 80 {
		t.Errorf("OutputTokens = %d, want 80 (last wins)", result.Stats.OutputTokens)
	}
}

func TestParseFile_TimeRange(t *testing.T) {
	df := writeSession(t,
		`{"type":"user","timestamp":"2025-06-01T08:00:00Z"}`,
		`{"type":"user","timestamp":"2025-06-01T12:00:00Z"}`,
		`{"type":"user","timestamp":"2025-06-01T10:00:00Z"}`,
	)

	result := ParseFile(df)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	wantStart := time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	if !result.Stats.StartTime.Equal(wantStart) {
		t.Errorf("StartTime = %v, want %v", result.Stats.StartTime, wantStart)
	}
	if !result.Stats.EndTime.Equal(wantEnd) {
		t.Errorf("EndTime = %v, want %v", result.Stats.EndTime, wantEnd)
	}
}

func TestParseFile_SystemDuration(t *testing.T) {
	df := writeSession(t,
		`{"type":"system","subtype":"turn_duration","timestamp":"2025-06-01T10:00:00Z","durationMs":5000}`,
		`{"type":"system","subtype":"turn_duration","timestamp":"2025-06-01T10:01:00Z","durationMs":3000}`,
	)

	result := ParseFile(df)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	if result.Stats.DurationSecs != 8 { // (5000+3000)/1000
		t.Errorf("DurationSecs = %d, want 8", result.Stats.DurationSecs)
	}
}

func TestParseFile_EmptyFile(t *testing.T) {
	df := writeSession(t)
	result := ParseFile(df)
	if result.Err != nil {
		t.Fatalf("unexpected error on empty file: %v", result.Err)
	}
	if result.Stats.UserMessages != 0 || result.Stats.APICalls != 0 {
		t.Error("expected zero stats for empty file")
	}
}

func TestParseFile_MalformedLines(t *testing.T) {
	df := writeSession(t,
		`not json at all`,
		`{"type":"user","timestamp":"2025-06-01T10:00:00Z"}`,
		`{"type":"assistant","broken json`,
	)

	result := ParseFile(df)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	// Malformed lines should be skipped, not cause a fatal error.
	if result.Stats.UserMessages != 1 {
		t.Errorf("UserMessages = %d, want 1", result.Stats.UserMessages)
	}
}

func TestParseFile_CacheTokens(t *testing.T) {
	df := writeSession(t,
		`{"type":"assistant","timestamp":"2025-06-01T10:00:00Z","message":{"id":"msg1","model":"claude-sonnet-4-6","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":500,"cache_creation":{"ephemeral_5m_input_tokens":200,"ephemeral_1h_input_tokens":300}}}}`,
	)

	result := ParseFile(df)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}

	s := result.Stats
	if s.CacheReadTokens != 500 {
		t.Errorf("CacheReadTokens = %d, want 500", s.CacheReadTokens)
	}
	if s.CacheCreation5mTokens != 200 {
		t.Errorf("CacheCreation5mTokens = %d, want 200", s.CacheCreation5mTokens)
	}
	if s.CacheCreation1hTokens != 300 {
		t.Errorf("CacheCreation1hTokens = %d, want 300", s.CacheCreation1hTokens)
	}
}

func TestExtractTopLevelType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"user", `{"type":"user","foo":"bar"}`, "user"},
		{"assistant", `{"type":"assistant","message":{}}`, "assistant"},
		{"system", `{"type": "system","subtype":"turn_duration"}`, "system"},
		{"nested type ignored", `{"data":{"type":"progress"},"type":"user"}`, "user"},
		{"unknown type", `{"type":"progress","data":{}}`, ""},
		{"no type field", `{"message":"hello"}`, ""},
		{"empty", `{}`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTopLevelType([]byte(tt.input))
			if got != tt.want {
				t.Errorf("extractTopLevelType(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// FuzzExtractTopLevelType tests that the byte-level parser never panics
// on arbitrary input, which is important since it processes untrusted files.
func FuzzExtractTopLevelType(f *testing.F) {
	// Seed corpus with realistic patterns
	f.Add([]byte(`{"type":"user","timestamp":"2025-06-01T10:00:00Z"}`))
	f.Add([]byte(`{"type":"assistant","message":{"id":"x","usage":{}}}`))
	f.Add([]byte(`{"type":"system","subtype":"turn_duration","durationMs":5000}`))
	f.Add([]byte(`{"data":{"type":"nested"},"type":"user"}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"type":null}`))
	f.Add([]byte(`{"type":123}`))
	f.Add([]byte(``))
	f.Add([]byte(`{"type":"user`)) // unterminated string

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic
		result := extractTopLevelType(data)

		// Result must be one of the known types or empty
		switch result {
		case "", "user", "assistant", "system":
			// ok
		default:
			t.Errorf("unexpected type %q from input %q", result, data)
		}
	})
}
