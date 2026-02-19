package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS sessions (
    session_id           TEXT PRIMARY KEY,
    project              TEXT NOT NULL,
    project_path         TEXT,
    file_path            TEXT NOT NULL,
    is_subagent          INTEGER NOT NULL DEFAULT 0,
    parent_session       TEXT,
    start_time           TEXT,
    end_time             TEXT,
    duration_secs        INTEGER,
    user_messages        INTEGER,
    api_calls            INTEGER,
    input_tokens         INTEGER,
    output_tokens        INTEGER,
    cache_creation_5m    INTEGER,
    cache_creation_1h    INTEGER,
    cache_read_tokens    INTEGER,
    estimated_cost       REAL,
    cache_hit_rate       REAL,
    file_mtime_ns        INTEGER NOT NULL,
    file_size            INTEGER NOT NULL,
    parsed_at            TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS session_models (
    session_id           TEXT NOT NULL REFERENCES sessions(session_id) ON DELETE CASCADE,
    model                TEXT NOT NULL,
    api_calls            INTEGER,
    input_tokens         INTEGER,
    output_tokens        INTEGER,
    cache_creation_5m    INTEGER,
    cache_creation_1h    INTEGER,
    cache_read_tokens    INTEGER,
    estimated_cost       REAL,
    PRIMARY KEY (session_id, model)
);

CREATE TABLE IF NOT EXISTS file_tracker (
    file_path            TEXT PRIMARY KEY,
    mtime_ns             INTEGER NOT NULL,
    size_bytes           INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_start ON sessions(start_time);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project);
`
