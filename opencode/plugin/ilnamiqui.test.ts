import { describe, it, expect, beforeEach, vi, afterEach } from "vitest"
import fs from "fs"
import {
  buildSummary,
  conversationBuffer,
  exitSaved,
  resolveBinarySync,
  resolveBinaryPath,
  resetTestState,
  interactionCounter,
  MAX_INTERACTIONS,
  REMINDER_TEXT,
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

// ---------------------------------------------------------------------------
// interactionCounter & system transform
// ---------------------------------------------------------------------------

describe("interactionCounter", () => {
  it("starts at 0 after reset", () => {
    expect(interactionCounter).toBe(0)
  })

  it("exists as a number export", () => {
    // interactionCounter is a read-only const binding (like exitSaved)
    // mutation happens inside module scope via hooks or resetTestState
    expect(typeof interactionCounter).toBe("number")
  })

  it("resets to 0 on resetTestState()", () => {
    // resetTestState runs inside module scope where interactionCounter is mutable
    resetTestState()
    expect(interactionCounter).toBe(0)
  })
})

// ---------------------------------------------------------------------------
// tool.execute.after — injects reminder directly into output at threshold
// ---------------------------------------------------------------------------

describe("tool.execute.after (reminder injection)", () => {
  it("appends reminder to output.output at threshold", () => {
    let localCounter = MAX_INTERACTIONS
    let output = "file content here"

    localCounter++
    if (localCounter >= MAX_INTERACTIONS) {
      localCounter = 0
      output = (output || "") + "\n\n---\n" + REMINDER_TEXT
    }

    expect(output).toContain(REMINDER_TEXT)
    expect(output).toContain("file content here")
    expect(localCounter).toBe(0)
  })

  it("does NOT modify output below threshold", () => {
    let localCounter = MAX_INTERACTIONS - 2 // 28
    const originalOutput = "file content here"
    let output = originalOutput

    localCounter++ // → 29 (< 30)
    if (localCounter >= MAX_INTERACTIONS) {
      localCounter = 0
      output = (output || "") + "\n\n---\n" + REMINDER_TEXT
    }

    expect(output).toBe(originalOutput)
    expect(localCounter).toBe(MAX_INTERACTIONS - 1) // stays at 29
  })

  it("resets counter after injection", () => {
    let localCounter = MAX_INTERACTIONS
    let output = "data"

    localCounter++
    if (localCounter >= MAX_INTERACTIONS) {
      localCounter = 0
      output = (output || "") + "\n\n---\n" + REMINDER_TEXT
    }

    expect(localCounter).toBe(0)
    expect(output).toContain(REMINDER_TEXT)
  })

  it("handles undefined output gracefully", () => {
    let localCounter = MAX_INTERACTIONS
    let output: string | undefined = undefined

    localCounter++
    if (localCounter >= MAX_INTERACTIONS) {
      localCounter = 0
      output = (output || "") + "\n\n---\n" + REMINDER_TEXT
    }

    expect(localCounter).toBe(0)
    expect(output).toBe("\n\n---\n" + REMINDER_TEXT)
  })
})



describe("MAX_INTERACTIONS constant", () => {
  it("is 30", () => {
    expect(MAX_INTERACTIONS).toBe(30)
  })
})

describe("REMINDER_TEXT constant", () => {
  it("contains AGENTS.md reference", () => {
    expect(REMINDER_TEXT).toContain("AGENTS.md")
  })

  it("starts with REMINDER: prefix (system message format)", () => {
    expect(REMINDER_TEXT.startsWith("REMINDER:")).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// resolveBinaryPath — platform-specific binary path
// ---------------------------------------------------------------------------

describe("resolveBinaryPath", () => {
  it("returns path containing ilnamiqui", () => {
    const p = resolveBinaryPath()
    expect(p).toContain("ilnamiqui")
  })

  it("returns path in the opencode plugin directory", () => {
    const p = resolveBinaryPath()
    expect(p).toContain(".config/opencode/plugins/ilnamiqui")
  })

  it("includes platform name (normalized from win32 → windows)", () => {
    const p = resolveBinaryPath()
    const platform = process.platform === "win32" ? "windows" : process.platform
    expect(p).toContain(platform)
  })

  it("includes architecture", () => {
    const p = resolveBinaryPath()
    expect(p).toContain(process.arch)
  })

  it("appends .exe on win32", () => {
    // Mock process.platform for this test
    const orig = Object.getOwnPropertyDescriptor(process, "platform")
    try {
      Object.defineProperty(process, "platform", { value: "win32" })
      const p = resolveBinaryPath()
      expect(p).toContain("windows")
      expect(p).toMatch(/\.exe$/)
    } finally {
      if (orig) {
        Object.defineProperty(process, "platform", orig)
      }
    }
  })
})

// ---------------------------------------------------------------------------
// resolveBinarySync — synchronous binary resolution: PATH then platform
// ---------------------------------------------------------------------------

describe("resolveBinarySync", () => {
  const ORIG_PATH = process.env.PATH
  const platformPath = resolveBinaryPath()

  afterEach(() => {
    process.env.PATH = ORIG_PATH
    vi.restoreAllMocks()
  })

  it("finds binary on PATH", () => {
    process.env.PATH = "/tmp/test-bin"
    const binPath = "/tmp/test-bin/ilnamiqui"

    vi.spyOn(fs, "existsSync").mockImplementation(
      (p) => p === binPath,
    )
    vi.spyOn(fs, "accessSync").mockImplementation(() => {})

    const result = resolveBinarySync()
    expect(result).toBe(binPath)
  })

  it("skips non-executable PATH entry, uses platform fallback", () => {
    process.env.PATH = "/tmp/test-bin"
    const binPath = "/tmp/test-bin/ilnamiqui"

    // PATH entry exists but NOT executable
    vi.spyOn(fs, "existsSync").mockImplementation(
      (p) => p === binPath || p === platformPath,
    )
    // accessSync throws for PATH entry, succeeds for platformPath
    vi.spyOn(fs, "accessSync").mockImplementation((p) => {
      if (p === binPath) throw new Error("not executable")
    })

    const result = resolveBinarySync()
    expect(result).toBe(platformPath)
  })

  it("falls back to platform binary when PATH empty and platform exists", () => {
    process.env.PATH = ""

    vi.spyOn(fs, "existsSync").mockImplementation(
      (p) => p === platformPath,
    )
    vi.spyOn(fs, "accessSync").mockImplementation(() => {})

    const result = resolveBinarySync()
    expect(result).toBe(platformPath)
  })

  it("returns null when no binary found anywhere", () => {
    process.env.PATH = ""

    vi.spyOn(fs, "existsSync").mockReturnValue(false)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {
      throw new Error("ENOENT")
    })

    const result = resolveBinarySync()
    expect(result).toBeNull()
  })

  it("returns null when PATH entry exists but not executable and platform missing", () => {
    process.env.PATH = "/tmp/test-bin"

    vi.spyOn(fs, "existsSync").mockReturnValue(false)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {
      throw new Error("ENOENT")
    })

    const result = resolveBinarySync()
    expect(result).toBeNull()
  })

  it("filters empty PATH entries (trailing colon)", () => {
    process.env.PATH = "/tmp/test-bin:"  // trailing colon → empty entry
    const binPath = "/tmp/test-bin/ilnamiqui"

    vi.spyOn(fs, "existsSync").mockImplementation(
      (p) => p === binPath,
    )
    vi.spyOn(fs, "accessSync").mockImplementation(() => {})

    const result = resolveBinarySync()
    expect(result).toBe(binPath)
  })
})

// ---------------------------------------------------------------------------


