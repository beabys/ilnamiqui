---
name: ilnamiqui
description: Session memory backed by SQLite for opencode. Load when session starts, save at exit, or query mid-conversation. Use when user says "load session", "restore context", "resume", "/exit", or "save session".
license: MIT
compatibility: opencode
metadata:
  tags: ["session", "memory", "context", "persistence", "sqlite"]
  version: "0.1.0"
---

# ilnamiqui — SQLite Session Memory for opencode

Preserves project context between opencode chats using a per-project SQLite
database. Patterns, decisions, bugs, and file changes persist across sessions.

## Architecture

```
opencode session
├── plugin/ilnamiqui.ts   ← auto-hooks (load on start, save on exit)
├── skill/SKILL.md         ← this file — instructs AI for ad-hoc queries
└── ilnamiqui CLI binary   ← single source of truth
    └── .opencode/ilnamiqui.db  ← per-project SQLite (WAL mode)
```

## First-time setup

Run this command in the project root to create the database:

```bash
ilnamiqui init
```

This creates `.opencode/ilnamiqui.db` with tables for sessions and memories.

After initialization, tell the user:
> "ilnamiqui initialized. Start by saving memories with `ilnamiqui save <key> <value>`"

## At chat start — load previous context

The plugin automatically loads context when opencode starts. If the plugin
is not yet active (e.g., manual session), run:

```bash
ilnamiqui load --pretty
```

Present a summary:
> "Previous session found — loaded context from <date>"

The AI now has full context of decisions, bugs, architecture, and pending
work from previous sessions.

If no sessions exist yet, skip the summary.

## During task execution — save important updates immediately

Use `ilnamiqui save` whenever notable context is discovered:

| Trigger | Command |
|---|---|
| Architecture decision | `ilnamiqui save "architecture" "<decision and rationale>"` |
| Bug found + fix | `ilnamiqui save "bug" "<root cause and fix>"` |
| Blocked / incomplete work | `ilnamiqui save "blocked" "<what was tried, what failed>"` |
| File created | `ilnamiqui save "file" "<path> — <what it does>"` |
| Config change | `ilnamiqui save "config" "<what changed>"` |
| External dependency | `ilnamiqui save "dependency" "<package/tool/service>"` |
| General note | `ilnamiqui save "<key>" "<value>"` |

Keep values terse — one sentence per entry. Multiple entries for the same
key append (not overwrite).

## Mid-conversation queries

When context feels thin or you need to recall what happened:

```bash
ilnamiqui list --limit 5 --pretty
```
Show recent memory entries.

```bash
ilnamiqui search "<query>" --pretty
```
Search memories by key or value text.

```bash
ilnamiqui load --pretty
```
Refresh full context.

## At `/exit` or "save session"

The plugin automatically saves on `/exit`. If manual:

```bash
ilnamiqui session end --summary "<summary of session>"
```

This persists the session with a summary for future context restoration.

## Tips

- Save before opening a new chat — `ilnamiqui load` picks up everything
- Use descriptive keys: `architecture`, `bug`, `blocked`, `decision`, `note`
- Include exact CLI commands in memory values so next session picks up
  without guessing
- Re-run `ilnamiqui load --pretty` proactively when context feels thin
- Each project has its own `.opencode/ilnamiqui.db` — no cross-project
  leakage
