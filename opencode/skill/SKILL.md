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
(`.ilnamiqui/ilnamiqui.db`).

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

## Before you save — discover existing keys

Call `keys` first to see which keys already exist in this project.
Reuse existing keys so related context stays grouped together:

```bash
ilnamiqui keys --pretty
```

Example output:
```
Key             Critical  Last Used
project-path    true      2026-06-04T10:30:00Z
architecture    false     2026-06-04T10:30:00Z
bug             false     2026-06-03T15:20:00Z
```

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
ilnamiqui keys --limit 10 --pretty       # discover existing keys (+ critical flag)
ilnamiqui list --limit 5 --pretty        # recent sessions
ilnamiqui search "<query>" --pretty                  # search keys only (fast, uses index)
ilnamiqui search "<query>" --mode content --pretty   # search content (FTS5 full-text)
ilnamiqui search "<query>" --mode both --pretty       # search both keys and content
ilnamiqui search "<query>" --after 2026-01-01         # search by date range
ilnamiqui load --pretty                # refresh all context
```

**Search behavior:**
- **Default** (`--mode key`): searches **keys** using prefix match (`key LIKE "query%"`) — fast, uses B-tree index.
- **`--mode content`**: searches **content/value** using FTS5 full-text search (token-aware, supports prefix with `*`) — ideal when key search isn't enough.
- **`--mode both`**: searches both keys AND content.
- **FTS5 tip**: content search is word-based. `ilnamiqui search "hex" --mode content` matches entries containing words starting with "hex" (hexagonal, hexagon). It is NOT substring LIKE — `ilnamiqui search "lago" --mode content` will NOT match "hexagonal".

---

---

## Prune old memories

Remove old non-critical memories to keep the database lean:

```bash
ilnamiqui prune --before 2026-04-01                  # all non-critical keys
ilnamiqui prune --before 2026-04-01 --key test        # specific key only
```

**Behavior:**
- Only deletes entries whose key is **not critical** (`critical = false` in `memory_keys`)
- Critical keys (like `project-path`) are **never deleted** by prune
- After deletion, orphaned `memory_keys` rows are auto-cleaned
- `--before` is required (YYYY-MM-DD or RFC3339)
- `--key` is optional — defaults to `*` (all non-critical keys)
- If no matches, prints `0 deleted` — no error

**Example sequence:**
```bash
ilnamiqui keys --pretty                              # see which keys are critical
ilnamiqui prune --before 2026-04-01                  # delete old non-critical entries
ilnamiqui prune --before 2026-04-01 --key test       # delete only key "test"
```

---

## Protect keys from prune

Mark a key as **critical** so prune never touches it:

```bash
ilnamiqui key update --critical testkey              # protect from prune
ilnamiqui key update --critical=false testkey        # allow prune again
```

**Behavior:**
- Updates the `critical` flag on an existing key
- Critical keys are completely excluded from prune — no matter how old
- If key doesn't exist, returns an error
- Use `ilnamiqui keys --pretty` to see current flag status

**Common workflow:**
```bash
ilnamiqui keys --pretty                              # check which keys are critical
ilnamiqui key update --critical architecture         # protect architecture decisions
ilnamiqui prune --before 2026-04-01                  # safe — skips critical keys
```

---

## On `/exit`

Auto-saved by the plugin. Manual fallback:

```bash
ilnamiqui session end --summary "<session summary>"
```
