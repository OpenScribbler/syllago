#!/bin/sh
# Install syllago — AI coding tool content manager
# Usage: curl -fsSL https://raw.githubusercontent.com/OpenScribbler/syllago/main/install.sh | sh
# Or:    INSTALL_DIR=/usr/local/bin sh install.sh
set -e

REPO="OpenScribbler/syllago"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  ;;
  darwin) ;;
  msys*|mingw*|cygwin*) OS="windows" ;;
  *) echo "Error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

BINARY="syllago-${OS}-${ARCH}"
if [ "$OS" = "windows" ]; then
  BINARY="${BINARY}.exe"
fi

echo "Detected: ${OS}/${ARCH}"

# Fetch latest release version
echo "Fetching latest syllago release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "Error: could not determine latest version" >&2
  exit 1
fi

echo "Latest version: ${LATEST}"

BASE_URL="https://github.com/${REPO}/releases/download/${LATEST}"

# Download binary and checksums to temp dir
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${BINARY}..."
curl -fsSL "${BASE_URL}/${BINARY}" -o "${TMP_DIR}/${BINARY}"
curl -fsSL "${BASE_URL}/checksums.txt" -o "${TMP_DIR}/checksums.txt"

# Verify checksum
echo "Verifying checksum..."
EXPECTED=$(grep "${BINARY}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED" ]; then
  echo "Error: no checksum found for ${BINARY}" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL=$(sha256sum "${TMP_DIR}/${BINARY}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL=$(shasum -a 256 "${TMP_DIR}/${BINARY}" | awk '{print $1}')
else
  echo "Warning: no sha256sum or shasum found, skipping checksum verification" >&2
  ACTUAL="$EXPECTED"
fi

if [ "$ACTUAL" != "$EXPECTED" ]; then
  echo "Error: checksum mismatch!" >&2
  echo "  Expected: ${EXPECTED}" >&2
  echo "  Got:      ${ACTUAL}" >&2
  exit 1
fi

echo "Checksum verified."

# Install
mkdir -p "$INSTALL_DIR"
DEST="${INSTALL_DIR}/syllago"
if [ "$OS" = "windows" ]; then
  DEST="${INSTALL_DIR}/syllago.exe"
fi

cp "${TMP_DIR}/${BINARY}" "$DEST"
chmod 755 "$DEST"

# Create short alias symlink
SYLL_DEST="${INSTALL_DIR}/syll"
if [ "$OS" = "windows" ]; then
  SYLL_DEST="${INSTALL_DIR}/syll.exe"
fi
ln -sf "$(basename "$DEST")" "$SYLL_DEST"

echo "Installed syllago to ${DEST} (also available as 'syll')"

# PATH guidance
case ":$PATH:" in
  *":${INSTALL_DIR}:"*)
    ;;
  *)
    echo ""
    echo "Note: ${INSTALL_DIR} is not on your PATH."
    echo "Add this to your shell config (~/.bashrc, ~/.zshrc, etc.):"
    echo ""
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo ""
    ;;
esac

echo "Done! Run: syllago --help"
