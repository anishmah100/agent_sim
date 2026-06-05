#!/usr/bin/env bash
# qwen_smoke.sh — Phase AGENT-A9 driver. Stands up the engine + Qwen
# llama-server + N qwen_agent processes against worlds/eldoria, then
# tails the historian's events.jsonl for the configured pass criteria.
#
# Usage:
#   tools/dev-scripts/qwen_smoke.sh [n_agents=3] [minutes=5]
#
# Pass criteria (from PROGRESS.md row 10):
#   - zero engine panics + zero harness crashes
#   - every core verb fires at least once
#   - >= 2 multi-turn dialogue exchanges (>= 3 turns)
#   - >= 1 trade/payment between agents
#   - >= 1 building entered AND exited
#   - reflection notes show learning over time
#   - first-order ToM updates have non-default values
#   - p99 tactical cycle wall-clock <= 3 s
#
# Plus the user's subjective taste call on a sample of reasoning traces.

set -euo pipefail
N_AGENTS="${1:-3}"
MINUTES="${2:-5}"
PORT="${PORT:-8088}"
QWEN_PORT="${QWEN_PORT:-8782}"
RUN_DIR=".runlog/a9_smoke_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RUN_DIR"

echo "==> AGENT-A9 smoke: $N_AGENTS agents × $MINUTES min on Eldoria"
echo "    engine on :$PORT  ·  qwen on :$QWEN_PORT  ·  logs $RUN_DIR/"

# 1. Confirm Qwen is up.
if ! curl -sf -m 2 "http://127.0.0.1:$QWEN_PORT/v1/models" > /dev/null; then
  echo "Qwen llama-server not reachable on :$QWEN_PORT. Start it with:"
  echo "  cd ~/projects"
  echo "  ./llama.cpp/build/bin/llama-server \\"
  echo "      -m models/Qwen3.6-27B-Q4_K_M.gguf -t 32 \\"
  echo "      --reasoning-budget 0 --port $QWEN_PORT"
  exit 2
fi

# 2. Build + launch engine with capture-reasoning on.
echo "==> building engine"
( cd engine && go build -o ../"$RUN_DIR"/engine ./cmd/engine )

EVENT_LOG="$RUN_DIR/events.jsonl"
"$RUN_DIR"/engine \
  -addr "127.0.0.1:$PORT" \
  -bundle worlds/eldoria \
  -event-log "$EVENT_LOG" \
  -capture-reasoning \
  -register-rate 100 -register-burst 100 \
  > "$RUN_DIR/engine.log" 2>&1 &
ENGINE_PID=$!
trap 'echo stopping; kill $ENGINE_PID 2>/dev/null || true; jobs -p | xargs -r kill 2>/dev/null || true' EXIT

sleep 2
if ! curl -sf -m 2 "http://127.0.0.1:$PORT/api/v1/world/info" > /dev/null; then
  echo "engine failed to come up"; exit 3
fi

# 3. Spawn N qwen agents with distinct personas.
PERSONAS=(
  "merchant:trainer:An apple merchant trying to undercut competition."
  "guard:trainer:Town guard wary of strangers and quick to investigate."
  "traveler:trainer:A wanderer collecting tales from every village."
  "thief:trainer:Down-on-luck pickpocket looking for easy gold."
  "smith:trainer:Blacksmith trying to drum up custom hammers business."
)

for ((i=0; i<N_AGENTS && i<${#PERSONAS[@]}; i++)); do
  IFS=":" read -r name arch bio <<< "${PERSONAS[$i]}"
  python -m examples.qwen_agent.main \
      --server "http://127.0.0.1:$PORT" \
      --token dev \
      --name "$name" \
      --archetype "$arch" \
      --bio "$bio" \
      --runtime-seconds $(( MINUTES * 60 )) \
      > "$RUN_DIR/agent_$name.log" 2>&1 &
  echo "==> spawned agent $name (bio: $bio)"
done

# 4. Wait for the runtime to elapse.
echo "==> running for $MINUTES minutes..."
sleep $(( MINUTES * 60 ))

# 5. Tear down.
kill $ENGINE_PID 2>/dev/null || true
wait 2>/dev/null || true

# 6. Score against pass criteria.
echo
echo "==> scoring against AGENT-A9 pass criteria"
python tools/dev-scripts/score_a9.py "$EVENT_LOG" || true
echo
echo "logs in $RUN_DIR/"
