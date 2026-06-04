package cli

import (
	"bytes"
	"encoding/json"
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
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v"}).
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
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v"}).
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

func TestCLI_SessionStart(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().StartSession(mock.Anything, &service.StartSessionRequest{}).
		Return(&service.StartSessionResponse{Session: &memory.Session{ID: "new-sess-id"}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"session", "start"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLI_SessionEnd(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().EndSession(mock.Anything, &service.EndSessionRequest{Summary: "done"}).
		Return(&service.EndSessionResponse{Session: &memory.Session{ID: "sess-1"}}, nil)

	cli := New(mockSvc)
	err := cli.Run([]string{"session", "end", "--summary", "done"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
