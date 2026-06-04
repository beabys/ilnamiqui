package db

import (
	"database/sql"
	"fmt"
)

// ─── Migration System ─────────────────────────────────────────────────────
//
// How to add a new migration version:
//
//  1. Create a `const versionN` string with your DDL (always use IF NOT EXISTS).
//  2. Append a migration{} entry to the `migrations` slice with the SQL + desc.
//  3. If Go post-tasks needed, add `case N:` to `runPostMigrationV`.
//
//  Example:
//
//    const version2 = `ALTER TABLE sessions ADD COLUMN user_agent TEXT DEFAULT '';`
//    var migrations = append(migrations, migration{
//        sql:  version2,
//        desc: "add user_agent to sessions",
//    })
//    // runPostMigrationV: case 2 → runPostMigrationV2(db)
//
//  No const to bump — LatestVersion() reads len(migrations) automatically.
//
// Safety: each version is recorded in `schema_versions` immediately after its
// SQL + post-tasks succeed. If vN+1 fails, vN is already committed.
//
// ────────────────────────────────────────────────────────────────────────────

// versions are immutable. Never edit existing versions — only append new ones.
// Each version must be idempotent (CREATE IF NOT EXISTS).

const version2 = `
ALTER TABLE sessions ADD COLUMN agent TEXT NOT NULL DEFAULT 'opencode';
UPDATE sessions SET agent = '_system' WHERE id = '00000000-0000-0000-0000-000000000000';
`
const version1 = `
CREATE TABLE IF NOT EXISTS schema_versions (
    version     INTEGER PRIMARY KEY,
    applied_at  TEXT NOT NULL DEFAULT (datetime('now')),
    description TEXT NOT NULL DEFAULT ''
);

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

// migration pairs SQL DDL with a human-readable description.
// Index in the slice = version number (1-indexed). Never remove or reorder entries.
type migration struct {
	sql  string
	desc string
}

// migrations is the ordered list of all schema migrations. Append new versions
// at the end — never edit or remove existing entries.
var migrations = []migration{
	{
		sql:  version1,
		desc: "initial schema (sessions, memory_entries, memory_fts, memory_keys, triggers, indexes)",
	},
	{
		sql:  version2,
		desc: "add agent column to sessions",
	},
}

// LatestVersion returns the latest schema version, derived from the number of
// migration entries. Appending to migrations automatically updates this.
func LatestVersion() int {
	return len(migrations)
}

// RunMigrations executes migrations idempotently, tracking version in schema_versions.
// Each version is recorded immediately after its SQL + post-tasks succeed, so partial
// upgrades are safe: if v2 fails, v1 is already committed and won't re-run.
func RunMigrations(db *sql.DB) error {
	// Ensure schema_versions table exists first
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_versions (
		version     INTEGER PRIMARY KEY,
		applied_at  TEXT NOT NULL DEFAULT (datetime('now')),
		description TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		return fmt.Errorf("create schema_versions: %w", err)
	}

	// Check current applied version
	var currentVersion int
	if err := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_versions").Scan(&currentVersion); err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	latest := LatestVersion()
	if currentVersion >= latest {
		return nil // Already up to date
	}

	for v := currentVersion + 1; v <= latest; v++ {
		m := migrations[v-1] // version is 1-indexed
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("run migration v%d: %w", v, err)
		}

		// Version-specific post-migration tasks
		if err := runPostMigrationV(v, db); err != nil {
			return fmt.Errorf("post-migration v%d: %w", v, err)
		}

		// Record version immediately — if next version fails, this one is saved
		if _, err := db.Exec(`INSERT OR IGNORE INTO schema_versions (version, description) VALUES (?, ?)`, v, m.desc); err != nil {
			return fmt.Errorf("record schema version %d: %w", v, err)
		}
	}

	return nil
}

// runPostMigrationV handles version-specific tasks (e.g., data backfills) that run
// after the SQL migration but before the version is recorded.
//
// To add post-tasks for a new version: add a `case N:` and implement the
// corresponding `runPostMigrationVN` function.
func runPostMigrationV(v int, db *sql.DB) error {
	switch v {
	case 1:
		return runPostMigrationV1(db)
	case 2:
		return runPostMigrationV2(db)
	default:
		return nil
	}
}

func runPostMigrationV2(db *sql.DB) error {
	// No post-tasks for v2 yet. Placeholder for future use.
	_ = db
	return nil
}

func runPostMigrationV1(db *sql.DB) error {
	// Backfill FTS5 with existing entries (idempotent)
	var ftsCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM memory_fts").Scan(&ftsCount); err != nil {
		return fmt.Errorf("check fts5 count: %w", err)
	}
	if ftsCount == 0 {
		if _, err := db.Exec(`INSERT INTO memory_fts (rowid, key, value) SELECT id, key, value FROM memory_entries`); err != nil {
			return fmt.Errorf("backfill fts5: %w", err)
		}
	}

	// Backfill memory_keys with existing distinct keys (idempotent)
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
