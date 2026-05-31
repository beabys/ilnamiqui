//go:build integration

package cli

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestIntegrationInitSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	if err := Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// Open DB directly and verify schema
	dbPath := dir + "/.opencode/ilnamiqui.db"
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Verify tables (exclude sqlite_sequence auto-created by AUTOINCREMENT)
	var tables []string
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	rows.Close()

	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d: %v", len(tables), tables)
	}

	// Verify indexes
	var indexes []string
	rows, err = db.Query("SELECT name FROM sqlite_master WHERE type='index' AND name LIKE 'idx_%' ORDER BY name")
	if err != nil {
		t.Fatalf("list indexes: %v", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		indexes = append(indexes, name)
	}
	rows.Close()

	if len(indexes) != 3 {
		t.Fatalf("expected 3 indexes, got %d: %v", len(indexes), indexes)
	}
}

func TestIntegrationLoadPretty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	// Init
	if err := Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// Save some data
	if err := Run([]string{"save", "arch", "hexagonal architecture"}); err != nil {
		t.Fatalf("save error: %v", err)
	}
	if err := Run([]string{"save", "bug", "nil pointer in handler"}); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Load with --pretty
	output := captureStdout(t, func() {
		if err := Run([]string{"load", "--pretty"}); err != nil {
			t.Fatalf("load --pretty error: %v", err)
		}
	})

	// Verify table headers present
	if !strings.Contains(output, "ID") {
		t.Fatalf("expected table header 'ID', got: %s", output)
	}
	if !strings.Contains(output, "Key") {
		t.Fatalf("expected table header 'Key', got: %s", output)
	}
	if !strings.Contains(output, "Value") {
		t.Fatalf("expected table header 'Value', got: %s", output)
	}

	// Verify data present
	if !strings.Contains(output, "hexagonal architecture") {
		t.Fatalf("expected entry 'hexagonal architecture' in output: %s", output)
	}
	if !strings.Contains(output, "nil pointer in handler") {
		t.Fatalf("expected entry 'nil pointer in handler' in output: %s", output)
	}
}

func TestIntegrationInitReusesExistingDir(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	// Pre-create .opencode directory
	if err := os.MkdirAll(dir+"/.opencode", 0o755); err != nil {
		t.Fatal(err)
	}

	// Init should succeed reusing existing dir
	if err := Run([]string{"init"}); err != nil {
		t.Fatalf("init error with existing dir: %v", err)
	}

	if _, err := os.Stat(dir + "/.opencode/ilnamiqui.db"); os.IsNotExist(err) {
		t.Fatal("db file not created after init with existing dir")
	}
}
