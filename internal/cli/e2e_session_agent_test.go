//go:build e2e

package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beabys/ilnamiqui/internal/service"
	_ "modernc.org/sqlite"
)

// TestE2E_SessionAutoClose verifies that starting a new session for an agent
// auto-closes the previous active session for the same agent.
//
// Plan spec:
//   - Init → start session for agent=opencode (session A)
//   - Save an entry (goes to session A)
//   - Start another session for agent=opencode (session B, different from A)
//   - Save another entry (goes to session B)
//   - Session A must have ended_at set, session B must be active
func TestE2E_SessionAutoClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	cli := New(svc)

	if err := cli.Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// 1. Start session A (opencode)
	sessionA := strings.TrimSpace(captureStdout(t, func() {
		if err := cli.Run([]string{"session", "start", "--agent", "opencode"}); err != nil {
			t.Fatalf("session start A error: %v", err)
		}
	}))
	if sessionA == "" {
		t.Fatal("expected non-empty session ID A")
	}
	t.Logf("Session A: %s", sessionA)

	// 2. Save entry — goes to session A
	if err := cli.Run([]string{"save", "--agent", "opencode", "key-a", "value-from-a"}); err != nil {
		t.Fatalf("save to A error: %v", err)
	}

	// 3. Start session B (opencode) — should auto-close session A
	sessionB := strings.TrimSpace(captureStdout(t, func() {
		if err := cli.Run([]string{"session", "start", "--agent", "opencode"}); err != nil {
			t.Fatalf("session start B error: %v", err)
		}
	}))
	if sessionB == "" {
		t.Fatal("expected non-empty session ID B")
	}
	t.Logf("Session B: %s", sessionB)

	if sessionA == sessionB {
		t.Fatal("expected different session IDs for A and B")
	}

	// 4. Save another entry — goes to session B
	if err := cli.Run([]string{"save", "--agent", "opencode", "key-b", "value-from-b"}); err != nil {
		t.Fatalf("save to B error: %v", err)
	}

	// 5. Open DB and verify session states
	dbPath := filepath.Join(dir, ".ilnamiqui", "ilnamiqui.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Session A should have ended_at set
	var endedAtA *string
	if err := db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", sessionA).Scan(&endedAtA); err != nil {
		t.Fatalf("query session A: %v", err)
	}
	if endedAtA == nil || *endedAtA == "" {
		t.Fatal("expected session A to have ended_at set (auto-closed)")
	}
	t.Logf("Session A ended_at: %s", *endedAtA)

	// Session B should have ended_at NULL (active)
	var endedAtB *string
	if err := db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", sessionB).Scan(&endedAtB); err != nil {
		t.Fatalf("query session B: %v", err)
	}
	if endedAtB != nil {
		t.Fatalf("expected session B to be active (ended_at NULL), got: %s", *endedAtB)
	}

	// Verify entries are assigned to correct sessions
	var entrySessionA string
	if err := db.QueryRow("SELECT session_id FROM memory_entries WHERE key = 'key-a'").Scan(&entrySessionA); err != nil {
		t.Fatalf("query key-a session: %v", err)
	}
	if entrySessionA != sessionA {
		t.Fatalf("expected key-a in session %s, got %s", sessionA, entrySessionA)
	}

	var entrySessionB string
	if err := db.QueryRow("SELECT session_id FROM memory_entries WHERE key = 'key-b'").Scan(&entrySessionB); err != nil {
		t.Fatalf("query key-b session: %v", err)
	}
	if entrySessionB != sessionB {
		t.Fatalf("expected key-b in session %s, got %s", sessionB, entrySessionB)
	}
}

