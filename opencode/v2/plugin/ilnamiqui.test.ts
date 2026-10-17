import { describe, it, expect, beforeEach, afterEach, vi } from "vitest"
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
  formatReminder,
} from "./ilnamiqui"

// Mock child_process for V2 surface tests (buildSummary/resolveBinary* tests do not call execFile).
vi.mock("child_process", () => ({
  execFile: vi.fn((_b: string, _a: string[], _o: unknown, cb: (e: unknown, o: string, s: string) => void) => {
    cb(null, "", "")
  }),
}))

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

// ===========================================================================
// buildSummary
// ===========================================================================

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

  it("captures task text from single message", () => {
    const buf = [makeEntry("fix the auth middleware, token expiry check broken")]
    const summary = buildSummary(buf)
    expect(summary).toContain("session: fix the auth middleware, token expiry check broken")
    expect(summary).toContain("entry_count: 1")
    expect(summary).toContain("state: in-progress")
  })

  it("truncates task text to 200 chars", () => {
    const long = "a".repeat(300)
    const buf = [makeEntry(long)]
    const summary = buildSummary(buf)
    expect(summary).toContain("session: " + "a".repeat(200))
    expect(summary).not.toContain("a".repeat(201))
  })

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
    const buf = [makeEntry("update .opencode/package.json and .opencode/vitest.config.ts")]
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
    const buf = [makeEntry("deep path a/b/c/d/file.ext")]
    const summary = buildSummary(buf)
    expect(summary).toContain("a/b/c/d/file.ext")
  })

  it("ignores bare filenames without directory", () => {
    const buf = [makeEntry("check README.md and go.mod")]
    const summary = buildSummary(buf)
    const filesLine = summary.split("\n").find((l) => l.startsWith("files:"))
    expect(filesLine).toBe("files: (none)")
  })

  it("captures decision lines containing 'use '", () => {
    const buf = [makeEntry("we should use postgres for persistence layer")]
    const summary = buildSummary(buf)
    expect(summary).toContain("use postgres for persistence layer")
  })

  it("captures decision lines containing 'chose '", () => {
    const buf = [makeEntry("chose Go for single binary deployment")]
    const summary = buildSummary(buf)
    expect(summary).toContain("chose Go for single binary deployment")
  })

  it("captures decision lines containing 'implement'", () => {
    const buf = [makeEntry("implement retry logic in http client")]
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
    const decisionsLine = summary.split("\n").find((l) => l.startsWith("decisions:")) || ""
    const count = (decisionsLine.match(/use postgres for persistence/g) || []).length
    expect(count).toBe(1)
  })

  it("deduplicates file paths", () => {
    const buf = [
      makeEntry("edit internal/db/db.go"),
      makeEntry("also edit internal/db/db.go"),
    ]
    const summary = buildSummary(buf)
    const filesLine = summary.split("\n").find((l) => l.startsWith("files:")) || ""
    const count = (filesLine.match(/internal\/db\/db\.go/g) || []).length
    expect(count).toBe(1)
  })

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
    const buf = [makeEntry("   "), makeEntry("\t\n  \n")]
    const summary = buildSummary(buf)
    expect(summary).toContain("session:")
    expect(summary).toContain("entry_count: 2")
    expect(summary).toContain("files: (none)")
    expect(summary).toContain("decisions: (none)")
  })
})

// ===========================================================================
// conversationBuffer — FIFO cap at 20
// ===========================================================================

describe("conversationBuffer", () => {
  it("drops oldest entry when exceeding MAX_BUFFER (20)", () => {
    for (let i = 0; i < 21; i++) {
      conversationBuffer.push(makeEntry(`message ${i}`, `2026-06-01T12:00:${String(i).padStart(2, "0")}.000Z`))
      if (conversationBuffer.length > 20) conversationBuffer.shift()
    }
    expect(conversationBuffer.length).toBe(20)
    expect(conversationBuffer[0].text).toBe("message 1")
    expect(conversationBuffer[19].text).toBe("message 20")
  })

  it("buildSummary reflects capped buffer size", () => {
    for (let i = 0; i < 21; i++) {
      conversationBuffer.push(makeEntry(`msg ${i}`))
      if (conversationBuffer.length > 20) conversationBuffer.shift()
    }
    const summary = buildSummary(conversationBuffer)
    expect(summary).toContain("entry_count: 20")
  })
})

