#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <xray-version-without-v> [install-dir]"
  echo "example: $0 1.8.24 ./bin"
  exit 1
fi

VERSION="$1"
ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
INSTALL_DIR="${2:-$ROOT_DIR/bin}"

OS_RAW="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH_RAW="$(uname -m)"

case "$ARCH_RAW" in
  x86_64|amd64) ARCH="64" ;;
  aarch64|arm64) ARCH="arm64-v8a" ;;
  *)
    echo "unsupported architecture: $ARCH_RAW"
    exit 1
    ;;
esac

if [[ "$OS_RAW" != "linux" && "$OS_RAW" != "darwin" ]]; then
  echo "unsupported os: $OS_RAW"
  exit 1
fi

if [[ "$OS_RAW" == "darwin" ]]; then
  OS_TAG="macos"
else
  OS_TAG="linux"
fi

if [[ "$ARCH" == "arm64-v8a" ]]; then
  PKG_CANDIDATES=(
    "Xray-${OS_TAG}-arm64-v8a.zip"
    "Xray-${OS_TAG}-arm64.zip"
  )
else
  PKG_CANDIDATES=(
    "Xray-${OS_TAG}-64.zip"
    "Xray-${OS_TAG}-amd64.zip"
  )
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

DOWNLOADED_PKG=""
for PKG in "${PKG_CANDIDATES[@]}"; do
  URL="https://github.com/XTLS/Xray-core/releases/download/v${VERSION}/${PKG}"
  echo "[xray] trying: $URL"
  if curl -fL "$URL" -o "$TMP_DIR/$PKG"; then
    DOWNLOADED_PKG="$PKG"
    break
  fi
done

if [[ -z "$DOWNLOADED_PKG" ]]; then
  echo "[xray] download failed for version v${VERSION}"
  echo "[xray] tried packages:"
  for one in "${PKG_CANDIDATES[@]}"; do
    echo "  - $one"
  done
  exit 1
fi

echo "[xray] extracting package"
if command -v unzip >/dev/null 2>&1; then
  unzip -q "$TMP_DIR/$DOWNLOADED_PKG" -d "$TMP_DIR"
else
  bsdtar -xf "$TMP_DIR/$DOWNLOADED_PKG" -C "$TMP_DIR"
fi

mkdir -p "$INSTALL_DIR"
install -m 0755 "$TMP_DIR/xray" "$INSTALL_DIR/xray"

echo "[xray] installed: $INSTALL_DIR/xray"
"$INSTALL_DIR/xray" version || true