// TestE2E_AgentIsolation verifies that different agents maintain independent sessions.
//
// Plan spec:
//   - Start session for opencode → ID_A
//   - Start session for claude-code → ID_B (should NOT close ID_A)
//   - Both sessions active (ended_at = NULL)
//   - Save with --agent opencode → goes to ID_A
//   - Save with --agent claude-code → goes to ID_B
func TestE2E_AgentIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	cli := New(svc)

	if err := cli.Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// 1. Start session for opencode
	idA := strings.TrimSpace(captureStdout(t, func() {
		if err := cli.Run([]string{"session", "start", "--agent", "opencode"}); err != nil {
			t.Fatalf("session start opencode error: %v", err)
		}
	}))
	if idA == "" {
		t.Fatal("expected non-empty session ID A")
	}
	t.Logf("Session A (opencode): %s", idA)

	// 2. Start session for claude-code — should NOT close opencode session
	idB := strings.TrimSpace(captureStdout(t, func() {
		if err := cli.Run([]string{"session", "start", "--agent", "claude-code"}); err != nil {
			t.Fatalf("session start claude-code error: %v", err)
		}
	}))
	if idB == "" {
		t.Fatal("expected non-empty session ID B")
	}
	t.Logf("Session B (claude-code): %s", idB)

	if idA == idB {
		t.Fatal("expected different session IDs for different agents")
	}

	// 3. Verify both sessions active
	dbPath := filepath.Join(dir, ".ilnamiqui", "ilnamiqui.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var activeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions WHERE ended_at IS NULL").Scan(&activeCount); err != nil {
		t.Fatalf("query active sessions: %v", err)
	}
	if activeCount != 2 {
		t.Fatalf("expected 2 active sessions, got %d", activeCount)
	}

	// 4. Save entry with --agent opencode → verify it goes to opencode session
	if err := cli.Run([]string{"save", "--agent", "opencode", "oc-key", "oc-value"}); err != nil {
		t.Fatalf("save to opencode error: %v", err)
	}
	var sessID string
	if err := db.QueryRow("SELECT session_id FROM memory_entries WHERE key = 'oc-key'").Scan(&sessID); err != nil {
		t.Fatalf("query oc-key session: %v", err)
	}
	if sessID != idA {
		t.Fatalf("expected oc-key in opencode session %s, got %s", idA, sessID)
	}

	// 5. Save entry with --agent claude-code → verify it goes to claude-code session
	if err := cli.Run([]string{"save", "--agent", "claude-code", "cc-key", "cc-value"}); err != nil {
		t.Fatalf("save to claude-code error: %v", err)
	}
	if err := db.QueryRow("SELECT session_id FROM memory_entries WHERE key = 'cc-key'").Scan(&sessID); err != nil {
		t.Fatalf("query cc-key session: %v", err)
	}
	if sessID != idB {
		t.Fatalf("expected cc-key in claude-code session %s, got %s", idB, sessID)
	}
}

// TestE2E_SaveAutoCreatesSession verifies that save auto-creates a session
// with the correct agent when no session was started.
//
// Plan spec:
//   - save --agent opencode k1 v1 → auto-creates opencode session
//   - save --agent claude-code k2 v2 → auto-creates claude-code session (different)
//   - Both sessions have correct agent values
func TestE2E_SaveAutoCreatesSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	cli := New(svc)

	if err := cli.Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// 1. Save with --agent opencode (no prior session start)
	if err := cli.Run([]string{"save", "--agent", "opencode", "k1", "v1"}); err != nil {
		t.Fatalf("save opencode error: %v", err)
	}

	// 2. Save with --agent claude-code (no prior session start)
	if err := cli.Run([]string{"save", "--agent", "claude-code", "k2", "v2"}); err != nil {
		t.Fatalf("save claude-code error: %v", err)
	}

	// 3. Verify both sessions exist with correct agent values
	dbPath := filepath.Join(dir, ".ilnamiqui", "ilnamiqui.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Verify opencode session exists
	var ocSessions int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions WHERE agent = 'opencode'").Scan(&ocSessions); err != nil {
		t.Fatalf("query opencode sessions: %v", err)
	}
	if ocSessions != 1 {
		t.Fatalf("expected 1 opencode session, got %d", ocSessions)
	}

	// Verify claude-code session exists
	var ccSessions int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions WHERE agent = 'claude-code'").Scan(&ccSessions); err != nil {
		t.Fatalf("query claude-code sessions: %v", err)
	}
	if ccSessions != 1 {
		t.Fatalf("expected 1 claude-code session, got %d", ccSessions)
	}

	// Verify the two sessions are different (have different IDs)
	var ocID, ccID string
	if err := db.QueryRow("SELECT id FROM sessions WHERE agent = 'opencode' AND ended_at IS NULL").Scan(&ocID); err != nil {
		t.Fatalf("query opencode session ID: %v", err)
	}
	if err := db.QueryRow("SELECT id FROM sessions WHERE agent = 'claude-code' AND ended_at IS NULL").Scan(&ccID); err != nil {
		t.Fatalf("query claude-code session ID: %v", err)
	}
	if ocID == ccID {
		t.Fatal("expected different session IDs for different agents")
	}

	// Verify entries are assigned to the correct agent sessions
	var entrySession string
	if err := db.QueryRow("SELECT session_id FROM memory_entries WHERE key = 'k1'").Scan(&entrySession); err != nil {
		t.Fatalf("query k1 session: %v", err)
	}
	if entrySession != ocID {
		t.Fatalf("expected k1 in opencode session %s, got %s", ocID, entrySession)
	}

	if err := db.QueryRow("SELECT session_id FROM memory_entries WHERE key = 'k2'").Scan(&entrySession); err != nil {
		t.Fatalf("query k2 session: %v", err)
	}
	if entrySession != ccID {
		t.Fatalf("expected k2 in claude-code session %s, got %s", ccID, entrySession)
	}
}

