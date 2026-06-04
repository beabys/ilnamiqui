#!/bin/bash
set -euo pipefail
# Load memories after compaction completes.
# Mirrors opencode plugin behavior: refresh memories after compaction.
# Note: stdout not injected as context in Claude Code PostCompact,
# but data is available for next SessionStart.

ilnamiqui load --limit 10 --pretty 2>/dev/null || true
exit 0
