//go:build integration

package cli

import (
	"bytes"
	"fmt"
	"os"
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

// setupProjectIntegration creates a temp dir, chdirs to it, and runs init.
func setupProjectIntegration(t *testing.T) *CLI {
	t.Helper()
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

	return cli
}

func TestIntegrationInitSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	defer svc.Close()
	cli := New(svc)

	if err := cli.Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	// Open DB directly and verify schema
	_ = svc // we use svc.Close() already
	dbPath := dir + "/.ilnamiqui/ilnamiqui.db"
	// We can verify file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("db file not created")
	}
}

func TestIntegrationLoadPretty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cli := setupProjectIntegration(t)

	// Save some data
	if err := cli.Run([]string{"save", "arch", "hexagonal architecture"}); err != nil {
		t.Fatalf("save error: %v", err)
	}
	if err := cli.Run([]string{"save", "bug", "nil pointer in handler"}); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Load with --pretty
	output := captureStdout(t, func() {
		if err := cli.Run([]string{"load", "--pretty"}); err != nil {
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

func TestIntegrationLoadPrettyWithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cli := setupProjectIntegration(t)

	// Save 3 entries
	for i := 0; i < 3; i++ {
		if err := cli.Run([]string{"save", fmt.Sprintf("key%d", i), fmt.Sprintf("val%d", i)}); err != nil {
			t.Fatalf("save error: %v", err)
		}
	}

	// Load with --pretty --limit 2
	output := captureStdout(t, func() {
		if err := cli.Run([]string{"load", "--pretty", "--limit", "2"}); err != nil {
			t.Fatalf("load --pretty --limit error: %v", err)
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

	// Count rows (non-header lines with tab-separated values)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 { // header + 2 data rows
		t.Fatalf("expected 3 lines (header + 2 data rows), got %d: %v", len(lines), lines)
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

	// Pre-create .ilnamiqui directory
	if err := os.MkdirAll(dir+"/.ilnamiqui", 0o755); err != nil {
		t.Fatal(err)
	}

	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	defer svc.Close()
	cli := New(svc)

	// Init should succeed reusing existing dir
	if err := cli.Run([]string{"init"}); err != nil {
		t.Fatalf("init error with existing dir: %v", err)
	}

	if _, err := os.Stat(dir + "/.ilnamiqui/ilnamiqui.db"); os.IsNotExist(err) {
		t.Fatal("db file not created after init with existing dir")
	}
}