// TestE2E_SessionEndWithAgent verifies that session end targets the correct agent's session.
//
// Plan spec:
//   - Start session for opencode
//   - Start session for claude-code
//   - End session for opencode (session end --agent opencode)
//   - Verify opencode session closed, claude-code session still active
func TestE2E_SessionEndWithAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	cli := New(svc)

	if err := cli.Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// 1. Start session for opencode
	idOC := strings.TrimSpace(captureStdout(t, func() {
		if err := cli.Run([]string{"session", "start", "--agent", "opencode"}); err != nil {
			t.Fatalf("session start opencode error: %v", err)
		}
	}))
	if idOC == "" {
		t.Fatal("expected non-empty opencode session ID")
	}
	t.Logf("OpenCode session: %s", idOC)

	// 2. Start session for claude-code
	idCC := strings.TrimSpace(captureStdout(t, func() {
		if err := cli.Run([]string{"session", "start", "--agent", "claude-code"}); err != nil {
			t.Fatalf("session start claude-code error: %v", err)
		}
	}))
	if idCC == "" {
		t.Fatal("expected non-empty claude-code session ID")
	}
	t.Logf("ClaudeCode session: %s", idCC)

	// 3. End session for opencode
	if err := cli.Run([]string{"session", "end", "--agent", "opencode", "--summary", "test done"}); err != nil {
		t.Fatalf("session end opencode error: %v", err)
	}

	// 4. Verify via DB
	dbPath := filepath.Join(dir, ".ilnamiqui", "ilnamiqui.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Opencode session should be closed
	var endedOC *string
	if err := db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", idOC).Scan(&endedOC); err != nil {
		t.Fatalf("query opencode session: %v", err)
	}
	if endedOC == nil || *endedOC == "" {
		t.Fatal("expected opencode session to have ended_at set")
	}
	t.Logf("Opencode ended_at: %s", *endedOC)

	// Verify summary was set
	var summaryOC string
	if err := db.QueryRow("SELECT COALESCE(summary, '') FROM sessions WHERE id = ?", idOC).Scan(&summaryOC); err != nil {
		t.Fatalf("query opencode summary: %v", err)
	}
	if !strings.Contains(summaryOC, "test done") {
		t.Fatalf("expected opencode summary to contain 'test done', got %q", summaryOC)
	}

	// Claude-code session should still be active
	var endedCC *string
	if err := db.QueryRow("SELECT ended_at FROM sessions WHERE id = ?", idCC).Scan(&endedCC); err != nil {
		t.Fatalf("query claude-code session: %v", err)
	}
	if endedCC != nil {
		t.Fatalf("expected claude-code session to be active (ended_at NULL), got: %s", *endedCC)
	}

	// Verify only 1 active session remains (claude-code)
	var activeCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM sessions WHERE ended_at IS NULL").Scan(&activeCount); err != nil {
		t.Fatalf("query active sessions: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected 1 active session, got %d", activeCount)
	}
}
