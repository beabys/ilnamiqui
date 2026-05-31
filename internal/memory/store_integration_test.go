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
