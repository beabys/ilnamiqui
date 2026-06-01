#!/bin/bash
set -euo pipefail

# ilnamiqui installer
# Usage: curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --target opencode
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --target claude
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --version v1.0.0
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
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    *)
      echo "Usage: $0 [--target opencode|claude] [--version VERSION] [--dry-run]"
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
BIN_DIR="${HOME}/.config/opencode/plugins/ilnamiqui"
SYMLINK_PATH="${HOME}/.local/bin/ilnamiqui"
OPENCODE_CONFIG="${HOME}/.config/opencode/opencode.json"
CLAUDE_CONFIG="${HOME}/.claude/claude.json"
CLAUDE_SKILL_DIR="${HOME}/.claude/skills/ilnamiqui"
CLAUDE_SKILL_PATH="${CLAUDE_SKILL_DIR}/CLAUDE.md"
OPENCODE_SKILL_DIR="${HOME}/.config/opencode/skills/ilnamiqui"
OPENCODE_PLUGIN_DIR="${HOME}/.config/opencode/plugins"

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
OPENCODE_PLUGIN_REPO_PATH="${RAW_BASE}/opencode/plugin/ilnamiqui.ts"
CLAUDE_SKILL_REPO_PATH="${RAW_BASE}/claude/skill/CLAUDE.md"

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
" 2>/dev/null || {
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

# ─── Main ──────────────────────────────────────────────────────────────────────
echo ""
echo "  ilnamiqui installer"
echo "  Version: $VERSION"
echo "  OS: $OS  Arch: $ARCH"
echo "  Target: $TARGET"
echo ""

# ── Steps common to both targets ──────────────────────────────────────────────

# 1. Create directories
action "Creating directories..."
mkdir -p "$BIN_DIR"
info "  ${BIN_DIR}/"

# 2. Download and extract binary archive (contains both binaries)
echo ""
if [ "$DRY_RUN" = true ]; then
  action "[dry-run] Would download archive: ${DOWNLOAD_URL}"
  info "[dry-run] Would extract and install binary to: ${BINARY_PATH}"
  info "[dry-run] Would create symlink: ${SYMLINK_PATH}"
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

  # Create symlink in ~/.local/bin for CLI access
  mkdir -p "${HOME}/.local/bin"
  ln -sf "$BINARY_PATH" "$SYMLINK_PATH"
  info "Symlink created: ${SYMLINK_PATH} → ${BINARY_PATH}"
fi

# ── Target-specific steps ──────────────────────────────────────────────────────
case "$TARGET" in
  opencode)
    # 3. Download SKILL.md
    echo ""
    action "Downloading opencode skill..."
    mkdir -p "$OPENCODE_SKILL_DIR"
    download "$OPENCODE_SKILL_REPO_PATH" "${OPENCODE_SKILL_DIR}/SKILL.md" "SKILL.md"

    # 4. Download plugin.ts
    echo ""
    action "Downloading plugin..."
    mkdir -p "$OPENCODE_PLUGIN_DIR"
    download "$OPENCODE_PLUGIN_REPO_PATH" "${OPENCODE_PLUGIN_DIR}/ilnamiqui.ts" "plugin.ts"

    # 5. Register plugin in opencode.json
    echo ""
    action "Updating opencode.json..."
    if [ ! -f "$OPENCODE_CONFIG" ]; then
      echo '{}' > "$OPENCODE_CONFIG"
      info "Created ${OPENCODE_CONFIG}"
    fi

    PLUGIN_ENTRY="./plugins/ilnamiqui.ts"
    update_json "$OPENCODE_CONFIG" \
      ".plugin |= (. // [])" \
      "data.setdefault('plugin', []); entry = '${PLUGIN_ENTRY}'; if entry not in data['plugin']: data['plugin'].append(entry)"
    # jq version uses a safer approach above; fallback handled in update_json
    if command -v jq &>/dev/null; then
      update_json "$OPENCODE_CONFIG" \
        "if (.plugin // false) then if (.plugin | index(\"${PLUGIN_ENTRY}\")) then . else .plugin += [\"${PLUGIN_ENTRY}\"] end else .plugin = [\"${PLUGIN_ENTRY}\"] end" \
        ""
    fi
    ;;

  claude)
    # 3. Download CLAUDE.md
    echo ""
    action "Downloading Claude Code skill..."
    mkdir -p "$CLAUDE_SKILL_DIR"
    download "$CLAUDE_SKILL_REPO_PATH" "$CLAUDE_SKILL_PATH" "CLAUDE.md"

    # 4. Register MCP server in claude.json
    echo ""
    action "Updating claude.json..."
    if [ ! -f "$CLAUDE_CONFIG" ]; then
      mkdir -p "$(dirname "$CLAUDE_CONFIG")"
      echo '{}' > "$CLAUDE_CONFIG"
      info "Created ${CLAUDE_CONFIG}"
    fi

    MCP_COMMAND="$MCP_BINARY_PATH"
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
" 2>/dev/null || {
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
  echo "  Restart opencode to activate."
elif [ "$TARGET" = "claude" ]; then
  echo "  MCP server: ${MCP_BINARY_PATH}"
  echo "  Restart Claude Code to activate."
fi
echo ""

exit 0
