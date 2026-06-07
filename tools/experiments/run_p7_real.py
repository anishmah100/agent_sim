"""P7.2 — the first REAL emergence experiment.

Mixed cast: LLM focal agents (Qwen) + rule-based archetype background.
Runs against a live engine, with the narrator process in parallel,
then collects the social heatmap + per-agent gold and scores the run
with tools.metrics.score_run.

Unlike run_demo.py (rule-based only, for substrate smoke), this run
includes the SUBJECTS of the experiment — the LLMs whose social
behaviour we're studying.

Pre-req: substrate validation must be GREEN (tools/validate_substrate.py).
The runner refuses to start if /api/v1/debug/vision can't see items at
the hub (the canary for the D8 class of bugs).

Usage (engine must already be running, ideally with -time-mult 4):
    cd ~/projects/agent_sim
    PYTHONPATH=sdk/python:. python3 -m tools.experiments.run_p7_real \\
        --wall-seconds 240 --llm 3 --narrator

Output dir (default .runlog/p7_real/<timestamp>):
    summary.json   — cast, metrics, social heatmap, gold
    score.json     — tools.metrics.score_run output
    narrator.jsonl — narrator output (if --narrator)
    REPORT.md      — human-readable digest + L4 closing narrative
"""
from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import signal
import subprocess
import sys
import time
from dataclasses import asdict
from pathlib import Path
from typing import Optional

from agent_sim_sdk import AgentCredentials, VisionMode, register_agent

from agents.baselines import Killer, Manipulator, Scavenger, Survivor
from agents.llm.qwen_focal import QwenFocalAgent, FocalConfig
from tools.metrics.score_run import load_events, score_events

log = logging.getLogger("p7_real")
REPO = Path(__file__).resolve().parents[2]


# Persona seeds for the LLM focal agents. Variety drives different
# strategies → richer emergence.
FOCAL_PERSONAS = [
    ("Mara the trader",
     "You are Mara, a shrewd trader. You believe wealth comes from deals, "
     "not violence. You seek gold, trade fairly when it profits you, and "
     "remember who cheats you.",
     "Accumulate gold through trade and smart pickups. Avoid fights."),
    ("Bram the cautious",
     "You are Bram, cautious and survival-focused. You fear the armed and "
     "the desperate. You gather food first, gold second, and flee danger.",
     "Stay fed and alive. Gather food, keep distance from armed strangers."),
    ("Dalia the social",
     "You are Dalia, warm and gregarious. You believe alliances keep you "
     "safe. You greet others, propose mutual-aid contracts, and share food "
     "with allies.",
     "Build alliances. Propose contracts, share resources, make friends."),
    ("Krell the opportunist",
     "You are Krell, an opportunist who takes the main chance. You'll "
     "cooperate or betray depending on what pays. Gold is everything.",
     "Get rich by any means. Cooperate when useful, defect when profitable."),
    ("Wren the wanderer",
     "You are Wren, curious and independent. You explore, pick up what you "
     "find, and avoid entanglements unless they clearly benefit you.",
     "Explore and gather. Pick up gold and food, stay flexible."),
    ("Odo the desperate",
     "You are Odo, down on your luck and hungry. You need food and gold "
     "urgently and will take risks others won't.",
     "Survive at any cost. Get food and gold fast, even risky moves."),
]


def http_json(engine: str, path: str, timeout: float = 5.0) -> dict:
    import urllib.request
    with urllib.request.urlopen(engine + path, timeout=timeout) as r:
        return json.loads(r.read())


