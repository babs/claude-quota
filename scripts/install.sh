#!/bin/bash
set -euo pipefail

REPO="babs/claude-quota"
BINARY="claude-quota"
INSTALL_DIR="/usr/local/bin"
PLIST_LABEL="com.claude-quota"
PLIST_PATH="$HOME/Library/LaunchAgents/${PLIST_LABEL}.plist"
DESKTOP_PATH="$HOME/.config/autostart/claude-quota.desktop"

usage() {
  echo "Usage: $0 [--uninstall]"
  exit 1
}

uninstall() {
  if [ "$OS" = "darwin" ]; then
    launchctl unload "$PLIST_PATH" 2>/dev/null && echo "LaunchAgent stopped." || true
    rm -f "$PLIST_PATH" && echo "Removed $PLIST_PATH"
  elif [ "$OS" = "linux" ]; then
    rm -f "$DESKTOP_PATH" && echo "Removed $DESKTOP_PATH"
    pkill -x "$BINARY" 2>/dev/null && echo "Process stopped." || true
  fi
  sudo rm -f "${INSTALL_DIR}/${BINARY}" && echo "Removed ${INSTALL_DIR}/${BINARY}"
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
  darwin|linux) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

UNINSTALL=0
for arg in "$@"; do
  case "$arg" in
    --uninstall) UNINSTALL=1 ;;
    *) usage ;;
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
sudo mv "$TMP/${ASSET}" "${INSTALL_DIR}/${BINARY}"
echo "Installed: ${INSTALL_DIR}/${BINARY}"

# Configure autostart
if [ "$OS" = "darwin" ]; then
  mkdir -p "$(dirname "$PLIST_PATH")"
  cat > "$PLIST_PATH" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${PLIST_LABEL}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY}</string>
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
  launchctl unload "$PLIST_PATH" 2>/dev/null || true
  launchctl load "$PLIST_PATH"
  echo "LaunchAgent installed and started."

elif [ "$OS" = "linux" ]; then
  mkdir -p "$(dirname "$DESKTOP_PATH")"
  cat > "$DESKTOP_PATH" << EOF
[Desktop Entry]
Type=Application
Name=Claude Quota Widget
Exec=${INSTALL_DIR}/${BINARY}
Hidden=false
NoDisplay=false
X-GNOME-Autostart-enabled=true
StartupNotify=false
Terminal=false
EOF
  echo "Autostart entry installed: $DESKTOP_PATH"
  # Restart if already running, otherwise start
  pkill -x "$BINARY" 2>/dev/null || true
  nohup "${INSTALL_DIR}/${BINARY}" &>/dev/null &
  echo "Started."
fi

echo "Done. Run '${BINARY} -version' to verify."
