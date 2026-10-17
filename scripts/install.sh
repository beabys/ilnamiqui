#!/bin/bash
set -euo pipefail

# ilnamiqui installer
# Usage: curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --target opencode
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --target claude
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --version v1.0.0
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --opencode-version v1
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --dry-run

# ─── Config ───────────────────────────────────────────────────────────────────
REPO_OWNER="beabys"
REPO_NAME="ilnamiqui"
RAW_BASE="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main"
GH_API="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"

# Defaults
VERSION=""
DRY_RUN=false
TARGET=""
OPENCODE_VERSION=""

# ─── Detect OS & Arch ─────────────────────────────────────────────────────────
detect_os() {
  local uname
  uname=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$uname" in
    linux)  echo "linux" ;;
    darwin) echo "darwin" ;;
    windows|msys*|cygwin*) echo "windows" ;;
    *)      echo "unknown" ;;
  esac
}

detect_arch() {
  local uname_m
  uname_m=$(uname -m)
  case "$uname_m" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)             echo "unknown" ;;
  esac
}

# Detect opencode plugin version (v1 or v2).
# Precedence: --opencode-version flag > OPENCODE_PLUGIN_VERSION env > opencode --version > fallback v2.
detect_opencode_version() {
  # 1. Explicit flag already captured in OPENCODE_VERSION
  if [ -n "$OPENCODE_VERSION" ]; then
    case "$OPENCODE_VERSION" in
      v1|1) echo "v1"; return 0 ;;
      v2|2) echo "v2"; return 0 ;;
      *)
        error "Invalid --opencode-version '$OPENCODE_VERSION'. Use 'v1' or 'v2'."
        return 1
        ;;
    esac
  fi

  # 2. Env var override
  if [ -n "${OPENCODE_PLUGIN_VERSION:-}" ]; then
    case "$OPENCODE_PLUGIN_VERSION" in
      v1|1) echo "v1"; return 0 ;;
      v2|2) echo "v2"; return 0 ;;
      *)
        error "Invalid OPENCODE_PLUGIN_VERSION='$OPENCODE_PLUGIN_VERSION'. Use 'v1' or 'v2'."
        return 1
        ;;
    esac
  fi

  # 3. Probe opencode --version
  # Sandbox into a throwaway HOME so opencode's own side-effects
  # (cache/state/config dirs) never land in the caller's real $HOME.
  # All XDG base dirs redirected to the sandbox; removed afterwards on every path.
  if command -v opencode &>/dev/null; then
    local raw major probe_home
    probe_home=$(mktemp -d)
    # shellcheck disable=SC2064
    trap "rm -rf '$probe_home'" EXIT
    mkdir -p "$probe_home/tmp"
    raw=$(
      HOME="$probe_home" \
      TMPDIR="$probe_home/tmp" \
      XDG_CONFIG_HOME="$probe_home/.config" \
      XDG_DATA_HOME="$probe_home/.local/share" \
      XDG_CACHE_HOME="$probe_home/.cache" \
      XDG_STATE_HOME="$probe_home/.local/state" \
      opencode --version 2>/dev/null | head -1
    ) || true
    rm -rf "$probe_home"
    trap - EXIT
    # Extract semantic version pattern (X.Y.Z) from first line, then get major.
    # Handles: "opencode v2.0.29", "2.0.29", "v1.5.0", etc.
    # If no version pattern found, major stays empty → fallback.
    major=$(printf '%s' "$raw" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1 | cut -d. -f1) || true
    if [ -n "$major" ]; then
      if [ "$major" -ge 2 ] 2>/dev/null; then
        echo "v2"; return 0
      else
        echo "v1"; return 0
      fi
    fi
  fi

  # 4. Fallback: v2
  echo "v2"
}

OS=$(detect_os)
ARCH=$(detect_arch)

if [ "$OS" = "unknown" ] || [ "$ARCH" = "unknown" ]; then
  echo "ERROR: Unsupported platform: $(uname -s) / $(uname -m)"
  exit 1
fi

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
    --version)
      VERSION="$2"
      shift 2
      ;;
    --opencode-version)
      OPENCODE_VERSION="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    *)
      echo "Usage: $0 [--target opencode|claude] [--version VERSION] [--opencode-version v1|v2] [--dry-run]"
      exit 1
      ;;
  esac
