package service

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/beabys/ilnamiqui/internal/config"
	"github.com/beabys/ilnamiqui/internal/db"
	"github.com/beabys/ilnamiqui/internal/memory"
)

// realConfig implements Config using real config functions (for tests that need real DB).
type realConfig struct{}

func (realConfig) DBPath() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return d + "/.ilnamiqui/ilnamiqui.db", nil
}
func (realConfig) ProjectSlug() (string, error) { return "test", nil }
func (realConfig) FindProjectRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return d, nil
}

// realDBOpener implements DBOpener using the real db package.
type realDBOpener struct{}

func (realDBOpener) NewDB(path string) (DB, error)     { return db.NewDB(path) }
func (realDBOpener) RunMigrations(sqlDB *sql.DB) error { return db.RunMigrations(sqlDB) }

// TestService_SaveAndLoad tests the full lifecycle via service methods.
func TestService_SaveAndLoad(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	// Init
	resp, err := svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if resp.DBPath == "" {
		t.Fatal("expected non-empty DBPath")
	}

	// Save
	saveResp, err := svc.Save(context.Background(), &SaveRequest{Key: "test", Value: "hello"})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if saveResp.Entry.Key != "test" {
		t.Fatalf("expected key 'test', got %q", saveResp.Entry.Key)
	}

	// Load
	loadResp, err := svc.Load(context.Background(), &LoadRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loadResp.Entries) != 3 {
		t.Fatalf("expected 3 entries (project-path + system + test), got %d", len(loadResp.Entries))
	}

	// Search by key (default mode)
	searchResp, err := svc.Search(context.Background(), &SearchRequest{Query: "test", After: nil, Before: nil})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(searchResp.Entries) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(searchResp.Entries))
	}

	// Search with both mode (searches key and value via FTS5)
	searchRespBoth, err := svc.Search(context.Background(), &SearchRequest{Query: "hello", Mode: memory.SearchModeBoth, After: nil, Before: nil})
	if err != nil {
		t.Fatalf("Search both mode failed: %v", err)
	}
	if len(searchRespBoth.Entries) != 1 {
		t.Fatalf("expected 1 search result in both mode, got %d", len(searchRespBoth.Entries))
	}

	// List sessions
	listResp, err := svc.ListSessions(context.Background(), &ListSessionsRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(listResp.Sessions) == 0 {
		t.Fatal("expected at least 1 session")
	}

	// Delete
	delResp, err := svc.Delete(context.Background(), &DeleteRequest{ID: int64(saveResp.Entry.ID)})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if delResp == nil {
		t.Fatal("expected non-nil DeleteResponse")
	}

	// Verify deleted (project-path entry should remain)
	loadResp2, _ := svc.Load(context.Background(), &LoadRequest{Limit: 10})
	if len(loadResp2.Entries) != 2 {
		t.Fatalf("expected 2 entries (project-path + system) after test entry delete, got %d", len(loadResp2.Entries))
	}
}

func TestService_Init_Idempotent(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	// Init twice should succeed
	_, err := svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("first Init failed: %v", err)
	}
	_, err = svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("second Init (idempotent) failed: %v", err)
	}
}

func TestService_ListKeys(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	_, err := svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Save some entries
	_, err = svc.Save(context.Background(), &SaveRequest{Key: "alpha", Value: "first"})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	_, err = svc.Save(context.Background(), &SaveRequest{Key: "beta", Value: "second"})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// List keys
	resp, err := svc.ListKeys(context.Background(), &ListKeysRequest{Limit: 0})
	if err != nil {
		t.Fatalf("ListKeys failed: %v", err)
	}
	if len(resp.Keys) == 0 {
		t.Fatal("expected at least 1 key")
	}
}

func TestService_SessionLifecycle(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	_, err := svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Start a new session
	startResp, err := svc.StartSession(context.Background(), &StartSessionRequest{})
	if err != nil {
		t.Fatalf("StartSession failed: %v", err)
	}
	if startResp.Session.ID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// End with summary
	endResp, err := svc.EndSession(context.Background(), &EndSessionRequest{Summary: "test summary"})
	if err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}
	if endResp.Session == nil {
		t.Fatal("expected session in response")
	}
}

