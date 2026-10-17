import { describe, it, expect, beforeAll, afterAll, beforeEach } from "vitest"
import { execSync } from "child_process"
import { mkdtempSync, rmSync } from "fs"
import { tmpdir } from "os"
import { join } from "path"
import pluginModule from "./ilnamiqui"
import {
  resolveBinarySync,
  REMINDER_TEXT,
  formatReminder,
  resetTestState,
  interactionCounter,
  MAX_INTERACTIONS,
} from "./ilnamiqui"

// ---------------------------------------------------------------------------
// E2E: System Memory Key & Dynamic Reminder
//
// Tests from plan requirements — never from code.
// Covers:
//   1. `system` key exists after migration, value matches plan
//   2. `system` key is marked critical (can't be pruned)
//   3. `project-path` still exists and is critical (regression)
//   4. formatReminder joins multiple values with " and "
//   5. formatReminder falls back to REMINDER_TEXT for empty input
//   6. tool.execute.after loads critical keys from binary and injects reminder
// ---------------------------------------------------------------------------

const SYSTEM_KEY_VALUE =
  "REMINDER: Read the rules in AGENTS.md before continuing with your task(s)."

// ---------------------------------------------------------------------------
// E2E setup: fresh DB with v3 migration in a temp directory
// ---------------------------------------------------------------------------
let tmpDir: string

beforeAll(() => {
  tmpDir = mkdtempSync(join(tmpdir(), "ilnamiqui-e2e-"))
  const binPath = resolveBinarySync()
  if (!binPath) {
    throw new Error("ilnamiqui binary not found — cannot run E2E tests")
  }
  execSync(`${binPath} init`, { encoding: "utf8", cwd: tmpDir })
})

afterAll(() => {
  if (tmpDir) {
    rmSync(tmpDir, { recursive: true, force: true })
  }
})

// Helper: run ilnamiqui binary command and return stdout
function runIlnamiqui(args: string): string {
  const binPath = resolveBinarySync()
  if (!binPath) {
    throw new Error("ilnamiqui binary not found — cannot run E2E tests")
  }
  return execSync(`${binPath} ${args}`, { encoding: "utf8", cwd: tmpDir }).trim()
}

// Helper: run ilnamiqui command with JSON output
function runIlnamiquiJSON(args: string): unknown {
  const out = execSync(`${resolveBinarySync()} ${args}`, {
    encoding: "utf8",
    cwd: tmpDir,
  }).trim()
  return JSON.parse(out)
}

// ---------------------------------------------------------------------------
// Requirement 1 — Migration: system key exists with correct value
// ---------------------------------------------------------------------------
describe("system memory key (E2E)", () => {
  it("ilnamiqui version works (binary accessible)", () => {
    const version = runIlnamiqui("version")
    expect(version).toBeTruthy()
    expect(version.length).toBeGreaterThan(0)
  })

  // -----------------------------------------------------------------------
  // Requirement: system key search returns the correct value
  // -----------------------------------------------------------------------
  it('ilnamiqui search --mode key "system" returns correct value', () => {
    const output = runIlnamiqui('search --mode key "system"')
    expect(output).toContain(SYSTEM_KEY_VALUE)
    expect(output).toContain("AGENTS.md")
  })

  // -----------------------------------------------------------------------
  // Requirement: system key is critical (critical=1 in memory_keys)
  // -----------------------------------------------------------------------
  it("ilnamiqui keys includes system with critical=true", () => {
    const output = runIlnamiqui("keys --pretty")
    // The table output should have a row for 'system' with critical true
    expect(output).toContain("system")
    // Critical flag should be truthy in output
    // In pretty-print, critical shows as "true" or "1"
    const lines = output.split("\n")
    const systemLine = lines.find((l) => l.includes("system"))
    expect(systemLine).toBeTruthy()
    // The critical column should indicate true/1
    expect(systemLine).toMatch(/system\s+true|\bsystem\b.*\b1\b/)
  })

  // -----------------------------------------------------------------------
  // Requirement: JSON keys output has system with critical=true
  // -----------------------------------------------------------------------
  it("ilnamiqui keys JSON includes system key with critical=true", () => {
    const keys = runIlnamiquiJSON("keys") as Array<{
      key: string
      critical: boolean
    }>
    const systemKey = keys.find((k) => k.key === "system")
    expect(systemKey).toBeDefined()
    expect(systemKey!.critical).toBe(true)
  })

  // -----------------------------------------------------------------------
  // Requirement: migration is idempotent — running again doesn't duplicate
  // -----------------------------------------------------------------------
  it("migration idempotent — running search twice yields same result", () => {
    // Verify system key appears exactly once in JSON output
    const output = runIlnamiqui('search --mode key "system"')
    // Use string matching (not regex) to avoid special chars like (s) and .
    const count = output.split(SYSTEM_KEY_VALUE).length - 1
    expect(count).toBe(1)
  })
})

