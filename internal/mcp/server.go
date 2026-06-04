// Package mcp implements the ilnamiqui MCP server for Claude Code.
// It provides tools for managing session memory via the Model Context Protocol.
package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/beabys/ilnamiqui/internal/memory"
	"github.com/beabys/ilnamiqui/internal/service"
)

// Handler manages MCP tool handlers for ilnamiqui memory operations.
type Handler struct {
	svc service.Service
}

// NewHandler creates a new MCP handler with the given service.
func NewHandler(svc service.Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterTools registers all ilnamiqui MCP tools on the given server.
func (h *Handler) RegisterTools(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool("init_memory",
			mcp.WithDescription("Initialize the ilnamiqui database in the current project. Creates the .ilnamiqui directory and runs database migrations."),
		),
		h.handleInit,
	)

	s.AddTool(
		mcp.NewTool("save_memory",
			mcp.WithDescription("Save a memory entry for the active session"),
			mcp.WithString("key",
				mcp.Required(),
				mcp.Description("The memory key (e.g., architecture, bug, decision, file, config, dependency)"),
			),
			mcp.WithString("value",
				mcp.Required(),
				mcp.Description("The memory value — keep terse, one sentence"),
			),
		),
		h.handleSave,
	)

	s.AddTool(
		mcp.NewTool("load_memories",
			mcp.WithDescription("Load memory entries for the project"),
			mcp.WithInteger("limit",
				mcp.Description("Maximum number of entries to return (default 50)"),
			),
			mcp.WithBoolean("session_only",
				mcp.Description("Only load entries for the active session"),
			),
		),
		h.handleLoad,
	)

	s.AddTool(
		mcp.NewTool("search_memories",
			mcp.WithDescription("Search memory entries by key (default), content (FTS5), or both, optionally filtered by date range"),
			mcp.WithString("query",
				mcp.Description("Search query to match against key or value (optional if after or before is set)"),
			),
			mcp.WithString("mode",
				mcp.Description("Search mode: key (default, uses index), content (value FTS5), both (key+content)"),
			),
			mcp.WithInteger("limit",
				mcp.Description("Maximum number of results to return (default 10)"),
			),
			mcp.WithString("after",
				mcp.Description("Only entries created after this date (RFC3339 format, e.g. 2026-05-01T00:00:00Z)"),
			),
			mcp.WithString("before",
				mcp.Description("Only entries created before this date (RFC3339 format)"),
			),
		),
		h.handleSearch,
	)

	s.AddTool(
		mcp.NewTool("list_sessions",
			mcp.WithDescription("List recent sessions for the project"),
			mcp.WithInteger("limit",
				mcp.Description("Number of sessions to list (default 10)"),
			),
		),
		h.handleListSessions,
	)

	s.AddTool(
		mcp.NewTool("delete_memory",
			mcp.WithDescription("Delete a memory entry by its ID"),
			mcp.WithInteger("id",
				mcp.Required(),
				mcp.Description("The memory entry ID to delete"),
			),
		),
		h.handleDelete,
	)

	s.AddTool(
		mcp.NewTool("list_keys",
			mcp.WithDescription("List distinct memory keys in use, ordered by critical (critical first) then most recently used"),
			mcp.WithInteger("limit",
				mcp.Description("Maximum number of keys to return (default 50)"),
			),
		),
		h.handleListKeys,
	)
}

func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{Text: msg},
		},
		IsError: true,
	}
}

func (h *Handler) handleInit(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp, err := h.svc.Init(ctx, &service.InitRequest{})
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("ilnamiqui initialized at %s", resp.DBPath)), nil
}

func (h *Handler) handleSave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	key := req.GetString("key", "")
	value := req.GetString("value", "")
	if key == "" {
		return errResult("missing required parameter 'key'"), nil
	}
	if value == "" {
		return errResult("missing required parameter 'value'"), nil
	}

	resp, err := h.svc.Save(ctx, &service.SaveRequest{Key: key, Value: value})
	if err != nil {
		return errResult(err.Error()), nil
	}

	result := fmt.Sprintf("Entry %d saved\nKey: %s\nValue: %s\nCreated: %s",
		resp.Entry.ID, resp.Entry.Key, resp.Entry.Value, resp.Entry.CreatedAt.Format(time.RFC3339))
	return mcp.NewToolResultText(result), nil
}

