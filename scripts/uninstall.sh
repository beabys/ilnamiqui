#!/bin/bash
set -euo pipefail

# ilnamiqui uninstaller
# Usage: bash scripts/uninstall.sh [--target opencode|claude|all] [--dry-run] [--quiet] [--help]

# ─── Paths (must match install.sh) ─────────────────────────────────────────────
BIN_DIR="${HOME}/.config/opencode/plugins/ilnamiqui"
OPENCODE_SKILL_DIR="${HOME}/.config/opencode/skills/ilnamiqui"
OPENCODE_PLUGIN_PATH="${HOME}/.config/opencode/plugins/ilnamiqui.ts"
OPENCODE_CONFIG="${HOME}/.config/opencode/opencode.json"
CLAUDE_CONFIG="${HOME}/.claude/claude.json"
CLAUDE_SKILL_DIR="${HOME}/.claude/skills/ilnamiqui"
SYMLINK_PATH="${HOME}/.local/bin/ilnamiqui"

# ─── Flags ─────────────────────────────────────────────────────────────────────
DRY_RUN=false
QUIET=false
TARGET="all"

# ─── Helpers ───────────────────────────────────────────────────────────────────
usage() {
  echo "Usage: $0 [--target opencode|claude|all] [--dry-run] [--quiet] [--help]"
  echo ""
  echo "Remove ilnamiqui from assistant configurations."
  echo ""
  echo "Flags:"
  echo "  --target    What to remove: opencode, claude, or all (default: all)"
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

# ─── Detect TTY & Parse flags ─────────────────────────────────────────────────
IS_TTY=false
if [ -t 0 ]; then
  IS_TTY=true
fi

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      TARGET="$2"
      shift 2
      ;;
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

# ─── Resolve target when interactive ──────────────────────────────────────────
if [ "$TARGET" = "all" ] && [ "$IS_TTY" = true ] && [ $# -eq 0 ]; then
  # Only prompt if --target was not explicitly provided
  echo ""
  echo "Which assistant to uninstall?"
  echo "  1) opencode"
  echo "  2) Claude Code"
  echo "  3) Both (default)"
  read -p "Select [3]: " choice
  echo ""
  case "$choice" in
    1|opencode) TARGET="opencode" ;;
    2|claude) TARGET="claude" ;;
    *) TARGET="all" ;;
  esac
fi

# Validate target
if [ "$TARGET" != "opencode" ] && [ "$TARGET" != "claude" ] && [ "$TARGET" != "all" ]; then
  error "Invalid target '$TARGET'. Use 'opencode', 'claude', or 'all'."
  exit 1
fi

# ─── Main ──────────────────────────────────────────────────────────────────────
echo ""
echo "  ilnamiqui uninstaller"
echo "  Target: $TARGET"
echo ""

# ── Remove opencode components ─────────────────────────────────────────────────
if [ "$TARGET" = "opencode" ] || [ "$TARGET" = "all" ]; then
  # 1. Remove skill directory
  if [ -d "$OPENCODE_SKILL_DIR" ]; then
    action "Removing opencode skill directory: ${OPENCODE_SKILL_DIR}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would run: rm -rf ${OPENCODE_SKILL_DIR}"
    else
      rm -rf "$OPENCODE_SKILL_DIR"
      info "Removed ${OPENCODE_SKILL_DIR}"
    fi
  else
    info "Opencode skill directory not found, skipping: ${OPENCODE_SKILL_DIR}"
  fi

  # 2. Remove plugin file
  if [ -f "$OPENCODE_PLUGIN_PATH" ] || [ -L "$OPENCODE_PLUGIN_PATH" ]; then
    action "Removing opencode plugin file: ${OPENCODE_PLUGIN_PATH}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would run: rm -f ${OPENCODE_PLUGIN_PATH}"
    else
      rm -f "$OPENCODE_PLUGIN_PATH"
      info "Removed ${OPENCODE_PLUGIN_PATH}"
    fi
  else
    info "Opencode plugin file not found, skipping: ${OPENCODE_PLUGIN_PATH}"
  fi

  # 3. Remove plugin entry from opencode.json
  if [ -f "$OPENCODE_CONFIG" ]; then
    action "Updating opencode.json..."
    PLUGIN_ENTRY="./plugins/ilnamiqui.ts"

    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would remove plugin entry '${PLUGIN_ENTRY}' from ${OPENCODE_CONFIG}"
    else
      if command -v jq &>/dev/null; then
        tmp=$(mktemp /tmp/ilnamiqui-uninstall.XXXXXXXX)
        if jq --arg p "$PLUGIN_ENTRY" '
          if .plugin then
            .plugin -= [$p] | if .plugin == [] then del(.plugin) else . end
          else
            .
          end
        ' "$OPENCODE_CONFIG" > "$tmp" 2>/dev/null; then
          mv "$tmp" "$OPENCODE_CONFIG"
          info "Updated using jq"
        else
          rm -f "$tmp"
          error "jq failed to parse ${OPENCODE_CONFIG}"
          exit 1
        fi

      elif command -v node &>/dev/null; then
        node -e "
          const fs = require('fs');
          const cfgPath = '${OPENCODE_CONFIG}';
          let cfg;
          try { cfg = JSON.parse(fs.readFileSync(cfgPath, 'utf-8')); }
          catch(e) { process.stderr.write('ERROR: Failed to parse ' + cfgPath + '\n'); process.exit(1); }
          const entry = '${PLUGIN_ENTRY}';
          if (cfg.plugin && Array.isArray(cfg.plugin)) {
            cfg.plugin = cfg.plugin.filter(p => p !== entry);
            if (cfg.plugin.length === 0) delete cfg.plugin;
          }
          fs.writeFileSync(cfgPath, JSON.stringify(cfg, null, 2) + '\n');
        " 2>/dev/null || {
          error "node script failed"
          exit 1
        }
        info "Updated using node"

      elif command -v python3 &>/dev/null; then
        python3 -c "
