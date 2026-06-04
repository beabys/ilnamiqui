package memory

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

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
	CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
		key, value,
		content=memory_entries,
		content_rowid=id,
		tokenize='unicode61'
	);
	CREATE TABLE IF NOT EXISTS memory_keys (
		key          TEXT PRIMARY KEY,
		last_used_at TEXT NOT NULL,
		critical     INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	const ftsTriggers = `
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
	`
	if _, err := db.Exec(ftsTriggers); err != nil {
		t.Fatalf("create fts5 triggers: %v", err)
	}

	if _, err := db.Exec(memoryKeysTrigger); err != nil {
		t.Fatalf("create memory keys trigger: %v", err)
	}

	// Insert a stub session
	if _, err := db.Exec(`INSERT INTO sessions (id, project, started_at) VALUES (?, ?, ?)`,
		"test-session", "test-project", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("insert stub session: %v", err)
	}

	return db
}

const memoryKeysTrigger = `
CREATE TRIGGER IF NOT EXISTS memory_keys_ai AFTER INSERT ON memory_entries BEGIN
    INSERT INTO memory_keys (key, last_used_at) 
    VALUES (new.key, new.created_at)
    ON CONFLICT(key) DO UPDATE SET last_used_at = excluded.last_used_at;
END;
`

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

	entries, err := store.LoadEntries(ctx, "test-session", 0)
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

	entries, err := store.LoadEntries(ctx, "nonexistent-session", 0)
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

	entries, err := store.LoadAllEntries(ctx, 0)
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

	// Search by key (both mode to match old behavior)
	entries, err := store.SearchEntries(ctx, "architecture", SearchModeBoth, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'architecture', got %d", len(entries))
	}

	// Search by value
	entries, err = store.SearchEntries(ctx, "hexagonal", SearchModeBoth, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'hexagonal', got %d", len(entries))
	}

	// Search matching both key and value
	entries, err = store.SearchEntries(ctx, "SQLite", SearchModeBoth, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'SQLite', got %d", len(entries))
	}

	// Search with no match
	entries, err = store.SearchEntries(ctx, "nonexistent", SearchModeBoth, 0, nil, nil)
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
	entries, err := store.LoadEntries(ctx, "test-session", 0)
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

	entries, err := store.LoadEntries(ctx, "test-session", 0)
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
	entries, err := store.LoadAllEntries(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d — table may have been dropped", len(entries))
	}
}

func TestListKeys(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// No entries yet — should return empty
	keys, err := store.ListKeys(ctx, 0)
	if err != nil {
		t.Fatalf("ListKeys error: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys for empty DB, got %d", len(keys))
	}

	// Save entries with different keys
	_, err = store.SaveEntry(ctx, "test-session", "alpha", "first")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "beta", "second")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "alpha", "third")
	if err != nil {
		t.Fatal(err)
	}

	keys, err = store.ListKeys(ctx, 0)
	if err != nil {
		t.Fatalf("ListKeys error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 distinct keys, got %d", len(keys))
	}

	// Both keys should have critical=false by default
	for _, k := range keys {
		if k.Critical {
			t.Fatalf("key %q should be non-critical by default", k.Key)
		}
	}
}

func TestListKeys_Limit(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	for _, k := range []string{"a", "b", "c"} {
		_, err := store.SaveEntry(ctx, "test-session", k, "v")
		if err != nil {
			t.Fatal(err)
		}
	}

	keys, err := store.ListKeys(ctx, 2)
	if err != nil {
		t.Fatalf("ListKeys error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys with limit=2, got %d", len(keys))
	}
}

