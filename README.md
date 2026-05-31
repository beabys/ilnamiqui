<p align="center">
  <img src="icon.svg" alt="ilnamiqui" width="128" height="128">
</p>

<h1 align="center">ilnamiqui</h1>

<p align="center">
  <em>From Nahuatl, an ancient Mexican language: "to remember or recall"</em><br>
  <strong>Persistent session memory for opencode.</strong>
</p>

<p align="center">
  Your memory, stored where you can see it. Locally.<br>
  Every decision, every bug fix, every architecture choice stays alive between sessions.
</p>

---

**ilnamiqui** gives your AI companion persistent memory across opencode chats. Everything is stored in a local SQLite database inside `.opencode/` — you own it, you control it. The binary itself makes zero network calls, has no telemetry, and requires no accounts.  
Each project gets its own isolated database — no cross-project leakage.

Stop losing context. Stop repeating yourself. Just remember.

---

## Installation

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash
```

Restart opencode. Done.

### Windows

Same one-liner works in **Git Bash** or **WSL** (the installer auto-detects Windows and handles `.zip` archives):

```bash
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash
```

Restart opencode. Done.

Alternatively:

- **Go install** (requires Go 1.26+):
  ```bash
  go install github.com/beabys/ilnamiqui/cmd/ilnamiqui@latest
  ```

- **Download manually** from [GitHub Releases](https://github.com/beabys/ilnamiqui/releases) — grab the `.zip` for Windows, extract, and place `ilnamiqui.exe` somewhere in your `PATH`.

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/uninstall.sh | bash
```

---

## opencode Integration

ilnamiqui is a **3-layer** system that wires together automatically:

| Layer | Role |
|-------|------|
| **Binary** — Go CLI (`ilnamiqui`) | Manages SQLite database, exposes commands |
| **Plugin** — `ilnamiqui.ts` | Hooks into opencode lifecycle: auto-loads context on session start, auto-saves on `/exit` |
| **Skill** — `SKILL.md` | Teaches the AI mid-conversation how to use `ilnamiqui save`, `search`, etc. |

The install script:

1. Downloads the binary to `~/.config/opencode/plugins/ilnamiqui/`
2. Installs the plugin to `~/.config/opencode/plugins/ilnamiqui.ts`
3. Installs the skill to `~/.config/opencode/skills/ilnamiqui/SKILL.md`
4. Auto-registers the plugin in `~/.config/opencode/opencode.json` (`"plugin": ["./plugins/ilnamiqui.ts"]`)

**What you need to do**: one command per project.

```bash
cd your-project
ilnamiqui init
```

That's it. Every new opencode chat in that project automatically loads past context. The AI knows how to save and recall memories thanks to the skill.

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
ilnamiqui search "middleware"
```

Every new opencode chat picks up right where the last one left off — automatically. The plugin loads context on start; the skill lets the AI save new memories mid-conversation without you lifting a finger.

---

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create `.opencode/ilnamiqui.db` and run migrations |
| `save <key> <value>` | Save a memory entry for the active session |
| `load [--session] [--limit N] [--pretty]` | Load memory entries (all or current session) |
| `list [--limit N] [--pretty]` | List recent sessions |
| `search <query> [--limit N] [--pretty]` | Search memories by key or value |
| `delete <id>` | Delete a memory entry by ID |
| `session start` | Start a new session manually |
| `session end --summary "..."` | End the active session with a summary |
| `version` | Print CLI version |
| `help` | Show usage |

Commands `save`, `load`, `list`, and `search` accept `--pretty` for human-readable tables instead of JSON output.

---

## Privacy

**Your memory is stored locally — in a file you control.** Here's what that means in practice:

- **Local storage.** Every memory lives in `.opencode/ilnamiqui.db` inside your project. No cloud, no servers — just a file on your machine.
- **ilnamiqui makes zero network calls.** The binary never phones home, sends no analytics, and has no telemetry.
- **You see everything.** The database is a plain SQLite file. Open it with any SQLite browser, inspect it, edit it, delete it — it's yours.
- **No accounts.** No sign-up, no API keys, no user management. It just works.
- **Per-project isolation.** Each project has its own database. What happens in one project stays there.
- **What about the AI?** When opencode loads memories into a chat, that context is included in the conversation sent to whichever LLM provider you have configured (OpenAI, Anthropic, etc.). This is standard for any AI coding tool — ilnamiqui simply brings your past context along. You control what gets saved, and you can delete anything at any time.

---

## License

MIT
