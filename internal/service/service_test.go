package service

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/beabys/ilnamiqui/internal/db"
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
	if len(loadResp.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loadResp.Entries))
	}

	// Search
	searchResp, err := svc.Search(context.Background(), &SearchRequest{Query: "hello", After: nil, Before: nil})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(searchResp.Entries) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(searchResp.Entries))
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

	// Verify deleted
	loadResp2, _ := svc.Load(context.Background(), &LoadRequest{Limit: 10})
	if len(loadResp2.Entries) != 0 {
		t.Fatalf("expected 0 entries after delete, got %d", len(loadResp2.Entries))
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
