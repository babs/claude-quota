#!/bin/bash
set -euo pipefail

REPO="babs/claude-quota"
BINARY="claude-quota"
ICON_URL="https://raw.githubusercontent.com/${REPO}/master/img/claude-quota.svg"

for cmd in curl xz; do
  command -v "$cmd" &>/dev/null || { echo "Required command not found: $cmd"; exit 1; }
done

usage() {
  echo "Usage: $0 [--uninstall] [flags for ${BINARY}...]"
  echo "  --uninstall  Remove ${BINARY} and autostart config"
  echo "  Any other flags are passed to ${BINARY} and persisted in autostart config"
  echo ""
  echo "Example: $0 -stats -indicator bar-proj"
  exit 0
}

uninstall() {
  if [ "$OS" = "darwin" ]; then
    launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null && echo "LaunchAgent stopped." || true
    rm -f "$PLIST_PATH" && echo "Removed $PLIST_PATH"
    sudo rm -f "${INSTALL_DIR}/${BINARY}" && echo "Removed ${INSTALL_DIR}/${BINARY}"
  elif [ "$OS" = "linux" ]; then
    pkill -x "$BINARY" 2>/dev/null && echo "Process stopped." || true
    rm -f "$DESKTOP_PATH" && echo "Removed $DESKTOP_PATH"
    rm -f "${INSTALL_DIR}/${BINARY}" "${INSTALL_DIR}/claude-quota.svg" && echo "Removed binary and icon"
    rmdir "$INSTALL_DIR" 2>/dev/null && echo "Removed $INSTALL_DIR" || echo "Note: $INSTALL_DIR not empty, kept in place"
  fi
  echo "Uninstalled."
}

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac
case "$OS" in
  darwin)
    INSTALL_DIR="/usr/local/bin"
    PLIST_LABEL="com.claude-quota"
    PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
    ;;
  linux)
    INSTALL_DIR="$HOME/.local/share/claude-quota"
    DESKTOP_PATH="$HOME/.config/autostart/claude-quota.desktop"
    ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

UNINSTALL=0
BINARY_FLAGS=""
for arg in "$@"; do
  case "$arg" in
    --uninstall) UNINSTALL=1 ;;
    -h|--help)   usage ;;
    *) BINARY_FLAGS="${BINARY_FLAGS:+$BINARY_FLAGS }$arg" ;;
  esac
done

[ "$UNINSTALL" = "1" ] && { uninstall; exit 0; }

# Fetch latest release tag
echo "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -1 | cut -d'"' -f4)
[ -z "$LATEST" ] && { echo "Failed to fetch latest release."; exit 1; }
echo "Latest: $LATEST"

# Download, extract, install
ASSET="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${LATEST}/${ASSET}.xz"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "Downloading $URL..."
curl -fsSL "$URL" -o "$TMP/${ASSET}.xz"
xz -d "$TMP/${ASSET}.xz"
chmod +x "$TMP/${ASSET}"
mkdir -p "$INSTALL_DIR"
if [ "$OS" = "darwin" ]; then
  sudo mv "$TMP/${ASSET}" "${INSTALL_DIR}/${BINARY}"
else
  mv "$TMP/${ASSET}" "${INSTALL_DIR}/${BINARY}"
fi
echo "Installed: ${INSTALL_DIR}/${BINARY}"

# Configure autostart
if [ "$OS" = "darwin" ]; then
  mkdir -p "$(dirname "$PLIST_PATH")"
  PLIST_ARGS="        <string>${INSTALL_DIR}/${BINARY}</string>"
  for flag in $BINARY_FLAGS; do
    PLIST_ARGS="${PLIST_ARGS}
        <string>${flag}</string>"
  done
  cat > "$PLIST_PATH" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
${PLIST_ARGS}
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/claude-quota.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/claude-quota.log</string>
</dict>
</plist>
EOF
  # Reload (handles both first install and update)
  launchctl bootout "gui/$(id -u)/${PLIST_LABEL}" 2>/dev/null || true
  launchctl bootstrap "gui/$(id -u)" "$PLIST_PATH"
  echo "LaunchAgent installed and started."

elif [ "$OS" = "linux" ]; then
  # Download icon
  ICON_PATH="${INSTALL_DIR}/claude-quota.svg"
  echo "Downloading icon..."
  curl -fsSL "$ICON_URL" -o "$ICON_PATH" || echo "Warning: icon download failed, continuing without icon"

  mkdir -p "$(dirname "$DESKTOP_PATH")"
  cat > "$DESKTOP_PATH" << EOF
[Desktop Entry]
Type=Application
Name=Claude Quota Widget
Exec=${INSTALL_DIR}/${BINARY}${BINARY_FLAGS:+ $BINARY_FLAGS}
Icon=${ICON_PATH}
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
StartupNotify=false
Terminal=false
EOF
  echo "Autostart entry installed: $DESKTOP_PATH"
  # Restart if already running, otherwise start
  pkill -x "$BINARY" 2>/dev/null || true
  if command -v gio &>/dev/null; then
    gio launch "$DESKTOP_PATH"
  elif command -v gtk-launch &>/dev/null; then
    gtk-launch claude-quota
  elif command -v dex &>/dev/null; then
    dex "$DESKTOP_PATH"
  else
    # intentional word splitting
    # shellcheck disable=SC2086
    nohup "${INSTALL_DIR}/${BINARY}" $BINARY_FLAGS &>/dev/null &
  fi
  echo "Started."
fi

echo "Done. Run '${BINARY} -version' to verify."