def preflight(engine: str) -> bool:
    """Refuse to run if the substrate canary fails: items must be
    visible at the spawn hub via the debug endpoint. This is the cheap
    guard against launching a big run on a broken environment."""
    try:
        info = http_json(engine, "/api/v1/world/info")
        log.info("engine up: world=%s tick=%s", info.get("world"), info.get("tick"))
    except Exception as e:
        log.error("PREFLIGHT FAIL: engine not reachable: %s", e)
        return False
    # Find a position where an item actually exists, then probe THAT
    # via debug/vision. This separates two failure modes:
    #   - D8 wire bug: items exist in the world but vision returns 0
    #     at a tile we KNOW has an item → hard fail.
    #   - depletion: few items left anywhere → soft warn (the run can
    #     proceed but won't have much economy; respawn replenishes).
    try:
        world = http_json(engine, "/worlds/eldoria.json")
    except Exception as e:
        log.error("PREFLIGHT FAIL: cannot read world snapshot: %s", e)
        return False
    items = [e for e in (world.get("entities") or [])
             if e.get("archetype") == "item"]
    if len(items) < 20:
        log.warning("PREFLIGHT WARN: only %d items in world — economy will "
                    "be thin. Consider restarting the engine fresh "
                    "(clear .runlog/snapshots).", len(items))
    if not items:
        log.error("PREFLIGHT FAIL: zero items in world")
        return False
    # Probe at an actual item's tile.
    sample = items[len(items) // 2]
    sx, sy = sample["pos"]
    try:
        vis = http_json(engine, f"/api/v1/debug/vision?x={sx}&y={sy}")
        if vis.get("v_items", 0) < 1:
            log.error("PREFLIGHT FAIL: item %s exists at (%d,%d) but "
                      "debug/vision sees 0 items there — D8 wire bug?",
                      sample.get("entity_id"), sx, sy)
            return False
        log.info("preflight: vision OK — %d items visible at a known "
                 "item tile (%d total in world)", vis["v_items"], len(items))
    except Exception as e:
        log.error("PREFLIGHT FAIL: debug/vision unreachable (%s)", e)
        return False
    return True


# ---- registration ----

ARCHETYPES = {
    "survivor": Survivor, "scavenger": Scavenger,
    "killer": Killer, "manipulator": Manipulator,
}


class Handle:
    def __init__(self, name, kind, creds, bot):
        self.name = name
        self.kind = kind          # "llm" or archetype name
        self.creds = creds
        self.bot = bot
        self.task: Optional[asyncio.Task] = None


async def register_llm(engine: str, idx: int, brain: str = "qwen") -> Handle:
    name, persona, goal = FOCAL_PERSONAS[idx % len(FOCAL_PERSONAS)]
    creds = await register_agent(
        engine, user_token="dev",
        persona={"name": name, "bio": persona, "archetype_tag": "llm"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True,
        cadence_ms=1000)
    if brain == "claude":
        from agents.llm.claude_focal import ClaudeFocalAgent
        bot = ClaudeFocalAgent(creds=creds, persona=persona, goal=goal)
    else:
        bot = QwenFocalAgent(creds=creds, persona=persona, goal=goal,
                             cfg=FocalConfig(timeout_s=90))
    return Handle(name, "llm", creds, bot)


async def register_archetype(engine: str, arch: str, idx: int) -> Handle:
    cls = ARCHETYPES[arch]
    name = f"{arch}_{idx}"
    creds = await register_agent(
        engine, user_token="dev",
        persona={"name": name, "bio": f"rule-based {arch}",
                 "archetype_tag": arch},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True,
        cadence_ms=1000)
    bot = cls(creds=creds, archetype_name=arch)
    return Handle(name, arch, creds, bot)


# ---- narrator subprocess ----

def start_narrator(events: Path, out: Path, max_claude: int) -> subprocess.Popen:
    cmd = [
        sys.executable, "-m", "tools.narrator",
        "--events", str(events), "--out", str(out),
        "--max-qwen-calls", "300", "--max-claude-calls", str(max_claude),
        "--idle-exit", "10",
    ]
    log.info("spawning narrator: %s", " ".join(cmd))
    return subprocess.Popen(cmd, cwd=str(REPO),
                            env={**os.environ, "PYTHONUNBUFFERED": "1",
                                 "PYTHONPATH": "sdk/python:."},
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT)


# ---- driver ----

async def main_run(args) -> int:
    logging.basicConfig(level=logging.INFO,
                        format="%(asctime)s %(levelname)s %(name)s %(message)s")
    engine = args.engine
    if not preflight(engine):
        return 2

    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    start_tick = int(http_json(engine, "/api/v1/world/info").get("tick", 0))

    # Build cast.
    handles: list[Handle] = []
    for i in range(args.llm):
        handles.append(await register_llm(engine, i, getattr(args, "brain", "qwen")))
    cast_spec = _parse_cast(args.cast)
    for arch, count in cast_spec:
        for i in range(count):
            handles.append(await register_archetype(engine, arch, i))
    log.info("registered %d agents (%d LLM + %d rule-based)",
             len(handles), args.llm, sum(c for _, c in cast_spec))

    # Narrator.
    narrator_proc = None
    events_path = REPO / ".runlog" / "events.jsonl"
    if args.narrator:
        narrator_proc = start_narrator(
            events_path, out_dir / "narrator.jsonl", args.max_claude)

    # Launch all bots.
    for h in handles:
        h.task = asyncio.create_task(h.bot.run(), name=h.name)

    # Run window with periodic progress. We ALSO poll each agent's
    # gold every tick of the loop and keep the last NON-ZERO reading
    # per agent — robust against an entity vanishing mid-run (which
    # would otherwise make end-of-run gold read 0). The poll doubles
    # as a diagnostic: a gold value that goes positive then drops to a
    # missing entity tells us exactly when a body disappeared.
    t0 = time.monotonic()
    gold_hist: dict[str, int] = {}
    start_gold: dict[str, int] = {}
    early_stop_reason = None
    try:
        while time.monotonic() - t0 < args.wall_seconds:
            await asyncio.sleep(15)
            elapsed = int(time.monotonic() - t0)
            try:
                live = len(http_json(engine, "/api/v1/agents", 3).get("agents", []))
            except Exception:
                live = -1
            # Poll gold + detect vanished entities.
            vanished = []
            for h in handles:
                eid = getattr(h.bot, "entity_id", None)
                if not eid:
                    continue
                try:
                    ms = http_json(engine, f"/api/v1/agent/{eid}/mental_state", 3)
                    v = ms.get("vitals") or {}
                    g = v.get("gold", 0)
                    if v.get("max_hp", 0) == 0:
                        vanished.append(h.name)
                    else:
                        gold_hist[h.name] = g
                        start_gold.setdefault(h.name, g)
                except Exception:
                    pass
            cyc = sum(getattr(h.bot, "cycles", 0) for h in handles if h.kind == "llm")
            acc = sum(getattr(h.bot, "accepted", 0) for h in handles if h.kind == "llm")

            # ── Live scoring on the event window so far (cheap; the
            # event file is local). This gives a real emergence pulse
            # every checkpoint instead of waiting for the run to end.
            try:
                ev_now = [e for e in load_events(str(events_path))
                          if int(e.get("tick", 0)) >= start_tick]
                live_score = score_events(ev_now)
                gold_moved = any(gold_hist.get(n, 0) != start_gold.get(n, 0)
                                 for n in gold_hist)
                pulse = (f"contracts={live_score.contracts_proposed}p/"
                         f"{live_score.contracts_accepted}a "
                         f"kills={live_score.kills} dmg={live_score.damage_events} "
                         f"pays={live_score.pay_transfers} "
                         f"speak={live_score.speech_count} "
                         f"gold_moved={gold_moved}")
            except Exception:
                live_score = None
                gold_moved = False
                pulse = "(score unavailable)"

            log.info("t+%ds live=%d cyc=%d acc=%d gold=%s | %s%s",
                     elapsed, live, cyc, acc, gold_hist, pulse,
                     f" VANISHED={vanished}" if vanished else "")

            # ── Early-abort heuristics (fast iteration). ───────────
            # DEAD RUN: by 60s, if the LLM agents have produced zero
            # accepted actions, the harness/LLM pipeline is broken —
            # don't waste the full window.
            if elapsed >= 60 and args.llm > 0 and acc == 0:
                early_stop_reason = ("DEAD: 0 accepted LLM actions by 60s — "
                                     "LLM/harness pipeline broken")
                break
            # STALLED ECONOMY: by 90s, if NOTHING economic/social has
            # happened (no gold movement, no contracts, no combat, no
            # speech), the setup is inert — bail and iterate on it.
            if (elapsed >= 90 and live_score is not None
                    and not gold_moved
                    and live_score.contracts_proposed == 0
                    and live_score.damage_events == 0
                    and live_score.pay_transfers == 0):
                early_stop_reason = ("STALLED: no gold movement / contracts / "
                                     "combat / pays by 90s — setup is inert")
                break
        if early_stop_reason:
            log.warning("EARLY STOP — %s", early_stop_reason)
        # Final collect; merge with last-known gold for any vanished body.
        gold, heatmap, manip_eids = collect_state(engine, handles)
        for name, g in gold_hist.items():
            if gold.get(name, 0) == 0 and g > 0:
                gold[name] = g  # restore last-known before it vanished
    finally:
        log.info("stopping bots…")
        for h in handles:
            try:
                h.bot.stop()
            except Exception:
                pass
        await asyncio.sleep(1.5)
        for h in handles:
            if h.task and not h.task.done():
                h.task.cancel()
        await asyncio.gather(*(h.task for h in handles if h.task),
                             return_exceptions=True)
        if narrator_proc:
            log.info("terminating narrator…")
            narrator_proc.send_signal(signal.SIGINT)
            try:
                narrator_proc.wait(timeout=25)
            except subprocess.TimeoutExpired:
                narrator_proc.kill()

    # Score.
    events = [e for e in load_events(str(events_path))
              if int(e.get("tick", 0)) >= start_tick]
    score = score_events(events, gold_by_agent=gold, manipulators=manip_eids)

    summary = {
        "engine": engine,
        "wall_seconds": args.wall_seconds,
        "early_stop_reason": early_stop_reason,
        "start_tick": start_tick,
        "cast": {"llm": args.llm, "rule_based": dict(cast_spec)},
        "agents": [{"name": h.name, "kind": h.kind,
                    "agent_id": h.creds.agent_id} for h in handles],
        "llm_stats": [{"name": h.name,
                       "cycles": getattr(h.bot, "cycles", 0),
                       "accepted": getattr(h.bot, "accepted", 0),
                       "rejected": getattr(h.bot, "rejected", 0)}
                      for h in handles if h.kind == "llm"],
        "social_heatmap": heatmap,
        "gold_by_agent": gold,
    }
    (out_dir / "summary.json").write_text(json.dumps(summary, indent=2, default=str))
    (out_dir / "score.json").write_text(json.dumps(score.to_dict(), indent=2, default=str))
    _write_report(out_dir, summary, score)
    log.info("wrote %s", out_dir / "REPORT.md")
    _print_digest(summary, score)
    return 0


def collect_state(engine: str, handles: list[Handle]):
    """Gather per-agent gold, the social heatmap, and the set of
    manipulator entity_ids — all BEFORE bots disconnect. Uses each
    bot's self-recorded entity_id (set from its first obs) rather than
    the /agents endpoint, which races disconnect and previously caused
    LLM agents to silently drop out of the gold table."""
    gold: dict[str, int] = {}
    heatmap: dict[str, dict] = {}
    manip_eids: set[str] = set()
    eid_to_name = {}
    for h in handles:
        eid = getattr(h.bot, "entity_id", None)
        if eid:
            eid_to_name[eid] = h.name
            if h.kind == "manipulator":
                manip_eids.add(eid)
    for h in handles:
        eid = getattr(h.bot, "entity_id", None)
        if not eid:
            log.warning("collect_state: %s never recorded an entity_id "
                        "(no obs received?)", h.name)
            continue
        try:
            ms = http_json(engine, f"/api/v1/agent/{eid}/mental_state", 5)
        except Exception as e:
            log.warning("collect_state: mental_state(%s/%s) failed: %s",
                        h.name, eid, e)
            continue
        v = ms.get("vitals") or {}
        gold[h.name] = v.get("gold", 0)
        peers = ms.get("peers") or {}
        heatmap[h.name] = {eid_to_name.get(pe, pe): c for pe, c in peers.items()}
    return gold, heatmap, manip_eids


def _write_report(out_dir: Path, summary: dict, score) -> None:
    s = score
    lines = [
        f"# P7 Experiment Report",
        "",
        f"- Engine: {summary['engine']}",
        f"- Wall seconds: {summary['wall_seconds']}",
        f"- Cast: {summary['cast']['llm']} LLM focal + "
        f"{summary['cast']['rule_based']}",
        "",
        "## Mechanical metrics",
        f"- total events (this run): {s.total_events}",
        f"- contracts: {s.contracts_proposed} proposed / "
        f"{s.contracts_accepted} accepted / {s.contracts_completed} completed / "
        f"{s.contracts_broken} broken",
        f"- kills: {s.kills}  | damage events: {s.damage_events}",
        f"- item transfers: {s.item_transfers}  | pay transfers: {s.pay_transfers}",
        f"- gold Gini (end): {s.gold_gini_end}  | total gold: {s.gold_total_end}",
        f"- manipulator: {s.manipulator_defections} defections / "
        f"{s.manipulator_contracts} contracts",
        f"- comms: {s.speech_count} speak / {s.whisper_count} whisper / "
        f"{s.shout_count} shout  | mental notes: {s.mental_notes}",
        "",
        "## Gold by agent (end)",
    ]
    for name, g in sorted(summary["gold_by_agent"].items(),
                          key=lambda kv: -kv[1]):
        lines.append(f"- {name}: {g}")
    lines += ["", "## LLM focal stats"]
    for st in summary.get("llm_stats", []):
        lines.append(f"- {st['name']}: {st['cycles']} cycles, "
                     f"{st['accepted']} accepted, {st['rejected']} rejected")
    # L4 closing narrative if narrator ran.
    nfile = out_dir / "narrator.jsonl"
    if nfile.exists():
        l4 = None
        for line in nfile.read_text().splitlines():
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            if rec.get("level") == "L4" and rec.get("text"):
                l4 = rec["text"]
        if l4:
            lines += ["", "## Closing narrative (L4)", "", l4]
    (out_dir / "REPORT.md").write_text("\n".join(lines))


def _print_digest(summary: dict, score) -> None:
    s = score
    print("\n" + "=" * 60)
    print("P7 EXPERIMENT DIGEST")
    print("=" * 60)
    print(f"contracts: {s.contracts_proposed}p/{s.contracts_accepted}a/"
          f"{s.contracts_completed}c/{s.contracts_broken}broken")
    print(f"kills: {s.kills}  damage: {s.damage_events}  "
          f"transfers: {s.item_transfers}  pays: {s.pay_transfers}")
    print(f"gold Gini: {s.gold_gini_end}  total: {s.gold_total_end}")
    print(f"manipulator defections: {s.manipulator_defections}/"
          f"{s.manipulator_contracts}")
    print(f"comms: {s.speech_count}sp/{s.whisper_count}wh  "
          f"mental_notes: {s.mental_notes}")
    print("gold leaderboard:")
    for name, g in sorted(summary["gold_by_agent"].items(),
                          key=lambda kv: -kv[1])[:6]:
        print(f"  {name}: {g}")
    print("=" * 60)


def _parse_cast(spec: str) -> list[tuple[str, int]]:
    out = []
    for chunk in spec.split(","):
        chunk = chunk.strip()
        if not chunk:
            continue
        k, _, v = chunk.partition(":")
        if k in ARCHETYPES:
            out.append((k, int(v)))
    return out


def parse_args(argv):
    import datetime
    ts = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    p = argparse.ArgumentParser(prog="run_p7_real")
    p.add_argument("--engine", default="http://127.0.0.1:8080")
    p.add_argument("--wall-seconds", type=int, default=240)
    p.add_argument("--llm", type=int, default=3, help="number of LLM focal agents")
    p.add_argument("--brain", default="qwen", choices=["qwen", "claude"],
                   help="LLM brain for focal agents (qwen=local, claude=Anthropic API)")
    p.add_argument("--cast", default="survivor:2,killer:1,manipulator:1",
                   help="rule-based cast e.g. survivor:2,killer:1,manipulator:1")
    p.add_argument("--narrator", action="store_true")
    p.add_argument("--max-claude", type=int, default=3)
    p.add_argument("--out", default=f".runlog/p7_real/{ts}")
    return p.parse_args(argv)


def main(argv=None) -> int:
    return asyncio.run(main_run(parse_args(argv if argv is not None else sys.argv[1:])))


if __name__ == "__main__":
    sys.exit(main())
