package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/beabys/ilnamiqui/internal/memory"
)

// setupProject creates a temp dir with .opencode/ and runs init.
func setupProject(t *testing.T) string {
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

	// Run init
	if err := Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	return dir
}

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

func TestRun_init(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	output := captureStdout(t, func() {
		if err := Run([]string{"init"}); err != nil {
			t.Fatalf("init error: %v", err)
		}
	})

	if !strings.Contains(output, "ilnamiqui initialized at") {
		t.Fatalf("unexpected init output: %s", output)
	}

	// Verify DB file exists
	if _, err := os.Stat(dir + "/.opencode/ilnamiqui.db"); os.IsNotExist(err) {
		t.Fatal("db file not created after init")
	}
}

func TestRun_saveAndLoad(t *testing.T) {
	_ = setupProject(t)

	// Save
	output := captureStdout(t, func() {
		if err := Run([]string{"save", "mykey", "myvalue"}); err != nil {
			t.Fatalf("save error: %v", err)
		}
	})

	var entry memory.MemoryEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("unmarshal save output: %v\noutput: %s", err, output)
	}
	if entry.Key != "mykey" {
		t.Fatalf("expected key 'mykey', got %q", entry.Key)
	}
	if entry.Value != "myvalue" {
		t.Fatalf("expected value 'myvalue', got %q", entry.Value)
	}

	// Load all
	output = captureStdout(t, func() {
		if err := Run([]string{"load"}); err != nil {
			t.Fatalf("load error: %v", err)
		}
	})

	var entries []memory.MemoryEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("unmarshal load output: %v\noutput: %s", err, output)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestRun_search(t *testing.T) {
	_ = setupProject(t)

	// Save two entries
	if err := Run([]string{"save", "arch", "hexagonal architecture"}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"save", "bug", "null pointer in handler"}); err != nil {
		t.Fatal(err)
	}

	// Search
	output := captureStdout(t, func() {
		if err := Run([]string{"search", "hexagonal"}); err != nil {
			t.Fatalf("search error: %v", err)
		}
	})

	var entries []memory.MemoryEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("unmarshal search output: %v\noutput: %s", err, output)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(entries))
	}
}

func TestRun_delete(t *testing.T) {
	setupProject(t)

	// Save entry
	output := captureStdout(t, func() {
		if err := Run([]string{"save", "delete-me", "value"}); err != nil {
			t.Fatal(err)
		}
	})

	var entry memory.MemoryEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatal(err)
	}

	// Delete
	output = captureStdout(t, func() {
		if err := Run([]string{"delete", fmt.Sprintf("%d", entry.ID)}); err != nil {
			t.Fatalf("delete error: %v", err)
		}
	})

	if !strings.Contains(output, "deleted entry") {
		t.Fatalf("unexpected delete output: %s", output)
	}
}

func TestRun_sessionStartEnd(t *testing.T) {
	_ = setupProject(t)

	// Start session
	output := captureStdout(t, func() {
		if err := Run([]string{"session", "start"}); err != nil {
			t.Fatalf("session start error: %v", err)
		}
	})

	sessionID := strings.TrimSpace(output)
	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// End session
	output = captureStdout(t, func() {
		if err := Run([]string{"session", "end", "--summary", "test done"}); err != nil {
			t.Fatalf("session end error: %v", err)
		}
	})

	if !strings.Contains(output, "ended session") {
		t.Fatalf("unexpected session end output: %s", output)
	}
}

func TestRun_list(t *testing.T) {
	_ = setupProject(t)

	// Create a session so list has data
	output := captureStdout(t, func() {
		if err := Run([]string{"session", "start"}); err != nil {
			t.Fatal(err)
		}
	})
	sessionID := strings.TrimSpace(output)

	// End it with summary
	if err := Run([]string{"session", "end", "--summary", "completed"}); err != nil {
		t.Fatal(err)
	}

	// List sessions
	output = captureStdout(t, func() {
		if err := Run([]string{"list"}); err != nil {
			t.Fatalf("list error: %v", err)
		}
	})

	var sessions []memory.Session
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		t.Fatalf("unmarshal list output: %v\noutput: %s", err, output)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != sessionID {
		t.Fatalf("expected session ID %q, got %q", sessionID, sessions[0].ID)
	}
}

func TestRun_version(t *testing.T) {
	output := captureStdout(t, func() {
		if err := Run([]string{"version"}); err != nil {
			t.Fatalf("version error: %v", err)
		}
	})
	if !strings.Contains(output, "dev") {
		t.Fatalf("expected 'dev' version, got %q", strings.TrimSpace(output))
	}
}

func TestRun_help(t *testing.T) {
	output := captureStdout(t, func() {
		if err := Run([]string{"help"}); err != nil {
			t.Fatalf("help error: %v", err)
		}
	})
	if !strings.Contains(output, "ilnamiqui") {
		t.Fatalf("help output should contain 'ilnamiqui': %s", output)
	}
}

func TestRun_noArgs(t *testing.T) {
	output := captureStdout(t, func() {
		if err := Run([]string{}); err != nil {
			t.Fatalf("empty args error: %v", err)
		}
	})
	if !strings.Contains(output, "ilnamiqui") {
		t.Fatalf("expected help output: %s", output)
	}
}

