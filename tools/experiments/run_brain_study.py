"""Cross-brain study driver — the continue/abandon validation experiment.

Runs the SAME world + tunings + background cast with the agent BRAIN as the
only variable, several repetitions per arm, then aggregates the per-run
scores (tools.metrics.score_run via run_p7_real) into one comparison table.

Arms:
  qwen   — N Qwen focal agents (local llama.cpp) + base rule cast
  claude — N Claude focal agents (Anthropic API)  + base rule cast
  rule   — 0 LLM focal; base cast + N stand-in rule agents (population control)

Each run gets a FRESH engine boot from the bundle (no snapshot restore) and a
truncated event log, so arms are comparable and runs are independent.

Usage:
    PYTHONPATH=sdk/python:. python3 -m tools.experiments.run_brain_study \\
        --arms qwen,claude,rule --reps 2 --wall-seconds 1200 --llm 5

Output: .runlog/brain_study/<stamp>/<arm>_r<rep>/ (per-run run_p7_real output)
        .runlog/brain_study/<stamp>/comparison.{json,md}
"""
from __future__ import annotations

import argparse
import datetime
import json
import logging
import os
import signal
import subprocess
import sys
import time
import urllib.request
from pathlib import Path

log = logging.getLogger("brain_study")
REPO = Path(__file__).resolve().parents[2]
ENGINE_BIN = REPO / ".runlog" / "engine"
EVENTS = REPO / ".runlog" / "events.jsonl"

# Identical across every run in every arm. Moderate item respawn so the
# economy has material to move, 2x time so 20 wall-minutes is ~40 world-min.
ENGINE_FLAGS = [
    "-addr", "127.0.0.1:8080",
    "-bundle", "worlds/eldoria",
    "-event-log", str(EVENTS),
    "-capture-reasoning",
    "-register-rate", "200", "-register-burst", "200",
    "-event-ring", "16384",
    "-time-mult", "2.0",
    "-tuning", "respawn_cap=400,respawn_batch=16,respawn_radius=50,respawn_interval_ticks=150",
    "-npc-config", "worlds/eldoria/npcs.json",
]

BASE_CAST = "survivor:2,scavenger:1,killer:1,manipulator:1,avenger:1"   # 6 agents
# Stand-ins for the focal slots in the control arm, keeping population equal
# (count must match --llm).
RULE_STANDINS = "survivor:2,scavenger:1,manipulator:1"


def http_ok(url: str, timeout: float = 2.0) -> bool:
    try:
        urllib.request.urlopen(url, timeout=timeout)
        return True
    except Exception:
        return False


def start_engine() -> subprocess.Popen:
    EVENTS.write_text("")          # truncate BEFORE the engine opens it
    eng_log = open(REPO / ".runlog" / "brain_study_engine.log", "a")
    proc = subprocess.Popen([str(ENGINE_BIN), *ENGINE_FLAGS], cwd=REPO,
                            stdout=eng_log, stderr=subprocess.STDOUT)
    deadline = time.monotonic() + 120
    while time.monotonic() < deadline:
        if http_ok("http://127.0.0.1:8080/api/v1/world/info"):
            return proc
        if proc.poll() is not None:
            raise RuntimeError("engine died on boot; see brain_study_engine.log")
        time.sleep(1.5)
    proc.kill()
    raise RuntimeError("engine did not come up in 120s")


def stop_engine(proc: subprocess.Popen) -> None:
    if proc.poll() is None:
        proc.send_signal(signal.SIGINT)
        try:
            proc.wait(timeout=15)
        except subprocess.TimeoutExpired:
            proc.kill()