done

# ─── Resolve target ────────────────────────────────────────────────────────────
if [ -n "$TARGET" ]; then
  if [ "$TARGET" != "opencode" ] && [ "$TARGET" != "claude" ]; then
    echo "ERROR: Invalid target '$TARGET'. Use 'opencode' or 'claude'."
    exit 1
  fi
else
  echo ""
  echo "Which AI assistant?"
  echo "  1) opencode (default)"
  echo "  2) Claude Code"
  if [ "$IS_TTY" = true ]; then
    read -p "Select [1]: " choice
  elif [ -e /dev/tty ]; then
    read -p "Select [1]: " choice < /dev/tty
  else
    echo "Warning: no terminal available, defaulting to opencode"
    echo "  Use --target to specify: curl ... | bash -s -- --target claude"
    choice=""
  fi
  echo ""
  case "$choice" in
    2|claude) TARGET="claude" ;;
    *) TARGET="opencode" ;;
  esac
fi

# ─── Resolve opencode plugin version (v1/v2) ──────────────────────────────────
if [ "$TARGET" = "opencode" ]; then
  OPENCODE_VERSION_RESOLVED=$(detect_opencode_version) || exit 1
else
  OPENCODE_VERSION_RESOLVED=""
fi

# ─── Resolve version ──────────────────────────────────────────────────────────
if [ -z "$VERSION" ]; then
  if command -v curl &>/dev/null; then
    VERSION=$(curl -fsSL "$GH_API" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)",.*/\1/') || true
  elif command -v wget &>/dev/null; then
    VERSION=$(wget -qO- "$GH_API" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)",.*/\1/') || true
  fi

  if [ -z "$VERSION" ]; then
    echo "ERROR: Could not determine latest version. Set VERSION env var or use --version."
    echo "  curl -fsSL ... | VERSION=v1.0.0 bash"
    echo "  curl -fsSL ... | bash -s -- --version v1.0.0"
    exit 1
  fi
fi

# ─── Paths ─────────────────────────────────────────────────────────────────────
SYMLINK_PATH="${HOME}/.local/bin/ilnamiqui"

case "$TARGET" in
  opencode)
    BIN_DIR="${HOME}/.config/opencode/plugins/ilnamiqui"
    ;;
  claude)
    BIN_DIR="${HOME}/.claude/plugins/ilnamiqui"
    ;;
esac

OPENCODE_CONFIG="${HOME}/.config/opencode/opencode.json"
CLAUDE_CONFIG="${HOME}/.claude/claude.json"
CLAUDE_SKILL_DIR="${HOME}/.claude/skills/ilnamiqui"
CLAUDE_SKILL_PATH="${CLAUDE_SKILL_DIR}/SKILL.md"
OPENCODE_SKILL_DIR="${HOME}/.config/opencode/skills/ilnamiqui"
OPENCODE_PLUGIN_DIR="${HOME}/.config/opencode/plugins"
CLAUDE_HOOKS_DIR="${HOME}/.claude/hooks/ilnamiqui"
CLAUDE_SETTINGS="${HOME}/.claude/settings.json"

BINARY_NAME="ilnamiqui-${OS}-${ARCH}"
MCP_BINARY_NAME="ilnamiqui-mcp-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
  BINARY_NAME="${BINARY_NAME}.exe"
  MCP_BINARY_NAME="${MCP_BINARY_NAME}.exe"
fi
BINARY_PATH="${BIN_DIR}/${BINARY_NAME}"
MCP_BINARY_PATH="${BIN_DIR}/${MCP_BINARY_NAME}"

# URLs
OPENCODE_SKILL_REPO_PATH="${RAW_BASE}/opencode/skill/SKILL.md"
# OPENCODE_PLUGIN_REPO_PATH is set per-version after detection (see opencode branch)
CLAUDE_SKILL_REPO_PATH="${RAW_BASE}/claude/skill/SKILL.md"
HOOK_SESSION_START_REPO="${RAW_BASE}/claude/hooks/ilnamiqui-session-start.sh"
HOOK_PRECOMPACT_REPO="${RAW_BASE}/claude/hooks/ilnamiqui-precompact.sh"
HOOK_POSTCOMPACT_REPO="${RAW_BASE}/claude/hooks/ilnamiqui-postcompact.sh"
HOOK_SESSION_END_REPO="${RAW_BASE}/claude/hooks/ilnamiqui-session-end.sh"

