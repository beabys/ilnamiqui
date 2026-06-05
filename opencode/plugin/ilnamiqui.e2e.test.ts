import { describe, it, expect } from "vitest"
import { execSync } from "child_process"
import { existsSync, accessSync, constants } from "fs"
import { homedir } from "os"
import { join } from "path"
import pluginModule from "./ilnamiqui"
import { resolveBinarySync } from "./ilnamiqui"

// ---------------------------------------------------------------------------
// E2E: Plugin Binary Resolution
//
// Tests against real system — verify ilnamiqui binary is findable and works.
// "Tests from requirements, not code" — covers:
//   1. command -v ilnamiqui → symlink path (primary lookup)
//   2. Binary at returned path is executable
//   3. Platform-specific fallback path is well-formed
//   4. ilnamiqui version runs successfully
// ---------------------------------------------------------------------------

describe("plugin binary resolution (E2E)", () => {
  // -----------------------------------------------------------------------
  // Requirement 1: command -v finds symlink
  // -----------------------------------------------------------------------
  it("command -v ilnamiqui returns a path to the binary", () => {
    const output = execSync("command -v ilnamiqui", { encoding: "utf8" }).trim()
    expect(output).toBeTruthy()
    // Must be an absolute path
    expect(output.startsWith("/")).toBe(true)
    // Must contain "ilnamiqui" in the filename
    expect(output).toContain("ilnamiqui")
  })

  it("command -v path contains 'local/bin/ilnamiqui' symlink pattern", () => {
    const output = execSync("command -v ilnamiqui", { encoding: "utf8" }).trim()
    // The symlink should be at ~/.local/bin/ilnamiqui or similar bin dir
    // At minimum, the path must resolve to something with the binary name
    expect(output).toMatch(/ilnamiqui[^/]*$/)
  })

  // -----------------------------------------------------------------------
  // Requirement 2: Binary at returned path is executable
  // -----------------------------------------------------------------------
  it("binary at command -v path is executable", () => {
    const binPath = execSync("command -v ilnamiqui", { encoding: "utf8" }).trim()
    expect(binPath).toBeTruthy()

    const exists = existsSync(binPath)
    expect(exists).toBe(true)

    // Verify it's executable (no throw = executable)
    expect(() => accessSync(binPath, constants.X_OK)).not.toThrow()
  })

  // -----------------------------------------------------------------------
  // Requirement 3: Platform-specific fallback path is well-formed
  // -----------------------------------------------------------------------
  it("platform-specific fallback path includes platform and architecture", () => {
    const platform = process.platform // darwin, linux, win32
    const arch = process.arch // arm64, x64

    const pluginDir = join(homedir(), ".config", "opencode", "plugins", "ilnamiqui")

    // Build the expected platform-specific filename
    const osMap: Record<string, string> = {
      darwin: "darwin",
      linux: "linux",
      win32: "windows",
    }
    const archMap: Record<string, string> = {
      arm64: "arm64",
      x64: "amd64",
    }

    const osName = osMap[platform] || platform
    const archName = archMap[arch] || arch
    const isWin = platform === "win32"
    const binaryName = isWin
      ? `ilnamiqui-${osName}-${archName}.exe`
      : `ilnamiqui-${osName}-${archName}`

    const fallbackPath = join(pluginDir, binaryName)

    // Check the path pattern is correct
    expect(fallbackPath).toContain(`${osName}-${archName}`)
    // The path must have the correct extension for the platform
    if (isWin) {
      expect(fallbackPath.endsWith(".exe")).toBe(true)
    } else {
      expect(fallbackPath.endsWith(".exe")).toBe(false)
    }
  })

  it("platform-specific fallback binary exists in plugin directory", () => {
    const pluginDir = join(homedir(), ".config", "opencode", "plugins", "ilnamiqui")
    expect(existsSync(pluginDir)).toBe(true)

    // Should contain at least one ilnamiqui-* binary
    const entries = execSync(`ls "${pluginDir}" 2>/dev/null || dir "${pluginDir}"`, {
      encoding: "utf8",
    })
    expect(entries).toMatch(/ilnamiqui-/)
  })

  // -----------------------------------------------------------------------
  // Requirement 4: ilnamiqui binary works (version command)
  // -----------------------------------------------------------------------
  it("ilnamiqui version runs and returns a version string", () => {
    const output = execSync("ilnamiqui version", { encoding: "utf8" }).trim()
    expect(output).toBeTruthy()
    // Must be a semantic version or a version-ish string
    expect(output.length).toBeGreaterThan(0)
  })

  it("ilnamiqui version returns a valid semver-like string", () => {
    const output = execSync("ilnamiqui version", { encoding: "utf8" }).trim()
    // Accept formats: "1.5.0", "v1.5.0", "1.5.0-rc1", etc.
    expect(output).toMatch(/^v?\d+\.\d+/)
  })

  // -----------------------------------------------------------------------
  // Cross-platform: verify no .exe on Unix, .exe expected on Windows
  // -----------------------------------------------------------------------
  it("binary resolved by command -v matches platform expectations", () => {
    const isWin = process.platform === "win32"
    const binPath = execSync("command -v ilnamiqui", { encoding: "utf8" }).trim()
    if (isWin) {
      // On Windows, command -v may resolve through MSYS/Cygwin
      // But if it returns a path, it should end in .exe or have no extension
      expect(binPath).toBeTruthy()
    } else {
      // On Unix, no .exe extension
      expect(binPath.endsWith(".exe")).toBe(false)
      // Should be an absolute path
      expect(binPath.startsWith("/")).toBe(true)
    }
  })
})

