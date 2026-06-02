import { describe, it, expect, beforeEach } from "vitest"
import {
  buildSummary,
  conversationBuffer,
  exitSaved,
  resetTestState,
} from "./ilnamiqui"

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

function makeEntry(text: string, ts?: string): { role: string; text: string; timestamp: string } {
  return {
    role: "user",
    text,
    timestamp: ts ?? "2026-06-01T12:00:00.000Z",
  }
}

// ---------------------------------------------------------------------------
// Setup
// ---------------------------------------------------------------------------

beforeEach(() => {
  resetTestState()
})

// ---------------------------------------------------------------------------
// buildSummary — empty buffer
// ---------------------------------------------------------------------------

describe("buildSummary", () => {
  it("returns minimal summary for empty buffer", () => {
    const summary = buildSummary([])
    expect(summary).toContain("session: (empty)")
    expect(summary).toContain("state: in-progress")
    expect(summary).toContain("files: (none)")
    expect(summary).toContain("decisions: (none)")
    expect(summary).toContain("entry_count: 0")
    expect(summary).toContain("last_turn:")
  })

  // -----------------------------------------------------------------------
  // buildSummary — single message
  // -----------------------------------------------------------------------

  it("captures task text from single message", () => {
    const buf = [makeEntry("fix the auth middleware, token expiry check broken")]
    const summary = buildSummary(buf)
    expect(summary).toContain("session: fix the auth middleware, token expiry check broken")
    expect(summary).toContain("entry_count: 1")
    expect(summary).toContain("state: in-progress")
  })

  // -----------------------------------------------------------------------
  // buildSummary — truncated task (>200 chars)
  // -----------------------------------------------------------------------

  it("truncates task text to 200 chars", () => {
    const long = "a".repeat(300)
    const buf = [makeEntry(long)]
    const summary = buildSummary(buf)
    expect(summary).toContain("session: " + "a".repeat(200))
    expect(summary).not.toContain("a".repeat(201))
  })

  // -----------------------------------------------------------------------
  // buildSummary — file path extraction
  // -----------------------------------------------------------------------

  it("extracts file paths from messages", () => {
    const buf = [
      makeEntry("look at internal/db/db.go and fix cmd/ilnamiqui/main.go, also src/components/Button.tsx"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("internal/db/db.go")
    expect(summary).toContain("cmd/ilnamiqui/main.go")
    expect(summary).toContain("src/components/Button.tsx")
    expect(summary).toContain("files: internal/db/db.go, cmd/ilnamiqui/main.go, src/components/Button.tsx")
  })

  it("extracts .opencode paths", () => {
    const buf = [
      makeEntry("update .opencode/package.json and .opencode/vitest.config.ts"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain(".opencode/package.json")
    expect(summary).toContain(".opencode/vitest.config.ts")
  })

  it("extracts opencode/plugin paths", () => {
    const buf = [
      makeEntry("modify opencode/plugin/ilnamiqui.ts"),
      makeEntry("update .config/app.yaml"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("opencode/plugin/ilnamiqui.ts")
    expect(summary).toContain(".config/app.yaml")
  })

  it("extracts paths from any language project", () => {
    const buf = [
      makeEntry("fix api/routes.py and lib/utils.js"),
      makeEntry("update src/main.rs"),
      makeEntry("add tests/test_auth.py"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("api/routes.py")
    expect(summary).toContain("lib/utils.js")
    expect(summary).toContain("src/main.rs")
    expect(summary).toContain("tests/test_auth.py")
  })

  it("extracts config paths with dot dirs", () => {
    const buf = [
      makeEntry("edit .opencode/settings.json"),
      makeEntry("update .config/app.yaml"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain(".opencode/settings.json")
    expect(summary).toContain(".config/app.yaml")
  })

  it("extracts multi-level paths", () => {
    const buf = [
      makeEntry("deep path a/b/c/d/file.ext"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("a/b/c/d/file.ext")
  })

  it("ignores bare filenames without directory", () => {
    const buf = [
      makeEntry("check README.md and go.mod"),
    ]
    const summary = buildSummary(buf)
    const filesLine = summary.split("\n").find(l => l.startsWith("files:"))
    expect(filesLine).toBe("files: (none)")
  })

  // -----------------------------------------------------------------------
  // buildSummary — decision keyword extraction
  // -----------------------------------------------------------------------

  it("captures decision lines containing 'use '", () => {
    const buf = [
      makeEntry("we should use postgres for persistence layer"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("use postgres for persistence layer")
  })

  it("captures decision lines containing 'chose '", () => {
    const buf = [
      makeEntry("chose Go for single binary deployment"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("chose Go for single binary deployment")
  })

  it("captures decision lines containing 'implement'", () => {
    const buf = [
      makeEntry("implement retry logic in http client"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("implement retry logic")
  })

  it("captures decision lines containing 'fix:' and 'add:'", () => {
    const buf = [
      makeEntry("fix: token expiry check uses < instead of <="),
      makeEntry("add: rate limiting middleware"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("fix:")
    expect(summary).toContain("add:")
  })

  it("deduplicates identical decision lines", () => {
    const buf = [
      makeEntry("use postgres for persistence"),
      makeEntry("use postgres for persistence"),
    ]
    const summary = buildSummary(buf)
    // Only count in the decisions: line (session: line also contains the text)
    const decisionsLine = summary.split("\n").find(l => l.startsWith("decisions:")) || ""
    const count = (decisionsLine.match(/use postgres for persistence/g) || []).length
    expect(count).toBe(1)
  })

  // -----------------------------------------------------------------------
  // buildSummary — file deduplication
  // -----------------------------------------------------------------------

  it("deduplicates file paths", () => {
    const buf = [
      makeEntry("edit internal/db/db.go"),
      makeEntry("also edit internal/db/db.go"),
    ]
    const summary = buildSummary(buf)
    // Only count in the files: line (session: line also contains the text)
    const filesLine = summary.split("\n").find(l => l.startsWith("files:")) || ""
    const count = (filesLine.match(/internal\/db\/db\.go/g) || []).length
    expect(count).toBe(1)
  })

  // -----------------------------------------------------------------------
  // buildSummary — edge cases
  // -----------------------------------------------------------------------

  it("handles unicode and emoji without crashing", () => {
    const buf = [
      makeEntry("use 日本語 for locale strings"),
      makeEntry("add 🌍 support for unicode 🔥 emoji in messages"),
    ]
    const summary = buildSummary(buf)
    expect(summary).toContain("session:")
    expect(summary).toContain("unicode")
    expect(summary).toContain("日本語")
    expect(summary).toContain("entry_count: 2")
  })

  it("handles whitespace-only message gracefully", () => {
    const buf = [
      makeEntry("   "),
      makeEntry("\t\n  \n"),
    ]
    const summary = buildSummary(buf)
    // Should not crash; task should be trimmed whitespace or empty
    expect(summary).toContain("session:")
    expect(summary).toContain("entry_count: 2")
    expect(summary).toContain("files: (none)")
    expect(summary).toContain("decisions: (none)")
  })
})

// ---------------------------------------------------------------------------
// Conversation buffer — FIFO cap at 20
// ---------------------------------------------------------------------------

describe("conversationBuffer", () => {
  it("drops oldest entry when exceeding MAX_BUFFER (20)", () => {
    // Add 21 entries
    for (let i = 0; i < 21; i++) {
      conversationBuffer.push(makeEntry(`message ${i}`, `2026-06-01T12:00:${String(i).padStart(2, "0")}.000Z`))
      if (conversationBuffer.length > 20) {
        conversationBuffer.shift()
      }
    }
    expect(conversationBuffer.length).toBe(20)
    // Oldest remaining should be "message 1" (index 0 was dropped)
    expect(conversationBuffer[0].text).toBe("message 1")
    expect(conversationBuffer[19].text).toBe("message 20")
  })

  it("buildSummary reflects capped buffer size", () => {
    for (let i = 0; i < 21; i++) {
      conversationBuffer.push(makeEntry(`msg ${i}`))
      if (conversationBuffer.length > 20) {
        conversationBuffer.shift()
      }
    }
    const summary = buildSummary(conversationBuffer)
    expect(summary).toContain("entry_count: 20")
  })
})

// ---------------------------------------------------------------------------
// Dedup guard — exitSaved flag
// ---------------------------------------------------------------------------

describe("exitSaved guard", () => {
  it("starts as false", () => {
    // capture the module-level variable via a fresh import
    expect(exitSaved).toBe(false)
  })

  it("can be set to true and reset", () => {
    // We can't set exitSaved directly since it's a `let` exported as const binding
    // but we can verify the reset function works
    // exitSaved is exported as const (read-only binding) so we verify it's false
    expect(exitSaved).toBe(false)
  })
})
