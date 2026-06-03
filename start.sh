#!/usr/bin/env bash
# One-command bringup for agent_sim.
#
# Starts:
#   - the Go engine on :8080 (worlds/dev_test.json, scenario=fantasy_town)
#   - the Vite frontend dev server (default :5173)
#   - the NPC supervisor (2x heuristic_bot.py) if examples/heuristic_bot.py
#     resolves and Python is installed
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
WORLD="${WORLD:-worlds/dev_test.json}"
SCENARIO="${SCENARIO:-fantasy_town}"
EVENT_LOG="${EVENT_LOG:-.runlog/events.jsonl}"
NPC_CONFIG="${NPC_CONFIG:-scenarios/fantasy_town/npcs.json}"

# Build the engine binary in-place; the same process will run it.
echo "==> building engine"
( cd engine && go build -o ../.runlog/engine ./cmd/engine )

# Decide whether to wire NPCs. If python3 isn't on PATH or the config
# isn't there, skip and tell the user.
NPC_FLAG=""
if [[ -f "$NPC_CONFIG" ]] && command -v python3 >/dev/null 2>&1; then
  NPC_FLAG="-npc-config $NPC_CONFIG"
  echo "==> NPC supervisor will load $NPC_CONFIG"
else
  echo "==> NPC supervisor disabled (config missing or python3 not on PATH)"
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
.runlog/engine \
  -addr "$ENGINE_ADDR" \
  -world "$WORLD" \
  -scenario "$SCENARIO" \
  -event-log "$EVENT_LOG" \
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