func TestRun_unknownCommand(t *testing.T) {
	err := Run([]string{"foobar"})
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Fatalf("expected error to mention 'foobar', got %v", err)
	}
}

func TestRun_saveWithoutInit(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	err := Run([]string{"save", "key", "value"})
	if err == nil {
		t.Fatal("expected error when saving without init")
	}
	if !strings.Contains(err.Error(), "init") {
		t.Fatalf("expected error to suggest 'init', got %v", err)
	}
}

func TestRun_saveMissingArgs(t *testing.T) {
	_ = setupProject(t)

	err := Run([]string{"save"})
	if err == nil {
		t.Fatal("expected error for missing save args")
	}
}

func TestRun_deleteMissingArgs(t *testing.T) {
	_ = setupProject(t)

	err := Run([]string{"delete"})
	if err == nil {
		t.Fatal("expected error for missing delete args")
	}
}

func TestRun_deleteInvalidID(t *testing.T) {
	_ = setupProject(t)

	err := Run([]string{"delete", "not-a-number"})
	if err == nil {
		t.Fatal("expected error for non-numeric delete ID")
	}
}

func TestRun_searchMissingArgs(t *testing.T) {
	_ = setupProject(t)

	err := Run([]string{"search"})
	if err == nil {
		t.Fatal("expected error for missing search args")
	}
}

func TestRun_sessionNoSubcommand(t *testing.T) {
	err := Run([]string{"session"})
	if err == nil {
		t.Fatal("expected error for session without subcommand")
	}
}

func TestRun_sessionEndNoActive(t *testing.T) {
	_ = setupProject(t)

	// If no session exists, session end will create one via GetActiveSession then end it.
	// This should succeed.
	output := captureStdout(t, func() {
		if err := Run([]string{"session", "end", "--summary", "auto"}); err != nil {
			t.Fatalf("session end without start: %v", err)
		}
	})
	if !strings.Contains(output, "ended session") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestRun_loadSessionFlag(t *testing.T) {
	_ = setupProject(t)

	// Save without explicit session start — auto-creates session
	if err := Run([]string{"save", "key1", "val1"}); err != nil {
		t.Fatal(err)
	}

	// Load with --session flag
	output := captureStdout(t, func() {
		if err := Run([]string{"load", "--session"}); err != nil {
			t.Fatalf("load --session error: %v", err)
		}
	})

	var entries []memory.MemoryEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 session entry, got %d", len(entries))
	}
}

func TestRun_initCreatesDotOpenCode(t *testing.T) {
	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	// No .opencode dir — init should create it
	if err := Run([]string{"init"}); err != nil {
		t.Fatalf("init error: %v", err)
	}

	info, err := os.Stat(dir + "/.opencode")
	if err != nil {
		t.Fatal(".opencode dir not created")
	}
	if !info.IsDir() {
		t.Fatal(".opencode is not a directory")
	}
}

func TestRun_prettyFlag(t *testing.T) {
	_ = setupProject(t)

	// Save
	if err := Run([]string{"save", "key", "value"}); err != nil {
		t.Fatal(err)
	}

	// Load with --pretty
	output := captureStdout(t, func() {
		if err := Run([]string{"load", "--pretty"}); err != nil {
			t.Fatalf("load --pretty error: %v", err)
		}
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Key") {
		t.Fatalf("expected table headers in pretty output: %s", output)
	}
}

func TestRun_loadEmpty(t *testing.T) {
	_ = setupProject(t)

	output := captureStdout(t, func() {
		if err := Run([]string{"load"}); err != nil {
			t.Fatalf("load error: %v", err)
		}
	})

	var entries []memory.MemoryEntry
	if err := json.Unmarshal([]byte(output), &entries); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty array, got %d entries", len(entries))
	}
}

func TestRun_listEmpty(t *testing.T) {
	_ = setupProject(t)

	output := captureStdout(t, func() {
		if err := Run([]string{"list"}); err != nil {
			t.Fatalf("list error: %v", err)
		}
	})

	var sessions []memory.Session
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected empty array, got %d sessions", len(sessions))
	}
}

func TestRun_listPretty(t *testing.T) {
	_ = setupProject(t)

	// Create a session so list has data
	if err := Run([]string{"session", "start"}); err != nil {
		t.Fatal(err)
	}
	if err := Run([]string{"session", "end", "--summary", "done"}); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		if err := Run([]string{"list", "--pretty"}); err != nil {
			t.Fatalf("list --pretty error: %v", err)
		}
	})

	if !strings.Contains(output, "ID") || !strings.Contains(output, "Project") {
		t.Fatalf("expected table headers in pretty list: %s", output)
	}
}

func TestRun_listWithLimit(t *testing.T) {
	_ = setupProject(t)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		if err := Run([]string{"session", "start"}); err != nil {
			t.Fatal(err)
		}
		if err := Run([]string{"session", "end", "--summary", "test"}); err != nil {
			t.Fatal(err)
		}
	}

	output := captureStdout(t, func() {
		if err := Run([]string{"list", "--limit", "2"}); err != nil {
			t.Fatalf("list --limit error: %v", err)
		}
	})

	var sessions []memory.Session
	if err := json.Unmarshal([]byte(output), &sessions); err != nil {
		t.Fatalf("unmarshal: %v\noutput: %s", err, output)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions with limit=2, got %d", len(sessions))
	}
}
