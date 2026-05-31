#!/bin/bash
set -euo pipefail

# ilnamiqui uninstaller
# Usage: bash scripts/uninstall.sh [--dry-run] [--quiet] [--help]

# ─── Paths (must match install.sh) ─────────────────────────────────────────────
BIN_DIR="${HOME}/.config/opencode/plugins/ilnamiqui"
SKILL_DIR="${HOME}/.config/opencode/skills/ilnamiqui"
PLUGIN_PATH="${HOME}/.config/opencode/plugins/ilnamiqui.ts"
OPENCODE_CONFIG="${HOME}/.config/opencode/opencode.json"

# ─── Flags ─────────────────────────────────────────────────────────────────────
DRY_RUN=false
QUIET=false

# ─── Helpers ───────────────────────────────────────────────────────────────────
usage() {
  echo "Usage: $0 [--dry-run] [--quiet] [--help]"
  echo ""
  echo "Remove ilnamiqui from opencode configuration."
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

# ─── Parse flags ──────────────────────────────────────────────────────────────
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
echo ""

# 1. Remove binary directory
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

# 1.a Remove CLI symlink
SYMLINK_PATH="${HOME}/.local/bin/ilnamiqui"
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

# 2. Remove plugin file
if [ -f "$PLUGIN_PATH" ] || [ -L "$PLUGIN_PATH" ]; then
  action "Removing plugin file: ${PLUGIN_PATH}"
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would run: rm -f ${PLUGIN_PATH}"
  else
    rm -f "$PLUGIN_PATH"
    info "Removed ${PLUGIN_PATH}"
  fi
else
  info "Plugin file not found, skipping: ${PLUGIN_PATH}"
fi

# 3. Remove skill directory
if [ -d "$SKILL_DIR" ]; then
  action "Removing skill directory: ${SKILL_DIR}"
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would run: rm -rf ${SKILL_DIR}"
  else
    rm -rf "$SKILL_DIR"
    info "Removed ${SKILL_DIR}"
  fi
else
  info "Skill directory not found, skipping: ${SKILL_DIR}"
fi

# 4. Remove plugin entry from opencode.json
echo ""
action "Updating opencode.json..."

PLUGIN_ENTRY="./plugins/ilnamiqui.ts"

if [ ! -f "$OPENCODE_CONFIG" ]; then
  info "Config not found, skipping: ${OPENCODE_CONFIG}"
else
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would remove plugin entry '${PLUGIN_ENTRY}' from ${OPENCODE_CONFIG}"
  else
    if command -v jq &>/dev/null; then
      # jq approach
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
      # node approach
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
      # python3 approach
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
    sys.stderr.write(f"ERROR: Failed to parse {cfg_path}: {e}\n")
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
fi

# 5. Success message
echo ""
echo "  ✓ ilnamiqui uninstalled. Restart opencode to complete."
echo ""

exit 0
