// Package store provides a SQLite-backed cache for parsed session data.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cburn/internal/model"

	_ "modernc.org/sqlite" // register sqlite driver
)

// Cache provides SQLite-backed session caching.
type Cache struct {
	db *sql.DB
}

// Open opens or creates the cache database at the given path.
func Open(dbPath string) (*Cache, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating cache dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(wal)&_pragma=synchronous(normal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("opening cache db: %w", err)
	}

	if _, err := db.Exec(schemaSQL); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &Cache{db: db}, nil
}

// Close closes the cache database.
func (c *Cache) Close() error {
	return c.db.Close()
}

// FileInfo holds the tracked mtime and size for a file.
type FileInfo struct {
	MtimeNs   int64
	SizeBytes int64
}

// GetTrackedFiles returns a map of file_path -> FileInfo for all tracked files.
func (c *Cache) GetTrackedFiles() (map[string]FileInfo, error) {
	rows, err := c.db.Query("SELECT file_path, mtime_ns, size_bytes FROM file_tracker")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]FileInfo)
	for rows.Next() {
		var path string
		var fi FileInfo
		if err := rows.Scan(&path, &fi.MtimeNs, &fi.SizeBytes); err != nil {
			return nil, err
		}
		result[path] = fi
	}
	return result, rows.Err()
}

// SaveSession stores a parsed session and its file tracking info.
func (c *Cache) SaveSession(s model.SessionStats, mtimeNs, sizeBytes int64) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339)
	startTime := ""
	if !s.StartTime.IsZero() {
		startTime = s.StartTime.UTC().Format(time.RFC3339)
	}
	endTime := ""
	if !s.EndTime.IsZero() {
		endTime = s.EndTime.UTC().Format(time.RFC3339)
	}

	isSubagent := 0
	if s.IsSubagent {
		isSubagent = 1
	}

	_, err = tx.Exec(`INSERT OR REPLACE INTO sessions
		(session_id, project, project_path, file_path, is_subagent, parent_session,
		 start_time, end_time, duration_secs, user_messages, api_calls,
		 input_tokens, output_tokens, cache_creation_5m, cache_creation_1h,
		 cache_read_tokens, estimated_cost, cache_hit_rate, file_mtime_ns, file_size, parsed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SessionID, s.Project, s.ProjectPath, s.FilePath, isSubagent, s.ParentSession,
		startTime, endTime, s.DurationSecs, s.UserMessages, s.APICalls,
		s.InputTokens, s.OutputTokens, s.CacheCreation5mTokens, s.CacheCreation1hTokens,
		s.CacheReadTokens, s.EstimatedCost, s.CacheHitRate, mtimeNs, sizeBytes, now,
	)
	if err != nil {
		return err
	}

	// Delete old model entries for this session
	_, err = tx.Exec("DELETE FROM session_models WHERE session_id = ?", s.SessionID)
	if err != nil {
		return err
	}

	// Insert model entries
	for modelName, mu := range s.Models {
		_, err = tx.Exec(`INSERT INTO session_models
			(session_id, model, api_calls, input_tokens, output_tokens,
			 cache_creation_5m, cache_creation_1h, cache_read_tokens, estimated_cost)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.SessionID, modelName, mu.APICalls, mu.InputTokens, mu.OutputTokens,
			mu.CacheCreation5mTokens, mu.CacheCreation1hTokens, mu.CacheReadTokens, mu.EstimatedCost,
		)
		if err != nil {
			return err
		}
	}

	// Update file tracker
	_, err = tx.Exec(`INSERT OR REPLACE INTO file_tracker (file_path, mtime_ns, size_bytes)
		VALUES (?, ?, ?)`, s.FilePath, mtimeNs, sizeBytes)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// LoadAllSessions reads all cached sessions from the database.
func (c *Cache) LoadAllSessions() ([]model.SessionStats, error) {
	rows, err := c.db.Query(`SELECT
		session_id, project, project_path, file_path, is_subagent, parent_session,
		start_time, end_time, duration_secs, user_messages, api_calls,
		input_tokens, output_tokens, cache_creation_5m, cache_creation_1h,
		cache_read_tokens, estimated_cost, cache_hit_rate
		FROM sessions`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []model.SessionStats
	for rows.Next() {
		var s model.SessionStats
		var startStr, endStr, parentSession, projectPath sql.NullString
		var isSubagent int

		err := rows.Scan(
			&s.SessionID, &s.Project, &projectPath, &s.FilePath, &isSubagent, &parentSession,
			&startStr, &endStr, &s.DurationSecs, &s.UserMessages, &s.APICalls,
			&s.InputTokens, &s.OutputTokens, &s.CacheCreation5mTokens, &s.CacheCreation1hTokens,
			&s.CacheReadTokens, &s.EstimatedCost, &s.CacheHitRate,
		)
		if err != nil {
			return nil, err
		}

		s.IsSubagent = isSubagent != 0
		if parentSession.Valid {
			s.ParentSession = parentSession.String
		}
		if projectPath.Valid {
			s.ProjectPath = projectPath.String
		}
		if startStr.Valid && startStr.String != "" {
			s.StartTime, _ = time.Parse(time.RFC3339, startStr.String)
		}
		if endStr.Valid && endStr.String != "" {
			s.EndTime, _ = time.Parse(time.RFC3339, endStr.String)
		}

		// Load model breakdown
		s.Models = make(map[string]*model.ModelUsage)
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Batch-load model data
	modelRows, err := c.db.Query(`SELECT
		session_id, model, api_calls, input_tokens, output_tokens,
		cache_creation_5m, cache_creation_1h, cache_read_tokens, estimated_cost
		FROM session_models`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelRows.Close() }()

	// Build session index for fast lookup
	sessionIdx := make(map[string]int)
	for i, s := range sessions {
		sessionIdx[s.SessionID] = i
	}

	for modelRows.Next() {
		var sid, modelName string
		var mu model.ModelUsage
		err := modelRows.Scan(&sid, &modelName, &mu.APICalls, &mu.InputTokens, &mu.OutputTokens,
			&mu.CacheCreation5mTokens, &mu.CacheCreation1hTokens, &mu.CacheReadTokens, &mu.EstimatedCost)
		if err != nil {
			return nil, err
		}
		if idx, ok := sessionIdx[sid]; ok {
			sessions[idx].Models[modelName] = &mu
		}
	}

	return sessions, modelRows.Err()
}

// DeleteSession removes a session and its associated data.
func (c *Cache) DeleteSession(sessionID string) error {
	_, err := c.db.Exec("DELETE FROM sessions WHERE session_id = ?", sessionID)
	return err
}

// DeleteFileTracker removes a file tracking entry.
func (c *Cache) DeleteFileTracker(filePath string) error {
	_, err := c.db.Exec("DELETE FROM file_tracker WHERE file_path = ?", filePath)
	return err
}

// SessionCount returns the number of cached sessions.
func (c *Cache) SessionCount() (int, error) {
	var count int
	err := c.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	return count, err
}