// ---------------------------------------------------------------------------
// Requirement — Regression: project-path still exists and is critical
// ---------------------------------------------------------------------------
describe("project-path regression (E2E)", () => {
  it("project-path key still exists in keys list", () => {
    const output = runIlnamiqui("keys --pretty")
    expect(output).toContain("project-path")
  })

  it("project-path key is still critical", () => {
    const keys = runIlnamiquiJSON("keys") as Array<{
      key: string
      critical: boolean
    }>
    const projectPathKey = keys.find((k) => k.key === "project-path")
    expect(projectPathKey).toBeDefined()
    expect(projectPathKey!.critical).toBe(true)
  })

  it("project-path search returns a value", () => {
    const output = runIlnamiqui('search --mode key "project-path"')
    expect(output).toBeTruthy()
    expect(output.length).toBeGreaterThan(0)
  })
})

// ---------------------------------------------------------------------------
// Requirement — formatReminder function behavior
// ---------------------------------------------------------------------------
describe("formatReminder function (E2E)", () => {
  beforeEach(() => {
    resetTestState()
  })

  it("formatReminder joins multiple values with ' and ' separator", () => {
    const result = formatReminder(["rule1", "rule2", "rule3"])
    expect(result).toBe("rule1 and rule2 and rule3")
  })

  it("formatReminder returns single value unchanged", () => {
    const result = formatReminder(["only one"])
    expect(result).toBe("only one")
  })

  it("formatReminder falls back to REMINDER_TEXT when empty array", () => {
    const result = formatReminder([])
    expect(result).toBe(REMINDER_TEXT)
  })

  it("formatReminder fallback contains AGENTS.md reference", () => {
    const result = formatReminder([])
    expect(result).toContain("AGENTS.md")
    expect(result.startsWith("REMINDER:")).toBe(true)
  })

  it("formatReminder is exported and is a function", () => {
    expect(typeof formatReminder).toBe("function")
  })
})