# Archive URL — GoReleaser produces archives containing both binaries
ARCHIVE_EXT=".tar.gz"
BINARY_IN_ARCHIVE="ilnamiqui"
MCP_BINARY_IN_ARCHIVE="ilnamiqui-mcp"
if [ "$OS" = "windows" ]; then
  ARCHIVE_EXT=".zip"
  BINARY_IN_ARCHIVE="${BINARY_IN_ARCHIVE}.exe"
  MCP_BINARY_IN_ARCHIVE="${MCP_BINARY_IN_ARCHIVE}.exe"
fi
DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/ilnamiqui-${OS}-${ARCH}${ARCHIVE_EXT}"

# ─── Helpers ───────────────────────────────────────────────────────────────────
info()    { echo "  $1"; }
action()  { echo "  → $1"; }
error()   { echo "  ERROR: $1" >&2; }

download() {
  local url="$1" dest="$2" desc="$3"
  if [ "$DRY_RUN" = true ]; then
    action "[dry-run] Would download $desc"
    info "  from: $url"
    info "  to:   $dest"
    return 0
  fi

  if command -v curl &>/dev/null; then
    curl -fsSL "$url" -o "$dest" 2>/dev/null || {
      error "Failed to download $desc from $url"
      return 1
    }
  elif command -v wget &>/dev/null; then
    wget -q "$url" -O "$dest" 2>/dev/null || {
      error "Failed to download $desc from $url"
      return 1
    }
  else
    error "Neither curl nor wget found. Install one of them first."
    return 1
  fi
  info "Downloaded $desc"
}

update_json() {
  local file="$1" key="$2" value="$3"
  if [ "$DRY_RUN" = true ]; then
    info "[dry-run] Would update ${file} to set ${key}: ${value}"
    return 0
  fi

  if command -v jq &>/dev/null; then
    tmp=$(mktemp /tmp/ilnamiqui.XXXXXXXX)
    if jq "${key}" "${file}" > "$tmp" 2>/dev/null; then
      mv "$tmp" "${file}"
      info "Updated ${file} using jq"
    else
      rm -f "$tmp"
      error "jq failed to update ${file}"
      return 1
    fi
  elif command -v python3 &>/dev/null; then
    python3 -c "
import json, os
file = '${file}'
try:
    with open(file) as f:
        data = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    data = {}
${value}
with open(file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" || {
      error "python3 script failed to update ${file}"
      return 1
    }
    info "Updated ${file} using python3"
  else
    error "Neither jq nor python3 found. Cannot update ${file}"
    error "Please install jq or python3, or update manually."
    return 1
  fi
}

# ─── Ensure ~/.local/bin is in PATH ──────────────────────────────────────────
ensure_path() {
  local dir="${HOME}/.local/bin"
  if echo "$PATH" | grep -q "${dir}"; then
    info "${dir} already in PATH"
    return 0
  fi

  local rc_file=""
  case "${SHELL:-}" in
    */bash) rc_file="${HOME}/.bashrc" ;;
    */zsh)  rc_file="${HOME}/.zshrc" ;;
    */fish)
      info "Add 'fish_add_path ${dir}' to your fish config manually"
      return 0
      ;;
    *)
      info "Add 'export PATH=\"${dir}:\$PATH\"' to your shell config manually"
      return 0
      ;;
  esac

  local line="export PATH=\"\${HOME}/.local/bin:\$PATH\""
  if [ ! -f "$rc_file" ]; then
    echo "$line" > "$rc_file"
    info "Created ${rc_file} with PATH entry"
  elif ! grep -qF ".local/bin" "$rc_file" 2>/dev/null; then
    echo "" >> "$rc_file"
    echo "$line" >> "$rc_file"
    info "Added ${dir} to PATH in ${rc_file}"
  else
    info "${dir} already configured in ${rc_file}"
    return 0
  fi

  info "Run 'source ${rc_file}' or open a new terminal to use ilnamiqui"
}