// TestService_Load_AutoReinit simulates an old DB missing new tables and verifies Load auto-creates them.
func TestService_Load_AutoReinit(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	_ = os.Chdir(tmpDir)

	// Create a DB with only minimal old tables (simulate pre-migration state)
	ilnamiquiDir := tmpDir + "/.ilnamiqui"
	_ = os.MkdirAll(ilnamiquiDir, 0o755)
	dbPath := ilnamiquiDir + "/ilnamiqui.db"
	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer database.Close()

	// Create only the old tables (sessions, memory_entries) — NOT memory_fts, memory_keys, schema_versions
	_, err = database.SQLDB().Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY, project TEXT NOT NULL, started_at TEXT NOT NULL,
			ended_at TEXT, summary TEXT DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		CREATE TABLE IF NOT EXISTS memory_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
			key TEXT NOT NULL, value TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);
		INSERT OR IGNORE INTO sessions (id, project, started_at, ended_at, summary)
		VALUES ('00000000-0000-0000-0000-000000000000', '_system', '1970-01-01T00:00:00Z', '1970-01-01T00:00:00Z', 'system session');
	`)
	if err != nil {
		t.Fatalf("create old tables: %v", err)
	}
	database.Close()

	// Write sentinel so ensureDB skips migration checks
	if err := config.WriteSentinel(tmpDir); err != nil {
		t.Fatalf("WriteSentinel: %v", err)
	}

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	// Call Load — should auto-reinit missing tables
	loadResp, err := svc.Load(context.Background(), &LoadRequest{Limit: 10})
	if err != nil {
		t.Fatalf("Load after auto-reinit failed: %v", err)
	}

	// Re-open to verify tables
	database2, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB reopen: %v", err)
	}
	defer database2.Close()

	// Verify new tables exist
	for _, tbl := range []string{"memory_fts", "memory_keys", "schema_versions"} {
		var found int
		_ = database2.SQLDB().QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&found)
		if found == 0 {
			t.Fatalf("table %s was not created by auto-reinit", tbl)
		}
	}

	// Verify schema_versions has the version recorded
	var version int
	_ = database2.SQLDB().QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_versions").Scan(&version)
	expected := db.LatestVersion()
	if version != expected {
		t.Fatalf("expected schema version %d, got %d", expected, version)
	}

	_ = loadResp // entries may be empty — that's fine
}

func TestService_KeyUpdate(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	_, err := svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Save an entry to create a key
	_, err = svc.Save(context.Background(), &SaveRequest{Key: "test-key", Value: "val"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Update to critical
	_, err = svc.KeyUpdate(context.Background(), &KeyUpdateRequest{Key: "test-key", Critical: true})
	if err != nil {
		t.Fatalf("KeyUpdate true: %v", err)
	}

	// Verify via ListKeys
	keysResp, err := svc.ListKeys(context.Background(), &ListKeysRequest{Limit: 0})
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	found := false
	for _, k := range keysResp.Keys {
		if k.Key == "test-key" && k.Critical {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'test-key' with critical=true after KeyUpdate")
	}

	// Test nonexistent key
	_, err = svc.KeyUpdate(context.Background(), &KeyUpdateRequest{Key: "does-not-exist", Critical: true})
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
}

func TestService_Prune(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	_, err := svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Save a non-critical entry
	_, err = svc.Save(context.Background(), &SaveRequest{Key: "test", Value: "recent"})
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Open DB directly to insert an old entry with past timestamp
	dbPath := tmpDir + "/.ilnamiqui/ilnamiqui.db"
	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer database.Close()

	_, err = database.SQLDB().Exec(
		`INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`,
		"00000000-0000-0000-0000-000000000000", "test", "old value", "2024-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert old entry: %v", err)
	}

	// Prune with Before = recent date, Key = "*"
	resp, err := svc.Prune(context.Background(), &PruneRequest{
		Before: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Key:    "*",
	})
	if err != nil {
		t.Fatalf("Prune failed: %v", err)
	}
	if resp.Deleted == 0 {
		t.Fatal("expected Deleted > 0, got 0")
	}
	if resp.OrphansCleaned != 0 {
		// Non-critical key "test" still has a recent entry, so no orphans expected
	}

	// Verify "project-path" still exists (critical, not deleted)
	loadResp, err := svc.Load(context.Background(), &LoadRequest{Limit: 0})
	if err != nil {
		t.Fatalf("Load after prune failed: %v", err)
	}
	foundProjectPath := false
	for _, e := range loadResp.Entries {
		if e.Key == "project-path" {
			foundProjectPath = true
			break
		}
	}
	if !foundProjectPath {
		t.Fatal("expected 'project-path' to still exist after prune (critical key)")
	}
}
