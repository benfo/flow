#!/bin/sh
# install.sh — download and install the latest flow binary
# Usage:
#   curl -sf https://raw.githubusercontent.com/benfo/flow/main/install.sh | sh
#   curl -sf .../install.sh | sh -s -- --pre-release   # include pre-releases
#   INSTALL_DIR=/usr/local/bin sh install.sh            # override install location
set -e

REPO="benfo/flow"
BINARY="flow"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
PRE_RELEASE=false

# ── Parse flags ──────────────────────────────────────────────────────────────
for arg in "$@"; do
  case "$arg" in
    --pre-release) PRE_RELEASE=true ;;
  esac
done

# ── Detect OS ────────────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux"  ;;
  darwin) OS="darwin" ;;
  *) echo "error: unsupported OS: $OS" >&2; exit 1 ;;
esac

# ── Detect architecture ──────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "error: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# ── Resolve version ──────────────────────────────────────────────────────────
if [ -z "$VERSION" ]; then
  if [ "$PRE_RELEASE" = "true" ]; then
    # Pick the first entry from all releases (includes pre-releases).
    VERSION=$(curl -sf "https://api.github.com/repos/${REPO}/releases" \
      | grep '"tag_name"' | head -1 \
      | sed -E 's/.*"v?([^"]+)".*/\1/')
  else
    # Try stable release first.
    VERSION=$(curl -sf "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"v?([^"]+)".*/\1/')
    # No stable release yet — fall back to the latest pre-release automatically.
    if [ -z "$VERSION" ]; then
      echo "  No stable release found, falling back to latest pre-release..."
      VERSION=$(curl -sf "https://api.github.com/repos/${REPO}/releases" \
        | grep '"tag_name"' | head -1 \
        | sed -E 's/.*"v?([^"]+)".*/\1/')
    fi
  fi
fi

if [ -z "$VERSION" ]; then
  echo "error: could not determine a release version. Set VERSION=x.y.z to override." >&2
  exit 1
fi

# ── Download ─────────────────────────────────────────────────────────────────
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE}"

echo "  Downloading flow v${VERSION} (${OS}/${ARCH})..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -sfL "$URL" -o "$TMP/$ARCHIVE"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP"

# ── Install ──────────────────────────────────────────────────────────────────
if [ ! -w "$INSTALL_DIR" ]; then
  echo "  $INSTALL_DIR is not writable — trying with sudo..."
  sudo install -m755 "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
else
  install -m755 "$TMP/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "  ✓ Installed flow v${VERSION} → $INSTALL_DIR/$BINARY"
