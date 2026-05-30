//go:build integration

package session

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// fileDB creates a file-based SQLite database for integration tests.
func fileDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sessions_test.db")
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
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return db
}

func TestIntegrationFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Start a session
	sess, err := mgr.StartSession(ctx, "integration-project")
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	// Verify it's active
	active, err := mgr.GetActiveSession(ctx, "integration-project")
	if err != nil {
		t.Fatalf("GetActiveSession error: %v", err)
	}
	if active.ID != sess.ID {
		t.Fatalf("expected active session %q, got %q", sess.ID, active.ID)
	}

	// End it
	if err := mgr.EndSession(ctx, sess.ID, "integration test complete"); err != nil {
		t.Fatalf("EndSession error: %v", err)
	}

	// Verify it's in the list
	sessions, err := mgr.ListSessions(ctx, "integration-project", 10)
	if err != nil {
		t.Fatalf("ListSessions error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != sess.ID {
		t.Fatalf("expected session ID %q, got %q", sess.ID, sessions[0].ID)
	}
	if sessions[0].EndedAt == nil {
		t.Fatal("expected ended_at to be non-nil")
	}
}

func TestIntegrationMultipleSessionsConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	const goroutines = 5
	var wg sync.WaitGroup
	errCh := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sess, err := mgr.StartSession(ctx, "concurrent-project")
			if err != nil {
				errCh <- err
				return
			}
			// End with a short delay to ensure unique timestamps
			time.Sleep(time.Millisecond * 5)
			if err := mgr.EndSession(ctx, sess.ID, ""); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent session error: %v", err)
	}

	sessions, err := mgr.ListSessions(ctx, "concurrent-project", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != goroutines {
		t.Fatalf("expected %d sessions, got %d", goroutines, len(sessions))
	}
}

func TestIntegrationActiveSessionPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	path := filepath.Join(t.TempDir(), "persist_test.db")

	// First connection: create schema and start a session
	db1, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db1.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatal(err)
	}
	const schema = `
	CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		project    TEXT NOT NULL,
		started_at TEXT NOT NULL,
		ended_at   TEXT,
		summary    TEXT DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`
	if _, err := db1.Exec(schema); err != nil {
		t.Fatal(err)
	}
	mgr1 := NewManager(db1)
	sess, err := mgr1.StartSession(context.Background(), "persist-project")
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()

	// Second connection: should see the same active session
	db2, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	mgr2 := NewManager(db2)
	active, err := mgr2.GetActiveSession(context.Background(), "persist-project")
	if err != nil {
		t.Fatalf("GetActiveSession error: %v", err)
	}
	if active.ID != sess.ID {
		t.Fatalf("expected persisted session %q, got %q", sess.ID, active.ID)
	}
}
