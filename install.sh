#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This installer only supports macOS. Download the Windows binary from:"
  echo "https://github.com/vsangava/sentinel/releases/latest"
  exit 1
fi

ARCH=$(uname -m)
case "$ARCH" in
  arm64)  BINARY="sentinel-macos-arm64" ;;
  x86_64) BINARY="sentinel-macos-amd64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "Fetching latest release info..."
URL=$(curl -fsSL https://api.github.com/repos/vsangava/sentinel/releases/latest \
  | grep browser_download_url \
  | grep "\"$BINARY\"" \
  | cut -d'"' -f4)

if [[ -z "$URL" ]]; then
  echo "Could not find a release asset for $BINARY. Check https://github.com/vsangava/sentinel/releases/latest"
  exit 1
fi

echo "Downloading $BINARY..."
TMP=$(mktemp)
curl -fsSL -o "$TMP" "$URL"

# Remove Gatekeeper quarantine flag (no-op if not set).
xattr -d com.apple.quarantine "$TMP" 2>/dev/null || true

chmod +x "$TMP"

echo "Installing..."
sudo "$TMP" --setup

rm -f "$TMP"