# ─── Main ──────────────────────────────────────────────────────────────────────
echo ""
echo "  ilnamiqui installer"
echo "  Version: $VERSION"
echo "  OS: $OS  Arch: $ARCH"
echo "  Target: $TARGET"
if [ "$TARGET" = "opencode" ] && [ -n "$OPENCODE_VERSION_RESOLVED" ]; then
  echo "  Opencode plugin version: $OPENCODE_VERSION_RESOLVED"
fi
echo ""

# ── Steps common to both targets ──────────────────────────────────────────────

# 1. Create directories
action "Creating directories..."
if [ "$DRY_RUN" = true ]; then
  info "[dry-run] Would create directory: ${BIN_DIR}/"
else
  mkdir -p "$BIN_DIR"
  info "  ${BIN_DIR}/"
fi

# 2. Download and extract binary archive (contains both binaries)
echo ""
if [ "$DRY_RUN" = true ]; then
  action "[dry-run] Would download archive: ${DOWNLOAD_URL}"
  info "[dry-run] Would extract and install binary to: ${BINARY_PATH}"
else
  action "Downloading archive..."
  TMP_DIR=$(mktemp -d /tmp/ilnamiqui.XXXXXXXX)
  ARCHIVE_PATH="${TMP_DIR}/ilnamiqui${ARCHIVE_EXT}"
  download "$DOWNLOAD_URL" "$ARCHIVE_PATH" "archive (${OS}/${ARCH})"

  action "Extracting binaries..."
  if [ "$OS" = "windows" ]; then
    unzip -o "$ARCHIVE_PATH" -d "$TMP_DIR" 2>/dev/null || {
      error "Failed to extract archive. Ensure unzip is installed."
      rm -rf "$TMP_DIR"
      exit 1
    }
  else
    tar xzf "$ARCHIVE_PATH" -C "$TMP_DIR" 2>/dev/null || {
      error "Failed to extract archive."
      rm -rf "$TMP_DIR"
      exit 1
    }
  fi

  # Install main binary (always needed for both targets)
  if [ ! -f "${TMP_DIR}/${BINARY_IN_ARCHIVE}" ]; then
    error "Binary '${BINARY_IN_ARCHIVE}' not found in archive."
    rm -rf "$TMP_DIR"
    exit 1
  fi
  mv "${TMP_DIR}/${BINARY_IN_ARCHIVE}" "$BINARY_PATH"
  chmod +x "$BINARY_PATH"
  info "Main binary installed to ${BINARY_PATH}"

  # Install MCP binary (always — used by claude target)
  if [ -f "${TMP_DIR}/${MCP_BINARY_IN_ARCHIVE}" ]; then
    mv "${TMP_DIR}/${MCP_BINARY_IN_ARCHIVE}" "$MCP_BINARY_PATH"
    chmod +x "$MCP_BINARY_PATH"
    info "MCP binary installed to ${MCP_BINARY_PATH}"
  else
    info "MCP binary not found in archive (may not be present in this release)"
  fi

  rm -rf "$TMP_DIR"


fi

