#!/usr/bin/env bash
# Robustly (re)start the audit sidecar engine on :8090, avoiding the recurring
# stale-engine trap (kill targets a dead pid -> relaunch fails to bind -> an
# OLD binary keeps serving and tests run against stale code).
#
# Kills ALL engine_doccap instances, waits for the port to free, builds fresh,
# launches, and verifies a low uptime. Usage: tools/audit/restart_sidecar.sh
set -euo pipefail
REPO="$(cd "$(dirname "$0")/../.." && pwd)"
PORT=8090
BIN=/tmp/engine_doccap

echo "building engine -> $BIN"
( cd "$REPO/engine" && go build -o "$BIN" ./cmd/engine )

echo "killing any engine_doccap instances..."
for p in $(ps -eo pid,cmd | grep "[e]ngine_doccap -addr" | awk '{print $1}'); do
  kill -9 "$p" 2>/dev/null && echo "  killed $p" || true
done

echo "waiting for :$PORT to free..."
for _ in $(seq 1 20); do
  code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$PORT/api/v1/world/info" || echo 000)
  [ "$code" = "000" ] && break
  sleep 0.5
done

: > /tmp/doccap_events.jsonl
echo "launching fresh engine..."
nohup "$BIN" -addr 127.0.0.1:$PORT -bundle "$REPO/worlds/eldoria" \
  -event-log /tmp/doccap_events.jsonl -register-rate 200 -register-burst 200 \
  -time-mult 4.0 > /tmp/doccap_engine.log 2>&1 &
echo "  pid $!"

for _ in $(seq 1 20); do
  curl -s -o /dev/null "http://127.0.0.1:$PORT/api/v1/world/info" && break
  sleep 0.5
done
up=$(curl -s "http://127.0.0.1:$PORT/api/v1/world/info" | python3 -c "import sys,json;print(round(json.load(sys.stdin)['uptime_s'],1))")
echo "engine up, uptime=${up}s (must be low — if high, a stale instance won the port)"
