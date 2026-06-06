"""End-to-end P7 demo run.

Registers a cast of rule-based archetype bots against a live engine,
starts the narrator as a subprocess, runs for ``--wall-seconds``, then
writes a structured metrics summary + closes everything cleanly.

The cast defaults match the design doc:
- 4 survivors  (population baseline, victim class for killers)
- 2 scavengers (death-driven looters; force opportunistic dynamics)
- 2 killers    (predators; force survival pressure on survivors)
- 2 manipulators (test the D13 contract substrate adversarially)

Usage:
    python -m tools.experiments.run_demo \\
        --wall-seconds 90 \\
        --engine http://127.0.0.1:8080 \\
        --narrator        \\
        --out .runlog/p7_demo

The runner DOES NOT spawn the engine — that needs to already be
running on the target host (use ./agent_sim start for the canonical
dev rig).
"""
from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import shutil
import signal
import subprocess
import sys
import time
from dataclasses import asdict, dataclass, field
from pathlib import Path
from typing import Optional

from agent_sim_sdk import (
    AgentCredentials, VisionMode, register_agent,
)
from agents.baselines import Killer, Manipulator, Scavenger, Survivor


log = logging.getLogger("agent_sim.experiments.run_demo")


REPO = Path(__file__).resolve().parents[2]
DEFAULT_ENGINE = os.environ.get("AGENT_SIM_ENGINE", "http://127.0.0.1:8080")


# ---- Cast spec -------------------------------------------------------


ARCHETYPES = {
    "survivor":    Survivor,
    "scavenger":   Scavenger,
    "killer":      Killer,
    "manipulator": Manipulator,
}


@dataclass
class CastEntry:
    archetype: str
    count: int


DEFAULT_CAST: list[CastEntry] = [
    CastEntry("survivor",    4),
    CastEntry("scavenger",   2),
    CastEntry("killer",      2),
    CastEntry("manipulator", 2),
]


@dataclass
class BotHandle:
    archetype: str
    name: str
    creds: AgentCredentials
    task: Optional[asyncio.Task] = None
    bot: Optional[object] = None


# ---- Register + start ------------------------------------------------


async def register_one(
    engine: str, archetype: str, idx: int,
) -> BotHandle:
    cls = ARCHETYPES[archetype]
    name = f"{archetype}_{idx}"
    persona = {
        "name": name,
        "bio":  f"P7 demo {archetype} bot {idx}.",
        "archetype_tag": archetype,
    }
    creds = await register_agent(
        engine, user_token="dev", persona=persona,
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True,
        cadence_ms=1000,
    )
    bot = cls(creds=creds, archetype_name=archetype)
    return BotHandle(archetype=archetype, name=name, creds=creds, bot=bot)


async def start_all(engine: str, cast: list[CastEntry]) -> list[BotHandle]:
    handles: list[BotHandle] = []
    for entry in cast:
        for i in range(entry.count):
            h = await register_one(engine, entry.archetype, i)
            handles.append(h)
            log.info("registered %s as %s (entity=%s)",
                     h.name, h.creds.agent_id, h.bot.creds.agent_id)
    # Start each bot's loop concurrently.
    for h in handles:
        h.task = asyncio.create_task(h.bot.run(), name=h.name)
    return handles


async def stop_all(handles: list[BotHandle]) -> None:
    for h in handles:
        if h.bot is not None:
            try:
                h.bot.stop()
            except Exception:
                pass
    # Give the bots a moment to notice the stop flag, then cancel
    # anything stuck.
    await asyncio.sleep(1.5)
    for h in handles:
        if h.task and not h.task.done():
            h.task.cancel()
    # Drain.
    await asyncio.gather(
        *(h.task for h in handles if h.task is not None),
        return_exceptions=True,
    )


# ---- Narrator subprocess --------------------------------------------


def start_narrator(
    *, events: Path, out: Path,
    max_qwen_calls: int = 200, max_claude_calls: int = 4,
    l1_ticks: int = 60 * 60, l2_ticks: int = 5 * 60 * 60,
    l3_ticks: int = 15 * 60 * 60,
) -> subprocess.Popen:
    cmd = [
        sys.executable, "-m", "tools.narrator",
        "--events", str(events),
        "--out",    str(out),
        "--max-qwen-calls",   str(max_qwen_calls),
        "--max-claude-calls", str(max_claude_calls),
        "--l1-ticks", str(l1_ticks),
        "--l2-ticks", str(l2_ticks),
        "--l3-ticks", str(l3_ticks),
        "--idle-exit", "8",
    ]
    log.info("spawning narrator: %s", " ".join(cmd))
    return subprocess.Popen(
        cmd, cwd=str(REPO),
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        env={**os.environ, "PYTHONUNBUFFERED": "1"},
    )


