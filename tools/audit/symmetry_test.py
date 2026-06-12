#!/usr/bin/env python3
"""Phase 0.6 — the SYMMETRY TEST (settles the parked determinism/fairness Q).

The open question: when two agents act on the SAME tick, the engine resolves
contention by arrival order (FIFO enqueue). Does that arrival order
SYSTEMATICALLY favor one connection — i.e. is there a fairness bias that
averaging can't remove (bias != variance)? If a faster-latency model would
always win contested resources, a cross-model finding would be a substrate
artifact, not a capability result.

This isolates the SUBSTRATE (no LLM noise): two IDENTICAL scripted agents speak
in lockstep for N rounds. For every tick where BOTH spoke, the engine assigned
the two Speech events sequence numbers in processing order — whoever got the
LOWER seq was processed first that tick. Over many ticks, a fair substrate
gives ~50/50; a systematic skew means arrival order is biased.

  symmetric  → averaging suffices; no determinism work needed.
  skewed     → add a deterministic same-tick tiebreak + randomized seats.

Usage (fresh engine, default :8090, event log path):
    python3 tools/audit/symmetry_test.py [engine_url] [event_log] [rounds]
"""
import asyncio
import json
import sys
from collections import defaultdict

from harness import connect

ENGINE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
EVLOG = sys.argv[2] if len(sys.argv) > 2 else "/tmp/sym_events.jsonl"
ROUNDS = int(sys.argv[3]) if len(sys.argv) > 3 else 80


async def main():
    a = await connect(ENGINE, name="SymA", cadence_ms=120)
    b = await connect(ENGINE, name="SymB", cadence_ms=120)
    await a.observe(); await b.observe()
    aid = a.obs["self"]["entity_id"]
    bid = b.obs["self"]["entity_id"]
    print(f"symmetry: A={aid} B={bid}, {ROUNDS} lockstep rounds")

    for n in range(ROUNDS):
        # Fire both speaks as simultaneously as possible so they contend for the
        # same tick's queue drain.
        await asyncio.gather(
            a.act("speak", wait_ack=False, text=f"SYM_A_{n}"),
            b.act("speak", wait_ack=False, text=f"SYM_B_{n}"),
        )
        await asyncio.sleep(0.10)

    await asyncio.sleep(1.5)  # let the tape flush

    # Read the tape: group Speech events by tick; for ticks where BOTH A and B
    # spoke, the lower seq was processed first.
    per_tick = defaultdict(dict)  # tick -> {speaker: seq}
    with open(EVLOG) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            if r.get("kind") != "Speech":
                continue
            p = r.get("payload") or {}
            sp = p.get("Speaker")
            txt = p.get("Text") or ""
            if sp not in (aid, bid):
                continue
            if not (txt.startswith("SYM_A_") or txt.startswith("SYM_B_")):
                continue
            per_tick[r.get("tick")][sp] = r.get("seq")

    a_first = b_first = contested = 0
    for tick, d in per_tick.items():
        if aid in d and bid in d:
            contested += 1
            if d[aid] < d[bid]:
                a_first += 1
            else:
                b_first += 1

    print(f"  contested ticks (both spoke): {contested}")
    if contested == 0:
        print("  INCONCLUSIVE — no ticks where both agents spoke; raise rounds/cadence.")
        return 2
    frac = a_first / contested
    print(f"  A-processed-first: {a_first}  B-processed-first: {b_first}  (A frac = {frac:.2f})")
    # Rough fairness band: with `contested` samples, ~50% ± 3*sqrt(0.25/n).
    import math
    band = 3 * math.sqrt(0.25 / contested)
    lo, hi = 0.5 - band, 0.5 + band
    if lo <= frac <= hi:
        print(f"  SYMMETRIC — A frac {frac:.2f} within fair band [{lo:.2f},{hi:.2f}]. "
              f"Arrival order is not systematically biased; averaging suffices.")
        return 0
    print(f"  SKEWED — A frac {frac:.2f} OUTSIDE [{lo:.2f},{hi:.2f}]. Same-tick contention "
          f"is biased by connection arrival order; add a deterministic tiebreak + randomized seats.")
    return 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
