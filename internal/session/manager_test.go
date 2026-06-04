package session

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// openTestDB creates an in-memory SQLite with sessions table.
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

func TestStartSession(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	sess, err := mgr.StartSession(ctx, "test-project", "opencode")
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.Project != "test-project" {
		t.Fatalf("expected project 'test-project', got %q", sess.Project)
	}
	if sess.Agent != "opencode" {
		t.Fatalf("expected agent 'opencode', got %q", sess.Agent)
	}
	if sess.StartedAt.IsZero() {
		t.Fatal("expected non-zero started_at")
	}
	if sess.EndedAt != nil {
		t.Fatal("expected nil ended_at for new session")
	}
}

func TestStartSession_withAgent(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	sess, err := mgr.StartSession(ctx, "test-project", "claude-code")
	if err != nil {
		t.Fatalf("StartSession error: %v", err)
	}

	if sess.Agent != "claude-code" {
		t.Fatalf("expected agent 'claude-code', got %q", sess.Agent)
	}

	// Verify by querying DB directly
	var agent string
	err = db.QueryRow("SELECT agent FROM sessions WHERE id = ?", sess.ID).Scan(&agent)
	if err != nil {
		t.Fatalf("query agent: %v", err)
	}
	if agent != "claude-code" {
		t.Fatalf("expected stored agent 'claude-code', got %q", agent)
	}
}

func TestEndSession(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	sess, err := mgr.StartSession(ctx, "test-project", "")
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.EndSession(ctx, sess.ID, "test summary"); err != nil {
		t.Fatalf("EndSession error: %v", err)
	}

	// Verify by listing sessions — it should have ended_at set
	sessions, err := mgr.ListSessions(ctx, "test-project", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].EndedAt == nil {
		t.Fatal("expected ended_at to be set after EndSession")
	}
	if sessions[0].Summary != "test summary" {
		t.Fatalf("expected summary 'test summary', got %q", sessions[0].Summary)
	}
}

func TestEndSession_notFound(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	err := mgr.EndSession(ctx, "nonexistent-id", "summary")
	if err == nil {
		t.Fatal("expected error for ending non-existent session")
	}
}

func TestGetActiveSession_createsNew(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// No sessions exist — GetActiveSession should create one
	sess, err := mgr.GetActiveSession(ctx, "test-project", "")
	if err != nil {
		t.Fatalf("GetActiveSession error: %v", err)
	}

	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.Project != "test-project" {
		t.Fatalf("expected project 'test-project', got %q", sess.Project)
	}
}

func TestGetActiveSession_returnsExisting(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Start a session explicitly
	sess1, err := mgr.StartSession(ctx, "test-project", "")
	if err != nil {
		t.Fatal(err)
	}

	// GetActiveSession should return the same active session
	sess2, err := mgr.GetActiveSession(ctx, "test-project", "")
	if err != nil {
		t.Fatalf("GetActiveSession error: %v", err)
	}

	if sess1.ID != sess2.ID {
		t.Fatalf("expected same session ID, got %q vs %q", sess1.ID, sess2.ID)
	}
}