# ---- Metrics --------------------------------------------------------


def http_json(url: str, timeout: float = 5.0) -> dict:
    import urllib.request
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return json.loads(r.read())


def summarize_events(events_path: Path, after_tick: int = 0) -> dict:
    """Scan the engine's events.jsonl from ``after_tick`` to EOF and
    count kinds + a few specific cross-cuts (kills, contracts,
    speech volume). Returns a dict ready to dump in the summary."""
    counts: dict[str, int] = {}
    kills: list[dict] = []
    contracts: dict[str, dict] = {}
    transfers: list[dict] = []
    speakers: dict[str, int] = {}
    mental_notes: int = 0
    with events_path.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                ev = json.loads(line)
            except json.JSONDecodeError:
                continue
            if int(ev.get("tick", 0) or 0) < after_tick:
                continue
            kind = ev.get("kind", "")
            counts[kind] = counts.get(kind, 0) + 1
            p = ev.get("payload") or {}
            if kind == "EntityDied":
                # Historian emits EntityID + Killer (not VictimID +
                # KillerID); see combat.go's EntityDied struct.
                kills.append({"victim": p.get("EntityID") or p.get("VictimID"),
                              "killer": p.get("Killer") or p.get("KillerID"),
                              "cause":  p.get("Cause"),
                              "tick":   ev.get("tick")})
            elif kind == "TaskProposed":
                cid = p.get("ID")
                if cid:
                    contracts[cid] = {"status": "proposed",
                                      "proposer": p.get("Proposer"),
                                      "target":   p.get("Target"),
                                      "tick":     ev.get("tick")}
            elif kind in ("TaskAccepted", "TaskRejected", "TaskCompleted"):
                cid = p.get("ID")
                if cid in contracts:
                    contracts[cid]["status"] = {
                        "TaskAccepted":  "accepted",
                        "TaskRejected":  "rejected",
                        "TaskCompleted": "completed",
                    }[kind]
                    contracts[cid][f"{kind.lower()}_tick"] = ev.get("tick")
            elif kind == "ItemTransferred":
                transfers.append({"from": p.get("From"),
                                  "to":   p.get("To"),
                                  "item": p.get("Item"),
                                  "tick": ev.get("tick")})
            elif kind == "Speech":
                speakers[p.get("Speaker", "?")] = (
                    speakers.get(p.get("Speaker", "?"), 0) + 1)
            elif kind == "MentalNote":
                mental_notes += 1
    accepted = sum(1 for c in contracts.values()
                   if c["status"] in ("accepted", "completed"))
    completed = sum(1 for c in contracts.values()
                    if c["status"] == "completed")
    return {
        "counts_by_kind":   counts,
        "kills":            kills,
        "contracts_total":  len(contracts),
        "contracts_accepted": accepted,
        "contracts_completed": completed,
        "item_transfers":   len(transfers),
        "mental_notes":     mental_notes,
        "top_speakers":     sorted(speakers.items(), key=lambda kv: -kv[1])[:5],
    }


def collect_social_heatmap(engine: str, handles: list[BotHandle]) -> dict:
    """Hit /mental_state for each bot and collect peer counters into
    a single matrix. Returns {agent_name: {peer_name_or_id: counts}}."""
    by_entity_to_name = {h.bot.creds.agent_id: h.name for h in handles}  # type: ignore[union-attr]
    # entity_id from /agents endpoint is what mental_state knows.
    agents = http_json(f"{engine}/api/v1/agents").get("agents", [])
    agent_id_to_entity = {a.get("agent_id"): a.get("entity_id") for a in agents}
    out: dict[str, dict] = {}
    for h in handles:
        eid = agent_id_to_entity.get(h.creds.agent_id)
        if not eid:
            continue
        try:
            ms = http_json(
                f"{engine}/api/v1/agent/{eid}/mental_state", timeout=5)
        except Exception as e:
            log.warning("mental_state(%s) failed: %s", h.name, e)
            continue
        peers = ms.get("peers") or {}
        # Translate peer entity_id -> friendly archetype/name where we can.
        named = {}
        # Build a lookup from entity_id back to name for THIS run's bots.
        entity_to_name = {}
        for h2 in handles:
            eid2 = agent_id_to_entity.get(h2.creds.agent_id)
            if eid2:
                entity_to_name[eid2] = h2.name
        for peer_eid, c in peers.items():
            label = entity_to_name.get(peer_eid, peer_eid)
            named[label] = c
        out[h.name] = named
    return out