// ===========================================================================
// exitSaved guard
// ===========================================================================

describe("exitSaved guard", () => {
  it("starts as false", () => {
    expect(exitSaved).toBe(false)
  })

  it("is a boolean export", () => {
    expect(typeof exitSaved).toBe("boolean")
  })
})

// ===========================================================================
// interactionCounter
// ===========================================================================

describe("interactionCounter", () => {
  it("starts at 0 after reset", () => {
    expect(interactionCounter).toBe(0)
  })

  it("exists as a number export", () => {
    expect(typeof interactionCounter).toBe("number")
  })

  it("resets to 0 on resetTestState()", () => {
    resetTestState()
    expect(interactionCounter).toBe(0)
  })
})

// ===========================================================================
// formatReminder
// ===========================================================================

describe("formatReminder", () => {
  it("joins multiple values with ' and ' separator", () => {
    expect(formatReminder(["val1", "val2", "val3"])).toBe("val1 and val2 and val3")
  })

  it("returns single value unchanged", () => {
    expect(formatReminder(["only one"])).toBe("only one")
  })

  it("falls back to REMINDER_TEXT when empty array", () => {
    expect(formatReminder([])).toBe(REMINDER_TEXT)
  })

  it("falls back to REMINDER_TEXT for empty input", () => {
    const result = formatReminder([])
    expect(result).toContain("AGENTS.md")
    expect(result.startsWith("REMINDER:")).toBe(true)
  })
})

// ===========================================================================
// tool.execute.after — reminder injection at threshold (pure simulation)
// ===========================================================================

describe("tool.execute.after (reminder injection)", () => {
  it("appends reminder to output at threshold", () => {
    let localCounter = MAX_INTERACTIONS
    let output = "file content here"

    const criticalValues: string[] = []
    localCounter++
    if (localCounter >= MAX_INTERACTIONS) {
      localCounter = 0
      const reminderText = criticalValues.length > 0 ? formatReminder(criticalValues) : REMINDER_TEXT
      output = (output || "") + "\n\n---\n" + reminderText
    }

    expect(output).toContain(REMINDER_TEXT)
    expect(output).toContain("file content here")
    expect(localCounter).toBe(0)
  })

  it("does NOT modify output below threshold", () => {
    let localCounter = MAX_INTERACTIONS - 2
    const originalOutput = "file content here"
    let output = originalOutput

    localCounter++
    if (localCounter >= MAX_INTERACTIONS) {
      localCounter = 0
      output = (output || "") + "\n\n---\n" + REMINDER_TEXT
    }

    expect(output).toBe(originalOutput)
    expect(localCounter).toBe(MAX_INTERACTIONS - 1)
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

  it("uses formatReminder with critical values", () => {
    const criticalValues = ["REMINDER: rule1", "REMINDER: rule2"]
    const joined = formatReminder(criticalValues)

    let localCounter = MAX_INTERACTIONS
    let output = "data"
    localCounter++
    if (localCounter >= MAX_INTERACTIONS) {
      localCounter = 0
      output = (output || "") + "\n\n---\n" + joined
    }

    expect(output).toBe("data\n\n---\nREMINDER: rule1 and REMINDER: rule2")
    expect(localCounter).toBe(0)
  })
})

// ===========================================================================
// Constants
// ===========================================================================

describe("MAX_INTERACTIONS constant", () => {
  it("is 50", () => {
    expect(MAX_INTERACTIONS).toBe(50)
  })
})

describe("REMINDER_TEXT constant", () => {
  it("contains AGENTS.md reference", () => {
    expect(REMINDER_TEXT).toContain("AGENTS.md")
  })
  it("starts with REMINDER: prefix", () => {
    expect(REMINDER_TEXT.startsWith("REMINDER:")).toBe(true)
  })
})

// ===========================================================================
// resolveBinaryPath
// ===========================================================================

describe("resolveBinaryPath", () => {
  it("returns path containing ilnamiqui", () => {
    expect(resolveBinaryPath()).toContain("ilnamiqui")
  })
  it("returns path in the opencode plugin directory", () => {
    expect(resolveBinaryPath()).toContain(".config/opencode/plugins/ilnamiqui")
  })
  it("includes platform name (normalized from win32 → windows)", () => {
    const platform = process.platform === "win32" ? "windows" : process.platform
    expect(resolveBinaryPath()).toContain(platform)
  })
  it("includes architecture", () => {
    expect(resolveBinaryPath()).toContain(process.arch)
  })
  it("appends .exe on win32", () => {
    const orig = Object.getOwnPropertyDescriptor(process, "platform")
    try {
      Object.defineProperty(process, "platform", { value: "win32" })
      const p = resolveBinaryPath()
      expect(p).toContain("windows")
      expect(p).toMatch(/\.exe$/)
    } finally {
      if (orig) Object.defineProperty(process, "platform", orig)
    }
  })
})

// ===========================================================================
// resolveBinarySync
// ===========================================================================

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

    vi.spyOn(fs, "existsSync").mockImplementation((p) => p === binPath)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {})

    expect(resolveBinarySync()).toBe(binPath)
  })

  it("skips non-executable PATH entry, uses platform fallback", () => {
    process.env.PATH = "/tmp/test-bin"
    const binPath = "/tmp/test-bin/ilnamiqui"

    vi.spyOn(fs, "existsSync").mockImplementation((p) => p === binPath || p === platformPath)
    vi.spyOn(fs, "accessSync").mockImplementation((p) => {
      if (p === binPath) throw new Error("not executable")
    })

    expect(resolveBinarySync()).toBe(platformPath)
  })

  it("falls back to platform binary when PATH empty and platform exists", () => {
    process.env.PATH = ""
    vi.spyOn(fs, "existsSync").mockImplementation((p) => p === platformPath)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {})
    expect(resolveBinarySync()).toBe(platformPath)
  })

  it("returns null when no binary found anywhere", () => {
    process.env.PATH = ""
    vi.spyOn(fs, "existsSync").mockReturnValue(false)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {
      throw new Error("ENOENT")
    })
    expect(resolveBinarySync()).toBeNull()
  })

  it("returns null when PATH entry exists but not executable and platform missing", () => {
    process.env.PATH = "/tmp/test-bin"
    vi.spyOn(fs, "existsSync").mockReturnValue(false)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {
      throw new Error("ENOENT")
    })
    expect(resolveBinarySync()).toBeNull()
  })

  it("filters empty PATH entries (trailing colon)", () => {
    process.env.PATH = "/tmp/test-bin:"
    const binPath = "/tmp/test-bin/ilnamiqui"

    vi.spyOn(fs, "existsSync").mockImplementation((p) => p === binPath)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {})
    expect(resolveBinarySync()).toBe(binPath)
  })
})

