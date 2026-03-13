#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:-v0.1.0}"
TARGET_OS="${2:-linux}"
TARGET_ARCH="${3:-amd64}"

PACKAGE_NAME="z-ui-${VERSION}-${TARGET_OS}-${TARGET_ARCH}"
OUT_DIR="$ROOT_DIR/release/$PACKAGE_NAME"
ARCHIVE_PATH="$ROOT_DIR/release/${PACKAGE_NAME}.tar.gz"
SHA_PATH="$ROOT_DIR/release/${PACKAGE_NAME}.sha256"

echo "[download] version : $VERSION"
echo "[download] target  : $TARGET_OS/$TARGET_ARCH"
echo "[download] out dir : $OUT_DIR"

env GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" "$ROOT_DIR/scripts/build-release.sh" "$OUT_DIR"

echo "[download] creating tar.gz"
tar -czf "$ARCHIVE_PATH" -C "$ROOT_DIR/release" "$PACKAGE_NAME"

if command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "$ARCHIVE_PATH" > "$SHA_PATH"
elif command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$ARCHIVE_PATH" > "$SHA_PATH"
fi

echo "[download] archive: $ARCHIVE_PATH"
if [[ -f "$SHA_PATH" ]]; then
  echo "[download] sha256 : $SHA_PATH"
fi