# ── Target-specific steps ──────────────────────────────────────────────────────
case "$TARGET" in
  opencode)
    # Set per-version source path and config key
    case "$OPENCODE_VERSION_RESOLVED" in
      v1)
        OPENCODE_PLUGIN_REPO_PATH="${RAW_BASE}/opencode/v1/plugin/ilnamiqui.ts"
        OPENCODE_CONFIG_KEY="plugin"
        ;;
      v2)
        OPENCODE_PLUGIN_REPO_PATH="${RAW_BASE}/opencode/v2/plugin/ilnamiqui.ts"
        OPENCODE_CONFIG_KEY="plugins"
        ;;
    esac

    # 3. Download SKILL.md
    echo ""
    action "Downloading opencode skill..."
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would create directory: ${OPENCODE_SKILL_DIR}/"
    else
      mkdir -p "$OPENCODE_SKILL_DIR"
    fi
    download "$OPENCODE_SKILL_REPO_PATH" "${OPENCODE_SKILL_DIR}/SKILL.md" "SKILL.md"

    # 4. Download plugin.ts
    echo ""
    action "Downloading plugin (${OPENCODE_VERSION_RESOLVED})..."
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would create directory: ${OPENCODE_PLUGIN_DIR}/"
    else
      mkdir -p "$OPENCODE_PLUGIN_DIR"
    fi
    download "$OPENCODE_PLUGIN_REPO_PATH" "${OPENCODE_PLUGIN_DIR}/ilnamiqui.ts" "plugin.ts (${OPENCODE_VERSION_RESOLVED})"

    # 5. Register plugin in opencode.json
    echo ""
    action "Updating opencode.json (key: ${OPENCODE_CONFIG_KEY})..."
    if [ "$DRY_RUN" = true ]; then
      if [ ! -f "$OPENCODE_CONFIG" ]; then
        info "[dry-run] Would create ${OPENCODE_CONFIG}"
      fi
    else
      if [ ! -f "$OPENCODE_CONFIG" ]; then
        echo '{}' > "$OPENCODE_CONFIG"
        info "Created ${OPENCODE_CONFIG}"
      fi
    fi

    PLUGIN_ENTRY="./plugins/ilnamiqui.ts"
    # jq filter: handle both string→array conversion and array append, dedupe
    JQ_FILTER=".${OPENCODE_CONFIG_KEY} |= ((if type == \"string\" then [.] else . // [] end) + [\"${PLUGIN_ENTRY}\"] | unique)"
    # python3 snippet: handle string→list conversion, setdefault, dedupe
    PY_SNIPPET="if isinstance(data.get('${OPENCODE_CONFIG_KEY}'), str): data['${OPENCODE_CONFIG_KEY}'] = [data['${OPENCODE_CONFIG_KEY}']]
data.setdefault('${OPENCODE_CONFIG_KEY}', [])
entry = '${PLUGIN_ENTRY}'
if entry not in data['${OPENCODE_CONFIG_KEY}']: data['${OPENCODE_CONFIG_KEY}'].append(entry)"

    update_json "$OPENCODE_CONFIG" "$JQ_FILTER" "$PY_SNIPPET"

    # 6. Create CLI symlink (opencode only)
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would create symlink: ${SYMLINK_PATH} → ${BINARY_PATH}"
    else
      if [ ! -L "$SYMLINK_PATH" ] && [ ! -f "$SYMLINK_PATH" ]; then
        mkdir -p "${HOME}/.local/bin"
        ln -s "$BINARY_PATH" "$SYMLINK_PATH"
        info "Symlink created: ${SYMLINK_PATH} → ${BINARY_PATH}"
      else
        info "Symlink already exists: ${SYMLINK_PATH}"
      fi
      ensure_path
    fi
    ;;

  claude)
    # 3. Download SKILL.md
    echo ""
    action "Downloading Claude Code skill..."
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would create directory: ${CLAUDE_SKILL_DIR}/"
    else
      mkdir -p "$CLAUDE_SKILL_DIR"
      # Remove old CLAUDE.md if exists (rename from previous installs)
      if [ -f "${CLAUDE_SKILL_DIR}/CLAUDE.md" ]; then
        rm -f "${CLAUDE_SKILL_DIR}/CLAUDE.md"
        info "Removed old CLAUDE.md (replaced by SKILL.md)"
      fi
    fi

    download "$CLAUDE_SKILL_REPO_PATH" "$CLAUDE_SKILL_PATH" "SKILL.md"

    # 4. Register MCP server in claude.json
    echo ""
    action "Updating claude.json..."
    if [ "$DRY_RUN" = true ]; then
      if [ ! -f "$CLAUDE_CONFIG" ]; then
        info "[dry-run] Would create ${CLAUDE_CONFIG}"
      fi
    else
      if [ ! -f "$CLAUDE_CONFIG" ]; then
        mkdir -p "$(dirname "$CLAUDE_CONFIG")"
        echo '{}' > "$CLAUDE_CONFIG"
        info "Created ${CLAUDE_CONFIG}"
      fi
    fi

    MCP_COMMAND="$MCP_BINARY_PATH"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would update ${CLAUDE_CONFIG} to register MCP server"
    else
      if command -v jq &>/dev/null; then
        tmp=$(mktemp /tmp/ilnamiqui.XXXXXXXX)
        if jq --arg cmd "$MCP_COMMAND" '
          .mcpServers //= {} |
          .mcpServers["ilnamiqui"] = {
            "command": $cmd,
            "args": []
          }
        ' "$CLAUDE_CONFIG" > "$tmp" 2>/dev/null; then
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
except (FileNotFoundError, json.JSONDecodeError):
    data = {}
