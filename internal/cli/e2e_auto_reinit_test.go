//go:build e2e

package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/beabys/ilnamiqui/internal/service"
	_ "modernc.org/sqlite"
)

// TestE2E_AutoReinitOnLoad verifies that Load() auto-creates missing tables
// when the existing DB only has the old schema (sessions + memory_entries).
func TestE2E_AutoReinitOnLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// 1. Create temp dir and chdir
	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// 2. Create .ilnamiqui/ dir
	ilnamiquiDir := filepath.Join(dir, ".ilnamiqui")
	if err := os.MkdirAll(ilnamiquiDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// 3. Create old DB with ONLY sessions + memory_entries (version 0 schema)
	dbPath := filepath.Join(ilnamiquiDir, "ilnamiqui.db")
	oldDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	// Build version-0 schema: sessions + memory_entries + indexes only
	// No schema_versions, no memory_fts, no memory_keys
	schemaV0 := `
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
	if _, err := oldDB.Exec(schemaV0); err != nil {
		oldDB.Close()
		t.Fatalf("create old schema: %v", err)
	}

	// Insert system session (required by memory_entries FK)
	if _, err := oldDB.Exec(`INSERT INTO sessions (id, project, started_at) VALUES ('00000000-0000-0000-0000-000000000000', '_system', '1970-01-01T00:00:00Z')`); err != nil {
		oldDB.Close()
		t.Fatalf("insert system session: %v", err)
	}

	// 4. Insert a test memory entry
	if _, err := oldDB.Exec(`INSERT INTO memory_entries (session_id, key, value, created_at) VALUES ('00000000-0000-0000-0000-000000000000', 'test-key', 'test-value', '2026-01-01T00:00:00Z')`); err != nil {
		oldDB.Close()
		t.Fatalf("insert test entry: %v", err)
	}
	oldDB.Close()

	// 5. Write .initialized sentinel file (simulate already-initialized project)
	sentinelPath := filepath.Join(ilnamiquiDir, ".initialized")
	if err := os.WriteFile(sentinelPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	// 6. Create service and CLI, run load --pretty
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })

	cli := New(svc)

	// First load — should auto-reinit and succeed
	if err := cli.Run([]string{"load", "--pretty"}); err != nil {
		t.Fatalf("first load --pretty failed: %v", err)
	}

	// 7. Verify new tables exist and data is preserved
	verifyDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer verifyDB.Close()

	// schema_versions table exists with version 1
	var versionCount int
	if err := verifyDB.QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&versionCount); err != nil {
		t.Fatalf("query schema_versions: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("expected 1 row in schema_versions, got %d", versionCount)
	}

	var appliedVersion int
	if err := verifyDB.QueryRow("SELECT MAX(version) FROM schema_versions").Scan(&appliedVersion); err != nil {
		t.Fatalf("query max version: %v", err)
	}
	if appliedVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", appliedVersion)
	}

	// memory_fts table exists
	var ftsCount int
	if err := verifyDB.QueryRow("SELECT COUNT(*) FROM memory_fts").Scan(&ftsCount); err != nil {
		t.Fatalf("query memory_fts: %v", err)
	}
	if ftsCount != 1 {
		t.Fatalf("expected 1 row in memory_fts (backfilled from test entry), got %d", ftsCount)
	}

	// memory_keys table exists with backfilled data
	var keyCount int
	if err := verifyDB.QueryRow("SELECT COUNT(*) FROM memory_keys").Scan(&keyCount); err != nil {
		t.Fatalf("query memory_keys: %v", err)
	}
	if keyCount != 1 {
		t.Fatalf("expected 1 row in memory_keys (backfilled from test entry), got %d", keyCount)
	}

	// Test memory entry still readable
	var value string
	if err := verifyDB.QueryRow("SELECT value FROM memory_entries WHERE key = 'test-key'").Scan(&value); err != nil {
		t.Fatalf("query test entry: %v", err)
	}
	if value != "test-value" {
		t.Fatalf("expected test-value, got %s", value)
	}

	// 8. Run load again — should be idempotent
	if err := cli.Run([]string{"load", "--pretty"}); err != nil {
		t.Fatalf("second load --pretty failed: %v", err)
	}

	// 9. Verify still only 1 record in schema_versions (second load didn't duplicate)
	if err := verifyDB.QueryRow("SELECT COUNT(*) FROM schema_versions").Scan(&versionCount); err != nil {
		t.Fatalf("query schema_versions after second load: %v", err)
	}
	if versionCount != 1 {
		t.Fatalf("expected 1 row in schema_versions after second load, got %d", versionCount)
	}
}
