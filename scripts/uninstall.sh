#!/bin/bash
set -euo pipefail

# ilnamiqui uninstaller
# Removes all ilnamiqui components for both opencode and Claude Code.
# Usage: bash scripts/uninstall.sh [--dry-run] [--quiet] [--help]

# ─── Paths (must match install.sh) ─────────────────────────────────────────────
OPENCODE_SKILL_DIR="${HOME}/.config/opencode/skills/ilnamiqui"
OPENCODE_PLUGIN_PATH="${HOME}/.config/opencode/plugins/ilnamiqui.ts"
OPENCODE_CONFIG="${HOME}/.config/opencode/opencode.json"
OPENCODE_BIN_DIR="${HOME}/.config/opencode/plugins/ilnamiqui"
CLAUDE_CONFIG="${HOME}/.claude/claude.json"
CLAUDE_SKILL_DIR="${HOME}/.claude/skills/ilnamiqui"
CLAUDE_BIN_DIR="${HOME}/.claude/plugins/ilnamiqui"
CLAUDE_HOOKS_DIR="${HOME}/.claude/hooks/ilnamiqui"
CLAUDE_SETTINGS="${HOME}/.claude/settings.json"
CLAUDE_HOME="${HOME}/.claude/CLAUDE.md"
SYMLINK_PATH="${HOME}/.local/bin/ilnamiqui"

# ─── Flags ─────────────────────────────────────────────────────────────────────
DRY_RUN=false
QUIET=false

# ─── Helpers (POSIX-safe) ──────────────────────────────────────────────────────
usage() {
  echo "Usage: $0 [--dry-run] [--quiet] [--help]"
  echo ""
  echo "Remove all ilnamiqui components (opencode + Claude Code)."
  echo ""
  echo "Flags:"
  echo "  --dry-run   Print what would be deleted, don't actually delete"
  echo "  --quiet     Suppress info messages, only show errors"
  echo "  --help      Print this usage"
  exit 0
}

info() {
  if [ "$QUIET" = false ]; then
    echo "  $1"
  fi
}

action() {
  if [ "$QUIET" = false ]; then
    echo "  → $1"
  fi
}

error() {
  echo "  ERROR: $1" >&2
}

remove_dir() {
  local path="$1" label="$2"
  if [ -d "$path" ]; then
    action "Removing ${label}: ${path}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would run: rm -rf ${path}"
    else
      rm -rf "$path"
      info "Removed ${path}"
    fi
  else
    info "${label} not found, skipping: ${path}"
  fi
}

remove_file() {
  local path="$1" label="$2"
  if [ -f "$path" ] || [ -L "$path" ]; then
    action "Removing ${label}: ${path}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would run: rm -f ${path}"
    else
      rm -f "$path"
      info "Removed ${path}"
    fi
  else
    info "${label} not found, skipping: ${path}"
  fi
}

# ─── Parse flags ───────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --quiet)
      QUIET=true
      shift
      ;;
    --help)
      usage
      ;;
    *)
      echo "Unknown flag: $1"
      usage
      ;;
  esac
done

# ─── Main ──────────────────────────────────────────────────────────────────────
echo ""
echo "  ilnamiqui uninstaller"
echo "  Removing all components (opencode + Claude Code)"
echo ""

# ── Opencode components ────────────────────────────────────────────────────────
echo "  ── opencode ──"
remove_dir "$OPENCODE_SKILL_DIR" "skill directory"
remove_file "$OPENCODE_PLUGIN_PATH" "plugin file"
remove_dir "$OPENCODE_BIN_DIR" "binary directory"

# Remove plugin + MCP entries from opencode.json
if [ -f "$OPENCODE_CONFIG" ]; then
  action "Updating opencode.json..."
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would remove plugin + mcp entries from ${OPENCODE_CONFIG}"
  else
    PLUGIN_ENTRY="./plugins/ilnamiqui.ts"
    if command -v jq &>/dev/null; then
      tmp=$(mktemp /tmp/ilnamiqui-uninstall.XXXXXXXX)
      if jq --arg p "$PLUGIN_ENTRY" '
        if .plugin then .plugin -= [$p] | if .plugin == [] then del(.plugin) else . end else . end |
        del(.mcp["ilnamiqui"]) | if .mcp == {} then del(.mcp) else . end
      ' "$OPENCODE_CONFIG" > "$tmp" 2>/dev/null; then
        mv "$tmp" "$OPENCODE_CONFIG"
        info "Updated opencode.json using jq"
      else
        rm -f "$tmp"
        error "jq failed to parse ${OPENCODE_CONFIG}"
      fi
    elif command -v python3 &>/dev/null; then
      python3 -c "
import json
file = '${OPENCODE_CONFIG}'
try:
    with open(file) as f:
        data = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    exit(0)
entry = '${PLUGIN_ENTRY}'
if 'plugin' in data and isinstance(data['plugin'], list):
    data['plugin'] = [p for p in data['plugin'] if p != entry]
    if not data['plugin']:
        del data['plugin']
if 'mcp' in data and 'ilnamiqui' in data['mcp']:
    del data['mcp']['ilnamiqui']
    if not data['mcp']:
        del data['mcp']