func TestGetActiveSession_endsThenCreates(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Start and end a session
	sess1, err := mgr.StartSession(ctx, "test-project", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.EndSession(ctx, sess1.ID, "done"); err != nil {
		t.Fatal(err)
	}

	// GetActiveSession should create a new one
	sess2, err := mgr.GetActiveSession(ctx, "test-project", "")
	if err != nil {
		t.Fatalf("GetActiveSession error: %v", err)
	}

	if sess1.ID == sess2.ID {
		t.Fatal("expected different session ID after ending previous")
	}
}

func TestGetActiveSession_byAgent(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Create session for agent A
	sessA, err := mgr.StartSession(ctx, "test-project", "agent-a")
	if err != nil {
		t.Fatal(err)
	}

	// GetActiveSession for agent A should return it
	activeA, err := mgr.GetActiveSession(ctx, "test-project", "agent-a")
	if err != nil {
		t.Fatalf("GetActiveSession for agent-a: %v", err)
	}
	if activeA.ID != sessA.ID {
		t.Fatalf("expected session %q for agent-a, got %q", sessA.ID, activeA.ID)
	}

	// GetActiveSession for agent B should create new (no active session for agent B)
	activeB, err := mgr.GetActiveSession(ctx, "test-project", "agent-b")
	if err != nil {
		t.Fatalf("GetActiveSession for agent-b: %v", err)
	}
	if activeB.ID == sessA.ID {
		t.Fatal("expected different session for agent-b")
	}
	if activeB.Agent != "agent-b" {
		t.Fatalf("expected agent 'agent-b', got %q", activeB.Agent)
	}
}

func TestCloseSessionsByAgent(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Create sessions for different agents
	sess1, err := mgr.StartSession(ctx, "test-project", "agent-a")
	if err != nil {
		t.Fatal(err)
	}
	sess2, err := mgr.StartSession(ctx, "test-project", "agent-b")
	if err != nil {
		t.Fatal(err)
	}

	// Close sessions for agent-a
	if err := mgr.CloseSessionsByAgent(ctx, "test-project", "agent-a"); err != nil {
		t.Fatalf("CloseSessionsByAgent: %v", err)
	}

	// Verify sess1 (agent-a) is closed
	var endedAt1 *string
	err = db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", sess1.ID).Scan(&endedAt1)
	if err != nil {
		t.Fatalf("query sess1: %v", err)
	}
	if endedAt1 == nil {
		t.Fatal("expected sess1 to be closed (agent-a)")
	}

	// Verify sess2 (agent-b) is still active
	var endedAt2 *string
	err = db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", sess2.ID).Scan(&endedAt2)
	if err != nil {
		t.Fatalf("query sess2: %v", err)
	}
	if endedAt2 != nil {
		t.Fatal("expected sess2 to still be active (agent-b)")
	}

	// Verify summary was set
	var summary string
	err = db.QueryRow("SELECT summary FROM sessions WHERE id = ?", sess1.ID).Scan(&summary)
	if err != nil {
		t.Fatalf("query summary: %v", err)
	}
	if summary != "auto-closed by new session" {
		t.Fatalf("expected 'auto-closed by new session', got %q", summary)
	}
}

func TestListSessions(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	// Create multiple sessions
	for i := 0; i < 5; i++ {
		sess, err := mgr.StartSession(ctx, "test-project", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := mgr.EndSession(ctx, sess.ID, ""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Millisecond) // ensure different timestamps
	}

	sessions, err := mgr.ListSessions(ctx, "test-project", 10)
	if err != nil {
		t.Fatalf("ListSessions error: %v", err)
	}

	if len(sessions) != 5 {
		t.Fatalf("expected 5 sessions, got %d", len(sessions))
	}

	// Should be ordered DESC by started_at
	if sessions[0].StartedAt.Before(sessions[1].StartedAt) {
		t.Fatal("expected sessions ordered by started_at DESC")
	}
}

func TestListSessions_limit(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		sess, err := mgr.StartSession(ctx, "test-project", "")
		if err != nil {
			t.Fatal(err)
		}
		_ = mgr.EndSession(ctx, sess.ID, "")
	}

	sessions, err := mgr.ListSessions(ctx, "test-project", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions with limit=3, got %d", len(sessions))
	}
}

func TestListSessions_empty(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	sessions, err := mgr.ListSessions(ctx, "test-project", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestListSessions_defaultLimit(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	for i := 0; i < 15; i++ {
		sess, err := mgr.StartSession(ctx, "test-project", "")
		if err != nil {
			t.Fatal(err)
		}
		_ = mgr.EndSession(ctx, sess.ID, "")
	}

	// Pass limit 0 — should default to 10
	sessions, err := mgr.ListSessions(ctx, "test-project", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 10 {
		t.Fatalf("expected 10 sessions with limit=0, got %d", len(sessions))
	}
}

func TestMultipleProjects(t *testing.T) {
	db := openTestDB(t)
	mgr := NewManager(db)
	ctx := context.Background()

	sess1, err := mgr.StartSession(ctx, "project-a", "")
	if err != nil {
		t.Fatal(err)
	}
	sess2, err := mgr.StartSession(ctx, "project-b", "")
	if err != nil {
		t.Fatal(err)
	}

	// GetActiveSession should return correct project
	activeA, err := mgr.GetActiveSession(ctx, "project-a", "")
	if err != nil {
		t.Fatal(err)
	}
	if activeA.ID != sess1.ID {
		t.Fatalf("expected session for project-a, got %q", activeA.ID)
	}

	activeB, err := mgr.GetActiveSession(ctx, "project-b", "")
	if err != nil {
		t.Fatal(err)
	}
	if activeB.ID != sess2.ID {
		t.Fatalf("expected session for project-b, got %q", activeB.ID)
	}
}
