package memory

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestDB creates an in-memory SQLite database with migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	// Create schema manually (not calling db.RunMigrations to keep test self-contained)
	const schema = `
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
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	// Insert a stub session
	if _, err := db.Exec(`INSERT INTO sessions (id, project, started_at) VALUES (?, ?, ?)`,
		"test-session", "test-project", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("insert stub session: %v", err)
	}

	return db
}

func TestSaveEntry(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	ctx := context.Background()
	entry, err := store.SaveEntry(ctx, "test-session", "key1", "value1")
	if err != nil {
		t.Fatalf("SaveEntry error: %v", err)
	}

	if entry.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if entry.Key != "key1" {
		t.Fatalf("expected key 'key1', got %q", entry.Key)
	}
	if entry.Value != "value1" {
		t.Fatalf("expected value 'value1', got %q", entry.Value)
	}
	if entry.SessionID != "test-session" {
		t.Fatalf("expected session 'test-session', got %q", entry.SessionID)
	}
}

func TestLoadEntries(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Save two entries
	_, err := store.SaveEntry(ctx, "test-session", "a", "1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "b", "2")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := store.LoadEntries(ctx, "test-session")
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Entries should be ordered by created_at DESC, so "b" should be first
	if entries[0].Key != "b" {
		t.Fatalf("expected first entry key 'b', got %q", entries[0].Key)
	}
}

func TestLoadEntries_empty(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	entries, err := store.LoadEntries(ctx, "nonexistent-session")
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestLoadAllEntries(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Insert a second session
	if _, err := db.Exec(`INSERT INTO sessions (id, project, started_at) VALUES (?, ?, ?)`,
		"test-session-2", "test-project", "2024-01-02T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	_, err := store.SaveEntry(ctx, "test-session", "a", "1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session-2", "b", "2")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := store.LoadAllEntries(ctx)
	if err != nil {
		t.Fatalf("LoadAllEntries error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestSearchEntries(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.SaveEntry(ctx, "test-session", "architecture", "using hexagonal architecture")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "bug", "fix null pointer in handler")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "decision", "use SQLite for memory")
	if err != nil {
		t.Fatal(err)
	}

	// Search by key
	entries, err := store.SearchEntries(ctx, "architecture")
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'architecture', got %d", len(entries))
	}

	// Search by value
	entries, err = store.SearchEntries(ctx, "hexagonal")
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'hexagonal', got %d", len(entries))
	}

	// Search matching both key and value
	entries, err = store.SearchEntries(ctx, "SQLite")
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'SQLite', got %d", len(entries))
	}

	// Search with no match
	entries, err = store.SearchEntries(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestDeleteEntry(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	entry, err := store.SaveEntry(ctx, "test-session", "delete-me", "value")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatalf("DeleteEntry error: %v", err)
	}

	// Verify deleted
	entries, err := store.LoadEntries(ctx, "test-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", len(entries))
	}
}

func TestDeleteEntry_notFound(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	err := store.DeleteEntry(ctx, 99999)
	if err == nil {
		t.Fatal("expected error for deleting non-existent entry")
	}
}

func TestDuplicateKeys(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Same key can have multiple entries — no unique constraint on (session_id, key)
	e1, err := store.SaveEntry(ctx, "test-session", "duplicate", "first")
	if err != nil {
		t.Fatal(err)
	}
	e2, err := store.SaveEntry(ctx, "test-session", "duplicate", "second")
	if err != nil {
		t.Fatal(err)
	}

	if e1.ID == e2.ID {
		t.Fatal("expected different IDs for duplicate key entries")
	}

	entries, err := store.LoadEntries(ctx, "test-session")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with same key, got %d", len(entries))
	}
}

func TestForeignKeys(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Try saving with invalid session ID — should fail
	_, err := store.SaveEntry(ctx, "nonexistent-session", "key", "value")
	if err == nil {
		t.Fatal("expected foreign key error for invalid session ID")
	}
}

func TestContextCancellation(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := store.SaveEntry(ctx, "test-session", "key", "value")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestSQLInjection(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Attempt SQL injection through key and value
	_, err := store.SaveEntry(ctx, "test-session", "'; DROP TABLE memory_entries; --", "malicious")
	if err != nil {
		t.Fatalf("SaveEntry error: %v", err)
	}

	// Verify table still exists
	entries, err := store.LoadAllEntries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d — table may have been dropped", len(entries))
	}
}

// File-based test helper for integration
func fileDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("enable WAL: %v", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		t.Fatalf("set busy timeout: %v", err)
	}
	db.SetMaxOpenConns(1)

	const schema = `
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
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (id, project, started_at) VALUES (?, ?, ?)`,
		"test-session", "test-project", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("insert stub session: %v", err)
	}
	return db
}
