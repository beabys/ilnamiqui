import type { Plugin } from "@opencode-ai/plugin"
import path from "path"
import os from "os"
import fs from "fs"

// ---------------------------------------------------------------------------
// Logger — writes to ~/.config/opencode/plugins/ilnamiqui/plugin.log
// ---------------------------------------------------------------------------

const LOG_FILE = path.join(
  os.homedir(),
  ".config",
  "opencode",
  "plugins",
  "ilnamiqui",
  "plugin.log",
)

function log(msg: string): void {
  const ts = new Date().toISOString()
  try {
    fs.appendFileSync(LOG_FILE, `[${ts}] ${msg}\n`)
  } catch {
    /* best effort */
  }
}

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
  let sessionContextLoaded = false

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
      return null
    }
  })()

  if (!binary) {
    log(
      `binary not found (tried ${resolved.name} and PATH) — plugin disabled`,
    )
    return {}
  }

  log(`binary found: ${binary}`)

  // ----- Event hooks -----
  return {
    event: async (event: unknown) => {
      try {
        const ev = event as Record<string, unknown>
        const type = typeof ev?.type === "string" ? ev.type : ""
        const name = typeof ev?.name === "string" ? ev.name : ""

        // session.start — load previous context
        if (type === "session.start" || name === "session.start") {
          if (sessionContextLoaded) return
          sessionContextLoaded = true
          log("session.start — loading context")
          await $`${binary} load --pretty --limit 50`.nothrow()
          return
        }

        // session.end — persist session data
        if (type === "session.end" || name === "session.end") {
          log("session.end — saving session")
          await $`${binary} session end --summary "session ended"`.quiet().nothrow()
          return
        }

        // chat.message — detect /exit command
        if (type === "chat.message" || name === "chat.message") {
          const content = typeof ev?.content === "string" ? ev.content : ""
          if (content.trim() === "/exit") {
            log("/exit — saving session")
            await $`${binary} session end --summary "session ended"`.quiet().nothrow()
          }
          return
        }
      } catch (e) {
        log(`event error: ${e}`)
      }
    },
  }
}

export default plugin
