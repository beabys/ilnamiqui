<p align="center">
  <img src="icon.svg" alt="ilnamiqui" width="128" height="128">
</p>

<h1 align="center">ilnamiqui</h1>

<p align="center">
  <em>From Nahuatl, an ancient Mexican language: "to remember or recall"</em><br>
  <strong>Persistent session memory for opencode &amp; Claude Code.</strong>
</p>

<p align="center">
  Your memory, stored where you can see it. Locally.<br>
  Every decision, every bug fix, every architecture choice stays alive between sessions.
</p>

---

**ilnamiqui** gives your AI companion persistent memory across chats. Everything is stored in a local SQLite database inside `.opencode/` — you own it, you control it. The binary itself makes zero network calls, has no telemetry, and requires no accounts.  
Each project gets its own isolated database — no cross-project leakage.

Stop losing context. Stop repeating yourself. Just remember.

---

## Architecture

ilnamiqui is a hexagonal system with adapters for different AI assistants:

```
┌─────────────────────────────────────────────────────┐
│                   ilnamiqui CLI                      │
│             (shared binary — ilnamiqui)              │
│         SQLite session memory in .opencode/          │
└────────────────────┬────────────────────────────────┘
                     │
          ┌──────────┴──────────┐
          ▼                     ▼
┌──────────────────┐  ┌──────────────────────┐
│   opencode       │  │    Claude Code        │
│   adapter        │  │    adapter            │
├──────────────────┤  ├──────────────────────┤
│ Plugin (TS)      │  │ MCP Server (Go)       │
│   → lifecycle    │  │   → 6 tools via      │
│     hooks        │  │     stdio transport   │
│ Skill (SKILL.md) │  │ Skill (CLAUDE.md)     │
│   → teaches AI   │  │   → teaches AI to    │
│     commands     │  │     call MCP tools    │
└──────────────────┘  └──────────────────────┘
```

| Layer | opencode | Claude Code |
|-------|----------|-------------|
| **Binary** | `ilnamiqui` CLI | `ilnamiqui` CLI + `ilnamiqui-mcp` MCP server |
| **Integration** | TypeScript plugin (lifecycle hooks) | MCP stdio server (6 tools) |
| **AI instructions** | `opencode/skill/SKILL.md` | `claude/skill/CLAUDE.md` |
| **Config** | `~/.config/opencode/opencode.json` | `~/.claude/claude.json` |

---

## Installation

### opencode Installation

```bash
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --target opencode
```

Restart opencode. Done.

The installer:
1. Downloads the binary to `~/.config/opencode/plugins/ilnamiqui/`
2. Installs the plugin to `~/.config/opencode/plugins/ilnamiqui.ts`
3. Installs the skill to `~/.config/opencode/skills/ilnamiqui/SKILL.md`
4. Auto-registers the plugin in `~/.config/opencode/opencode.json`

### Claude Code Installation

```bash
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --target claude
```

Restart Claude Code. Done.

The installer:
1. Downloads both binaries to `~/.config/opencode/plugins/ilnamiqui/`
2. Installs the skill to `~/.claude/skills/ilnamiqui/CLAUDE.md`
3. Auto-registers the MCP server in `~/.claude/claude.json`
4. Creates symlink at `~/.local/bin/ilnamiqui` for CLI access

### Interactive Install

When run in a terminal without `--target`, the installer prompts:

```
Which AI assistant?
  1) opencode (default)
  2) Claude Code
```

### Windows

Same one-liners work in **Git Bash** or **WSL** (the installer auto-detects Windows and handles `.zip` archives).

Alternatively:
- **Go install** (requires Go 1.26+):
  ```bash
  go install github.com/beabys/ilnamiqui/cmd/cli@latest
  go install github.com/beabys/ilnamiqui/cmd/mcp@latest
  ```

- **Download manually** from [GitHub Releases](https://github.com/beabys/ilnamiqui/releases) — grab the `.zip` for Windows, extract, and place both binaries somewhere in your `PATH`.

### Uninstall

```bash
# Remove everything
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/uninstall.sh | bash

# Remove only opencode integration
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/uninstall.sh | bash -s -- --target opencode

# Remove only Claude Code integration
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/uninstall.sh | bash -s -- --target claude
```

---

## Quick Start

```bash
# 1. One-time project setup
cd your-project
ilnamiqui init

# 2. Save context during your chat
ilnamiqui save "architecture" "chose Go for single binary deployment"
ilnamiqui save "bug" "auth token expired — fixed middleware check"

# 3. Next session: recall everything
ilnamiqui load --pretty

# 4. Find specifics instantly
ilnamiqui search "middleware" --after 2026-01-01 --before 2026-06-01
```

Every new chat picks up right where the last one left off — automatically.

---

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create `.opencode/ilnamiqui.db` and run migrations |
| `save <key> <value>` | Save a memory entry for the active session |
| `load [--session] [--limit N] [--pretty]` | Load memory entries (all or current session) |
| `list [--limit N] [--pretty]` | List recent sessions |
| `search <query> [--after DATE] [--before DATE] [--limit N] [--pretty]` | Search memories by key, value, or date range |
| `delete <id>` | Delete a memory entry by ID |
| `session start` | Start a new session manually |
| `session end --summary "..."` | End the active session with a summary |
| `version` | Print CLI version |
| `help` | Show usage |

Commands `save`, `load`, `list`, and `search` accept `--pretty` for human-readable tables instead of JSON output.

---

## MCP Server (Claude Code)

The MCP server (`ilnamiqui-mcp`) provides 6 tools over stdio:

| Tool | Parameters | Description |
|------|-----------|-------------|
| `init_memory` | _(none)_ | Initialize the database in `.opencode/` |
| `save_memory` | `key` (required), `value` (required) | Save a memory entry |
| `load_memories` | `limit` (opt, default 50), `session_only` (opt) | Load memory entries |
| `search_memories` | `query` (opt), `after` (opt, RFC3339), `before` (opt, RFC3339), `limit` (opt, default 10) | Search by key, value, or date range |
| `list_sessions` | `limit` (opt, default 10) | List recent sessions |
| `delete_memory` | `id` (required) | Delete a memory entry |

The Claude Code skill (`claude/skill/CLAUDE.md`) instructs the AI to call these tools automatically.

---

## Privacy

**Your memory is stored locally — in a file you control.** Here's what that means in practice:

- **Local storage.** Every memory lives in `.opencode/ilnamiqui.db` inside your project. No cloud, no servers — just a file on your machine.
- **ilnamiqui makes zero network calls.** The binary never phones home, sends no analytics, and has no telemetry.
- **You see everything.** The database is a plain SQLite file. Open it with any SQLite browser, inspect it, edit it, delete it — it's yours.
- **No accounts.** No sign-up, no API keys, no user management. It just works.
- **Per-project isolation.** Each project has its own database. What happens in one project stays there.
- **What about the AI?** When memories are loaded into a chat, that context is included in the conversation sent to whichever LLM provider you have configured (OpenAI, Anthropic, etc.). This is standard for any AI coding tool — ilnamiqui simply brings your past context along. You control what gets saved, and you can delete anything at any time.

---

## License

MIT
