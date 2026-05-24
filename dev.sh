#!/usr/bin/env bash
set -euo pipefail

# AI LocalBase - Local Dev Launcher
# Starts qdrant (if not running), then backend in background, then frontend in foreground.
# Ctrl+C exits cleanly, killing backend along with frontend.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_LOG="${BACKEND_LOG:-/tmp/knowport-backend.log}"
FRONTEND_PORT="${FRONTEND_PORT:-5174}"

# Pick first non-loopback, non-docker-bridge IPv4 (override via HOST_IP env)
HOST_IP="${HOST_IP:-$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+\.' | grep -vE '^(127\.|172\.(1[6-9]|2[0-9]|3[01])\.)' | head -1)}"
HOST_IP="${HOST_IP:-127.0.0.1}"

# Make `go` discoverable even when /usr/local/go/bin is not on PATH
export PATH="/usr/local/go/bin:$PATH"

# CN mirrors for first-time installs (no-op if already faster)
export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"

# ---------- Qdrant ----------
# Dev mode only needs Qdrant from the main compose stack; backend/frontend run locally.
if ! docker ps --format '{{.Names}}' | grep -qE '^(qdrant|knowport-qdrant|ai-localbase-qdrant)$'; then
  echo "[dev.sh] Qdrant not running, starting via docker-compose.yml (service: qdrant)..."
  (cd "$ROOT_DIR" && docker compose up -d qdrant)
  sleep 3
else
  echo "[dev.sh] Qdrant already running"
fi

# ---------- Backend (background) ----------
echo "[dev.sh] Starting backend (go run .) -> $BACKEND_LOG"
(
  cd "$ROOT_DIR/backend"
  exec go run . > "$BACKEND_LOG" 2>&1
) &
BACKEND_PID=$!

cleanup() {
  echo ""
  echo "[dev.sh] Stopping backend (PID $BACKEND_PID)..."
  kill "$BACKEND_PID" 2>/dev/null || true
  wait "$BACKEND_PID" 2>/dev/null || true
  echo "[dev.sh] Backend stopped. Qdrant container left running."
}
trap cleanup EXIT INT TERM

# ---------- Wait for backend health ----------
echo "[dev.sh] Waiting for backend health (up to 60s, first run downloads Go modules)..."
for i in $(seq 1 60); do
  if curl -sf http://localhost:8080/health >/dev/null 2>&1; then
    echo "[dev.sh] Backend ready (took ${i}s)"
    break
  fi
  if [ "$i" = "60" ]; then
    echo "[dev.sh] Backend not healthy in 60s. Tail of $BACKEND_LOG:"
    tail -20 "$BACKEND_LOG" || true
    exit 1
  fi
  sleep 1
done

# ---------- Banner ----------
echo ""
echo "================================================"
echo " AI LocalBase - Local Dev (ctrl+c to stop)"
echo "================================================"
echo " Frontend : http://localhost:$FRONTEND_PORT"
echo "            http://$HOST_IP:$FRONTEND_PORT"
echo " Backend  : http://localhost:8080"
echo "            http://$HOST_IP:8080"
echo " Qdrant   : http://localhost:6333"
echo "            http://$HOST_IP:6333"
echo " Backend log : $BACKEND_LOG"
echo "================================================"
echo ""

# ---------- Frontend (foreground) ----------
cd "$ROOT_DIR/frontend"
if [ ! -d node_modules ]; then
  echo "[dev.sh] Installing frontend dependencies (first run)..."
  npm install --registry=https://registry.npmmirror.com
fi
exec npm run dev -- --port "$FRONTEND_PORT" --strictPort --host 0.0.0.0
