package db

import (
	"database/sql"
	"fmt"
)

const migrationsSQL = `
CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    project    TEXT NOT NULL,
    started_at TEXT NOT NULL,
    ended_at   TEXT,
    summary    TEXT DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS memory_entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_memory_session_id ON memory_entries(session_id);
CREATE INDEX IF NOT EXISTS idx_memory_key ON memory_entries(key);
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project);
`

// RunMigrations executes all CREATE TABLE / INDEX statements idempotently.
func RunMigrations(db *sql.DB) error {
	if _, err := db.Exec(migrationsSQL); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
