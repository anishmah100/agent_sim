"""Anthropic spend tracker (#232).

Every Anthropic call (Claude focal agents + the narrator) records its token
usage here; we estimate cost from a per-model price table, append a line to
.runlog/anthropic_spend.jsonl, and trip a proactive warning when cumulative
spend crosses $5 / $15 / $20 / $24 so a long run can't silently blow past
the $25 cap.

Usage:
  - Library:   from tools.budget_log import record
               record("claude-haiku-4-5-20251001", in_tok=1200, out_tok=180)
  - CLI:       python3 tools/budget_log.py            # print running total
               python3 tools/budget_log.py --reset    # archive + zero the log
"""
from __future__ import annotations

import json
import os
import sys
import threading
from pathlib import Path

_REPO = Path(__file__).resolve().parents[1]
SPEND_LOG = _REPO / ".runlog" / "anthropic_spend.jsonl"
HARD_CAP_USD = 25.0
TRIPWIRES_USD = (5.0, 15.0, 20.0, 24.0)

# USD per 1M tokens (input, output). Extend as models are used.
PRICES = {
    "claude-haiku-4-5-20251001": (1.00, 5.00),
    "claude-haiku-4-5": (1.00, 5.00),
    "claude-opus-4-8": (15.00, 75.00),
    "claude-sonnet-4-6": (3.00, 15.00),
    "_default": (3.00, 15.00),
}

_lock = threading.Lock()


def est_cost(model: str, in_tok: int, out_tok: int) -> float:
    pin, pout = PRICES.get(model, PRICES["_default"])
    return (in_tok / 1_000_000) * pin + (out_tok / 1_000_000) * pout


def _totals() -> tuple[float, int, int]:
    """(usd, in_tok, out_tok) summed over the log."""
    usd = 0.0
    itok = otok = 0
    if SPEND_LOG.exists():
        for line in SPEND_LOG.read_text().splitlines():
            try:
                r = json.loads(line)
            except json.JSONDecodeError:
                continue
            usd += r.get("usd", 0.0)
            itok += r.get("in_tok", 0)
            otok += r.get("out_tok", 0)
    return usd, itok, otok


def record(model: str, in_tok: int, out_tok: int, *, source: str = "") -> float:
    """Log one Anthropic call's usage; return the new cumulative USD.

    Thread-safe and best-effort: never raises into the caller's LLM path.
    Emits a tripwire warning line (stderr) when a threshold is first crossed.
    """
    try:
        cost = est_cost(model, in_tok, out_tok)
        with _lock:
            SPEND_LOG.parent.mkdir(parents=True, exist_ok=True)
            prev_usd, _, _ = _totals()
            new_usd = prev_usd + cost
            with SPEND_LOG.open("a") as f:
                f.write(json.dumps({
                    "model": model, "in_tok": in_tok, "out_tok": out_tok,
                    "usd": round(cost, 6), "cum_usd": round(new_usd, 4),
                    "source": source,
                }) + "\n")
            for t in TRIPWIRES_USD:
                if prev_usd < t <= new_usd:
                    msg = (f"[budget] Anthropic spend crossed ${t:.0f} "
                           f"(now ${new_usd:.2f} of ${HARD_CAP_USD:.0f} cap)")
                    print(msg, file=sys.stderr, flush=True)
            if new_usd >= HARD_CAP_USD:
                print(f"[budget] HARD CAP HIT: ${new_usd:.2f} >= "
                      f"${HARD_CAP_USD:.0f} — STOP launching paid runs",
                      file=sys.stderr, flush=True)
            return new_usd
    except Exception:
        return -1.0


def over_cap() -> bool:
    usd, _, _ = _totals()
    return usd >= HARD_CAP_USD


def _cli() -> int:
    if "--reset" in sys.argv:
        if SPEND_LOG.exists():
            archive = SPEND_LOG.with_suffix(".jsonl.bak")
            os.replace(SPEND_LOG, archive)
            print(f"archived -> {archive}")
        else:
            print("no spend log to reset")
        return 0
    usd, itok, otok = _totals()
    n = len(SPEND_LOG.read_text().splitlines()) if SPEND_LOG.exists() else 0
    print(f"Anthropic spend: ${usd:.4f} of ${HARD_CAP_USD:.0f} cap "
          f"({100*usd/HARD_CAP_USD:.1f}%)")
    print(f"  calls={n}  in_tok={itok:,}  out_tok={otok:,}")
    for t in TRIPWIRES_USD:
        mark = "✓" if usd >= t else " "
        print(f"  [{mark}] ${t:.0f} tripwire")
    return 0


if __name__ == "__main__":
    raise SystemExit(_cli())
