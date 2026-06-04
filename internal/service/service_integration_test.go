//go:build integration

package service

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/beabys/ilnamiqui/internal/db"
)

func TestIntegrationService_FullLifecycle(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	tmpDir := t.TempDir()
	os.Chdir(tmpDir)

	svc := New(realConfig{}, realDBOpener{})
	defer svc.Close()

	// Init
	initResp, err := svc.Init(context.Background(), &InitRequest{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := os.Stat(initResp.DBPath); os.IsNotExist(err) {
		t.Fatalf("DB file not created at %s", initResp.DBPath)
	}

	// Save multiple entries
	keys := []string{"arch", "bug", "decision"}
	for i, k := range keys {
		_, err := svc.Save(context.Background(), &SaveRequest{Key: k, Value: "value"})
		if err != nil {
			t.Fatalf("Save %d (%s): %v", i, k, err)
		}
	}

	// Load with limit
	loadResp, err := svc.Load(context.Background(), &LoadRequest{Limit: 2})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loadResp.Entries) > 2 {
		t.Fatalf("Load limit=2 returned %d entries", len(loadResp.Entries))
	}

	// Search
	searchResp, err := svc.Search(context.Background(), &SearchRequest{Query: "arch", Limit: 5, After: nil, Before: nil})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(searchResp.Entries) != 1 {
		t.Fatalf("Search 'arch' expected 1 result, got %d", len(searchResp.Entries))
	}

	// List sessions
	listResp, err := svc.ListSessions(context.Background(), &ListSessionsRequest{Limit: 5})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(listResp.Sessions) < 1 {
		t.Fatal("expected at least 1 session")
	}

	// Close
	if err := svc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestIntegrationService_Prune(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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

	// Save some entries to create active session
	_, err = svc.Save(context.Background(), &SaveRequest{Key: "arch", Value: "hexagonal"})
	if err != nil {
		t.Fatalf("Save arch: %v", err)
	}
	_, err = svc.Save(context.Background(), &SaveRequest{Key: "bug", Value: "nil pointer"})
	if err != nil {
		t.Fatalf("Save bug: %v", err)
	}

	// Open DB directly to insert old entries
	dbPath := tmpDir + "/.ilnamiqui/ilnamiqui.db"
	database, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer database.Close()

	systemSession := "00000000-0000-0000-0000-000000000000"
	_, err = database.SQLDB().Exec(
		`INSERT INTO memory_entries (session_id, key, value, created_at) VALUES (?, ?, ?, ?)`,
		systemSession, "old-key", "old-value", "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("insert old entry: %v", err)
	}

	// Prune old entries
	resp, err := svc.Prune(context.Background(), &PruneRequest{
		Before: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Key:    "*",
	})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if resp.Deleted == 0 {
		t.Fatal("expected Deleted > 0")
	}
	if resp.OrphansCleaned == 0 {
		t.Fatal("expected OrphansCleaned > 0 (orphan key 'old-key')")
	}

	// Verify project-path still exists (critical key)
	loadResp, err := svc.Load(context.Background(), &LoadRequest{Limit: 0})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	foundProjectPath := false
	for _, e := range loadResp.Entries {
		if e.Key == "project-path" {
			foundProjectPath = true
		}
		if e.Key == "old-key" {
			t.Fatal("expected 'old-key' to be pruned")
		}
	}
	if !foundProjectPath {
		t.Fatal("expected 'project-path' to still exist after prune")
	}

	// Close
	if err := svc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}
