#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
OUT_DIR="${1:-$ROOT_DIR/release/z-ui-$(date +%Y%m%d-%H%M%S)}"
GOCACHE_DIR="${GOCACHE:-$ROOT_DIR/.gocache-release}"

if [[ "$OUT_DIR" != /* ]]; then
  OUT_DIR="$ROOT_DIR/$OUT_DIR"
fi

GOOS_TARGET="${GOOS:-}"
GOARCH_TARGET="${GOARCH:-}"

echo "[release] root: $ROOT_DIR"
echo "[release] out : $OUT_DIR"
echo "[release] gocache: $GOCACHE_DIR"

mkdir -p "$OUT_DIR/backend" "$OUT_DIR/front" "$OUT_DIR/ops" "$OUT_DIR/runtime" "$OUT_DIR/bin" "$OUT_DIR/data"
mkdir -p "$GOCACHE_DIR"

echo "[release] building frontend"
(
  cd "$ROOT_DIR/front"
  npm run build
)
cp -R "$ROOT_DIR/front/dist" "$OUT_DIR/front/dist"

echo "[release] building backend"
(
  cd "$ROOT_DIR/backend"
  if [[ -n "$GOOS_TARGET" && -n "$GOARCH_TARGET" ]]; then
    env GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" CGO_ENABLED=0 GOCACHE="$GOCACHE_DIR" go build -o "$OUT_DIR/backend/z-ui" .
  else
    env CGO_ENABLED=0 GOCACHE="$GOCACHE_DIR" go build -o "$OUT_DIR/backend/z-ui" .
  fi
)

cp "$ROOT_DIR/start.sh" "$OUT_DIR/start.sh"
cp "$ROOT_DIR/install-xray.sh" "$OUT_DIR/install-xray.sh"
cp "$ROOT_DIR/ops/README.md" "$OUT_DIR/ops/README.md"
cp "$ROOT_DIR/ops/z-ui.service" "$OUT_DIR/ops/z-ui.service"
cp "$ROOT_DIR/ops/nginx-z-ui.conf" "$OUT_DIR/ops/nginx-z-ui.conf"
cp "$ROOT_DIR/ops/z-ui.env.example" "$OUT_DIR/ops/z-ui.env.example"

chmod +x "$OUT_DIR/start.sh" "$OUT_DIR/install-xray.sh" "$OUT_DIR/backend/z-ui"

echo "[release] done"
echo "[release] package directory: $OUT_DIR"
