<p align="center">
  <img src="icon.svg" alt="ilnamiqui" width="128" height="128">
</p>

<h1 align="center">ilnamiqui</h1>

<p align="center">
  <em>Nahuatl: "to remember or recall"</em><br>
  <strong>Never lose context between opencode sessions again.</strong>
</p>

<p align="center">
  ilnamiqui gives your AI persistent memory across chats — every decision, every bug fix, every architecture choice stays alive between sessions. Per-project context that just works.
</p>

---

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash
```

Restart opencode. Done.

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/uninstall.sh | bash
```

Restart opencode. Done.

## Usage

```bash
# Start remembering in any project
cd your-project
ilnamiqui init

# Save what matters — decisions, bugs, architecture
ilnamiqui save "architecture" "chose Go for single binary deployment"
ilnamiqui save "bug" "auth token expired — fixed middleware check"

# Recall everything next session
ilnamiqui load --pretty

# Find specific memories instantly
ilnamiqui search "auth"

# See your session history
ilnamiqui list --limit 5 --pretty
```

That's it. Every new chat picks up right where the last one left off.

## How it works

| Layer | Job |
|---|---|
| **Binary** — Go CLI | One command per action. Fast, small, zero deps. |
| **Plugin** — TypeScript | Hooks into opencode — auto-saves on exit, auto-loads on start. |
| **Skill** — SKILL.md | Teaches the AI how to use ilnamiqui mid-conversation. |

Each project gets its own isolated context — no cross-project contamination, no surprises.

## Build from source

Requires Go 1.23+:

```bash
git clone https://github.com/beabys/ilnamiqui
cd ilnamiqui
make build
```

## License

MIT
