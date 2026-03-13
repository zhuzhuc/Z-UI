#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONT_DIR="$ROOT_DIR/front"
if [[ -d "$ROOT_DIR/front/dist" ]]; then
  FRONT_DIR="$ROOT_DIR/front/dist"
fi

BACKEND_PORT="${BACKEND_PORT:-}"
FRONT_PORT="${FRONT_PORT:-}"
DB_PATH="${ZUI_DB:-$BACKEND_DIR/zui-local.db}"
XRAY_CONFIG="${XRAY_CONFIG:-$BACKEND_DIR/runtime/xray-config.json}"
XRAY_ACCESS_LOG="${XRAY_ACCESS_LOG:-$BACKEND_DIR/runtime/xray-access.log}"
XRAY_ERROR_LOG="${XRAY_ERROR_LOG:-$BACKEND_DIR/runtime/xray-error.log}"

if [[ -z "${XRAY_BIN:-}" ]]; then
  if [[ -x "$ROOT_DIR/bin/xray" ]]; then
    XRAY_BIN="$ROOT_DIR/bin/xray"
  else
    XRAY_BIN="xray"
  fi
fi

cleanup() {
  echo ""
  echo "[z-ui] stopping services..."
  if [[ -n "${BACKEND_PID:-}" ]] && kill -0 "$BACKEND_PID" 2>/dev/null; then
    kill "$BACKEND_PID" 2>/dev/null || true
  fi
  if [[ -n "${FRONT_PID:-}" ]] && kill -0 "$FRONT_PID" 2>/dev/null; then
    kill "$FRONT_PID" 2>/dev/null || true
  fi
}

port_in_use() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"$port" -sTCP:LISTEN -n -P >/dev/null 2>&1
    return $?
  fi
  return 1
}

pick_port() {
  local candidates=("$@")
  local port
  for port in "${candidates[@]}"; do
    if ! port_in_use "$port"; then
      echo "$port"
      return 0
    fi
  done
  return 1
}

trap cleanup INT TERM EXIT

if [[ -z "$BACKEND_PORT" ]]; then
  BACKEND_PORT="$(pick_port 8081 8080 18081)"
fi

if [[ -z "$FRONT_PORT" ]]; then
  FRONT_PORT="$(pick_port 5500 5173 4173 3000)"
fi

echo "[z-ui] starting backend on :$BACKEND_PORT"
echo "[z-ui] using xray bin: $XRAY_BIN"
echo "[z-ui] xray config path: $XRAY_CONFIG"
echo "[z-ui] xray access log: $XRAY_ACCESS_LOG"
echo "[z-ui] xray error  log: $XRAY_ERROR_LOG"
mkdir -p "$(dirname "$XRAY_CONFIG")"
(
  cd "$BACKEND_DIR"
  PORT="$BACKEND_PORT" ZUI_DB="$DB_PATH" XRAY_CONTROL=process XRAY_BIN="$XRAY_BIN" XRAY_CONFIG="$XRAY_CONFIG" XRAY_ACCESS_LOG="$XRAY_ACCESS_LOG" XRAY_ERROR_LOG="$XRAY_ERROR_LOG" go run .
) &
BACKEND_PID=$!

echo "[z-ui] starting frontend on :$FRONT_PORT"
(
  cd "$FRONT_DIR"
  python3 -m http.server "$FRONT_PORT"
) &
FRONT_PID=$!

echo "[z-ui] frontend: http://127.0.0.1:$FRONT_PORT/login.html"
echo "[z-ui] backend : http://127.0.0.1:$BACKEND_PORT/api/v1/health"
echo "[z-ui] press Ctrl+C to stop"

wait "$BACKEND_PID" "$FRONT_PID"
