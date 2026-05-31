---
name: ilnamiqui
description: Session memory backed by SQLite for opencode. Load when session starts, save at exit, or query mid-conversation. Use when user says "load session", "restore context", "resume", "/exit", or "save session".
license: MIT
compatibility: opencode
metadata:
  tags: ["session", "memory", "context", "persistence", "sqlite"]
  version: "1.0.0"
---

# ilnamiqui — Session Memory for opencode

**What:** Persists project context (decisions, bugs, architecture, notes)
between opencode chats. Data stays in a local SQLite file per project
(`.opencode/ilnamiqui.db`).

**When:** Use at chat start (load past context), during conversation (save
new info), and on `/exit` (auto-saved).

**How:** Three layers working together — binary (CLI commands), plugin
(auto-load/auto-save hooks), and this skill (teaches you when/how to
use the commands).

---

## Setup

If the user hasn't initialized ilnamiqui in their project:

```bash
ilnamiqui init
```

Then prompt: *"ilnamiqui initialized. Start saving with `ilnamiqui save <key> <value>`"*

---

## On chat start

Plugin auto-loads context. If it didn't, or you need a refresh:

```bash
ilnamiqui load --pretty
```

If entries exist, summarize: *"Previous session found — loaded context from <date>"*
Otherwise skip.

---

## During conversation

Save whenever notable context appears:

| Trigger | Command |
|---|---|
| Architecture decision | `ilnamiqui save "architecture" "<decision>"` |
| Bug found + fix | `ilnamiqui save "bug" "<root cause and fix>"` |
| Blocked / incomplete work | `ilnamiqui save "blocked" "<what was tried>"` |
| File created | `ilnamiqui save "file" "<path> — <what it does>"` |
| Config change | `ilnamiqui save "config" "<what changed>"` |
| External dependency | `ilnamiqui save "dependency" "<package/tool>"` |
| General note | `ilnamiqui save "<key>" "<value>"` |

Keep values terse (one sentence). Same key appends.

---

## Queries

```bash
ilnamiqui list --limit 5 --pretty     # recent sessions
ilnamiqui search "<query>" --pretty   # search memories
ilnamiqui load --pretty               # refresh all context
```

---

## On `/exit`

Auto-saved by the plugin. Manual fallback:

```bash
ilnamiqui session end --summary "<session summary>"
```
