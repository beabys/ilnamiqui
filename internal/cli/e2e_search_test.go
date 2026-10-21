//go:build e2e

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/beabys/ilnamiqui/internal/service"
)

// captureStdoutStderr captures both stdout and stderr during fn execution.
func captureStdoutStderr(t *testing.T, fn func()) (stdout, stderr string) {
	t.Helper()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = wOut
	os.Stderr = wErr
	fn()
	wOut.Close()
	wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	var bufOut, bufErr bytes.Buffer
	if _, err := bufOut.ReadFrom(rOut); err != nil {
		t.Fatal(err)
	}
	if _, err := bufErr.ReadFrom(rErr); err != nil {
		t.Fatal(err)
	}
	return bufOut.String(), bufErr.String()
}

// countRows counts entries in JSON array output. Returns -1 for pretty/table output.
func countRows(output string) int {
	output = strings.TrimSpace(output)
	if output == "[]" {
		return 0
	}
	if strings.HasPrefix(output, "[") {
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(output), &arr); err != nil {
			return -1
		}
		return len(arr)
	}
	// Pretty/table output - count non-header lines
	lines := strings.Split(output, "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ID") || strings.HasPrefix(line, "Key") {
			continue
		}
		count++
	}
	return count
}

// TestE2E_SearchFix validates the search bug fix:
// - Flags before OR after query
// - FTS5 metacharacters never error
// - Default mode stays key; zero results prints hint
// - save accepts flags after positionals
// - No regressions
func TestE2E_SearchFix(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Setup: temp dir, init, seed
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

	// Seed deterministic entries
	if err := cli.Run([]string{"save", "architecture", "hexagonal design with ports and adapters"}); err != nil {
		t.Fatalf("save architecture error: %v", err)
	}
	if err := cli.Run([]string{"save", "bug", "flag parsing broken in search"}); err != nil {
		t.Fatalf("save bug error: %v", err)
	}

	// Scenario A: flags AFTER query
	t.Run("A_flags_after_query", func(t *testing.T) {
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "hexagonal", "--mode", "content", "--pretty"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		count := countRows(out)
		if count != 1 {
			t.Errorf("expected 1 row, got %d. Output:\n%s", count, out)
		}
	})

	// Scenario B: flags BEFORE query
	t.Run("B_flags_before_query", func(t *testing.T) {
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "--mode", "content", "hexagonal", "--pretty"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		count := countRows(out)
		if count != 1 {
			t.Errorf("expected 1 row, got %d. Output:\n%s", count, out)
		}
	})

	// Scenario C: key mode (default), flags after
	t.Run("C_key_mode_flags_after", func(t *testing.T) {
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "architecture", "--pretty"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		count := countRows(out)
		if count != 1 {
			t.Errorf("expected 1 row, got %d. Output:\n%s", count, out)
		}
	})

	// Scenario D: FTS5 metachar (date with hyphens), 0 rows, no error
	t.Run("D_fts5_date_no_error", func(t *testing.T) {
		out, stderr := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "2026-01-01", "--mode", "content"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		if strings.Contains(stderr, "fts5") && strings.Contains(stderr, "syntax error") {
			t.Errorf("stderr has FTS5 error: %s", stderr)
		}
		count := countRows(out)
		if count != 0 {
			t.Errorf("expected 0 rows, got %d. Output:\n%s", count, out)
		}
	})

	// Scenario E: FTS5 metachar (c++), exit 0, no syntax error
	t.Run("E_fts5_cplusplus", func(t *testing.T) {
		_, stderr := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "c++", "--mode", "content"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		if strings.Contains(stderr, "fts5") && strings.Contains(stderr, "syntax error") {
			t.Errorf("stderr has FTS5 syntax error: %s", stderr)
		}
	})

	// Scenario F: FTS5 metachar (parens), 0 rows, exit 0
	t.Run("F_fts5_parens", func(t *testing.T) {
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "()", "--mode", "content"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		count := countRows(out)
		if count != 0 {
			t.Errorf("expected 0 rows, got %d. Output:\n%s", count, out)
		}
	})

	// Scenario G: no results in content mode, no hint on stderr
	t.Run("G_no_results_content_no_hint", func(t *testing.T) {
		out, stderr := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "no-such-term-xyz", "--mode", "content", "--pretty"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		if strings.Contains(stderr, "try --mode both") {
			t.Errorf("stderr has hint in content mode: %s", stderr)
		}
		if strings.Contains(out, "try --mode both") {
			t.Errorf("stdout has stray hint text: %s", out)
		}
	})

	// Scenario H: no results in key mode (default), hint on stderr
	t.Run("H_no_results_key_hint", func(t *testing.T) {
		_, stderr := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "zzzz"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		if !strings.Contains(stderr, "no entries found — try --mode both to search values") {
			t.Errorf("stderr missing hint. Got: %s", stderr)
		}
	})

	// Scenario I: --flag=value form
	t.Run("I_flag_value_form", func(t *testing.T) {
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "--mode=content", "--limit=5", "hexagonal"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		count := countRows(out)
		if count != 1 {
			t.Errorf("expected 1 row, got %d. Output:\n%s", count, out)
		}
	})

	// Scenario J: invalid mode, exit non-zero, error contains "invalid --mode"
	t.Run("J_invalid_mode", func(t *testing.T) {
		_, _ = captureStdoutStderr(t, func() {
			err := cli.Run([]string{"search", "--mode", "invalid", "test"})
			if err == nil {
				t.Fatal("expected non-zero exit for invalid mode")
			}
			if !strings.Contains(err.Error(), "invalid --mode") {
				t.Errorf("error missing 'invalid --mode'. Got: %v", err)
			}
		})
	})

	// Scenario K: save with flags after positionals
	t.Run("K_save_flags_after", func(t *testing.T) {
		_, _ = captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"save", "bug2", "new flag parsing test", "--pretty"}); err != nil {
				t.Fatalf("save with --pretty failed: %v", err)
			}
		})
		// Verify persisted
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "bug2", "--pretty"}); err != nil {
				t.Fatalf("search bug2 failed: %v", err)
			}
		})
		count := countRows(out)
		if count != 1 {
			t.Errorf("expected 1 row after save, got %d. Output:\n%s", count, out)
		}
	})

	// Scenario L: -- terminator, literal query
	t.Run("L_dash_dash_terminator", func(t *testing.T) {
		_, stderr := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"search", "--", "--mode"}); err != nil {
				t.Fatalf("exit code non-zero: %v", err)
			}
		})
		// Should treat --mode as literal query, 0 results
		// In key mode (default), 0 results → hint on stderr
		if !strings.Contains(stderr, "no entries found") {
			t.Logf("stderr: %s", stderr)
		}
	})

	// Scenario M: regression - other commands still work
	t.Run("M_regression", func(t *testing.T) {
		// load
		_, _ = captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"load", "--limit", "5"}); err != nil {
				t.Errorf("load failed: %v", err)
			}
		})
		// keys
		_, _ = captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"keys", "--pretty"}); err != nil {
				t.Errorf("keys failed: %v", err)
			}
		})
		// list
		_, _ = captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"list", "--limit", "5"}); err != nil {
				t.Errorf("list failed: %v", err)
			}
		})
		// session start/end
		_, _ = captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"session", "start"}); err != nil {
				t.Errorf("session start failed: %v", err)
			}
		})
		_, _ = captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"session", "end", "--summary", "test"}); err != nil {
				t.Errorf("session end failed: %v", err)
			}
		})
	})
}