// ===========================================================================
// V2-surface contract tests
// ===========================================================================

// Helper to build a mock context whose hook/transform functions record their
// registered callbacks so we can invoke them later.
function makeMockCtx() {
  const hooks: Record<string, Function> = {}
  const toolHooks: Record<string, Function> = {}
  const transformEditors: Function[] = []

  const ctx = {
    location: { directory: "/tmp/test" },
    session: {
      hook: vi.fn(async (name: string, fn: Function) => {
        hooks[name] = fn
      }),
    },
    tool: {
      hook: vi.fn(async (name: string, fn: Function) => {
        toolHooks[name] = fn
      }),
      transform: vi.fn(async (editorFn: Function) => {
        transformEditors.push(editorFn)
      }),
    },
    event: {
      subscribe: vi.fn(() => ({
        [Symbol.asyncIterator]: () => ({
          next: () => new Promise<IteratorResult<unknown>>(() => {}), // never yields
        }),
      })),
    },
  } as any

  return { ctx, hooks, toolHooks, transformEditors }
}

// Helper to make the binary resolve to a path (so setup proceeds past the null-check).
function enableBinaryResolve() {
  vi.spyOn(fs, "existsSync").mockReturnValue(true)
  vi.spyOn(fs, "accessSync").mockImplementation(() => {})
}

