//go:build integration

package memory

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	// Ensure the sqlite driver is registered
	os.Exit(m.Run())
}

func TestIntegrationConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	store := NewStore(db)
	ctx := context.Background()

	const goroutines = 3
	const writesPerGoroutine = 10

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*writesPerGoroutine)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				key := "concurrent-key"
				value := "goroutine"
				_, err := store.SaveEntry(ctx, "test-session", key, value)
				if err != nil {
					errCh <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent write error: %v", err)
	}

	// Verify all entries were saved (no data loss)
	entries, err := store.LoadEntries(ctx, "test-session", 0)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}

	expected := goroutines * writesPerGoroutine
	if len(entries) != expected {
		t.Fatalf("expected %d entries after concurrent writes, got %d", expected, len(entries))
	}
}

func TestIntegrationSaveAndLoadCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Save an entry and verify persistence
	entry, err := store.SaveEntry(ctx, "test-session", "persist-key", "persist-value")
	if err != nil {
		t.Fatalf("SaveEntry error: %v", err)
	}

	// Load by session
	entries, err := store.LoadEntries(ctx, "test-session", 0)
	if err != nil {
		t.Fatalf("LoadEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Key != "persist-key" {
		t.Fatalf("expected key 'persist-key', got %q", entries[0].Key)
	}
	if entries[0].Value != "persist-value" {
		t.Fatalf("expected value 'persist-value', got %q", entries[0].Value)
	}

	// Delete and verify
	if err := store.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatalf("DeleteEntry error: %v", err)
	}

	entries, err = store.LoadEntries(ctx, "test-session", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", len(entries))
	}
}

func TestIntegrationFileBasedWAL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	path := filepath.Join(t.TempDir(), "wal_integration.db")
	d, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer d.Close()

	// Check WAL was enabled
	var journalMode string
	if err := d.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("scan journal_mode: %v", err)
	}
	t.Logf("journal_mode: %s", journalMode)
}

func TestIntegrationSearchEntries_WithDateRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	store := NewStore(db)
	ctx := context.Background()
	// Use the stub session inserted by fileDB
	sessionID := "test-session"

	// Insert entries with explicit dates
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

	// Only date range, no query
	results, err := store.SearchEntries(ctx, "", SearchModeBoth, 0, &after, &before)
	if err != nil {
		t.Fatalf("SearchEntries date range: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 entries in date range, got %d: %+v", len(results), results)
	}

	// Query + date range combined
	results, err = store.SearchEntries(ctx, "mid", SearchModeBoth, 0, &after, &before)
	if err != nil {
		t.Fatalf("SearchEntries query+date: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 entry for 'mid' in range, got %d", len(results))
	}

	// Only after, no before
	results, err = store.SearchEntries(ctx, "", SearchModeBoth, 0, &after, nil)
	if err != nil {
		t.Fatalf("SearchEntries after only: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 entries after 2026, got %d", len(results))
	}

	// Before only, no after
	results, err = store.SearchEntries(ctx, "", SearchModeBoth, 0, nil, &before)
	if err != nil {
		t.Fatalf("SearchEntries before only: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 entries before 2027-06, got %d", len(results))
	}
}

func TestIntegrationSearchEntries_ContentMode_FTS5(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := store.SaveEntry(ctx, "test-session", "arch", "hexagonal architecture pattern")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "bug", "fixed hexagon shape renderer")
	if err != nil {
		t.Fatal(err)
	}

	// FTS5 should only match "hexagonal" — it's token-aware, "hexagon" is a different token
	entries, err := store.SearchEntries(ctx, "hexagonal", SearchModeContent, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry for 'hexagonal' FTS5, got %d", len(entries))
	}

	// Prefix search "hex*" matches both "hexagonal" and "hexagon"
	entries, err = store.SearchEntries(ctx, "hex", SearchModeContent, 0, nil, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for 'hex*' FTS5 prefix, got %d", len(entries))
	}
}

func TestIntegrationSearchEntries_FTS5_WithDateRange(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	store := NewStore(db)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`,
		"test-session", "old", "old entry value", "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `
		INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`,
		"test-session", "new", "new entry value", "2027-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	after := parseTime(t, "2026-01-01T00:00:00Z")

	entries, err := store.SearchEntries(ctx, "entry", SearchModeContent, 0, &after, nil)
	if err != nil {
		t.Fatalf("SearchEntries error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after 2026 with 'entry', got %d", len(entries))
	}
	if entries[0].Key != "new" {
		t.Fatalf("expected entry 'new', got %q", entries[0].Key)
	}
}

func TestIntegrationListKeys_AfterInserts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	store := NewStore(db)
	ctx := context.Background()

	// Save entries with mixed keys
	_, err := store.SaveEntry(ctx, "test-session", "arch", "hexagonal")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "bug", "nil pointer")
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveEntry(ctx, "test-session", "arch", "clean arch")
	if err != nil {
		t.Fatal(err)
	}

	keys, err := store.ListKeys(ctx, 0)
	if err != nil {
		t.Fatalf("ListKeys error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %+v", len(keys), keys)
	}
}


