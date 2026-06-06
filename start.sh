#!/usr/bin/env bash
# One-command bringup for agent_sim.
#
# Starts:
#   - the Go engine on :8080 (worlds/eldoria bundle, scenario=fantasy_town)
#   - the Vite frontend dev server (default :5173)
#   - the NPC supervisor (bundle-declared NPCs) if Python is installed
#
# Logs:
#   .runlog/engine.log
#   .runlog/frontend.log
#
# Ctrl-C stops everything.

set -euo pipefail
cd "$(dirname "$0")"

mkdir -p .runlog

ENGINE_ADDR="${ENGINE_ADDR:-127.0.0.1:8080}"
# Worlds — each is a self-contained bundle under worlds/<name>/ holding
# world.json + bundle.toml + npcs.json + design/.
#   worlds/eldoria          — 1500×1500 fantasy continent with 6 regions
#                             and 6 towns. DEFAULT.
#   worlds/dev_test         — 60×40 hand-designed Oak Hollow village.
#   worlds/dev_wilderness   — 200×120 wilderness map; viewport streaming.
#   worlds/soak_1000x1000   — 1000×1000 soak test.
BUNDLE="${BUNDLE:-worlds/eldoria}"
EVENT_LOG="${EVENT_LOG:-.runlog/events.jsonl}"
# Optional: override the bundle's bundled NPC config.
NPC_CONFIG="${NPC_CONFIG:-}"
SNAP_DIR="${SNAP_DIR:-.runlog/snapshots}"
SNAP_EVERY="${SNAP_EVERY:-60s}"
# Local dev CORS — allow the Vite dev server origins. Override with
# CORS_ALLOW="https://your-prod-domain" in production.
CORS_ALLOW="${CORS_ALLOW:-http://127.0.0.1:5173,http://localhost:5173}"
# Local dev = no auth. Set JWT_SECRET to a real value in prod (see
# tools/issue_jwt.py to mint tokens).
JWT_SECRET="${JWT_SECRET:-}"

# Build the engine binary in-place; the same process will run it.
echo "==> building engine"
( cd engine && go build -o ../.runlog/engine ./cmd/engine )

# NPC config: if NPC_CONFIG is unset, the engine falls back to the bundle's
# bundle.toml [npcs] config field. If python3 isn't on PATH, force-disable.
NPC_FLAG=""
if command -v python3 >/dev/null 2>&1; then
  if [[ -n "$NPC_CONFIG" ]]; then
    NPC_FLAG="-npc-config $NPC_CONFIG"
    echo "==> NPC supervisor will load $NPC_CONFIG (override)"
  else
    echo "==> NPC supervisor will use bundle's bundled config (if any)"
  fi
else
  echo "==> NPC supervisor disabled (python3 not on PATH)"
fi

pids=()

cleanup() {
  echo
  echo "==> stopping (pids: ${pids[*]:-none})"
  for pid in "${pids[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
  exit 0
}
trap cleanup INT TERM

echo "==> starting engine on $ENGINE_ADDR (logs: .runlog/engine.log)"
# shellcheck disable=SC2086
# Every observable engine feature ON by default — dev start should
# expose every panel populated, every event logged, every trace
# captured. Production deployments override the flags they need to
# tighten (rate limits, CORS, JWT). The user explicitly asked for
# "all features enabled" so they never hit another empty UI tab
# because a flag was off.
.runlog/engine \
  -addr "$ENGINE_ADDR" \
  -bundle "$BUNDLE" \
  -event-log "$EVENT_LOG" \
  -snapshot-dir "$SNAP_DIR" \
  -snapshot-every "$SNAP_EVERY" \
  -cors-allow "$CORS_ALLOW" \
  -jwt-secret "$JWT_SECRET" \
  -capture-reasoning \
  -register-rate 100 \
  -register-burst 100 \
  -event-ring 8192 \
  $NPC_FLAG \
  > .runlog/engine.log 2>&1 &
pids+=($!)

# Give the engine a moment to bind so the frontend's first /api/v1/world/info fetch doesn't race.
sleep 0.6

echo "==> starting frontend (logs: .runlog/frontend.log)"
( cd frontend && npm run dev ) > .runlog/frontend.log 2>&1 &
pids+=($!)

echo
echo "agent_sim is up."
echo "  engine:   http://$ENGINE_ADDR"
echo "  frontend: http://127.0.0.1:5173"
echo "  rulebook: http://127.0.0.1:5173 -> click 'rulebook' in the toolbar"
echo "  history:  http://$ENGINE_ADDR/api/v1/world/history"
echo
echo "Ctrl-C to stop both."
wait
