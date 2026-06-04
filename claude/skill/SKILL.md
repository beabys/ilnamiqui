---
description: Session memory for Claude Code — persists decisions, bugs, architecture, and context between chats using per-project SQLite. Use at chat start to load past context, during conversation to save decisions/bugs/notes, and at chat end to summarize.
---

# ilnamiqui — Session Memory for Claude Code

**What:** Persists project context (decisions, bugs, architecture, notes)
between Claude Code chats. Data stays in local SQLite file per project
(`.ilnamiqui/ilnamiqui.db`).

**How:** Three parts:
- **Binary** — `ilnamiqui` CLI (manages SQLite)
- **MCP Server** — `ilnamiqui-mcp` (tools for Claude Code)
- **This file** — tells you when/how to use the tools

## Agent identity

Every session is tagged with an agent identifier (`opencode` or `claude-code`).
The lifecycle hooks pass `--agent claude-code` automatically on save/start/end.
If you run CLI commands manually, include the flag:

```bash
ilnamiqui save --agent claude-code "key" "value"
ilnamiqui session start --agent claude-code
ilnamiqui session end --agent claude-code --summary "done"
```

Omitting `--agent` defaults to `opencode` — always specify `--agent claude-code`
when running CLI commands manually.

---

## Setup

One-time per project:
```bash
ilnamiqui init
```

MCP tools are auto-registered — no manual config needed after install.

---

## On chat start

**Always call `load_memories` tool** to load past context:
```
load_memories(limit=50)
```
If entries exist, summarize: *"Previous session found — loaded context from <date>"*
Otherwise skip.

---

## Before you save — discover existing keys

Call `list_keys` first to see which keys already exist in this project.
Reuse existing keys so related context stays grouped together:

```
list_keys(limit=50)
```

Example response:
```
Key             Critical  Last Used
project-path    true      2026-06-04T10:30:00Z
architecture    false     2026-06-04T10:30:00Z
bug             false     2026-06-03T15:20:00Z
```

`project-path` stores the project's absolute root path. Before opening files
outside the project directory, verify context hasn't drifted:
```
search_memories(query="project-path")
```

## During conversation

Save whenever notable context appears:

| Trigger | Tool call |
|---|---|
| Architecture decision | `save_memory(key="architecture", value="<decision>")` |
| Bug found + fix | `save_memory(key="bug", value="<root cause and fix>")` |
| Blocked / incomplete work | `save_memory(key="blocked", value="<what was tried>")` |
| File created | `save_memory(key="file", value="<path> — <what it does>")` |
| Config change | `save_memory(key="config", value="<what changed>")` |
| External dependency | `save_memory(key="dependency", value="<package/tool>")` |
| General note | `save_memory(key="<key>", value="<value>")` |

Keep values terse (one sentence). Same key appends.

---

## Queries

```bash
# Via MCP tools (preferred):
list_keys(limit=50)                                                       # discover existing keys (+ critical flag)
search_memories(query="<query>")                                          # search keys only (fast, uses index)
search_memories(query="<query>", mode="content")                          # search content (FTS5 full-text)
search_memories(query="<query>", mode="both")                             # search both keys and content
search_memories(after="2026-01-01T00:00:00Z")                              # search by date range
search_memories(query="<query>", mode="content", after="2026-01-01")       # combined
list_sessions(limit=5)

# Via CLI (fallback):
ilnamiqui keys --limit 10 --pretty
ilnamiqui list --limit 5 --pretty
ilnamiqui search "<query>" --pretty                         # search keys only (default)
ilnamiqui search "<query>" --mode content --pretty          # search content (FTS5)
ilnamiqui search --after 2026-01-01 --before 2026-06-01     # date range only
ilnamiqui load --pretty
```

**Search behavior:**
- **Default** (`mode="key"`): searches **keys** using prefix match — fast, uses B-tree index.
- **`mode="content"`**: searches **content/value** using FTS5 full-text search (token-aware, supports prefix) — use when key search isn't enough.
- **`mode="both"`**: searches both keys AND content.
- **FTS5 tip**: content search is word-based. `search_memories(query="hex", mode="content")` matches entries with words starting with "hex" (hexagonal, hexagon). NOT substring LIKE — `search_memories(query="lago", mode="content")` will NOT match "hexagonal".

---

---

## Prune old memories

Remove old non-critical memories to keep the database lean:

```bash
# Via CLI:
ilnamiqui prune --before 2026-04-01                  # all non-critical keys
ilnamiqui prune --before 2026-04-01 --key test        # specific key only

# Via MCP tool:
prune_memories(before="2026-01-01T00:00:00Z")
prune_memories(before="2026-01-01T00:00:00Z", key="test")
```

**Behavior:**
- Only deletes entries whose key is **not critical** (`critical = false` in `memory_keys`)
- Critical keys (like `project-path`) are **never deleted** by prune — no matter how old
- After deletion, orphaned `memory_keys` rows are auto-cleaned
- `before` is required (RFC3339 format for MCP, YYYY-MM-DD or RFC3339 for CLI)
- `key` is optional — defaults to `*` (all non-critical keys)
- If no matches, returns `Pruned 0 entries` — no error

---

## Protect keys from prune

Mark a key as **critical** so prune never touches it:

```bash
# Via CLI:
ilnamiqui key update --critical architecture          # protect from prune
ilnamiqui key update --critical=false architecture    # allow prune again

# Via MCP tool:
update_key(key="architecture", critical=true)
update_key(key="architecture", critical=false)
```

**Behavior:**
- Updates the `critical` flag on an existing key
- Critical keys are completely excluded from prune
- If key doesn't exist, returns an error
- Use `list_keys` to see current flag status

**Common workflow:**
```bash
list_keys()                                            # check which keys are critical
update_key(key="architecture", critical=true)           # protect architecture decisions
prune_memories(before="2026-01-01T00:00:00Z")           # safe — skips critical keys
```

---

## On chat end

Save session summary before finishing:

Via MCP tool (preferred):
```
save_memory(key="session", value="<brief summary of what was done>")
```

Via CLI (fallback — always include agent):
```bash
ilnamiqui save --agent claude-code "session" "<brief summary of what was done>"
```
