#!/bin/bash
set -euo pipefail
# Start claude-code session so saves target correct agent
ilnamiqui session start --agent claude-code 2>/dev/null || true
# Load past memories and return as additionalContext (official SessionStart JSON output)
CONTENT=$(ilnamiqui load --limit 10 --pretty 2>/dev/null) || exit 0
if [ -n "$CONTENT" ] && [ "$CONTENT" != "no entries found" ]; then
  jq -n --arg ctx "Previous session memory (ilnamiqui):
$CONTENT" '{
    hookSpecificOutput: {
      hookEventName: "SessionStart",
      additionalContext: $ctx
    }
  }'
fi
exit 0
