import type { Plugin } from "@opencode-ai/plugin"
import path from "path"
import os from "os"

// ---------------------------------------------------------------------------
// Platform → binary name resolution
// ---------------------------------------------------------------------------

const PLATFORM_MAP: Record<string, string> = {
  darwin: "darwin",
  linux: "linux",
  win32: "windows",
}

const ARCH_MAP: Record<string, string> = {
  x64: "amd64",
  arm64: "arm64",
}

/**
 * Resolve the platform-specific binary path inside the opencode plugin
 * directory, e.g.:
 *   ~/.config/opencode/plugins/ilnamiqui/ilnamiqui-darwin-arm64
 *
 * Falls back to bare `ilnamiqui` if the platform-specific binary is missing
 * (handles dev installs via `go install`).
 */
function resolveBinaryPath(): {
  path: string
  name: string
} {
  const platform = PLATFORM_MAP[process.platform] || process.platform
  const arch = ARCH_MAP[process.arch] || process.arch
  const ext = platform === "windows" ? ".exe" : ""
  const name = `ilnamiqui-${platform}-${arch}${ext}`
  return {
    path: path.join(os.homedir(), ".config", "opencode", "plugins", "ilnamiqui", name),
    name,
  }
}

// ---------------------------------------------------------------------------
// Plugin definition
// ---------------------------------------------------------------------------

const plugin: Plugin = async ({ $ }) => {
  const resolved = resolveBinaryPath()

  // Try platform-specific binary first, then bare name (dev installs)
  const binary = await (async (): Promise<string | null> => {
    try {
      await $`test -x ${resolved.path}`.quiet()
      return resolved.path
    } catch {
      // not found at platform path — try PATH
    }
    try {
      await $`which ilnamiqui`.quiet()
      return "ilnamiqui"
    } catch {
      // not in PATH either — plugin is unusable
      console.warn(
        `[ilnamiqui] binary not found (tried ${resolved.name} and ilnamiqui in PATH) — plugin disabled`
      )
      return null
    }
  })()

  if (!binary) return {}

  // ----- Session start: inject previous context -----
  try {
    const { stdout } = await $`${binary} load --pretty`.quiet().nothrow()
    if (stdout) {
      // Context loaded — opencode will see the output in the system prompt
    }
  } catch {
    // DB not yet initialized or first session — no context to load
  }

  // ----- Event hooks -----
  return {
    event: async (event: unknown) => {
      try {
        const ev = event as Record<string, unknown>
        const type = typeof ev?.type === "string" ? ev.type : ""
        const name = typeof ev?.name === "string" ? ev.name : ""

        // session.end — persist session data
        if (type === "session.end" || name === "session.end") {
          await $`${binary} session end --summary "session ended"`.quiet().nothrow()
          return
        }

        // chat.message — detect /exit command
        if (type === "chat.message" || name === "chat.message") {
          const content = typeof ev?.content === "string" ? ev.content : ""
          if (content.trim() === "/exit") {
            await $`${binary} session end --summary "session ended"`.quiet().nothrow()
          }
          return
        }

        // session.start — already handled above in plugin init, but
        // catch restart events mid-session (e.g., /session-memory init)
        if (type === "session.start" || name === "session.start") {
          await $`${binary} load --pretty`.quiet().nothrow()
          return
        }
      } catch {
        // Never let a plugin error crash opencode
      }
    },
  }
}

export default plugin
