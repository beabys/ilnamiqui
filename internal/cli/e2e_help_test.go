//go:build e2e

package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/beabys/ilnamiqui/internal/service"
)

// TestE2E_HelpDocs validates per-command help + docs requirements 1-6:
// 1. `help` → exit 0; stdout contains `help [command]`, `--mode`, `--`, flags-anywhere note.
// 2. `help <cmd>` → exit 0 and non-empty for each known command.
// 3. `help bogus` → exit 1, stderr contains `unknown command "bogus"`.
// 4. `<cmd> --help` and `<cmd> -h` → exit 0, stdout has usage, stderr no `error:`.
// 5. `help search` stdout contains all required tokens.
// 6. Invalid flag still errors: `search --nope x` → exit 1.
func TestE2E_HelpDocs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// Setup: temp dir, init (self-seeding, never real DB)
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

	// Req 1: top-level help
	t.Run("R1_top_level_help", func(t *testing.T) {
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"help"}); err != nil {
				t.Fatalf("help exit non-zero: %v", err)
			}
		})
		if !strings.Contains(out, "help [command]") {
			t.Errorf("stdout missing 'help [command]'. Got:\n%s", out)
		}
		if !strings.Contains(out, "--mode") {
			t.Errorf("stdout missing '--mode'. Got:\n%s", out)
		}
		if !strings.Contains(out, "--") {
			t.Errorf("stdout missing '--' terminator. Got:\n%s", out)
		}
		if !strings.Contains(out, "before or after") {
			t.Errorf("stdout missing flags-anywhere note. Got:\n%s", out)
		}
	})

	// Req 2: per-command help for all known commands
	t.Run("R2_per_command_help", func(t *testing.T) {
		cmds := []string{"init", "save", "load", "list", "keys", "key", "search", "prune", "delete", "session", "version", "help"}
		for _, cmd := range cmds {
			out, _ := captureStdoutStderr(t, func() {
				if err := cli.Run([]string{"help", cmd}); err != nil {
					t.Fatalf("help %s exit non-zero: %v", cmd, err)
				}
			})
			if len(strings.TrimSpace(out)) == 0 {
				t.Errorf("help %s produced empty output", cmd)
			}
		}
	})

	// Req 3: unknown command
	t.Run("R3_help_bogus", func(t *testing.T) {
		var runErr error
		captureStdoutStderr(t, func() {
			runErr = cli.Run([]string{"help", "bogus"})
		})
		if runErr == nil {
			t.Fatal("expected non-zero exit for help bogus")
		}
		if !strings.Contains(runErr.Error(), `unknown command "bogus"`) {
			t.Errorf("error missing 'unknown command \"bogus\"'. Got: %v", runErr)
		}
	})

	// Req 4: --help and -h for each command
	t.Run("R4_help_flags", func(t *testing.T) {
		cmds := []string{"search", "save", "load", "keys", "prune", "delete"}
		for _, cmd := range cmds {
			// --help
			outLong, errLong := captureStdoutStderr(t, func() {
				if err := cli.Run([]string{cmd, "--help"}); err != nil {
					t.Fatalf("%s --help exit non-zero: %v", cmd, err)
				}
			})
			if len(strings.TrimSpace(outLong)) == 0 {
				t.Errorf("%s --help produced empty output", cmd)
			}
			if strings.Contains(errLong, "error:") {
				t.Errorf("%s --help stderr has 'error:': %s", cmd, errLong)
			}

			// -h
			outShort, errShort := captureStdoutStderr(t, func() {
				if err := cli.Run([]string{cmd, "-h"}); err != nil {
					t.Fatalf("%s -h exit non-zero: %v", cmd, err)
				}
			})
			if len(strings.TrimSpace(outShort)) == 0 {
				t.Errorf("%s -h produced empty output", cmd)
			}
			if strings.Contains(errShort, "error:") {
				t.Errorf("%s -h stderr has 'error:': %s", cmd, errShort)
			}
		}
	})

	// Req 5: help search contains all required tokens
	t.Run("R5_help_search_tokens", func(t *testing.T) {
		out, _ := captureStdoutStderr(t, func() {
			if err := cli.Run([]string{"help", "search"}); err != nil {
				t.Fatalf("help search exit non-zero: %v", err)
			}
		})
		required := []string{
			"--mode key|content|both",
			"--pretty",
			"--limit",
			"--after",
			"--before",
			"key",
			"content",
			"both",
			"FTS5",
			"default",
		}
		for _, tok := range required {
			if !strings.Contains(out, tok) {
				t.Errorf("help search missing token %q. Got:\n%s", tok, out)
			}
		}
	})

	// Req 6: invalid flag still errors
	t.Run("R6_invalid_flag_errors", func(t *testing.T) {
		_, _ = captureStdoutStderr(t, func() {
			err := cli.Run([]string{"search", "--nope", "x"})
			if err == nil {
				t.Fatal("expected non-zero exit for search --nope x")
			}
		})
	})
}
