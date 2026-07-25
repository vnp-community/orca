#!/bin/bash
# ============================================================
# health-check.sh — Kiểm tra sức khỏe Orca Server
# ============================================================
# Usage:
#   ./health-check.sh                          → check localhost
#   ./health-check.sh --host 172.20.2.39 --port 6769
#   ./health-check.sh --metrics               → print Prometheus metrics
#   ./health-check.sh --watch                 → watch mode (5s interval)
# ============================================================

set -euo pipefail

HOST="${ORCA_HOST:-localhost}"
PORT="${ORCA_HTTP_PORT:-6769}"
SHOW_METRICS=false
WATCH=false
WATCH_INTERVAL=5

# Parse args
while [[ $# -gt 0 ]]; do
  case "$1" in
    --host)     HOST="$2";           shift 2 ;;
    --port)     PORT="$2";           shift 2 ;;
    --metrics)  SHOW_METRICS=true;   shift ;;
    --watch)    WATCH=true;          shift ;;
    --interval) WATCH_INTERVAL="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: $0 [--host HOST] [--port PORT] [--metrics] [--watch]"
      exit 0 ;;
    *) echo "Unknown arg: $1"; exit 1 ;;
  esac
done

BASE_URL="http://${HOST}:${PORT}"

check_health() {
  echo "=== Orca Health Check — $(date '+%Y-%m-%d %H:%M:%S') ==="
  echo ""

  # /health — cached status
  echo -n "  /health        → "
  if RESP=$(curl -sf --max-time 3 "${BASE_URL}/health" 2>/dev/null); then
    STATUS=$(echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status','?'))" 2>/dev/null || echo "?")
    echo "✅ ${STATUS}"
  else
    echo "❌ UNREACHABLE"
  fi

  # /health/ready — live DB check
  echo -n "  /health/ready  → "
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --max-time 5 "${BASE_URL}/health/ready" 2>/dev/null || echo "000")
  if [ "$HTTP_CODE" = "200" ]; then
    echo "✅ 200 OK (DB live)"
  elif [ "$HTTP_CODE" = "503" ]; then
    echo "⚠️  503 DB unhealthy"
  else
    echo "❌ HTTP ${HTTP_CODE}"
  fi

  if [ "$SHOW_METRICS" = true ]; then
    echo ""
    echo "  /metrics:"
    curl -sf --max-time 5 "${BASE_URL}/metrics" 2>/dev/null | grep -E "^orca_" | head -20 || echo "  (metrics endpoint not enabled — set ORCA_FLEET_METRICS_ENABLED=true)"
  fi

  echo ""
}

if [ "$WATCH" = true ]; then
  echo "Watching ${BASE_URL} every ${WATCH_INTERVAL}s ... (Ctrl+C to stop)"
  echo ""
  while true; do
    check_health
    sleep "$WATCH_INTERVAL"
  done
else
  check_health
fi
