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
		agent      TEXT NOT NULL DEFAULT 'opencode',
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
	sess, err := mgr.StartSession(ctx, "integration-project", "opencode")
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	// Verify it's active
	active, err := mgr.GetActiveSession(ctx, "integration-project", "")
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
			sess, err := mgr.StartSession(ctx, "concurrent-project", "")
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
		agent      TEXT NOT NULL DEFAULT 'opencode',
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
	sess, err := mgr1.StartSession(context.Background(), "persist-project", "")
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
	active, err := mgr2.GetActiveSession(context.Background(), "persist-project", "")
	if err != nil {
		t.Fatalf("GetActiveSession error: %v", err)
	}
	if active.ID != sess.ID {
		t.Fatalf("expected persisted session %q, got %q", sess.ID, active.ID)
	}
}

func TestIntegrationAgent_Isolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Start session A (opencode) and session B (claude-code) — both active
	sessA, err := mgr.StartSession(ctx, "agent-project", AgentOpencode)
	if err != nil {
		t.Fatalf("StartSession opencode: %v", err)
	}
	sessB, err := mgr.StartSession(ctx, "agent-project", AgentClaudeCode)
	if err != nil {
		t.Fatalf("StartSession claude-code: %v", err)
	}

	// Both should be active
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE project = 'agent-project' AND ended_at IS NULL").Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 active sessions, got %d", count)
	}

	// CloseSessionsByAgent for opencode — only opencode sessions closed
	if err := mgr.CloseSessionsByAgent(ctx, "agent-project", AgentOpencode); err != nil {
		t.Fatalf("CloseSessionsByAgent: %v", err)
	}

	// Verify only opencode session is closed
	var aEnded *string
	err = db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", sessA.ID).Scan(&aEnded)
	if err != nil {
		t.Fatal(err)
	}
	if aEnded == nil {
		t.Fatal("expected opencode session to be closed")
	}

	var bEnded *string
	err = db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", sessB.ID).Scan(&bEnded)
	if err != nil {
		t.Fatal(err)
	}
	if bEnded != nil {
		t.Fatal("expected claude-code session to still be active")
	}

	// GetActiveSession for opencode should create a new one
	sessC, err := mgr.GetActiveSession(ctx, "agent-project", AgentOpencode)
	if err != nil {
		t.Fatalf("GetActiveSession for opencode: %v", err)
	}
	if sessC.ID == sessA.ID {
		t.Fatal("expected new session for opencode, not the closed one")
	}
	if sessC.Agent != AgentOpencode {
		t.Fatalf("expected agent 'opencode', got %q", sessC.Agent)
	}
}

func TestIntegrationGetActiveSession_byAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	db := fileDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Start a session for opencode
	sessOpen, err := mgr.StartSession(ctx, "multi-agent-project", AgentOpencode)
	if err != nil {
		t.Fatal(err)
	}

	// GetActiveSession for opencode returns existing
	active, err := mgr.GetActiveSession(ctx, "multi-agent-project", AgentOpencode)
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != sessOpen.ID {
		t.Fatalf("expected opencode session %q, got %q", sessOpen.ID, active.ID)
	}

	// GetActiveSession for claude-code creates new (no active session for that agent)
	activeCC, err := mgr.GetActiveSession(ctx, "multi-agent-project", AgentClaudeCode)
	if err != nil {
		t.Fatal(err)
	}
	if activeCC.ID == sessOpen.ID {
		t.Fatal("expected different session for claude-code")
	}
	if activeCC.Agent != AgentClaudeCode {
		t.Fatalf("expected claude-code agent, got %q", activeCC.Agent)
	}
}