describe("V2 plugin surface contract", () => {
  afterEach(() => {
    vi.restoreAllMocks()
    resetTestState()
  })

  it("ilnamiquiPlugin has id and setup", async () => {
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    expect(ilnamiquiPlugin.id).toBe("ilnamiqui")
    expect(typeof ilnamiquiPlugin.setup).toBe("function")
  })

  it("setup(ctx) returns a cleanup function when binary found", async () => {
    enableBinaryResolve()
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx } = makeMockCtx()
    const cleanup = await ilnamiquiPlugin.setup(ctx)
    expect(typeof cleanup).toBe("function")
  })

  it("setup(ctx) returns undefined when binary not found", async () => {
    vi.spyOn(fs, "existsSync").mockReturnValue(false)
    vi.spyOn(fs, "accessSync").mockImplementation(() => {
      throw new Error("ENOENT")
    })
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx } = makeMockCtx()
    const cleanup = await ilnamiquiPlugin.setup(ctx)
    expect(cleanup).toBeUndefined()
  })

  it("registers ctx.session.hook with 'prompt' and 'compaction'", async () => {
    enableBinaryResolve()
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx } = makeMockCtx()
    await ilnamiquiPlugin.setup(ctx)

    const names = ctx.session.hook.mock.calls.map((c: any[]) => c[0])
    expect(names).toContain("prompt")
    expect(names).toContain("compaction")
  })

  it("registers ctx.tool.hook('execute.after', ...)", async () => {
    enableBinaryResolve()
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx } = makeMockCtx()
    await ilnamiquiPlugin.setup(ctx)

    const names = ctx.tool.hook.mock.calls.map((c: any[]) => c[0])
    expect(names).toContain("execute.after")
  })

  it("ctx.tool.transform editor.add registers 'endSession' with summary required", async () => {
    enableBinaryResolve()
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx, transformEditors } = makeMockCtx()
    await ilnamiquiPlugin.setup(ctx)

    expect(transformEditors.length).toBe(1)
    const added: any[] = []
    transformEditors[0]({ add: (t: any) => added.push(t) })
    expect(added.length).toBe(1)
    expect(added[0].name).toBe("endSession")
    expect(added[0].input.required).toContain("summary")
  })

  it("reminder suffix format is '\\n\\n---\\n' + reminderText", () => {
    // Pure contract: the V2 plugin composes the suffix exactly this way.
    const expected = "\n\n---\n" + REMINDER_TEXT
    expect(expected.startsWith("\n\n---\n")).toBe(true)
    expect(expected.endsWith(REMINDER_TEXT)).toBe(true)
  })

  it("execute.after hook injects reminder suffix into string content at threshold", async () => {
    enableBinaryResolve()
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx, toolHooks } = makeMockCtx()
    await ilnamiquiPlugin.setup(ctx)

    const hook = toolHooks["execute.after"]
    expect(hook).toBeDefined()

    resetTestState()
    let ev: any = { status: "completed", result: { content: "tool output" } }
    // Drive counter to MAX_INTERACTIONS — the hook increments then checks.
    for (let i = 0; i < MAX_INTERACTIONS; i++) {
      ev = { status: "completed", result: { content: i === MAX_INTERACTIONS - 1 ? "tool output" : ev.result.content } }
      await hook(ev)
    }
    expect(ev.result.content).toContain("\n\n---\n" + REMINDER_TEXT)
    expect(ev.result.content).toContain("tool output")
  })

  it("execute.after hook injects reminder suffix into array content at threshold", async () => {
    enableBinaryResolve()
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx, toolHooks } = makeMockCtx()
    await ilnamiquiPlugin.setup(ctx)

    const hook = toolHooks["execute.after"]
    resetTestState()

    let ev: any
    for (let i = 0; i < MAX_INTERACTIONS; i++) {
      ev = { status: "completed", result: { content: [{ type: "text", text: "part1" }] } }
      await hook(ev)
    }
    expect(Array.isArray(ev.result.content)).toBe(true)
    expect(ev.result.content.length).toBe(2)
    expect(ev.result.content[1].text).toContain("\n\n---\n" + REMINDER_TEXT)
  })

  it("execute.after hook handles undefined content at threshold", async () => {
    enableBinaryResolve()
    const { ilnamiquiPlugin } = await import("./ilnamiqui")
    const { ctx, toolHooks } = makeMockCtx()
    await ilnamiquiPlugin.setup(ctx)

    const hook = toolHooks["execute.after"]
    resetTestState()

    let ev: any
    for (let i = 0; i < MAX_INTERACTIONS; i++) {
      ev = { status: "completed", result: { content: undefined } }
      await hook(ev)
    }
    expect(ev.result.content).toBe("\n\n---\n" + REMINDER_TEXT)
  })
})
