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
// Conversation buffer — accumulates last ~20 user messages for summary
// ---------------------------------------------------------------------------

const MAX_BUFFER = 20

interface BufferEntry {
  role: string
  text: string
  timestamp: string
}

const conversationBuffer: BufferEntry[] = []

// Guard to prevent double-save on repeated session.created/deleted events
let sessionInitialized = false
let exitSaved = false

// ---------------------------------------------------------------------------
// buildSummary — rule-based summary extraction from conversation buffer
// ---------------------------------------------------------------------------

function buildSummary(buffer: BufferEntry[]): string {
  const now = new Date().toISOString()

  if (buffer.length === 0) {
    return [
      "session: (empty)",
      "state: in-progress",
      "files: (none)",
      "decisions: (none)",
      `last_turn: ${now}`,
      "entry_count: 0",
    ].join("\n")
  }

  const last = buffer[buffer.length - 1]
  const task = last.text.replace(/\s+/g, " ").trim().substring(0, 200)

  // ── extract file paths ─────────────────────────────────────────────
  const filePathSet = new Set<string>()
  // Match any path with directory + filename.ext
  const pathPatterns = [
    /(?:[\w.-]+\/)+[\w.-]+\.[a-z]+/g,
  ]
  for (const entry of buffer) {
    for (const re of pathPatterns) {
      const matches = entry.text.match(re)
      if (matches) {
        for (const m of matches) {
          filePathSet.add(m)
        }
      }
    }
  }
  const files =
    filePathSet.size > 0 ? Array.from(filePathSet).join(", ") : "(none)"

  // ── extract decision lines ─────────────────────────────────────────
  const decisionLines: string[] = []
  for (const entry of buffer) {
    const lines = entry.text.split("\n")
    for (const line of lines) {
      const trimmed = line.trim()
      if (
        /(?:^|\s)(?:use\s|chose\s|implement|refactor|migrate|fix:|add:)/i.test(
          trimmed,
        )
      ) {
        const excerpt = trimmed.substring(0, 120)
        if (!decisionLines.includes(excerpt)) {
          decisionLines.push(excerpt)
        }
      }
    }
  }
  const decisions =
    decisionLines.length > 0 ? decisionLines.join("; ") : "(none)"

  return [
    `session: ${task}`,
    "state: in-progress",
    `files: ${files}`,
    `decisions: ${decisions}`,
    `last_turn: ${last.timestamp}`,
    `entry_count: ${buffer.length}`,
  ].join("\n")
}

// ---------------------------------------------------------------------------
// Platform → binary name resolution
// ---------------------------------------------------------------------------

/**
 * Resolve the platform-specific binary path inside the opencode plugin
 * directory, e.g.:
 *   ~/.config/opencode/plugins/ilnamiqui/ilnamiqui-darwin-arm64
 *
 * Falls back to bare `ilnamiqui` if the platform-specific binary is missing
 * (handles dev installs via `go install`).
 */
function resolveBinaryPath(): string {
  const p = process.platform === "win32" ? "windows" : process.platform
  const ext = p === "windows" ? ".exe" : ""
  const name = `ilnamiqui-${p}-${process.arch}${ext}`
  return path.join(
    os.homedir(),
    ".config",
    "opencode",
    "plugins",
    "ilnamiqui",
    name,
  )
}

// ---------------------------------------------------------------------------
// Binary finder — synchronous (no shell)
// ---------------------------------------------------------------------------

/**
 * Resolve ilnamiqui binary synchronously:
 *  1. Walk PATH in process.env.PATH (no shell)
 *  2. Fallback to platform-specific path in plugin dir
 *  3. Return null if not found
 */