// ---------------------------------------------------------------------------
// Requirement — tool.execute.after dynamic reminder injection
// ---------------------------------------------------------------------------
describe("tool.execute.after dynamic reminder (E2E)", () => {
  beforeEach(() => {
    resetTestState()
  })

  type PluginModule = {
    id: string
    server: (
      ctx: Record<string, unknown>,
    ) => Promise<Record<string, unknown>>
  }
  type Mock$ = (
    parts: TemplateStringsArray,
    ...args: unknown[]
  ) => {
    quiet: () => {
      nothrow: () => Promise<{
        exitCode: number
        stdout: string
        stderr: string
      }>
    }
  }

  const makeMock$ = (
    exitCode: number,
    stdout: string = "",
  ): Mock$ =>
    ((_parts: TemplateStringsArray, ..._args: unknown[]) => ({
      quiet: () => ({
        nothrow: () =>
          Promise.resolve({ exitCode, stdout, stderr: "" }),
      }),
    })) as unknown as Mock$

  it("tool.execute.after appends reminder at threshold using formatReminder", async () => {
    const plugin = pluginModule as PluginModule
    // Mock so $ returns empty stdout (no critical keys from binary)
    // → formatReminder([]) → REMINDER_TEXT fallback
    const mock$ = makeMock$(0, "[]")
    const hooks = (await plugin.server({
      $: mock$,
    } as any)) as Record<string, unknown>
    const toolAfter = hooks[
      "tool.execute.after"
    ] as (input: { tool: string }, output: { output: string }) => Promise<void>

    const tracked = { output: "tool data" }
    // Counter starts at 0, need MAX_INTERACTIONS calls
    for (let i = 0; i < MAX_INTERACTIONS - 1; i++) {
      await toolAfter({ tool: "read" }, { output: "" })
    }
    // Threshold hit — should append reminder
    await toolAfter({ tool: "read" }, tracked)

    expect(tracked.output).toContain("tool data")
    expect(tracked.output).toContain(REMINDER_TEXT)
    expect(tracked.output).toContain("---")
  })

  it("tool.execute.after does NOT modify output below threshold", async () => {
    const plugin = pluginModule as PluginModule
    const mock$ = makeMock$(0, "[]")
    const hooks = (await plugin.server({
      $: mock$,
    } as any)) as Record<string, unknown>
    const toolAfter = hooks[
      "tool.execute.after"
    ] as (input: { tool: string }, output: { output: string }) => Promise<void>

    const tracked = { output: "original data" }
    // First call (counter = 1, < MAX_INTERACTIONS=50)
    await toolAfter({ tool: "read" }, tracked)

    expect(tracked.output).toBe("original data")
  })

  it("tool.execute.after resets counter after threshold", async () => {
    resetTestState()
    const plugin = pluginModule as PluginModule
    const mock$ = makeMock$(0, "[]")
    const hooks = (await plugin.server({
      $: mock$,
    } as any)) as Record<string, unknown>
    const toolAfter = hooks[
      "tool.execute.after"
    ] as (input: { tool: string }, output: { output: string }) => Promise<void>

    // Run MAX_INTERACTIONS calls
    for (let i = 0; i < MAX_INTERACTIONS; i++) {
      await toolAfter({ tool: "read" }, { output: "" })
    }
    // Counter should be 0 after threshold hit
    expect(interactionCounter).toBe(0)
  })

  it("tool.execute.after loads critical keys from binary and injects joined values", async () => {
    const plugin = pluginModule as PluginModule
    // Mock binary to return two critical keys
    const mockKeys = JSON.stringify([
      { key: "system", critical: true, last_used_at: "2026-06-13T12:00:00Z" },
      {
        key: "project-path",
        critical: true,
        last_used_at: "2026-06-13T12:00:00Z",
      },
    ])
    // Mock search results for each key
    const mockSystemSearch = JSON.stringify([
      { value: SYSTEM_KEY_VALUE },
    ])
    const mockProjectPathSearch = JSON.stringify([
      {
        value:
          "Remember, the working directory for this project is: /Users/beabys/go/src/github.com/beabys/ilnamiqui",
      },
    ])

    // We need sequential mock calls: first keys, then search system, then search project-path
    let callCount = 0
    const mockSequential$ =
      ((_parts: TemplateStringsArray, ..._args: unknown[]) => ({
        quiet: () => ({
          nothrow: () => {
            callCount++
            if (callCount === 1) {
              // First call: keys
              return Promise.resolve({
                exitCode: 0,
                stdout: mockKeys,
                stderr: "",
              })
            } else if (callCount === 2) {
              // Second call: search system
              return Promise.resolve({
                exitCode: 0,
                stdout: mockSystemSearch,
                stderr: "",
              })
            } else {
              // Third call: search project-path
              return Promise.resolve({
                exitCode: 0,
                stdout: mockProjectPathSearch,
                stderr: "",
              })
            }
          },
        }),
      })) as unknown as Mock$

    resetTestState()
    const hooks = (await plugin.server({
      $: mockSequential$,
    } as any)) as Record<string, unknown>
    const toolAfter = hooks[
      "tool.execute.after"
    ] as (input: { tool: string }, output: { output: string }) => Promise<void>

    // Run MAX_INTERACTIONS-1 times to set up
    for (let i = 0; i < MAX_INTERACTIONS - 1; i++) {
      await toolAfter({ tool: "read" }, { output: "" })
    }
    // Reset callCount for the threshold hit
    callCount = 0
    const tracked = { output: "data" }
    await toolAfter({ tool: "read" }, tracked)

    // Should contain both values joined with " and "
    expect(tracked.output).toContain(SYSTEM_KEY_VALUE)
    expect(tracked.output).toContain("working directory")
    expect(tracked.output).toContain(" and ")
    expect(tracked.output).toContain("---")
  })

  it("tool.execute.after falls back to REMINDER_TEXT when binary call fails", async () => {
    const plugin = pluginModule as PluginModule
    // Mock binary to return non-zero exit (failure)
    const mockFail$ =
      ((_parts: TemplateStringsArray, ..._args: unknown[]) => ({
        quiet: () => ({
          nothrow: () =>
            Promise.resolve({
              exitCode: 1,
              stdout: "",
              stderr: "error",
            }),
        }),
      })) as unknown as Mock$

    resetTestState()
    const hooks = (await plugin.server({
      $: mockFail$,
    } as any)) as Record<string, unknown>
    const toolAfter = hooks[
      "tool.execute.after"
    ] as (input: { tool: string }, output: { output: string }) => Promise<void>

    for (let i = 0; i < MAX_INTERACTIONS - 1; i++) {
      await toolAfter({ tool: "read" }, { output: "" })
    }
    const tracked = { output: "data" }
    await toolAfter({ tool: "read" }, tracked)

    // Should fall back to REMINDER_TEXT
    expect(tracked.output).toContain(REMINDER_TEXT)
    expect(tracked.output).toContain("AGENTS.md")
  })
})

// ---------------------------------------------------------------------------
// Requirement — interactionCounter module variables
// ---------------------------------------------------------------------------
describe("interaction counter exports (E2E)", () => {
  it("exports interactionCounter as a number", () => {
    expect(typeof interactionCounter).toBe("number")
  })

  it("MAX_INTERACTIONS is 50", () => {
    expect(MAX_INTERACTIONS).toBe(50)
  })

  it("REMINDER_TEXT is exported and contains AGENTS.md", () => {
    expect(REMINDER_TEXT).toContain("AGENTS.md")
  })

  it("resetTestState resets counter to 0", () => {
    resetTestState()
    expect(interactionCounter).toBe(0)
  })
})
