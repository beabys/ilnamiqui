//go:build integration

package mcp

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/beabys/ilnamiqui/internal/service"
)

// TestIntegrationFullLifecycle tests init → save → load → search → list → delete
// via the MCP tool handlers with real service.
func TestIntegrationFullLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	ctx := context.Background()
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	h := NewHandler(svc)

	// 1. Init
	t.Log("Step 1: init_memory")
	result, err := h.handleInit(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("init error: %v", err)
	}
	if result.IsError {
		t.Fatalf("init failed: %s", extractText(result))
	}
	if !strings.Contains(extractText(result), "ilnamiqui initialized") {
		t.Fatalf("unexpected init output: %s", extractText(result))
	}

	// Verify DB file
	if _, err := os.Stat(dir + "/.ilnamiqui/ilnamiqui.db"); os.IsNotExist(err) {
		t.Fatal("DB file not created after init")
	}

	// 2. Save entries
	t.Log("Step 2: save_memory")
	saveReq := mcp.CallToolRequest{}
	saveReq.Params.Arguments = map[string]any{
		"key":   "architecture",
		"value": "hexagonal architecture chosen for multi-assistant support",
	}
	result, err = h.handleSave(ctx, saveReq)
	if err != nil {
		t.Fatalf("save error: %v", err)
	}
	if result.IsError {
		t.Fatalf("save failed: %s", extractText(result))
	}
	if !strings.Contains(extractText(result), "Entry 2 saved") { // project-path=1, architecture=2
		t.Fatalf("unexpected save output: %s", extractText(result))
	}

	// Save second entry
	saveReq2 := mcp.CallToolRequest{}
	saveReq2.Params.Arguments = map[string]any{
		"key":   "bug",
		"value": "fixed nil pointer in handler",
	}
	result, err = h.handleSave(ctx, saveReq2)
	if err != nil {
		t.Fatalf("save error: %v", err)
	}
	if result.IsError {
		t.Fatalf("save failed: %s", extractText(result))
	}
	if !strings.Contains(extractText(result), "Entry 3 saved") {
		t.Fatalf("unexpected save output: %s", extractText(result))
	}

	// 3. Load memories
	t.Log("Step 3: load_memories")
	loadReq := mcp.CallToolRequest{}
	loadReq.Params.Arguments = map[string]any{
		"limit": 50,
	}
	result, err = h.handleLoad(ctx, loadReq)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if result.IsError {
		t.Fatalf("load failed: %s", extractText(result))
	}

	loadText := extractText(result)
	if !strings.Contains(loadText, "architecture") {
		t.Fatalf("load result missing 'architecture': %s", loadText)
	}
	if !strings.Contains(loadText, "hexagonal") {
		t.Fatalf("load result missing 'hexagonal': %s", loadText)
	}
	if !strings.Contains(loadText, "nil pointer") {
		t.Fatalf("load result missing 'nil pointer': %s", loadText)
	}

	// 4. Search memories
	t.Log("Step 4: search_memories")
	searchReq := mcp.CallToolRequest{}
	searchReq.Params.Arguments = map[string]any{
		"query": "hexagonal",
		"limit": 10,
		"mode":  "both",
	}
	result, err = h.handleSearch(ctx, searchReq)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if result.IsError {
		t.Fatalf("search failed: %s", extractText(result))
	}

	searchText := extractText(result)
	if !strings.Contains(searchText, "architecture") {
		t.Fatalf("search result missing 'architecture': %s", searchText)
	}
	if !strings.Contains(searchText, "hexagonal") {
		t.Fatalf("search result missing 'hexagonal': %s", searchText)
	}

	// 5. List sessions
	t.Log("Step 5: list_sessions")
	listReq := mcp.CallToolRequest{}
	listReq.Params.Arguments = map[string]any{
		"limit": 10,
	}
	result, err = h.handleListSessions(ctx, listReq)
	if err != nil {
		t.Fatalf("list sessions error: %v", err)
	}
	if result.IsError {
		t.Fatalf("list sessions failed: %s", extractText(result))
	}

	listText := extractText(result)
	if !strings.Contains(listText, "Project") {
		t.Fatalf("list result missing 'Project' header: %s", listText)
	}

	// 6. Delete memory
	t.Log("Step 6: delete_memory")
	deleteReq := mcp.CallToolRequest{}
	deleteReq.Params.Arguments = map[string]any{
		"id": 2, // architecture entry (project-path=1, architecture=2, bug=3)
	}
	result, err = h.handleDelete(ctx, deleteReq)
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if result.IsError {
		t.Fatalf("delete failed: %s", extractText(result))
	}
	if !strings.Contains(extractText(result), "deleted entry 2") {
		t.Fatalf("unexpected delete output: %s", extractText(result))
	}

	// 7. Verify deletion by loading (should only have entry 2)
	t.Log("Step 7: verify deletion")
	loadReq2 := mcp.CallToolRequest{}
	loadReq2.Params.Arguments = map[string]any{
		"limit": 50,
	}
	result, err = h.handleLoad(ctx, loadReq2)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if result.IsError {
		t.Fatalf("load failed: %s", extractText(result))
	}

	loadText2 := extractText(result)
	if strings.Contains(loadText2, "hexagonal") {
		t.Fatalf("deleted entry still appears in load results: %s", loadText2)
	}
	if !strings.Contains(loadText2, "nil pointer") {
		t.Fatalf("remaining entry not found in load results: %s", loadText2)
	}
}

