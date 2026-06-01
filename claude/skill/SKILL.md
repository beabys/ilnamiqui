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
search_memories(query="<query>")                          # search by text
search_memories(after="2026-01-01T00:00:00Z")              # search by date range
search_memories(query="<query>", after="2026-01-01T00:00:00Z", before="2026-06-01T00:00:00Z")
list_sessions(limit=5)

# Via CLI (fallback):
ilnamiqui list --limit 5 --pretty
ilnamiqui search "<query>" --pretty                          # CLI fallback
ilnamiqui search --after 2026-01-01 --before 2026-06-01      # date range only
ilnamiqui load --pretty
```

---

## On chat end

Save session summary before finishing:
```
save_memory(key="session", value="<brief summary of what was done>")
```
