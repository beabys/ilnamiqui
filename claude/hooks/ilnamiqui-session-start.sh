#!/bin/bash
set -euo pipefail
# Load past memories and inject as conversation context
CONTENT=$(ilnamiqui load --limit 10 --pretty 2>/dev/null) || exit 0
if [ -n "$CONTENT" ] && [ "$CONTENT" != "no entries found" ]; then
  echo "Previous session memory (ilnamiqui):"
  echo "$CONTENT"
fi
exit 0
