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

CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
    key, value,
    content=memory_entries,
    content_rowid=id,
    tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS memory_fts_ai AFTER INSERT ON memory_entries BEGIN
    INSERT INTO memory_fts (rowid, key, value) VALUES (new.id, new.key, new.value);
END;

CREATE TRIGGER IF NOT EXISTS memory_fts_ad AFTER DELETE ON memory_entries BEGIN
    INSERT INTO memory_fts (memory_fts, rowid, key, value) VALUES ('delete', old.id, old.key, old.value);
END;

CREATE TRIGGER IF NOT EXISTS memory_fts_au AFTER UPDATE ON memory_entries BEGIN
    INSERT INTO memory_fts (memory_fts, rowid, key, value) VALUES ('delete', old.id, old.key, old.value);
    INSERT INTO memory_fts (rowid, key, value) VALUES (new.id, new.key, new.value);
END;

CREATE TABLE IF NOT EXISTS memory_keys (
    key          TEXT PRIMARY KEY,
    last_used_at TEXT NOT NULL,
    critical     INTEGER NOT NULL DEFAULT 0
);

CREATE TRIGGER IF NOT EXISTS memory_keys_ai AFTER INSERT ON memory_entries BEGIN
    INSERT INTO memory_keys (key, last_used_at) 
    VALUES (new.key, new.created_at)
    ON CONFLICT(key) DO UPDATE SET last_used_at = excluded.last_used_at;
END;

INSERT OR IGNORE INTO sessions (id, project, started_at, ended_at, summary)
VALUES ('00000000-0000-0000-0000-000000000000', '_system', '1970-01-01T00:00:00Z', '1970-01-01T00:00:00Z', 'system session for internal entries');
`

// RunMigrations executes all CREATE TABLE / INDEX statements idempotently.
func RunMigrations(db *sql.DB) error {
	if _, err := db.Exec(migrationsSQL); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Backfill FTS5 with existing memory entries (idempotent).
	var ftsCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM memory_fts").Scan(&ftsCount); err != nil {
		return fmt.Errorf("check fts5 count: %w", err)
	}
	if ftsCount == 0 {
		if _, err := db.Exec(`INSERT INTO memory_fts (rowid, key, value) SELECT id, key, value FROM memory_entries`); err != nil {
			return fmt.Errorf("backfill fts5: %w", err)
		}
	}

	// Backfill memory_keys with existing distinct keys (idempotent).
	var keyCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM memory_keys").Scan(&keyCount); err != nil {
		return fmt.Errorf("check memory_keys count: %w", err)
	}
	if keyCount == 0 {
		if _, err := db.Exec(`INSERT INTO memory_keys (key, last_used_at) SELECT key, MAX(created_at) FROM memory_entries GROUP BY key`); err != nil {
			return fmt.Errorf("backfill memory_keys: %w", err)
		}
	}

	return nil
}