# ---- Main driver ----------------------------------------------------


async def run(args: argparse.Namespace) -> int:
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    out_dir = Path(args.out)
    out_dir.mkdir(parents=True, exist_ok=True)

    # Snapshot the starting tick so the summarizer reads a clean window.
    start_meta = http_json(f"{args.engine}/api/v1/world/info")
    start_tick = int(start_meta.get("tick", 0) or 0)
    log.info("start_tick=%d engine=%s", start_tick, args.engine)

    # Build cast.
    cast = DEFAULT_CAST if args.cast is None else _parse_cast(args.cast)

    # Optional narrator.
    narrator_proc: Optional[subprocess.Popen] = None
    if args.narrator:
        events = Path(args.engine_events) if args.engine_events else \
            REPO / ".runlog" / "events.jsonl"
        narrator_out = out_dir / "narrator.jsonl"
        narrator_proc = start_narrator(
            events=events, out=narrator_out,
            max_qwen_calls=args.max_qwen_calls,
            max_claude_calls=args.max_claude_calls,
        )

    # Register + start bots.
    handles = await start_all(args.engine, cast)
    log.info("started %d bots", len(handles))

    # Watch loop with periodic progress logging.
    t0 = time.monotonic()
    heatmap: dict = {}
    try:
        while time.monotonic() - t0 < args.wall_seconds:
            await asyncio.sleep(min(10, args.wall_seconds))
            elapsed = int(time.monotonic() - t0)
            try:
                agents = http_json(
                    f"{args.engine}/api/v1/agents", timeout=3)
                live = len(agents.get("agents", []))
            except Exception:
                live = -1
            log.info("t+%ds live_agents=%d", elapsed, live)
        # Collect the social heatmap BEFORE we stop bots — once a
        # WS closes the engine drops the agent record so /agents no
        # longer maps agent_id → entity_id, and the heatmap rows
        # come back empty.
        heatmap = collect_social_heatmap(args.engine, handles)
        log.info("collected social heatmap (%d rows)", len(heatmap))
    finally:
        log.info("stopping bots…")
        await stop_all(handles)
        if narrator_proc is not None:
            log.info("terminating narrator…")
            narrator_proc.send_signal(signal.SIGINT)
            try:
                narrator_proc.wait(timeout=20)
            except subprocess.TimeoutExpired:
                narrator_proc.kill()

    # Collect outputs.
    events = Path(args.engine_events) if args.engine_events else \
        REPO / ".runlog" / "events.jsonl"
    metrics = summarize_events(events, after_tick=start_tick)

    summary = {
        "engine":           args.engine,
        "wall_seconds":     args.wall_seconds,
        "start_tick":       start_tick,
        "cast":             [asdict(c) for c in cast],
        "registered_bots":  [{"name": h.name, "agent_id": h.creds.agent_id,
                              "archetype": h.archetype} for h in handles],
        "metrics":          metrics,
        "social_heatmap":   heatmap,
        "narrator_jsonl":   str(out_dir / "narrator.jsonl") if args.narrator else None,
    }
    (out_dir / "summary.json").write_text(
        json.dumps(summary, indent=2, default=str), encoding="utf-8")
    log.info("wrote %s", out_dir / "summary.json")
    return 0


def _parse_cast(s: str) -> list[CastEntry]:
    """e.g. 'survivor:4,killer:2,manipulator:2'."""
    out: list[CastEntry] = []
    for chunk in s.split(","):
        chunk = chunk.strip()
        if not chunk:
            continue
        k, _, v = chunk.partition(":")
        out.append(CastEntry(k, int(v)))
    return out


def parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(prog="run_demo")
    p.add_argument("--engine", default=DEFAULT_ENGINE)
    p.add_argument("--wall-seconds", type=int, default=90)
    p.add_argument("--out", default=".runlog/p7_demo")
    p.add_argument("--cast", default=None,
                   help="override e.g. survivor:4,killer:2,manipulator:2")
    p.add_argument("--narrator", action="store_true",
                   help="start the narrator subprocess in parallel")
    p.add_argument("--engine-events", default=None,
                   help="path to engine events.jsonl (defaults to .runlog/events.jsonl)")
    p.add_argument("--max-qwen-calls", type=int, default=30)
    p.add_argument("--max-claude-calls", type=int, default=2)
    p.add_argument("--verbose", action="store_true")
    return p.parse_args(argv)


def main(argv: Optional[list[str]] = None) -> int:
    argv = list(argv if argv is not None else sys.argv[1:])
    args = parse_args(argv)
    return asyncio.run(run(args))


if __name__ == "__main__":
    sys.exit(main())
