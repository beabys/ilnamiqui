#!/bin/bash
set -euo pipefail
# Save recent conversation context before compaction.
# Reads transcript_path from hook JSON stdin, extracts last ~20 user messages,
# builds a summary with file paths and decisions (mirrors opencode plugin's buildSummary).

TRANSCRIPT=$(jq -r '.transcript_path // empty' 2>/dev/null) || exit 0
if [ -z "$TRANSCRIPT" ] || [ ! -f "$TRANSCRIPT" ]; then
  exit 0
fi

# Read last 60 lines (covers ~20 user messages + their responses)
RECENT=$(tail -n 60 "$TRANSCRIPT" 2>/dev/null) || exit 0
if [ -z "$RECENT" ]; then
  exit 0
fi

# Extract user message content (JSONL format: {"role":"user","content":"..."})
USER_MSGS=$(echo "$RECENT" | jq -r 'select(.role=="user") | .content // empty' 2>/dev/null) || true
if [ -z "$USER_MSGS" ]; then
  exit 0
fi

ENTRY_COUNT=$(echo "$USER_MSGS" | wc -l | tr -d ' ')

# Last user message as task description (first 200 chars)
TASK=$(echo "$USER_MSGS" | tail -1 | tr -s ' ' | head -c 200)

# Extract file paths: pattern like dir/file.ext or dir/dir/file.ext
FILES=$(echo "$USER_MSGS" | grep -oE '([a-zA-Z0-9._-]+/)+[a-zA-Z0-9._-]+\.[a-z]+' 2>/dev/null | sort -u | tr '\n' ', ' | sed 's/, $//') || true
if [ -z "$FILES" ]; then
  FILES="(none)"
fi

# Extract decision lines: lines containing use/chose/implement/refactor/migrate/fix:/add:
DECISIONS=$(echo "$USER_MSGS" | grep -iE '(use |chose |implement|refactor|migrate|fix:|add:)' 2>/dev/null | head -10 | while IFS= read -r line; do
  trimmed=$(echo "$line" | xargs | head -c 120)
  echo "$trimmed"
done | sort -u | tr '\n' '; ' | sed 's/; $//') || true
if [ -z "$DECISIONS" ]; then
  DECISIONS="(none)"
fi

TS=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

SUMMARY="session: $TASK
state: in-progress
files: $FILES
decisions: $DECISIONS
last_turn: $TS
entry_count: $ENTRY_COUNT"

ilnamiqui save "compact" "$SUMMARY" 2>/dev/null || true
exit 0
