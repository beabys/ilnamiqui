//go:build e2e

package cli

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beabys/ilnamiqui/internal/service"
	_ "modernc.org/sqlite"
)

// captureStdout captures stdout during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestE2E_Prune verifies the prune command end-to-end using a real SQLite DB.
func TestE2E_Prune(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// 1. Create temp dir and chdir
	dir := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// 2. Create service and init
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	cli := New(svc)

	if err := cli.Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// 3. Open DB directly to insert entries with controlled timestamps
	dbPath := filepath.Join(dir, ".ilnamiqui", "ilnamiqui.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Insert entries:
	//   - project-path (critical) — old date → should survive
	//   - test — old date → should be pruned
	//   - test — recent date → should survive
	//   - other — old date → should be pruned + orphan key cleaned
	entries := []struct {
		key       string
		value     string
		createdAt string
	}{
		{"project-path", "/home/user/proj", "2025-01-01T00:00:00Z"},
		{"test", "old-value", "2025-06-01T00:00:00Z"},
		{"test", "recent-value", "2026-06-01T00:00:00Z"},
		{"other", "other-old", "2025-03-01T00:00:00Z"},
	}
	for _, e := range entries {
		if _, err := db.Exec(
			`INSERT INTO memory_entries (session_id, key, value, created_at) VALUES ('00000000-0000-0000-0000-000000000000', ?, ?, ?)`,
			e.key, e.value, e.createdAt,
		); err != nil {
			t.Fatalf("insert %s=%s: %v", e.key, e.value, err)
		}
	}

	// 4. Mark project-path as critical
	if _, err := db.Exec(`UPDATE memory_keys SET critical = 1 WHERE key = 'project-path'`); err != nil {
		t.Fatalf("set critical flag: %v", err)
	}

	// 5. Run prune --before 2026-05-04 (between old and recent entries)
	output := captureStdout(t, func() {
		if err := cli.Run([]string{"prune", "--before", "2026-05-04"}); err != nil {
			t.Fatalf("prune error: %v", err)
		}
	})
	t.Logf("prune output: %s", output)

	if !strings.Contains(output, "pruned") {
		t.Fatalf("expected prune summary in output, got: %s", output)
	}

	// 6. Verify entries
	checkEntry := func(t *testing.T, key, value string, expectExists bool) {
		t.Helper()
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM memory_entries WHERE key = ? AND value = ?`, key, value).Scan(&count); err != nil {
			t.Fatalf("query %s=%s: %v", key, value, err)
		}
		if expectExists && count != 1 {
			t.Fatalf("expected %s=%s to exist, got count=%d", key, value, count)
		}
		if !expectExists && count != 0 {
			t.Fatalf("expected %s=%s to be deleted, got count=%d", key, value, count)
		}
	}

	checkEntry(t, "project-path", "/home/user/proj", true)  // critical → survives
	checkEntry(t, "test", "old-value", false)                // old → pruned
	checkEntry(t, "test", "recent-value", true)              // recent → survives
	checkEntry(t, "other", "other-old", false)               // old → pruned

	// 7. Verify orphan key "other" was cleaned from memory_keys
	var keyCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_keys WHERE key = 'other'`).Scan(&keyCount); err != nil {
		t.Fatalf("query memory_keys for 'other': %v", err)
	}
	if keyCount != 0 {
		t.Fatalf("expected orphan key 'other' cleaned, got count=%d", keyCount)
	}

	// 8. Run prune again with same date --key test (should be 0 — old test entry already gone,
	//    recent test entry is after 2026-05-04)
	output2 := captureStdout(t, func() {
		if err := cli.Run([]string{"prune", "--before", "2026-05-04", "--key", "test"}); err != nil {
			t.Fatalf("second prune error: %v", err)
		}
	})
	t.Logf("second prune output: %s", output2)

	if !strings.Contains(output2, "pruned 0 entries") {
		t.Fatalf("expected 0 entries pruned on second run, got: %s", output2)
	}

	// 9. Verify --before is required
	err = cli.Run([]string{"prune", "--key", "test"})
	if err == nil {
		t.Fatal("expected error for missing --before flag")
	}
	if !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected usage error for missing --before, got: %v", err)
	}

	// 10. Verify nonexistent key returns 0 deleted, no error
	output3 := captureStdout(t, func() {
		if err := cli.Run([]string{"prune", "--before", "2026-05-04", "--key", "nonexistent"}); err != nil {
			t.Fatalf("prune nonexistent key error: %v", err)
		}
	})
	t.Logf("nonexistent key prune output: %s", output3)

	if !strings.Contains(output3, "pruned 0 entries") {
		t.Fatalf("expected 0 deleted for nonexistent key, got: %s", output3)
	}

	// 11. Final sanity: recent test entry still exists
	checkEntry(t, "test", "recent-value", true)
}
