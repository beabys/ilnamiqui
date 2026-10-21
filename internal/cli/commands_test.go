package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/beabys/ilnamiqui/internal/memory"
	nmocks "github.com/beabys/ilnamiqui/internal/mocks"
	"github.com/beabys/ilnamiqui/internal/service"
	"github.com/stretchr/testify/mock"
)

func TestCLI_Save_Pretty(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	entry := &memory.MemoryEntry{ID: 1, Key: "k", Value: "v"}
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v", Agent: "opencode"}).
		Return(&service.SaveResponse{Entry: entry}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"save", "--pretty", "k", "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Init(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Init(mock.Anything, &service.InitRequest{}).
		Return(&service.InitResponse{DBPath: "/tmp/test/ilnamiqui.db"}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"init"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Save_JSON(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	entry := &memory.MemoryEntry{ID: 1, Key: "k", Value: "v"}
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v", Agent: "opencode"}).
		Return(&service.SaveResponse{Entry: entry}, nil)

	cli := New(mockSvc)

	// Capture stdout
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	err := cli.Run([]string{"save", "k", "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = stdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var decoded memory.MemoryEntry
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if decoded.Key != "k" {
		t.Fatalf("expected key 'k', got %q", decoded.Key)
	}
}

func TestCLI_Delete(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Delete(mock.Anything, &service.DeleteRequest{ID: 42}).
		Return(&service.DeleteResponse{}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"delete", "42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Load(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Load(mock.Anything, &service.LoadRequest{Limit: 10, SessionOnly: true}).
		Return(&service.LoadResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "k", Value: "v"},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"load", "--session", "--limit", "10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "test", Mode: memory.SearchModeKey, Limit: 5, After: nil, Before: nil}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "test", Value: "found"},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--limit", "5", "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_WithAfter(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	after := parseTime(t, "2026-01-01T00:00:00Z")
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "test", Mode: memory.SearchModeKey, Limit: 0, After: &after, Before: nil}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "test", Value: "found"},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--after", "2026-01-01", "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_ModeFlag(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "arch", Mode: memory.SearchModeKey, Limit: 0, After: nil, Before: nil}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "architecture", Value: "design pattern"},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--mode", "key", "arch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_ModeContent(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "hexagonal", Mode: memory.SearchModeContent, Limit: 0, After: nil, Before: nil}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "arch", Value: "hexagonal architecture"},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--mode", "content", "hexagonal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_ModeBoth(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "arch", Mode: memory.SearchModeBoth, Limit: 0, After: nil, Before: nil}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "architecture", Value: "hexagonal architecture"},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--mode", "both", "arch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_InvalidMode(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	cli := New(mockSvc)

	err := cli.Run([]string{"search", "--mode", "invalid", "test"})
	if err == nil {
		t.Fatal("expected error for invalid --mode")
	}
}

func parseTime(t *testing.T, s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

func TestCLI_Keys_FlagParsing(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().ListKeys(mock.Anything, &service.ListKeysRequest{Limit: 5}).
		Return(&service.ListKeysResponse{Keys: []memory.KeyInfo{
			{Key: "alpha", Critical: false},
			{Key: "beta", Critical: true},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"keys", "--limit", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Keys_Pretty(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().ListKeys(mock.Anything, &service.ListKeysRequest{Limit: 0}).
		Return(&service.ListKeysResponse{Keys: []memory.KeyInfo{
			{Key: "project-path", Critical: true},
		}}, nil)

	cli := New(mockSvc)

	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	err := cli.Run([]string{"keys", "--pretty"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = stdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()
	if !strings.Contains(output, "Key") {
		t.Fatalf("expected header 'Key' in pretty output, got: %s", output)
	}
	if !strings.Contains(output, "project-path") {
		t.Fatalf("expected 'project-path' in pretty output, got: %s", output)
	}
	if !strings.Contains(output, "true") {
		t.Fatalf("expected 'true' for critical in pretty output, got: %s", output)
	}
}

func TestCLI_List(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().ListSessions(mock.Anything, &service.ListSessionsRequest{Limit: 5}).
		Return(&service.ListSessionsResponse{Sessions: []memory.Session{
			{ID: "sess-1", Project: "test"},
		}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"list", "--limit", "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Save_WithAgent(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	entry := &memory.MemoryEntry{ID: 1, Key: "k", Value: "v"}
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v", Agent: "claude-code"}).
		Return(&service.SaveResponse{Entry: entry}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"save", "--agent", "claude-code", "k", "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SessionStart_WithAgent(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().StartSession(mock.Anything, &service.StartSessionRequest{Agent: "claude-code"}).
		Return(&service.StartSessionResponse{Session: &memory.Session{ID: "new-sess-id"}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"session", "start", "--agent", "claude-code"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SessionEnd_WithAgent(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().EndSession(mock.Anything, &service.EndSessionRequest{Summary: "done", Agent: "claude-code"}).
		Return(&service.EndSessionResponse{Session: &memory.Session{ID: "sess-1"}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"session", "end", "--agent", "claude-code", "--summary", "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SessionStart(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().StartSession(mock.Anything, &service.StartSessionRequest{Agent: "opencode"}).
		Return(&service.StartSessionResponse{Session: &memory.Session{ID: "new-sess-id"}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"session", "start"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SessionEnd(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().EndSession(mock.Anything, &service.EndSessionRequest{Summary: "done", Agent: "opencode"}).
		Return(&service.EndSessionResponse{Session: &memory.Session{ID: "sess-1"}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"session", "end", "--summary", "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// New tests for flags-anywhere parsing

func TestCLI_Search_FlagsAfterQuery(t *testing.T) {
	// Flags AFTER query — the bug we're fixing
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{
		Query: "hexagonal", Mode: memory.SearchModeContent, Limit: 0, After: nil, Before: nil,
	}).Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
		{ID: 1, Key: "arch", Value: "hexagonal architecture"},
	}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "hexagonal", "--mode", "content", "--pretty"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_FlagEqualsForm(t *testing.T) {
	// --flag=value form
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{
		Query: "test", Mode: memory.SearchModeContent, Limit: 5, After: nil, Before: nil,
	}).Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
		{ID: 1, Key: "test", Value: "value"},
	}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--mode=content", "--limit=5", "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_Terminator(t *testing.T) {
	// -- terminator stops flag parsing
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{
		Query: "--not-a-flag", Mode: memory.SearchModeKey, Limit: 0, After: nil, Before: nil,
	}).Return(&service.SearchResponse{Entries: []memory.MemoryEntry{}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--", "--not-a-flag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Save_FlagsAfterPositionals(t *testing.T) {
	// save key value --pretty — flags after positionals
	mockSvc := nmocks.NewService(t)
	entry := &memory.MemoryEntry{ID: 1, Key: "k", Value: "v"}
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v", Agent: "opencode"}).
		Return(&service.SaveResponse{Entry: entry}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"save", "k", "v", "--pretty"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Search_HintOnZeroResults(t *testing.T) {
	// When mode=key and query non-empty and 0 results → hint to stderr
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{
		Query: "nonexistent", Mode: memory.SearchModeKey, Limit: 0, After: nil, Before: nil,
	}).Return(&service.SearchResponse{Entries: []memory.MemoryEntry{}}, nil)

	cli := New(mockSvc)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := cli.Run([]string{"search", "nonexistent"})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "no entries found — try --mode both to search values") {
		t.Fatalf("expected hint on stderr, got: %q", output)
	}
}

func TestCLI_Search_NoHintWhenResultsFound(t *testing.T) {
	// When results found → no hint
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{
		Query: "test", Mode: memory.SearchModeKey, Limit: 0, After: nil, Before: nil,
	}).Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
		{ID: 1, Key: "test", Value: "value"},
	}}, nil)

	cli := New(mockSvc)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := cli.Run([]string{"search", "test"})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if strings.Contains(output, "no entries found") {
		t.Fatalf("did not expect hint when results found, got: %q", output)
	}
}

func TestCLI_Search_NoHintWhenModeContent(t *testing.T) {
	// When mode=content and 0 results → no hint (hint only for mode=key)
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{
		Query: "nonexistent", Mode: memory.SearchModeContent, Limit: 0, After: nil, Before: nil,
	}).Return(&service.SearchResponse{Entries: []memory.MemoryEntry{}}, nil)

	cli := New(mockSvc)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := cli.Run([]string{"search", "--mode", "content", "nonexistent"})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if strings.Contains(output, "no entries found") {
		t.Fatalf("did not expect hint for mode=content, got: %q", output)
	}
}

func TestCLI_Search_MultipleWordsQuery(t *testing.T) {
	// Multi-word query joined with spaces
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{
		Query: "hexagonal architecture", Mode: memory.SearchModeKey, Limit: 0, After: nil, Before: nil,
	}).Return(&service.SearchResponse{Entries: []memory.MemoryEntry{}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"search", "hexagonal", "architecture"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Save_FlagEqualsForm(t *testing.T) {
	// --agent=claude-code form
	mockSvc := nmocks.NewService(t)
	entry := &memory.MemoryEntry{ID: 1, Key: "k", Value: "v"}
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v", Agent: "claude-code"}).
		Return(&service.SaveResponse{Entry: entry}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"save", "--agent=claude-code", "k", "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_Save_Terminator(t *testing.T) {
	// -- terminator stops flag parsing
	mockSvc := nmocks.NewService(t)
	entry := &memory.MemoryEntry{ID: 1, Key: "key", Value: "--not-a-flag"}
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "key", Value: "--not-a-flag", Agent: "opencode"}).
		Return(&service.SaveResponse{Entry: entry}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"save", "--", "key", "--not-a-flag"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseFlagsAnywhere_Basic(t *testing.T) {
	// Unit test for parseFlagsAnywhere helper
	var fs flag.FlagSet
	pretty := fs.Bool("pretty", false, "")
	limit := fs.Int("limit", 0, "")
	mode := fs.String("mode", "key", "")

	args := []string{"query", "--mode", "content", "--pretty", "extra"}
	positionals, err := parseFlagsAnywhere(&fs, args, map[string]bool{"limit": true, "mode": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !*pretty {
		t.Fatal("expected --pretty to be true")
	}
	if *mode != "content" {
		t.Fatalf("expected --mode=content, got %q", *mode)
	}
	if *limit != 0 {
		t.Fatalf("expected --limit=0, got %d", *limit)
	}
	if len(positionals) != 2 || positionals[0] != "query" || positionals[1] != "extra" {
		t.Fatalf("expected positionals [query extra], got %v", positionals)
	}
}

func TestParseFlagsAnywhere_EqualsForm(t *testing.T) {
	var fs flag.FlagSet
	mode := fs.String("mode", "key", "")

	args := []string{"query", "--mode=content"}
	positionals, err := parseFlagsAnywhere(&fs, args, map[string]bool{"mode": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if *mode != "content" {
		t.Fatalf("expected --mode=content, got %q", *mode)
	}
	if len(positionals) != 1 || positionals[0] != "query" {
		t.Fatalf("expected positionals [query], got %v", positionals)
	}
}

func TestParseFlagsAnywhere_Terminator(t *testing.T) {
	var fs flag.FlagSet
	pretty := fs.Bool("pretty", false, "")

	args := []string{"--pretty", "--", "--not-a-flag"}
	positionals, err := parseFlagsAnywhere(&fs, args, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !*pretty {
		t.Fatal("expected --pretty to be true")
	}
	if len(positionals) != 1 || positionals[0] != "--not-a-flag" {
		t.Fatalf("expected positionals [--not-a-flag], got %v", positionals)
	}
}

func TestParseFlagsAnywhere_UnknownFlag(t *testing.T) {
	var fs flag.FlagSet
	args := []string{"query", "--unknown"}
	_, err := parseFlagsAnywhere(&fs, args, map[string]bool{})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

// Help system tests

func TestPrintCommandHelp_AllCommands(t *testing.T) {
	cases := []struct {
		name  string
		token string // substring that must appear in output
	}{
		{"init", "ilnamiqui init"},
		{"save", "ilnamiqui save"},
		{"load", "ilnamiqui load"},
		{"list", "ilnamiqui list"},
		{"keys", "ilnamiqui keys"},
		{"key", "ilnamiqui key"},
		{"key update", "ilnamiqui key update"},
		{"search", "ilnamiqui search"},
		{"prune", "ilnamiqui prune"},
		{"delete", "ilnamiqui delete"},
		{"session", "ilnamiqui session"},
		{"session start", "ilnamiqui session start"},
		{"session end", "ilnamiqui session end"},
		{"version", "ilnamiqui version"},
		{"help", "ilnamiqui help"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, w, _ := os.Pipe()
			stdout := os.Stdout
			os.Stdout = w
			err := printCommandHelp(tc.name)
			w.Close()
			os.Stdout = stdout
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var buf bytes.Buffer
			buf.ReadFrom(r)
			out := buf.String()
			if len(out) == 0 {
				t.Fatalf("empty help for %q", tc.name)
			}
			if !strings.Contains(out, tc.token) {
				t.Fatalf("help for %q missing token %q; got:\n%s", tc.name, tc.token, out)
			}
		})
	}
}

func TestPrintCommandHelp_Unknown(t *testing.T) {
	err := printCommandHelp("bogus")
	if err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error should mention 'unknown command': %v", err)
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("error should mention the command name: %v", err)
	}
}

func TestPrintCommandHelp_SearchContent(t *testing.T) {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	err := printCommandHelp("search")
	w.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()
	for _, tok := range []string{"--mode", "content", "FTS5", "--pretty", "default"} {
		if !strings.Contains(out, tok) {
			t.Fatalf("search help missing %q; got:\n%s", tok, out)
		}
	}
}

func TestPrintHelp_TopLevel(t *testing.T) {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w
	err := printHelp()
	w.Close()
	os.Stdout = stdout
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "help [command]") {
		t.Fatalf("top-level help missing 'help [command]'; got:\n%s", out)
	}
	if !strings.Contains(out, "--") {
		t.Fatalf("top-level help missing '--' terminator note; got:\n%s", out)
	}
}

func TestCLI_Search_Help(t *testing.T) {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	mockSvc := nmocks.NewService(t)
	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--help"})

	w.Close()
	os.Stdout = stdout

	if err != nil {
		t.Fatalf("expected nil error for search --help, got: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "--mode") {
		t.Fatalf("search --help output missing '--mode'; got:\n%s", out)
	}
}

func TestCLI_Search_InvalidFlag(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	cli := New(mockSvc)
	err := cli.Run([]string{"search", "--nope", "x"})
	if err == nil {
		t.Fatal("expected error for unknown flag --nope")
	}
}

func TestCLI_Save_Help(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	cli := New(mockSvc)
	err := cli.Run([]string{"save", "-h"})
	if err != nil {
		t.Fatalf("expected nil error for save -h, got: %v", err)
	}
}

func TestCLI_HelpSubcommand(t *testing.T) {
	r, w, _ := os.Pipe()
	stdout := os.Stdout
	os.Stdout = w

	mockSvc := nmocks.NewService(t)
	cli := New(mockSvc)
	err := cli.Run([]string{"help", "search"})

	w.Close()
	os.Stdout = stdout

	if err != nil {
		t.Fatalf("expected nil error for 'help search', got: %v", err)
	}
	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()
	if !strings.Contains(out, "ilnamiqui search") {
		t.Fatalf("'help search' output missing 'ilnamiqui search'; got:\n%s", out)
	}
}

func TestCLI_HelpSubcommand_Unknown(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	cli := New(mockSvc)
	err := cli.Run([]string{"help", "bogus"})
	if err == nil {
		t.Fatal("expected error for 'help bogus'")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error should mention 'unknown command': %v", err)
	}
}