function resolveBinarySync(): string | null {
  // 1. Scan PATH (no shell subprocess)
  const pathDirs = (process.env.PATH || "").split(path.delimiter).filter(Boolean)
  for (const dir of pathDirs) {
    const full = path.join(dir, "ilnamiqui")
    if (fs.existsSync(full)) {
      try {
        fs.accessSync(full, fs.constants.X_OK)
        return full
      } catch {
        /* not executable, continue */
      }
    }
  }

  // 2. Fallback: platform-specific binary in plugin directory
  const platformPath = resolveBinaryPath()
  try {
    fs.accessSync(platformPath, fs.constants.X_OK)
    return platformPath
  } catch {
    return null
  }
}

// ---------------------------------------------------------------------------
// Plugin definition
// ---------------------------------------------------------------------------

const plugin: Plugin = async ({ $ }) => {
  const binary = resolveBinarySync()

  if (!binary) {
    log("binary not found (PATH + plugin dir) — plugin disabled")
    return {}
  }

  log(`binary found: ${binary}`)

  // ----- Register chat.message hook to buffer user messages -----
  const chatMessageHook = async (_input: unknown, output: { parts: Array<{ type: string; text?: string }> }) => {
    const textPart = output.parts.find(
      (p): p is { type: string; text: string } => p.type === "text" && typeof p.text === "string",
    )
    if (textPart && textPart.text) {
      const entry: BufferEntry = {
        role: "user",
        text: textPart.text,
        timestamp: new Date().toISOString(),
      }
      conversationBuffer.push(entry)
      if (conversationBuffer.length > MAX_BUFFER) {
        conversationBuffer.shift()
      }
    }
  }

  // ----- Event hooks -----
  return {
    event: async ({ event }: { event: { type: string } }) => {
      try {
        log(`event: ${event.type}`)

        // session.created — start opencode session + load previous context
        if (event.type === "session.created") {
          if (sessionInitialized) return
          sessionInitialized = true
          log("session.created — starting session and loading context")
          await $`${binary} session start --agent opencode`.quiet().nothrow()
          await $`${binary} load --limit 50`.quiet().nothrow()
          return
        }

        // session.compacted — save memory entry + reload after compaction
        if (event.type === "session.compacted") {
          log("session.compacted — saving entry and reloading memories")
          await $`${binary} save --agent opencode "compact" "compaction completed at ${new Date().toISOString()}"`.quiet().nothrow()
          await $`${binary} load --limit 50`.quiet().nothrow()
          return
        }

        // session.deleted — user typed /exit or session ended
        if (event.type === "session.deleted") {
          if (exitSaved) return

          log("session.deleted — saving context")
          const summary = buildSummary(conversationBuffer)
          await $`${binary} save --agent opencode "session" ${summary}`.quiet().nothrow()
          await $`${binary} session end --agent opencode --summary ${summary}`.quiet().nothrow()
          exitSaved = true
          return
        }
      } catch (e) {
        log(`event error: ${e}`)
      }
    },

    // Save memory entry + inject memory context into compaction prompt
    "experimental.session.compacting": async (_input: unknown, output: { context?: string[] }) => {
      log("session.compacting — saving pre-compaction entry")
      const summary = buildSummary(conversationBuffer)
      await $`${binary} save --agent opencode "compact" ${summary}`.quiet().nothrow()
      log("session.compacting — injecting memory context")
      const result = await $`${binary} load --limit 10 --pretty`.quiet().nothrow()
      if (result.exitCode === 0 && result.stdout) {
        output.context = [
          "Recent session memory entries (ilnamiqui):",
          String(result.stdout).trim(),
        ]
      }
    },

    // Buffer user messages (no longer detects /exit — handled by session.deleted)
    "chat.message": chatMessageHook,
  }
}

export const ilnamiquiPlugin = plugin

export { buildSummary, conversationBuffer, sessionInitialized, exitSaved, BufferEntry, resolveBinarySync, resolveBinaryPath }

export function resetTestState(): void {
  conversationBuffer.length = 0
  sessionInitialized = false
  exitSaved = false
}

export default { id: "ilnamiqui", server: plugin }