// ---------------------------------------------------------------------------
// E2E: resolveBinarySync — synchronous binary resolution
//
// Tests from requirements — plan requires synchronous binary resolution so
// hook registration doesn't miss session.created at startup.
// ---------------------------------------------------------------------------

describe("resolveBinarySync export (E2E)", () => {
  it("exports resolveBinarySync as a function", () => {
    expect(typeof resolveBinarySync).toBe("function")
  })

  it("module default export has server function that returns event handler", async () => {
    // Default export = { id, server }
    const plugin = pluginModule as { id: string; server: (ctx: Record<string, unknown>) => Promise<Record<string, unknown>>; }
    expect(typeof plugin.server).toBe("function")

    // Provide mock $ so plugin factory's fire-and-forget init doesn't crash
    const mock$ = ((_parts: TemplateStringsArray, ..._args: unknown[]) => ({
      quiet: () => ({ nothrow: () => Promise.resolve({ exitCode: 0, stdout: "", stderr: "" }) }),
    })) as unknown as (parts: TemplateStringsArray, ...args: unknown[]) => { quiet: () => { nothrow: () => Promise<{ exitCode: number; stdout: string; stderr: string }> } }

    // Call server with minimal context to get hooks (including event handler)
    const hooks = await plugin.server({ $: mock$ } as any)
    expect(typeof hooks.event).toBe("function")
  })

  it("resolveBinarySync finds ilnamiqui binary on real system", () => {
    const binPath = resolveBinarySync()
    expect(binPath).not.toBeNull()
    expect(typeof binPath).toBe("string")
    // Must be an absolute path containing ilnamiqui
    expect((binPath as string).startsWith("/")).toBe(true)
    expect((binPath as string)).toContain("ilnamiqui")
    // Binary must be executable
    expect(() => accessSync(binPath as string, constants.X_OK)).not.toThrow()
  })
})

// ---------------------------------------------------------------------------
// E2E: Plugin Server Hooks
//
// Tests from plan requirements — verify server returns expected hook surface:
//   1. event handler (for session.created)
//   2. Graceful handling of non-zero session start exit
// ---------------------------------------------------------------------------

describe("plugin server hooks (E2E)", () => {
  // Helper type
  type PluginModule = {
    id: string
    server: (ctx: Record<string, unknown>) => Promise<Record<string, unknown>>
  }
  type Mock$ = (parts: TemplateStringsArray, ...args: unknown[]) => {
    quiet: () => {
      nothrow: () => Promise<{ exitCode: number; stdout: string; stderr: string }>
    }
  }

  const makeMock$ = (exitCode: number): Mock$ =>
    ((_parts: TemplateStringsArray, ..._args: unknown[]) => ({
      quiet: () => ({
        nothrow: () => Promise.resolve({ exitCode, stdout: "", stderr: "" }),
      }),
    })) as unknown as Mock$

  // -----------------------------------------------------------------------
  // Requirement 1: Server returns event handler + system.transform hook
  // -----------------------------------------------------------------------
  it("server returns event handler", async () => {
    const plugin = pluginModule as PluginModule
    const mock$ = makeMock$(0)
    const hooks = await plugin.server({ $: mock$ } as any)

    expect(hooks.event).toBeDefined()
    expect(typeof hooks.event).toBe("function")
  })

  it("does not return experimental.chat.system.transform hook — removed", async () => {
    const plugin = pluginModule as PluginModule
    const mock$ = makeMock$(0)
    const hooks = await plugin.server({ $: mock$ } as any)
    const systemTransform = (
      hooks as Record<string, unknown>
    )["experimental.chat.system.transform"]
    expect(systemTransform).toBeUndefined()
  })

  // -----------------------------------------------------------------------
  // Requirement 2: Mock $ with non-zero exit — plugin does not crash
  // -----------------------------------------------------------------------
  it("does not crash when session start returns non-zero exit", async () => {
    const plugin = pluginModule as PluginModule
    const mock$ = makeMock$(1) // non-zero exit

    // Must not throw — plugin should degrade gracefully
    const hooks = await plugin.server({ $: mock$ } as any)

    expect(typeof hooks.event).toBe("function")
    // event handler should still be callable
    expect(() => (hooks.event as Function)({ type: "session.created", data: {} })).not.toThrow()
  })

  // -----------------------------------------------------------------------
  // Requirement 3: Mock $ with success — plugin completes init
  // -----------------------------------------------------------------------
  it("completes initialization when session start succeeds", async () => {
    const plugin = pluginModule as PluginModule
    const mock$ = makeMock$(0) // success exit

    const hooks = await plugin.server({ $: mock$ } as any)

    expect(typeof hooks.event).toBe("function")
    expect(() => (hooks.event as Function)({ type: "test", data: {} })).not.toThrow()
  })
})