import json, os
cfg_path = '${OPENCODE_CONFIG}'
try:
    with open(cfg_path) as f:
        cfg = json.load(f)
except FileNotFoundError:
    cfg = {}
except json.JSONDecodeError as e:
    import sys
    sys.stderr.write(f'ERROR: Failed to parse {cfg_path}: {e}\n')
    sys.exit(1)
entry = '${PLUGIN_ENTRY}'
if 'plugin' in cfg and isinstance(cfg['plugin'], list):
    cfg['plugin'] = [p for p in cfg['plugin'] if p != entry]
    if not cfg['plugin']:
        del cfg['plugin']
with open(cfg_path, 'w') as f:
    json.dump(cfg, f, indent=2)
    f.write('\n')
" 2>/dev/null || {
          error "python3 script failed"
          exit 1
        }
        info "Updated using python3"

      else
        error "Neither jq, node, nor python3 found. Cannot update ${OPENCODE_CONFIG}"
        error "Remove this entry manually from ${OPENCODE_CONFIG}:"
        error "  \"plugin\": [\"${PLUGIN_ENTRY}\"]"
        exit 1
      fi
    fi
  else
    info "Opencode config not found, skipping: ${OPENCODE_CONFIG}"
  fi
fi

# ── Remove claude components ───────────────────────────────────────────────────
if [ "$TARGET" = "claude" ] || [ "$TARGET" = "all" ]; then
  # 1. Remove skill directory
  if [ -d "$CLAUDE_SKILL_DIR" ]; then
    action "Removing claude skill directory: ${CLAUDE_SKILL_DIR}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would run: rm -rf ${CLAUDE_SKILL_DIR}"
    else
      rm -rf "$CLAUDE_SKILL_DIR"
      info "Removed ${CLAUDE_SKILL_DIR}"
    fi
  else
    info "Claude skill directory not found, skipping: ${CLAUDE_SKILL_DIR}"
  fi

  # 2. Remove MCP server entry from claude.json
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
          info "Updated ${CLAUDE_CONFIG} using jq"
        else
          rm -f "$tmp"
          error "jq failed to update ${CLAUDE_CONFIG}"
          exit 1
        fi

      elif command -v python3 &>/dev/null; then
        python3 -c "
import json
file = '${CLAUDE_CONFIG}'
try:
    with open(file) as f:
        data = json.load(f)
except FileNotFoundError:
    data = {}
except json.JSONDecodeError as e:
    import sys
    sys.stderr.write(f'ERROR: Failed to parse {file}: {e}\n')
    sys.exit(1)
if 'mcpServers' in data and 'ilnamiqui' in data['mcpServers']:
    del data['mcpServers']['ilnamiqui']
    if not data['mcpServers']:
        del data['mcpServers']
with open(file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" 2>/dev/null || {
          error "python3 script failed"
          exit 1
        }
        info "Updated ${CLAUDE_CONFIG} using python3"

      else
        error "Neither jq nor python3 found. Cannot update ${CLAUDE_CONFIG}"
        error "Remove this entry manually from ${CLAUDE_CONFIG}:"
        error '  "mcpServers": { "ilnamiqui": { ... } }'
        exit 1
      fi
    fi
  else
    info "Claude config not found, skipping: ${CLAUDE_CONFIG}"
  fi
fi

# ── Remove shared components (when removing all or when both are gone) ─────────
if [ "$TARGET" = "all" ]; then
  # Remove binary directory
  if [ -d "$BIN_DIR" ]; then
    action "Removing binary directory: ${BIN_DIR}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would run: rm -rf ${BIN_DIR}"
    else
      rm -rf "$BIN_DIR"
      info "Removed ${BIN_DIR}"
    fi
  else
    info "Binary directory not found, skipping: ${BIN_DIR}"
  fi

  # Remove CLI symlink
  if [ -f "$SYMLINK_PATH" ] || [ -L "$SYMLINK_PATH" ]; then
    action "Removing CLI symlink: ${SYMLINK_PATH}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would run: rm -f ${SYMLINK_PATH}"
    else
      rm -f "$SYMLINK_PATH"
      info "Removed ${SYMLINK_PATH}"
    fi
  else
    info "CLI symlink not found, skipping: ${SYMLINK_PATH}"
  fi
fi

# Remove MCP binary when removing claude or all (but keep main binary for --target opencode only)
if [ "$TARGET" = "claude" ]; then
  MCP_BINARY_NAME="ilnamiqui-mcp"
  # Find and remove MCP binary in bin dir
  if [ -d "$BIN_DIR" ]; then
    action "Removing MCP binary from ${BIN_DIR}"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would remove MCP binaries from ${BIN_DIR}"
    else
      find "$BIN_DIR" -name "${MCP_BINARY_NAME}*" -exec rm -f {} \; 2>/dev/null || true
      info "Removed MCP binaries from ${BIN_DIR}"
    fi
  fi
fi

# 5. Success message
echo ""
echo "  ✓ ilnamiqui uninstalled for target: ${TARGET}"
if [ "$TARGET" = "opencode" ]; then
  echo "  Restart opencode to complete."
elif [ "$TARGET" = "claude" ]; then
  echo "  Restart Claude Code to complete."
else
  echo "  Restart opencode or Claude Code to complete."
fi
echo ""

exit 0
