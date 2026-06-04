#!/bin/bash
set -euo pipefail
# Save session end note with reason
REASON=$(jq -r '.reason // "unknown"' 2>/dev/null) || true
TS=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
ilnamiqui save --agent claude-code "session" "session ended at $TS (reason: $REASON)" 2>/dev/null || true
exit 0