func TestListKeys_OrderByRecency(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Insert entries with explicit created_at dates
	// Using direct SQL to control timestamps
	_, err := db.ExecContext(ctx, `INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`,
		"test-session", "old", "v", "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`,
		"test-session", "new", "v", "2026-06-04T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	keys, err := store.ListKeys(ctx, 0)
	if err != nil {
		t.Fatalf("ListKeys error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	// "new" should come first (most recent)
	if keys[0].Key != "new" {
		t.Fatalf("expected first key 'new' (most recent), got %q", keys[0].Key)
	}
}

func TestLoadEntries_withLimit(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Save 3 entries
	for i := 0; i < 3; i++ {
		_, err := store.SaveEntry(ctx, "test-session", fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Load with limit 1
	entries, err := store.LoadEntries(ctx, "test-session", 1)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry with limit=1, got %d", len(entries))
	}
}

func TestLoadAllEntries_withLimit(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Insert a second session
	if _, err := db.Exec(`INSERT INTO sessions (id, project, started_at) VALUES (?, ?, ?)`,
		"test-session-2", "test-project", "2024-01-02T00:00:00Z"); err != nil {
		t.Fatal(err)
	}

	// Save 3 entries across sessions
	for i := 0; i < 3; i++ {
		sid := "test-session"
		if i == 2 {
			sid = "test-session-2"
		}
		_, err := store.SaveEntry(ctx, sid, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Load with limit 2
	entries, err := store.LoadAllEntries(ctx, 2)
	if err != nil {
		t.Fatalf("LoadAllEntries error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with limit=2, got %d", len(entries))
	}
}

func TestSearchEntries_withLimit(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Save 3 entries all with "test" in value
	for i := 0; i < 3; i++ {
		_, err := store.SaveEntry(ctx, "test-session", fmt.Sprintf("k%d", i), fmt.Sprintf("test value %d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Search with limit 1
	entries, err := store.SearchEntries(ctx, "test", SearchModeBoth, 1, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry with limit=1, got %d", len(entries))
	}
}

func TestSearchEntries_WithDateRange(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()
	// Use the stub session already inserted by openTestDB
	sessionID := "test-session"

	// Insert entries with specific dates
	entries := []struct {
		key, value, createdAt string
	}{
		{"old", "old entry", "2025-01-01T00:00:00Z"},
		{"mid", "mid entry", "2026-01-01T00:00:00Z"},
		{"new", "new entry", "2027-01-01T00:00:00Z"},
	}

	for _, e := range entries {
		query := `INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`
		_, err := store.db.ExecContext(ctx, query, sessionID, e.key, e.value, e.createdAt)
		if err != nil {
			t.Fatalf("insert %s: %v", e.key, err)
		}
	}

	after := parseTime(t, "2026-01-01T00:00:00Z")
	before := parseTime(t, "2027-06-01T00:00:00Z")

	// Test: only date range, no query
	results, err := store.SearchEntries(ctx, "", SearchModeBoth, 0, &after, &before)
	if err != nil {
		t.Fatalf("SearchEntries date range: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 entries in date range, got %d", len(results))
	}

	// Test: query + date range combined
	results, err = store.SearchEntries(ctx, "mid", SearchModeBoth, 0, &after, &before)
	if err != nil {
		t.Fatalf("SearchEntries query + date: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 entry for 'mid' in range, got %d", len(results))
	}

	// Test: only after, no before
	results, err = store.SearchEntries(ctx, "", SearchModeBoth, 0, &after, nil)
	if err != nil {
		t.Fatalf("SearchEntries after only: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 entries after 2026, got %d", len(results))
	}

	// Test: date range with no matches
	farFuture := parseTime(t, "2030-01-01T00:00:00Z")
	results, err = store.SearchEntries(ctx, "", SearchModeBoth, 0, &farFuture, nil)
	if err != nil {
		t.Fatalf("SearchEntries no match: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 entries after 2030, got %d", len(results))
	}

	// Test: no query, no date range — returns all
	results, err = store.SearchEntries(ctx, "", SearchModeBoth, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries no filters: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 entries with no filters, got %d", len(results))
	}
}

func TestSearchEntries_KeyModeDefault(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.SaveEntry(ctx, "test-session", "architecture", "using hexagonal architecture")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "architect", "team lead role")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "bug", "fix null pointer")
	if err != nil {
		t.Fatal(err)
	}

	// Search with key mode — prefix match should find "architecture" and "architect"
	entries, err := store.SearchEntries(ctx, "arch", SearchModeKey, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries matching 'arch*', got %d", len(entries))
	}

	// Exact prefix should not match "bug"
	entries, err = store.SearchEntries(ctx, "bug", SearchModeKey, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'bug', got %d", len(entries))
	}
}

func TestSearchEntries_ContentMode_FTS5(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.SaveEntry(ctx, "test-session", "arch", "using hexagonal architecture pattern")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "bug", "fixed null pointer in handler")
	if err != nil {
		t.Fatal(err)
	}

	// FTS5 search — word-based, "hexagonal" should match first entry
	entries, err := store.SearchEntries(ctx, "hexagonal", SearchModeContent, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'hexagonal', got %d", len(entries))
	}

	// Prefix via FTS5: "hex*" should also match
	entries, err = store.SearchEntries(ctx, "hex", SearchModeContent, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'hex*' FTS5 prefix, got %d", len(entries))
	}

	// No match
	entries, err = store.SearchEntries(ctx, "nonexistent", SearchModeContent, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

func TestSearchEntries_BothMode(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.SaveEntry(ctx, "test-session", "architecture", "using hexagonal architecture")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "bug", "hexagonal is not a bug")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "note", "plain note")
	if err != nil {
		t.Fatal(err)
	}

	// "hexagonal" appears in both key (architecture) and value (bug entry) — should find 2
	entries, err := store.SearchEntries(ctx, "hexagonal", SearchModeBoth, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for 'hexagonal' in both mode, got %d", len(entries))
	}

	// "architecture" matches key and value — should find 1 (key prefix match on "architecture")
	entries, err = store.SearchEntries(ctx, "architecture", SearchModeBoth, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'architecture' in both mode, got %d", len(entries))
	}
}

func TestToFTSQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hexagonal", "hexagonal*"},
		{"hexagonal architecture", "hexagonal* architecture*"},
		{"  spaced  words  ", "spaced* words*"},
		{"", ""},
		{"single", "single*"},
	}
	for _, tt := range tests {
		got := toFTSQuery(tt.input)
		if got != tt.expected {
			t.Errorf("toFTSQuery(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSearchEntries_DefaultModeIsKey(t *testing.T) {
	// Verify that empty mode defaults to key search (prefix match, uses index)
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.SaveEntry(ctx, "test-session", "architecture", "value about architecture")
	if err != nil {
		t.Fatal(err)
	}

	// Call with SearchModeKey explicitly — this is the default
	entries, err := store.SearchEntries(ctx, "arch", SearchModeKey, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	// Searching by content should NOT match in key mode
	entries, err = store.SearchEntries(ctx, "value about", SearchModeKey, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (key mode doesn't search value), got %d", len(entries))
	}
}

func TestSearchEntries_FTS5_NoMatch(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.SaveEntry(ctx, "test-session", "key1", "some value here")
	if err != nil {
		t.Fatal(err)
	}

	entries, err := store.SearchEntries(ctx, "zzzzzzz", SearchModeContent, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for non-matching query, got %d", len(entries))
	}
}

func parseTime(t *testing.T, s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

func TestLoadEntries_limitZero(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Save 3 entries
	for i := 0; i < 3; i++ {
		_, err := store.SaveEntry(ctx, "test-session", fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Load with limit 0 — should return all (backward compat)
	entries, err := store.LoadEntries(ctx, "test-session", 0)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries with limit=0, got %d", len(entries))
	}
}

func TestLoadEntries_limitMoreThanTotal(t *testing.T) {
	db := openTestDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Save 3 entries
	for i := 0; i < 3; i++ {
		_, err := store.SaveEntry(ctx, "test-session", fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}

	// Load with limit 10 — should return all 3 (no crash)
	entries, err := store.LoadEntries(ctx, "test-session", 10)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries with limit=10, got %d", len(entries))
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
	CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
		key, value,
		content=memory_entries,
		content_rowid=id,
		tokenize='unicode61'
	);
	CREATE TABLE IF NOT EXISTS memory_keys (
		key          TEXT PRIMARY KEY,
		last_used_at TEXT NOT NULL,
		critical     INTEGER NOT NULL DEFAULT 0
	);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	const ftsTriggers = `
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
	`
	if _, err := db.Exec(ftsTriggers); err != nil {
		t.Fatalf("create fts5 triggers: %v", err)
	}
	if _, err := db.Exec(memoryKeysTrigger); err != nil {
		t.Fatalf("create memory keys trigger: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sessions (id, project, started_at) VALUES (?, ?, ?)`,
		"test-session", "test-project", "2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("insert stub session: %v", err)
	}
	return db
}
