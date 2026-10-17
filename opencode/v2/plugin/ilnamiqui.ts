import { Plugin } from "@opencode/plugin"
import { execFile } from "child_process"
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

// Interaction counter — tracks user messages + subagent task calls
// Separate from conversationBuffer (which FIFOs user text for summaries)
const MAX_INTERACTIONS = 50
let interactionCounter = 0

const REMINDER_TEXT = "REMINDER: Read the rules in AGENTS.md before continuing with your task(s)."

/**
 * Format critical reminder values for injection into tool output.
 * Joins multiple values with " and " separator.
 * Falls back to REMINDER_TEXT when no values provided.
 */
function formatReminder(values: string[]): string {
  if (values.length === 0) {
    return REMINDER_TEXT
  }
  return values.join(" and ")
}

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
// Process helper — replaces the V1 Bun `$` shell helper.
// Runs the binary directly (no shell), args passed as an array so values with
// spaces/quotes are preserved verbatim.
// `cwd` is the plugin instance's location, so the ilnamiqui binary discovers
// the memory project for the session's own directory (not the server cwd).
// ---------------------------------------------------------------------------

interface RunResult {
  exitCode: number
  stdout: string
  stderr: string
}

function run(binary: string, args: string[], cwd: string): Promise<RunResult> {
  return new Promise((resolve) => {
    execFile(
      binary,
      args,
      { maxBuffer: 16 * 1024 * 1024, cwd },
      (error, stdout, stderr) => {
        const code = (error as { code?: unknown } | null)?.code
        const exitCode =
          typeof code === "number" ? code : error ? 1 : 0
        resolve({
          exitCode,
          stdout: String(stdout ?? ""),
          stderr: String(stderr ?? ""),
        })
      },
    )
  })
}

// ---------------------------------------------------------------------------
// Plugin definition (V2: default export = Plugin.define({ id, setup }))
// ---------------------------------------------------------------------------