def run_arm(arm: str, rep: int, out_dir: Path, args) -> dict:
    """One fresh-engine run of one arm. Returns the run's score dict."""
    run_dir = out_dir / f"{arm}_r{rep}"
    cmd = [sys.executable, "-m", "tools.experiments.run_p7_real",
           "--engine", "http://127.0.0.1:8080",
           "--wall-seconds", str(args.wall_seconds),
           "--out", str(run_dir)]
    if arm == "rule":
        cmd += ["--llm", "0", "--cast", f"{BASE_CAST},{RULE_STANDINS}"]
    else:
        cmd += ["--llm", str(args.llm), "--brain", arm, "--cast", BASE_CAST]

    env = dict(os.environ)
    env["PYTHONPATH"] = f"{REPO}/sdk/python:{REPO}" + (
        ":" + env["PYTHONPATH"] if env.get("PYTHONPATH") else "")

    log.info("=== run %s_r%d starting (wall=%ds) ===", arm, rep, args.wall_seconds)
    engine = start_engine()
    try:
        rc = subprocess.run(cmd, cwd=REPO, env=env,
                            timeout=args.wall_seconds + 600).returncode
    finally:
        stop_engine(engine)

    # Archive the raw event log into the run dir — the next run truncates
    # EVENTS, and the reasoning traces are the qualitative half of the study.
    try:
        (run_dir / "events.jsonl").write_bytes(EVENTS.read_bytes())
    except OSError:
        log.warning("could not archive events.jsonl for %s_r%d", arm, rep)

    score_file = run_dir / "score.json"
    score = json.loads(score_file.read_text()) if score_file.exists() else {}
    summary_file = run_dir / "summary.json"
    summary = json.loads(summary_file.read_text()) if summary_file.exists() else {}
    score["_run"] = f"{arm}_r{rep}"
    score["_rc"] = rc
    score["_early_stop"] = summary.get("early_stop_reason")
    score["_llm_stats"] = summary.get("llm_stats", [])
    log.info("=== run %s_r%d done rc=%d ===", arm, rep, rc)
    return score


# Metrics that go in the comparison table, in display order.
TABLE_KEYS = [
    "total_events", "speech_count", "whisper_count", "shout_count",
    "contracts_proposed", "contracts_accepted", "contracts_completed",
    "contracts_broken", "manipulator_defections",
    "kills", "damage_events", "item_transfers", "pay_transfers",
    "gold_gini_end", "gold_total_end", "mental_notes",
]


def write_comparison(out_dir: Path, rows: list[dict]) -> None:
    (out_dir / "comparison.json").write_text(json.dumps(rows, indent=2))
    arms: dict[str, list[dict]] = {}
    for r in rows:
        arms.setdefault(r["_run"].rsplit("_r", 1)[0], []).append(r)

    lines = ["# Cross-brain study — comparison", "",
             "Mean per arm (per-run values in parentheses).", "",
             "| metric | " + " | ".join(arms) + " |",
             "|---|" + "---|" * len(arms)]
    for key in TABLE_KEYS:
        cells = []
        for arm, runs in arms.items():
            vals = [r.get(key) for r in runs if isinstance(r.get(key), (int, float))]
            if vals:
                mean = sum(vals) / len(vals)
                cells.append(f"{mean:.2f} ({', '.join(str(round(v, 2)) for v in vals)})")
            else:
                cells.append("—")
        lines.append(f"| {key} | " + " | ".join(cells) + " |")
    lines += ["", "## Run notes"]
    for r in rows:
        note = r.get("_early_stop") or "completed full window"
        st = r.get("_llm_stats") or []
        acc = sum(s.get("accepted", 0) for s in st)
        rej = sum(s.get("rejected", 0) for s in st)
        lines.append(f"- `{r['_run']}`: rc={r['_rc']} — {note}"
                     + (f" — LLM actions {acc} accepted / {rej} rejected" if st else ""))
    (out_dir / "comparison.md").write_text("\n".join(lines) + "\n")


def main() -> int:
    logging.basicConfig(level=logging.INFO,
                        format="%(asctime)s %(levelname)s %(name)s %(message)s")
    p = argparse.ArgumentParser(description="Cross-brain comparison study.")
    p.add_argument("--arms", default="qwen,claude,rule")
    p.add_argument("--reps", type=int, default=2)
    p.add_argument("--llm", type=int, default=4)
    p.add_argument("--wall-seconds", type=int, default=1200)
    p.add_argument("--out", default=None)
    args = p.parse_args()

    stamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    out_dir = Path(args.out) if args.out else REPO / ".runlog" / "brain_study" / stamp
    out_dir.mkdir(parents=True, exist_ok=True)

    rows = []
    # Interleave reps (qwen,claude,rule, qwen,claude,rule, ...) so a drifting
    # confound (e.g. machine load) spreads across arms instead of one arm.
    for rep in range(1, args.reps + 1):
        for arm in args.arms.split(","):
            try:
                rows.append(run_arm(arm.strip(), rep, out_dir, args))
            except Exception:
                log.exception("run %s_r%d failed; continuing", arm, rep)
                rows.append({"_run": f"{arm}_r{rep}", "_rc": -1,
                             "_early_stop": "driver exception"})
            write_comparison(out_dir, rows)   # refresh after every run
    log.info("study complete: %s", out_dir / "comparison.md")
    return 0


if __name__ == "__main__":
    sys.exit(main())