// TestIntegrationInitTwice verifies init is idempotent.
func TestIntegrationInitTwice(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	ctx := context.Background()
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	h := NewHandler(svc)

	// First init
	result, err := h.handleInit(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("first init error: %v", err)
	}
	if result.IsError {
		t.Fatalf("first init failed: %s", extractText(result))
	}

	// Second init — should succeed (idempotent)
	result, err = h.handleInit(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("second init error: %v", err)
	}
	if result.IsError {
		t.Fatalf("second init failed: %s", extractText(result))
	}
	if !strings.Contains(extractText(result), "ilnamiqui initialized") {
		t.Fatalf("second init output unexpected: %s", extractText(result))
	}
}

// TestIntegrationSaveLoadWithLimit verifies limit parameter works correctly.
func TestIntegrationSaveLoadWithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	ctx := context.Background()
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	h := NewHandler(svc)

	// Init
	_, err := h.handleInit(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("init error: %v", err)
	}

	// Save 3 entries
	for i := 0; i < 3; i++ {
		saveReq := mcp.CallToolRequest{}
		saveReq.Params.Arguments = map[string]any{
			"key":   "key",
			"value": "value",
		}
		result, err := h.handleSave(ctx, saveReq)
		if err != nil {
			t.Fatalf("save error: %v", err)
		}
		if result.IsError {
			t.Fatalf("save failed: %s", extractText(result))
		}
	}

	// Load with limit 2
	loadReq := mcp.CallToolRequest{}
	loadReq.Params.Arguments = map[string]any{
		"limit": 2,
	}
	result, err := h.handleLoad(ctx, loadReq)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if result.IsError {
		t.Fatalf("load failed: %s", extractText(result))
	}

	// Count data rows (skip header)
	lines := strings.Split(strings.TrimSpace(extractText(result)), "\n")
	if len(lines) != 3 { // header + 2 data rows
		t.Fatalf("expected 3 lines (header + 2 data rows) with limit=2, got %d", len(lines))
	}
}

// TestIntegrationSearchNoResults verifies search with no matches.
func TestIntegrationSearchNoResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := t.TempDir()
	origWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	_ = os.Chdir(dir)

	ctx := context.Background()
	svc := service.New(service.DefaultConfig(), service.DefaultDBOpener())
	t.Cleanup(func() { svc.Close() })
	h := NewHandler(svc)

	// Init
	_, err := h.handleInit(ctx, mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("init error: %v", err)
	}

	// Search for something that doesn't exist
	searchReq := mcp.CallToolRequest{}
	searchReq.Params.Arguments = map[string]any{
		"query": "nonexistent",
		"limit": 10,
	}
	result, err := h.handleSearch(ctx, searchReq)
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if result.IsError {
		t.Fatalf("search failed: %s", extractText(result))
	}
	if extractText(result) != "no entries found" {
		t.Fatalf("expected 'no entries found', got: %s", extractText(result))
	}
}
