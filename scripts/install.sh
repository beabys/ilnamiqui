#!/bin/bash
set -euo pipefail

# ilnamiqui installer
# Usage: curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --version v0.1.0
#        curl -fsSL https://raw.githubusercontent.com/beabys/ilnamiqui/main/scripts/install.sh | bash -s -- --dry-run

# ─── Config ───────────────────────────────────────────────────────────────────
REPO_OWNER="beabys"
REPO_NAME="ilnamiqui"
RAW_BASE="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main"
GH_API="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest"

# Defaults
VERSION=""
DRY_RUN=false

# ─── Parse flags ──────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    *)
      echo "Usage: $0 [--version VERSION] [--dry-run]"
      exit 1
      ;;
  esac
done

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

# ─── Resolve version ──────────────────────────────────────────────────────────
if [ -z "$VERSION" ]; then
  # Fetch latest version from GitHub API
  if command -v curl &>/dev/null; then
    VERSION=$(curl -fsSL "$GH_API" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)",.*/\1/') || true
  elif command -v wget &>/dev/null; then
    VERSION=$(wget -qO- "$GH_API" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": "\(.*\)",.*/\1/') || true
  fi

  if [ -z "$VERSION" ]; then
    echo "ERROR: Could not determine latest version. Set VERSION env var or use --version."
    echo "  curl -fsSL ... | VERSION=v0.1.0 bash"
    echo "  curl -fsSL ... | bash -s -- --version v0.1.0"
    exit 1
  fi
fi

# ─── Paths ─────────────────────────────────────────────────────────────────────
BIN_DIR="${HOME}/.config/opencode/plugins/ilnamiqui"
SKILL_DIR="${HOME}/.config/opencode/skills/ilnamiqui"
PLUGIN_DIR="${HOME}/.config/opencode/plugins"
OPENCODE_CONFIG="${HOME}/.config/opencode/opencode.json"

BINARY_NAME="ilnamiqui-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
  BINARY_NAME="${BINARY_NAME}.exe"
fi
BINARY_PATH="${BIN_DIR}/${BINARY_NAME}"
SKILL_PATH="${SKILL_DIR}/SKILL.md"
PLUGIN_PATH="${PLUGIN_DIR}/ilnamiqui.ts"
PLUGIN_REPO_PATH="${RAW_BASE}/plugin/ilnamiqui.ts"
SKILL_REPO_PATH="${RAW_BASE}/skill/SKILL.md"

# Archive URL — GoReleaser produces archives, not raw binaries
ARCHIVE_EXT=".tar.gz"
BINARY_IN_ARCHIVE="ilnamiqui"
if [ "$OS" = "windows" ]; then
  ARCHIVE_EXT=".zip"
  BINARY_IN_ARCHIVE="${BINARY_IN_ARCHIVE}.exe"
fi
DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${VERSION}/ilnamiqui-${OS}-${ARCH}${ARCHIVE_EXT}"

# ─── Helpers ───────────────────────────────────────────────────────────────────
info()  { echo "  $1"; }
action() { echo "  → $1"; }
error() { echo "  ERROR: $1" >&2; }

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

# ─── Main ──────────────────────────────────────────────────────────────────────
echo ""
echo "  ilnamiqui installer"
echo "  Version: $VERSION"
echo "  OS: $OS  Arch: $ARCH"
echo ""

# 1. Create directories
action "Creating directories..."
mkdir -p "$BIN_DIR"
mkdir -p "$SKILL_DIR"
mkdir -p "$PLUGIN_DIR"
info "  ${BIN_DIR}/"
info "  ${SKILL_DIR}/"
info "  ${PLUGIN_DIR}/"

# 2. Download and extract binary archive
echo ""
if [ "$DRY_RUN" = true ]; then
  action "[dry-run] Would download archive: ${DOWNLOAD_URL}"
  info "[dry-run] Would extract and install binary to: ${BINARY_PATH}"
  info "[dry-run] Would create symlink: ${BINARY_PATH} → ${HOME}/.local/bin/ilnamiqui"
else
  action "Downloading archive..."
  TMP_DIR=$(mktemp -d /tmp/ilnamiqui.XXXXXXXX)
  ARCHIVE_PATH="${TMP_DIR}/ilnamiqui${ARCHIVE_EXT}"
  download "$DOWNLOAD_URL" "$ARCHIVE_PATH" "archive (${OS}/${ARCH})"

  action "Extracting binary..."
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

  if [ ! -f "${TMP_DIR}/${BINARY_IN_ARCHIVE}" ]; then
    error "Binary '${BINARY_IN_ARCHIVE}' not found in archive."
    rm -rf "$TMP_DIR"
    exit 1
  fi

  mv "${TMP_DIR}/${BINARY_IN_ARCHIVE}" "$BINARY_PATH"
  chmod +x "$BINARY_PATH"
  rm -rf "$TMP_DIR"
  info "Binary installed to ${BINARY_PATH}"

  # Create symlink in ~/.local/bin for CLI access
  mkdir -p "${HOME}/.local/bin"
  ln -sf "$BINARY_PATH" "${HOME}/.local/bin/ilnamiqui"
  info "Symlink created: ${HOME}/.local/bin/ilnamiqui → ${BINARY_PATH}"
fi

# 3. Download SKILL.md
echo ""
action "Downloading SKILL.md..."
download "$SKILL_REPO_PATH" "$SKILL_PATH" "SKILL.md"

# 4. Download plugin.ts
echo ""
action "Downloading plugin.ts..."
download "$PLUGIN_REPO_PATH" "$PLUGIN_PATH" "plugin.ts"

# 5. Config injection
echo ""
action "Updating opencode.json..."

if [ "$DRY_RUN" = true ]; then
  info "[dry-run] Would update ${OPENCODE_CONFIG} to include plugin: ${PLUGIN_PATH}"
else
  # Read or init config
  if [ ! -f "$OPENCODE_CONFIG" ]; then
    echo '{}' > "$OPENCODE_CONFIG"
    info "Created ${OPENCODE_CONFIG}"
  fi

  # Safe JSON update: add plugin if not present
  # Try jq first, then node, then python3
  PLUGIN_ENTRY="./plugins/ilnamiqui.ts"

  if command -v jq &>/dev/null; then
    # jq approach
    tmp=$(mktemp /tmp/ilnamiqui.XXXXXXXX)
    if jq --arg p "$PLUGIN_ENTRY" '
      if (.plugin // false) then
        if (.plugin | index($p)) then . else .plugin += [$p] end
      else
        .plugin = [$p]
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
      const path = require('path');
      const cfgPath = '${OPENCODE_CONFIG}';
      let cfg = {};
      try { cfg = JSON.parse(fs.readFileSync(cfgPath, 'utf-8')); } catch(e) {}
      const entry = '${PLUGIN_ENTRY}';
      if (!cfg.plugin) cfg.plugin = [];
      if (!cfg.plugin.includes(entry)) cfg.plugin.push(entry);
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
except (FileNotFoundError, json.JSONDecodeError):
    cfg = {}
entry = '${PLUGIN_ENTRY}'
if 'plugin' not in cfg:
    cfg['plugin'] = []
if entry not in cfg['plugin']:
    cfg['plugin'].append(entry)
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
    error "Add this manually to ${OPENCODE_CONFIG}:"
    error '  "plugin": ["./plugins/ilnamiqui.ts"]'
    exit 1
  fi
fi

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
echo "  ✓ ilnamiqui installed at ${BINARY_PATH}"
echo "  Restart opencode to activate."
echo ""

exit 0