if 'mcpServers' not in data:
    data['mcpServers'] = {}
data['mcpServers']['ilnamiqui'] = {
    'command': '${MCP_COMMAND}',
    'args': []
}
with open(file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" || {
          error "python3 script failed to update ${CLAUDE_CONFIG}"
          exit 1
        }
        info "Updated ${CLAUDE_CONFIG} using python3"
      else
        error "Neither jq nor python3 found. Cannot update ${CLAUDE_CONFIG}"
        error "Add this to ${CLAUDE_CONFIG}:"
        error "  { \"mcpServers\": { \"ilnamiqui\": { \"command\": \"${MCP_COMMAND}\", \"args\": [] } } }"
        exit 1
      fi
    fi

    # 5. Install lifecycle hooks
    echo ""
    action "Installing lifecycle hooks..."
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would download hooks to ${CLAUDE_HOOKS_DIR}/"
    else
      mkdir -p "$CLAUDE_HOOKS_DIR"
      download "$HOOK_SESSION_START_REPO" "${CLAUDE_HOOKS_DIR}/session-start.sh" "session-start hook"
      download "$HOOK_PRECOMPACT_REPO" "${CLAUDE_HOOKS_DIR}/precompact.sh" "precompact hook"
      download "$HOOK_POSTCOMPACT_REPO" "${CLAUDE_HOOKS_DIR}/postcompact.sh" "postcompact hook"
      download "$HOOK_SESSION_END_REPO" "${CLAUDE_HOOKS_DIR}/session-end.sh" "session-end hook"
      chmod +x "${CLAUDE_HOOKS_DIR}"/*.sh
      echo "  Hooks installed to ${CLAUDE_HOOKS_DIR}/"
    fi

    # 6. Create CLI symlink (needed by hooks)
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would create symlink: ${SYMLINK_PATH} → ${BINARY_PATH}"
    else
      if [ ! -L "$SYMLINK_PATH" ] && [ ! -f "$SYMLINK_PATH" ]; then
        mkdir -p "${HOME}/.local/bin"
        ln -s "$BINARY_PATH" "$SYMLINK_PATH"
        info "Symlink created: ${SYMLINK_PATH} → ${BINARY_PATH}"
      else
        info "Symlink already exists: ${SYMLINK_PATH}"
      fi
      ensure_path
    fi

    # 7. Register hooks in settings.json
    echo ""
    action "Registering hooks in settings.json..."
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would update ${CLAUDE_SETTINGS}"
    else
      # Create settings.json if it doesn't exist
      if [ ! -f "$CLAUDE_SETTINGS" ]; then
        mkdir -p "$(dirname "$CLAUDE_SETTINGS")"
        echo '{}' > "$CLAUDE_SETTINGS"
        info "Created ${CLAUDE_SETTINGS}"
      fi

      HOOKS_DIR="${HOME}/.claude/hooks/ilnamiqui"
      SESSION_START_CMD="${HOOKS_DIR}/session-start.sh"
      PRECOMPACT_CMD="${HOOKS_DIR}/precompact.sh"
      POSTCOMPACT_CMD="${HOOKS_DIR}/postcompact.sh"
      SESSION_END_CMD="${HOOKS_DIR}/session-end.sh"

      if command -v jq &>/dev/null; then
        tmp=$(mktemp /tmp/ilnamiqui.XXXXXXXX)
        jq --arg ss "$SESSION_START_CMD" --arg pc "$PRECOMPACT_CMD" --arg poc "$POSTCOMPACT_CMD" --arg se "$SESSION_END_CMD" '
          .hooks //= {} |
          .hooks["SessionStart"] = [{"matcher": "startup|resume", "hooks": [{"type": "command", "command": $ss}]}] |
          .hooks["PreCompact"] = [{"hooks": [{"type": "command", "command": $pc}]}] |
          .hooks["PostCompact"] = [{"hooks": [{"type": "command", "command": $poc}]}] |
          .hooks["SessionEnd"] = [{"hooks": [{"type": "command", "command": $se, "timeout": 15}]}]
        ' "$CLAUDE_SETTINGS" > "$tmp" 2>/dev/null && mv "$tmp" "$CLAUDE_SETTINGS" || {
          rm -f "$tmp"
          error "jq failed to update ${CLAUDE_SETTINGS}"
          exit 1
        }
        info "Updated ${CLAUDE_SETTINGS} using jq"
      elif command -v python3 &>/dev/null; then
        python3 -c "
import json, os
file = '${CLAUDE_SETTINGS}'
try:
    with open(file) as f:
        data = json.load(f)
except (FileNotFoundError, json.JSONDecodeError):
    data = {}
data['hooks'] = data.get('hooks', {})
data['hooks']['SessionStart'] = [{'matcher': 'startup|resume', 'hooks': [{'type': 'command', 'command': '${SESSION_START_CMD}'}]}]
data['hooks']['PreCompact'] = [{'hooks': [{'type': 'command', 'command': '${PRECOMPACT_CMD}'}]}]
data['hooks']['PostCompact'] = [{'hooks': [{'type': 'command', 'command': '${POSTCOMPACT_CMD}'}]}]
data['hooks']['SessionEnd'] = [{'hooks': [{'type': 'command', 'command': '${SESSION_END_CMD}', 'timeout': 15}]}]
with open(file, 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
" || {
          error "python3 script failed to update ${CLAUDE_SETTINGS}"
          exit 1
        }
        info "Updated ${CLAUDE_SETTINGS} using python3"
      else
        error "Neither jq nor python3 found. Cannot update ${CLAUDE_SETTINGS}"
        error "Please install jq or python3, or update manually."
        exit 1
      fi
    fi

    # 8. Register skill in CLAUDE.md (so Claude Code loads it)
    echo ""
    action "Registering skill in CLAUDE.md..."
    CLAUDE_HOME="${HOME}/.claude/CLAUDE.md"
    INCLUDE_LINE="@skills/ilnamiqui/SKILL.md"
    if [ "$DRY_RUN" = true ]; then
      info "[dry-run] Would add '${INCLUDE_LINE}' to ${CLAUDE_HOME}"
    else
      if [ ! -f "$CLAUDE_HOME" ]; then
        echo "$INCLUDE_LINE" > "$CLAUDE_HOME"
        info "Created ${CLAUDE_HOME} with skill reference"
      elif ! grep -Fxq "$INCLUDE_LINE" "$CLAUDE_HOME" 2>/dev/null; then
        echo "$INCLUDE_LINE" >> "$CLAUDE_HOME"
        info "Added skill reference to ${CLAUDE_HOME}"
      else
        info "Skill reference already present in ${CLAUDE_HOME}"
      fi
    fi
    ;;
esac

# 6. Verify binary
echo ""
action "Verifying binary..."
if [ "$DRY_RUN" = false ]; then
  if "$BINARY_PATH" version 2>/dev/null; then
    info "Binary works!"
  else
    error "Binary verification failed. Try running: ${BINARY_PATH} version"
    exit 1
  fi
else
  info "[dry-run] Would run: ${BINARY_PATH} version"
fi

# 7. Success message
echo ""
echo "  ✓ ilnamiqui installed for ${TARGET}"
echo "  Binary: ${BINARY_PATH}"
if [ "$TARGET" = "opencode" ]; then
  echo "  Plugin: registered (${OPENCODE_VERSION_RESOLVED})"
  echo "  Config key: ${OPENCODE_CONFIG_KEY}"
  echo "  Skill: installed"
elif [ "$TARGET" = "claude" ]; then
  echo "  MCP server: ${MCP_BINARY_PATH}"
  echo "  Hooks: installed"
  echo "  Skill: installed"
  echo "  Restart Claude Code to activate."
fi

# Warn if ~/.local/bin not in PATH
if ! echo "$PATH" | grep -q "${HOME}/.local/bin"; then
  echo ""
  info "To use ilnamiqui from any terminal, add ~/.local/bin to your PATH:"
  info "  source ~/.bashrc   # (or ~/.zshrc — whichever was configured)"
  info "  or open a new terminal"
fi

echo ""

exit 0