func (h *Handler) handleLoad(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := req.GetInt("limit", 50)
	sessionOnly := req.GetBool("session_only", false)

	resp, err := h.svc.Load(ctx, &service.LoadRequest{Limit: limit, SessionOnly: sessionOnly})
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.NewToolResultText(formatEntries(resp.Entries)), nil
}

func (h *Handler) handleSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := req.GetString("query", "")
	limit := req.GetInt("limit", 10)
	afterStr := req.GetString("after", "")
	beforeStr := req.GetString("before", "")
	mode := req.GetString("mode", "key")

	if query == "" && afterStr == "" && beforeStr == "" {
		return errResult("at least one of 'query', 'after', or 'before' is required"), nil
	}

	switch mode {
	case "key", "content", "both":
		// valid
	default:
		return errResult(fmt.Sprintf("invalid mode %q: must be key, content, or both", mode)), nil
	}

	var afterTime, beforeTime *time.Time
	if afterStr != "" {
		t, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			return errResult(fmt.Sprintf("invalid 'after': expected RFC3339 format, got %q", afterStr)), nil
		}
		afterTime = &t
	}
	if beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return errResult(fmt.Sprintf("invalid 'before': expected RFC3339 format, got %q", beforeStr)), nil
		}
		beforeTime = &t
	}

	resp, err := h.svc.Search(ctx, &service.SearchRequest{Query: query, Mode: memory.SearchMode(mode), Limit: limit, After: afterTime, Before: beforeTime})
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.NewToolResultText(formatEntries(resp.Entries)), nil
}

func (h *Handler) handleListSessions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := req.GetInt("limit", 10)

	resp, err := h.svc.ListSessions(ctx, &service.ListSessionsRequest{Limit: limit})
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.NewToolResultText(formatSessions(resp.Sessions)), nil
}

func (h *Handler) handleDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id := req.GetInt("id", 0)
	if id <= 0 {
		return errResult("missing or invalid required parameter 'id': must be a positive integer"), nil
	}

	_, err := h.svc.Delete(ctx, &service.DeleteRequest{ID: int64(id)})
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("deleted entry %d", id)), nil
}

func (h *Handler) handleListKeys(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := req.GetInt("limit", 50)

	resp, err := h.svc.ListKeys(ctx, &service.ListKeysRequest{Limit: limit})
	if err != nil {
		return errResult(err.Error()), nil
	}
	return mcp.NewToolResultText(formatKeys(resp.Keys)), nil
}

// formatEntries formats memory entries as a text table.
func formatEntries(entries []memory.MemoryEntry) string {
	if len(entries) == 0 {
		return "no entries found"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-4s %-20s %-30s %-36s %s\n", "ID", "Key", "Value", "Session ID", "Created")
	for _, e := range entries {
		val := e.Value
		if len(val) > 28 {
			val = val[:28] + "..."
		}
		fmt.Fprintf(&b, "%-4d %-20s %-30s %-36s %s\n",
			e.ID, e.Key, val, e.SessionID, e.CreatedAt.Format(time.RFC3339))
	}
	return b.String()
}

func formatKeys(keys []memory.KeyInfo) string {
	if len(keys) == 0 {
		return "no keys found"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-8s %s\n", "Key", "Critical", "Last Used")
	for _, k := range keys {
		critical := "false"
		if k.Critical {
			critical = "true"
		}
		fmt.Fprintf(&b, "%-20s %-8s %s\n", k.Key, critical, k.LastUsedAt.Format(time.RFC3339))
	}
	return b.String()
}

// formatSessions formats sessions as a text table.
func formatSessions(sessions []memory.Session) string {
	if len(sessions) == 0 {
		return "no sessions found"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-36s %-8s %-24s %-24s %s\n", "ID", "Project", "Started", "Ended", "Summary")
	for _, s := range sessions {
		ended := ""
		if s.EndedAt != nil {
			ended = s.EndedAt.Format(time.RFC3339)
		}
		summary := s.Summary
		if len(summary) > 30 {
			summary = summary[:30] + "..."
		}
		fmt.Fprintf(&b, "%-36s %-8s %-24s %-24s %s\n",
			s.ID, s.Project, s.StartedAt.Format(time.RFC3339), ended, summary)
	}
	return b.String()
}