export const ilnamiquiPlugin = Plugin.define({
  id: "ilnamiqui",
  async setup(ctx) {
    const binary = resolveBinarySync()

    if (!binary) {
      log("binary not found (PATH + plugin dir) — plugin disabled")
      return
    }

    log(`binary found: ${binary}`)

    // Bind every binary call to this plugin instance's location so the
    // ilnamiqui binary resolves the memory project for the session's directory.
    const cwd = ctx.location.directory
    const callBinary = (args: string[]) => run(binary, args, cwd)

    // ----- Auto-initialize on plugin load (app startup) -----
    // opencode does not fire session.created on app open, only on first interaction.
    // We start session + load context immediately so memory is available from first turn.
    // The sessionInitialized guard prevents the session.created handler from duplicating.
    if (!sessionInitialized) {
      log("plugin loaded — auto-initializing session")
      const startResult = await callBinary(["session", "start", "--agent", "opencode"])
      if (startResult.exitCode === 0) {
        sessionInitialized = true
        log("session started successfully")
      } else {
        log(`session start failed (exit ${startResult.exitCode}) — will rely on session.created fallback`)
        // sessionInitialized stays false → session.created handler can create session
      }
    }

    // ----- Save context + end session (shared by session.deleted and
    //       location.shutdown) -----
    const saveAndEnd = async () => {
      if (exitSaved) return
      exitSaved = true
      log("session.deleted / location.shutdown — saving context")
      const summary = buildSummary(conversationBuffer)
      await callBinary(["save", "--agent", "opencode", "session", summary])
      await callBinary(["session", "end", "--agent", "opencode", "--summary", summary])
    }

    // ----- Register chat prompt hook to buffer user messages -----
    // V1 "chat.message" → V2 ctx.session.hook("prompt", ...)
    await ctx.session.hook("prompt", (event) => {
      const text = event.prompt?.text
      if (text) {
        const entry: BufferEntry = {
          role: "user",
          text,
          timestamp: new Date().toISOString(),
        }
        conversationBuffer.push(entry)
        if (conversationBuffer.length > MAX_BUFFER) {
          conversationBuffer.shift()
        }
        // increase interaction counter for user messages as well
        interactionCounter++
      }
    })

    // ----- Register event subscription -----
    // V1 `event` hook → V2 ctx.event.subscribe() driven by an AbortController,
    // aborted from the cleanup function returned by setup.
    const controller = new AbortController()
    void (async () => {
      try {
        for await (const event of ctx.event.subscribe({ signal: controller.signal })) {
          try {
            // session.created — start opencode session + load previous context
            if (event.type === "session.created") {
              if (sessionInitialized) continue
              sessionInitialized = true
              log("session.created — starting session and loading context")
              await callBinary(["session", "start", "--agent", "opencode"])
              continue
            }

            // session.compaction.ended — V2 replacement for V1 `session.compacted`
            // save memory entry + reload after compaction
            if (event.type === "session.compaction.ended") {
              log("session.compaction.ended — saving entry and reloading memories")
              await callBinary(["save", "--agent", "opencode", "compact", `compaction completed at ${new Date().toISOString()}`])
              await callBinary(["load", "--limit", "50"])
              continue
            }

            // session.deleted — user typed /exit or session ended
            // location.shutdown — V2 replacement for V1 `server.instance.disposed`
            //                     (fires when opencode disposes the location)
            if (event.type === "session.deleted" || event.type === "location.shutdown") {
              await saveAndEnd()
              continue
            }
          } catch (e) {
            log(`event error: ${e}`)
          }
        }
      } catch (e) {
        // AbortError on cleanup is expected; anything else is worth logging
        if ((e as { name?: string } | null)?.name !== "AbortError") {
          log(`event stream error: ${e}`)
        }
      }
    })()

    // ----- Compaction hook -----
    // V1 "experimental.session.compacting" → V2 ctx.session.hook("compaction", ...)
    // Save memory entry + inject memory context into the compaction request.
    await ctx.session.hook("compaction", async (event) => {
      log("session.compacting — saving pre-compaction entry")
      const summary = buildSummary(conversationBuffer)
      await callBinary(["save", "--agent", "opencode", "compact", summary])
      log("session.compacting — injecting memory context")
      const result = await callBinary(["load", "--limit", "10", "--pretty"])
      if (result.exitCode === 0 && result.stdout) {
        event.system.push({
          type: "text",
          text: [
            "Recent session memory entries (ilnamiqui):",
            result.stdout.trim(),
          ].join("\n"),
        })
      }
    })

    // ----- Tool execution hook -----
    // V1 "tool.execute.after" → V2 ctx.tool.hook("execute.after", ...)
    // Track ALL tool calls for interaction counter + inject reminder at threshold.
    // At threshold, load ALL critical keys from memory (via ilnamiqui binary),
    // join their values with " and ", and inject into the tool result.
    // Falls back to REMINDER_TEXT constant if binary calls fail.
    await ctx.tool.hook("execute.after", async (event) => {
      interactionCounter++
      if (interactionCounter >= MAX_INTERACTIONS) {
        interactionCounter = 0

        // Dynamically load critical memory keys from DB
        let reminderText = REMINDER_TEXT
        try {
          // 1. Fetch all keys to find which are critical
          // NOTE: flags MUST come before value (Go flag package quirk)
          const keysResult = await callBinary(["keys"])
          if (keysResult.exitCode === 0 && keysResult.stdout) {
            const keys = JSON.parse(keysResult.stdout) as Array<{ key: string; critical: boolean; last_used_at: string }>
            const criticalKeys = keys.filter(k => k.critical === true)

            if (criticalKeys.length > 0) {
              const values: string[] = []
              for (const k of criticalKeys) {
                // Search for entries with this critical key (flag before value)
                const searchResult = await callBinary(["search", "--mode", "key", k.key])
                if (searchResult.exitCode === 0 && searchResult.stdout) {
                  const entries = JSON.parse(searchResult.stdout) as Array<{ value: string }>
                  if (entries.length > 0) {
                    // Take latest entry value per key (last in array = most recent)
                    values.push(entries[entries.length - 1].value)
                  }
                }
              }
              if (values.length > 0) {
                reminderText = formatReminder(values)
              }
            }
          }
        } catch (e) {
          log(`reminder: binary call failed, using fallback: ${e}`)
        }

        // V1 appended to `output.output`; V2 carries the model-visible text in
        // `event.result.content` (string or content parts). Tool.Result fields
        // are readonly, so replace the whole result object.
        if (event.status === "completed") {
          const suffix = "\n\n---\n" + reminderText
          const result = event.result
          if (typeof result.content === "string") {
            event.result = { ...result, content: result.content + suffix }
          } else if (Array.isArray(result.content)) {
            event.result = { ...result, content: [...result.content, { type: "text", text: suffix }] }
          } else {
            event.result = { ...result, content: suffix }
          }
        }
      }
    })

    // ----- endSession — AI-callable tool to proactively end a session -----
    // V1 `tool` map → V2 synchronous ctx.tool.transform editor.
    await ctx.tool.transform((editor) => {
      editor.add({
        name: "endSession",
        description: "Save session context and end the current ilnamiqui session. Call when the conversation task is complete, user says goodbye, or work is done.",
        input: {
          type: "object",
          properties: {
            summary: {
              type: "string",
              description: "One-line summary of what was accomplished this session",
            },
          },
          required: ["summary"],
          additionalProperties: false,
        },
        async execute(input) {
          const { summary: arg } = input as { summary: string }
          const now = new Date().toISOString()
          const summary = [
            `session: ${arg}`,
            "state: completed",
            `last_turn: ${now}`,
          ].join("\n")
          await callBinary(["save", "--agent", "opencode", "session", summary])
          await callBinary(["session", "end", "--agent", "opencode", "--summary", summary])
          exitSaved = true
          return { content: `Session saved and ended: ${arg}` }
        },
      })
    })

    // ----- Cleanup (V1 `dispose` hook) -----
    // The dispose-side work (save + end) is event-driven above, mirroring V1:
    // `session.deleted` on /exit and `location.shutdown` on location teardown
    // (V2 rename of V1 `server.instance.disposed`). Cleanup itself only stops
    // the event subscription, so a plugin reload does not end the session.
    return () => {
      controller.abort()
    }
  },
})

export { buildSummary, conversationBuffer, sessionInitialized, exitSaved, BufferEntry, resolveBinarySync, resolveBinaryPath, interactionCounter, MAX_INTERACTIONS, REMINDER_TEXT, formatReminder }

export function resetTestState(): void {
  conversationBuffer.length = 0
  sessionInitialized = false
  exitSaved = false
  interactionCounter = 0
}

export default ilnamiquiPlugin
