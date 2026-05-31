//go:build integration

package service

import (
	"context"
	"os"
	"testing"
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
