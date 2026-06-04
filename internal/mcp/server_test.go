package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/mock"

	"github.com/beabys/ilnamiqui/internal/memory"
	nmocks "github.com/beabys/ilnamiqui/internal/mocks"
	"github.com/beabys/ilnamiqui/internal/service"
)

func TestHandler_RegisterTools(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	h := NewHandler(mockSvc)

	s := server.NewMCPServer("test", "1.0.0")
	h.RegisterTools(s)

	// Verify tools are registered by calling ListTools
	tools := s.ListTools()
	if len(tools) != 9 {
		t.Fatalf("expected 9 tools, got %d", len(tools))
	}
}

func TestHandler_Save(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	entry := &memory.MemoryEntry{ID: 1, Key: "k", Value: "v"}
	mockSvc.EXPECT().Save(mock.Anything, &service.SaveRequest{Key: "k", Value: "v"}).
		Return(&service.SaveResponse{Entry: entry}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"key":   "k",
		"value": "v",
	}

	result, err := h.handleSave(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSave error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
}

func TestHandler_Save_MissingKey(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	h := NewHandler(mockSvc)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"value": "v",
	}

	result, err := h.handleSave(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSave error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for missing key")
	}
}

func TestHandler_Load(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Load(mock.Anything, &service.LoadRequest{Limit: 50, SessionOnly: false}).
		Return(&service.LoadResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "k", Value: "v"},
		}}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := h.handleLoad(context.Background(), req)
	if err != nil {
		t.Fatalf("handleLoad error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
}

func TestHandler_Search(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "test", Mode: memory.SearchModeKey, Limit: 10, After: nil, Before: nil}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "test", Value: "found"},
		}}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"query": "test",
	}

	result, err := h.handleSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
}

func TestHandler_Search_WithDateRange(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	after := parseTimeMCP(t, "2026-01-01T00:00:00Z")
	before := parseTimeMCP(t, "2026-12-31T23:59:59Z")
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "", Mode: memory.SearchModeKey, Limit: 10, After: &after, Before: &before}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "test", Value: "found"},
		}}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"after":  "2026-01-01T00:00:00Z",
		"before": "2026-12-31T23:59:59Z",
	}

	result, err := h.handleSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
}

func TestHandler_Search_NoQueryNoDate(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	h := NewHandler(mockSvc)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := h.handleSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when no query, after, or before provided")
	}
}

func TestHandler_Search_ModeParam(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Search(mock.Anything, &service.SearchRequest{Query: "arch", Mode: memory.SearchModeContent, Limit: 10, After: nil, Before: nil}).
		Return(&service.SearchResponse{Entries: []memory.MemoryEntry{
			{ID: 1, Key: "arch", Value: "hexagonal architecture"},
		}}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"query": "arch",
		"mode":  "content",
	}

	result, err := h.handleSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
}

func TestHandler_Search_InvalidMode(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	h := NewHandler(mockSvc)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"query": "test",
		"mode":  "invalid",
	}

	result, err := h.handleSearch(context.Background(), req)
	if err != nil {
		t.Fatalf("handleSearch error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid mode")
	}
}

// extractText extracts the text content from a CallToolResult.
func extractText(result *mcp.CallToolResult) string {
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func parseTimeMCP(t *testing.T, s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time %q: %v", s, err)
	}
	return tm
}

func TestHandler_ListSessions(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().ListSessions(mock.Anything, &service.ListSessionsRequest{Limit: 10}).
		Return(&service.ListSessionsResponse{Sessions: []memory.Session{
			{ID: "sess-1", Project: "test"},
		}}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := h.handleListSessions(context.Background(), req)
	if err != nil {
		t.Fatalf("handleListSessions error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
}

func TestHandler_Delete(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().Delete(mock.Anything, &service.DeleteRequest{ID: 42}).
		Return(&service.DeleteResponse{}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"id": float64(42),
	}

	result, err := h.handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
}

func TestHandler_ListKeys(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().ListKeys(mock.Anything, &service.ListKeysRequest{Limit: 50}).
		Return(&service.ListKeysResponse{Keys: []memory.KeyInfo{
			{Key: "project-path", Critical: true},
			{Key: "arch", Critical: false},
		}}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}

	result, err := h.handleListKeys(context.Background(), req)
	if err != nil {
		t.Fatalf("handleListKeys error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := extractText(result)
	if !strings.Contains(text, "project-path") {
		t.Fatalf("expected 'project-path' in list_keys output, got: %s", text)
	}
	if !strings.Contains(text, "true") {
		t.Fatalf("expected 'true' for critical in list_keys output, got: %s", text)
	}
}

func TestHandler_ListKeys_WithLimit(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	mockSvc.EXPECT().ListKeys(mock.Anything, &service.ListKeysRequest{Limit: 10}).
		Return(&service.ListKeysResponse{Keys: []memory.KeyInfo{
			{Key: "a", Critical: false},
			{Key: "b", Critical: false},
		}}, nil)

	h := NewHandler(mockSvc)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"limit": float64(10),
	}

	result, err := h.handleListKeys(context.Background(), req)
	if err != nil {
		t.Fatalf("handleListKeys error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(mcp.TextContent).Text)
	}
	text := extractText(result)
	if !strings.Contains(text, "a") || !strings.Contains(text, "b") {
		t.Fatalf("expected both keys in output, got: %s", text)
	}
}

func TestHandler_Delete_InvalidID(t *testing.T) {
	mockSvc := nmocks.NewService(t)
	h := NewHandler(mockSvc)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"id": float64(0),
	}

	result, err := h.handleDelete(context.Background(), req)
	if err != nil {
		t.Fatalf("handleDelete error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error for invalid id")
	}
}