with open(file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" 2>/dev/null || error "python3 script failed"
      info "Updated opencode.json using python3"
    else
      error "Neither jq nor python3 found. Update ${OPENCODE_CONFIG} manually"
    fi
  fi
else
  info "opencode.json not found, skipping: ${OPENCODE_CONFIG}"
fi

# ── Claude Code components ─────────────────────────────────────────────────────
echo ""
echo "  ── Claude Code ──"
remove_dir "$CLAUDE_SKILL_DIR" "skill directory"
remove_dir "$CLAUDE_BIN_DIR" "binary directory"
remove_dir "$CLAUDE_HOOKS_DIR" "hooks directory"

# Remove MCP entry from claude.json
if [ -f "$CLAUDE_CONFIG" ]; then
  action "Updating claude.json..."
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would remove mcpServers.ilnamiqui from ${CLAUDE_CONFIG}"
  else
    if command -v jq &>/dev/null; then
      tmp=$(mktemp /tmp/ilnamiqui-uninstall.XXXXXXXX)
      if jq 'del(.mcpServers["ilnamiqui"]) | if .mcpServers == {} then del(.mcpServers) else . end' \
        "$CLAUDE_CONFIG" > "$tmp" 2>/dev/null; then
        mv "$tmp" "$CLAUDE_CONFIG"
        info "Updated claude.json using jq"
      else
        rm -f "$tmp"
        error "jq failed to parse ${CLAUDE_CONFIG}"
      fi
    elif command -v python3 &>/dev/null; then
      python3 -c "
import json
file = '${CLAUDE_CONFIG}'
try:
    with open(file) as f:
        data = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    exit(0)
if 'mcpServers' in data and 'ilnamiqui' in data['mcpServers']:
    del data['mcpServers']['ilnamiqui']
    if not data['mcpServers']:
        del data['mcpServers']
with open(file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" 2>/dev/null || error "python3 script failed"
      info "Updated claude.json using python3"
    else
      error "Neither jq nor python3 found. Update ${CLAUDE_CONFIG} manually"
    fi
  fi
else
  info "claude.json not found, skipping: ${CLAUDE_CONFIG}"
fi

# Remove ilnamiqui hooks from settings.json
if [ -f "$CLAUDE_SETTINGS" ]; then
  action "Updating settings.json..."
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would remove ilnamiqui hooks from ${CLAUDE_SETTINGS}"
  else
    if command -v jq &>/dev/null; then
      tmp=$(mktemp /tmp/ilnamiqui-uninstall.XXXXXXXX)
      if jq '
        if .hooks then
          .hooks["SessionStart"] |= [.[] | select(.hooks[0].command | test("ilnamiqui") | not)] |
          if .hooks["SessionStart"] == [] then del(.hooks["SessionStart"]) else . end |
          .hooks["PreCompact"] |= [.[] | select(.hooks[0].command | test("ilnamiqui") | not)] |
          if .hooks["PreCompact"] == [] then del(.hooks["PreCompact"]) else . end |
          .hooks["PostCompact"] |= [.[] | select(.hooks[0].command | test("ilnamiqui") | not)] |
          if .hooks["PostCompact"] == [] then del(.hooks["PostCompact"]) else . end |
          .hooks["SessionEnd"] |= [.[] | select(.hooks[0].command | test("ilnamiqui") | not)] |
          if .hooks["SessionEnd"] == [] then del(.hooks["SessionEnd"]) else . end |
          if .hooks == {} then del(.hooks) else . end
        else
          .
        end
      ' "$CLAUDE_SETTINGS" > "$tmp" 2>/dev/null; then
        mv "$tmp" "$CLAUDE_SETTINGS"
        info "Updated settings.json using jq"
      else
        rm -f "$tmp"
        error "jq failed to parse ${CLAUDE_SETTINGS}"
      fi
    elif command -v python3 &>/dev/null; then
      python3 -c "
import json
file = '${CLAUDE_SETTINGS}'
try:
    with open(file) as f:
        data = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    exit(0)
if 'hooks' in data:
    for event in list(data['hooks'].keys()):
        if isinstance(data['hooks'][event], list):
            data['hooks'][event] = [g for g in data['hooks'][event] if not any(
                isinstance(h, dict) and 'ilnamiqui' in h.get('command', '') for h in g.get('hooks', [])
            )]
            if not data['hooks'][event]:
                del data['hooks'][event]
    if not data['hooks']:
        del data['hooks']
with open(file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" 2>/dev/null || error "python3 script failed"
      info "Updated settings.json using python3"
    else
      error "Neither jq nor python3 found. Update ${CLAUDE_SETTINGS} manually"
    fi
  fi
else
  info "settings.json not found, skipping: ${CLAUDE_SETTINGS}"
fi

# Remove skill reference from CLAUDE.md
INCLUDE_LINE="@skills/ilnamiqui/SKILL.md"
if [ -f "$CLAUDE_HOME" ]; then
  action "Updating CLAUDE.md..."
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would remove '${INCLUDE_LINE}' from ${CLAUDE_HOME}"
  else
    if grep -Fxq "$INCLUDE_LINE" "$CLAUDE_HOME" 2>/dev/null; then
      tmp=$(mktemp /tmp/ilnamiqui.XXXXXXXX)
      grep -Fxv "$INCLUDE_LINE" "$CLAUDE_HOME" > "$tmp" 2>/dev/null || true
      if [ -s "$tmp" ]; then
        mv "$tmp" "$CLAUDE_HOME"
        info "Removed skill reference from CLAUDE.md"
      else
        mv "$tmp" "$CLAUDE_HOME"
        rm -f "$CLAUDE_HOME"
        info "Removed empty CLAUDE.md"
      fi
    else
      info "Skill reference not found in CLAUDE.md, skipping"
    fi
  fi
else
  info "CLAUDE.md not found, skipping"
fi

# ── Shared components (symlink) ────────────────────────────────────────────────
echo ""
echo "  ── shared ──"
remove_file "$SYMLINK_PATH" "CLI symlink"

# ── Done ───────────────────────────────────────────────────────────────────────
echo ""
echo "  ✓ ilnamiqui uninstalled"
echo "  Restart opencode or Claude Code to complete."
echo ""

exit 0
