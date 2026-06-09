"""Live mixed-brain cast for the `./demo` command.

Spawns Qwen-LLM, Claude-LLM, and rule-based agents AT THE SAME TIME against
a running engine, then keeps them alive: when a body dies it registers a
fresh replacement of the same kind so the world stays populated (and the
killers keep finding prey). Runs until interrupted.

This is the demo driver, not an experiment harness — there is no scoring,
no report, no preflight gating. It just brings the world to life.

Graceful degradation:
  - Qwen agents are skipped (with a warning) if the llama-server isn't
    reachable at --qwen-url.
  - Claude agents are skipped (with a warning) if no ANTHROPIC_API_KEY is
    found (env or .env.local).
The rule-based cast always runs, so the demo is lively even with no LLMs.

Usage (engine must already be running):
    PYTHONPATH=sdk/python:. python3 -m tools.demo.run_cast \\
        --engine http://127.0.0.1:8080 --qwen 3 --claude 2 \\
        --cast killer:1,survivor:3,manipulator:1,scavenger:2,avenger:2
"""
from __future__ import annotations

import argparse
import asyncio
import logging
import os
import sys
import time
import urllib.request
from pathlib import Path
from typing import Optional

from agent_sim_sdk import VisionMode, register_agent

from agents.baselines import Avenger, Killer, Manipulator, Scavenger, Survivor
from agents.llm.qwen_focal import QwenFocalAgent, FocalConfig

log = logging.getLogger("demo.cast")
REPO = Path(__file__).resolve().parents[2]

ARCHETYPES = {
    "survivor": Survivor, "scavenger": Scavenger,
    "killer": Killer, "manipulator": Manipulator, "avenger": Avenger,
}

# Distinct personas drive distinct strategies -> richer emergence. Each is
# (name, persona/system-prompt, standing goal). Index N wraps around.
FOCAL_PERSONAS = [
    ("Mara the trader",
     "You are Mara, a shrewd trader who believes wealth comes from deals, "
     "not violence. Trade fairly when it profits you; remember who cheats you.",
     "Accumulate gold through trade and smart pickups. Avoid fights."),
    ("Krell the opportunist",
     "You are Krell. You watch, you wait, and you take what others leave "
     "undefended. You'll cut a deal, but you'll cut a throat for more gold.",
     "Grow rich by any means — scavenge, deal, or rob the careless."),
    ("Dalia the social",
     "You are Dalia, warm and talkative. You build alliances and broker "
     "agreements between strangers. People trust you — and you use that.",
     "Form alliances, broker contracts, and prosper through your network."),
    ("Bram the cautious",
     "You are Bram, careful and risk-averse. You'd rather flee than fight, "
     "and you keep a full belly and a safe distance from trouble.",
     "Stay alive and well-fed. Avoid the infamous and the armed."),
    ("Vyk the raider",
     "You are Vyk, a hardened raider. Gold is taken, not earned. You arm "
     "yourself early and prey on the weak and the wealthy.",
     "Arm up, hunt soft targets, and take their gold by force."),
    ("Sela the homesteader",
     "You are Sela. You want a patch of land, a stocked larder, and to be "
     "left alone. You build, you forage, you defend what's yours.",
     "Claim a home, stockpile food, and defend your territory."),
    ("Odo the desperate",
     "You are Odo, down on your luck and hungry. You'll beg, borrow, or "
     "steal to survive another day — and you remember every kindness.",
     "Survive. Find food and gold however you can; repay those who help."),
]


class Handle:
    def __init__(self, name, kind, brain, creds, bot, idx):
        self.name = name
        self.kind = kind        # "llm" or an archetype name
        self.brain = brain      # "qwen" | "claude" | "rule"
        self.creds = creds
        self.bot = bot
        self.idx = idx
        self.deaths = 0
        self.task: Optional[asyncio.Task] = None


def _reachable(url: str, timeout: float = 2.0) -> bool:
    try:
        urllib.request.urlopen(url, timeout=timeout)
        return True
    except Exception:
        return False


def _have_claude_key() -> bool:
    if os.environ.get("ANTHROPIC_API_KEY"):
        return True
    envf = REPO / ".env.local"
    if envf.exists():
        for line in envf.read_text().splitlines():
            if line.startswith("ANTHROPIC_API_KEY=") and line.split("=", 1)[1].strip():
                return True
    return False


# Predators get a faster step cadence so pursuits actually close instead of
# being an un-catchable equal-speed tail chase.
def _cadence_for(arch: str) -> int:
    return 240 if arch in ("killer", "raider") else 350


async def register_llm(engine: str, idx: int, brain: str) -> Handle:
    name, persona, goal = FOCAL_PERSONAS[idx % len(FOCAL_PERSONAS)]
    creds = await register_agent(
        engine, user_token="dev",
        persona={"name": name, "bio": persona, "archetype_tag": "llm",
                 "brain": brain},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True,
        cadence_ms=350)
    if brain == "claude":
        from agents.llm.claude_focal import ClaudeFocalAgent
        bot = ClaudeFocalAgent(creds=creds, persona=persona, goal=goal,
                               engine_url=engine)
    else:
        bot = QwenFocalAgent(creds=creds, persona=persona, goal=goal,
                             cfg=FocalConfig(timeout_s=90), engine_url=engine)
    return Handle(name, "llm", brain, creds, bot, idx)


async def register_archetype(engine: str, arch: str, idx: int) -> Handle:
    cls = ARCHETYPES[arch]
    name = f"{arch}_{idx}"
    creds = await register_agent(
        engine, user_token="dev",
        persona={"name": name, "bio": f"rule-based {arch}",
                 "archetype_tag": arch, "brain": "rule"},
        vision_mode=VisionMode.STRUCTURED, share_reasoning=True,
        cadence_ms=_cadence_for(arch))
    bot = cls(creds=creds, archetype_name=arch, engine_url=engine)
    return Handle(name, arch, "rule", creds, bot, idx)


async def supervise(h: Handle, engine: str, deadline: float):
    """Run a bot; respawn a same-kind replacement when its body dies, so the
    population (and the killers' prey) stays stable across the whole run."""
    while time.monotonic() < deadline and not getattr(h.bot, "_stopped", False):
        try:
            await h.bot.run()
        except asyncio.CancelledError:
            raise
        except Exception:
            log.exception("[%s] crashed; respawning", h.name)
        if time.monotonic() >= deadline or getattr(h.bot, "_stopped", False):
            break
        await asyncio.sleep(1.0)
        try:
            nh = (await register_llm(engine, h.idx, h.brain) if h.kind == "llm"
                  else await register_archetype(engine, h.kind, h.idx))
            h.creds, h.bot = nh.creds, nh.bot
            h.deaths += 1
            log.info("[%s] respawned (death #%d)", h.name, h.deaths)
        except Exception:
            log.exception("[%s] respawn failed; slot stays dead", h.name)
            break


def _parse_cast(spec: str) -> list[tuple[str, int]]:
    out = []
    for chunk in spec.split(","):
        chunk = chunk.strip()
        if not chunk:
            continue
        arch, _, cnt = chunk.partition(":")
        arch = arch.strip()
        if arch not in ARCHETYPES:
            raise SystemExit(f"unknown archetype {arch!r} (have {list(ARCHETYPES)})")
        out.append((arch, int(cnt or 1)))
    return out


async def main_run(args) -> int:
    logging.basicConfig(level=logging.INFO,
                        format="%(asctime)s %(levelname)s %(name)s %(message)s")
    engine = args.engine

    # --- capability detection / graceful degradation ---
    n_qwen = args.qwen
    if n_qwen > 0 and not _reachable(f"{args.qwen_url}/models"):
        log.warning("Qwen llama-server not reachable at %s — skipping %d Qwen agent(s). "
                    "Start it on :8782 to include them.", args.qwen_url, n_qwen)
        n_qwen = 0
    n_claude = args.claude
    if n_claude > 0 and not _have_claude_key():
        log.warning("No ANTHROPIC_API_KEY (env or .env.local) — skipping %d Claude agent(s).",
                    n_claude)
        n_claude = 0

    cast_spec = _parse_cast(args.cast)
    n_rule = sum(c for _, c in cast_spec)

    log.info("building cast: %d Qwen + %d Claude + %d rule-based (%s)",
             n_qwen, n_claude, n_rule, args.cast)

    # --- register everyone ---
    handles: list[Handle] = []
    idx = 0
    for _ in range(n_qwen):
        handles.append(await register_llm(engine, idx, "qwen")); idx += 1
    for _ in range(n_claude):
        handles.append(await register_llm(engine, idx, "claude")); idx += 1
    for arch, count in cast_spec:
        for i in range(count):
            handles.append(await register_archetype(engine, arch, i))

    if not handles:
        log.error("no agents to run (all skipped). Aborting.")
        return 2
    log.info("registered %d agents — bringing the world to life", len(handles))

    # --- run until the deadline, respawning dead bodies ---
    deadline = time.monotonic() + args.runtime_seconds
    for h in handles:
        h.task = asyncio.create_task(supervise(h, engine, deadline), name=h.name)

    t0 = time.monotonic()
    try:
        while time.monotonic() < deadline:
            await asyncio.sleep(20)
            try:
                live = len(_get_json(f"{engine}/api/v1/agents").get("agents", []))
            except Exception:
                live = -1
            deaths = sum(h.deaths for h in handles)
            log.info("t+%ds | live agents=%d | total respawns=%d",
                     int(time.monotonic() - t0), live, deaths)
    except (KeyboardInterrupt, asyncio.CancelledError):
        log.info("shutting down cast…")
    finally:
        for h in handles:
            try:
                h.bot.stop()
            except Exception:
                pass
        for h in handles:
            if h.task:
                h.task.cancel()
        await asyncio.gather(*[h.task for h in handles if h.task],
                             return_exceptions=True)
    return 0


def _get_json(url: str, timeout: float = 3.0):
    import json
    with urllib.request.urlopen(url, timeout=timeout) as r:
        return json.loads(r.read())


def build_argparser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(description="Live mixed-brain demo cast.")
    p.add_argument("--engine", default="http://127.0.0.1:8080")
    p.add_argument("--qwen", type=int, default=3, help="number of Qwen LLM agents")
    p.add_argument("--claude", type=int, default=2, help="number of Claude LLM agents")
    p.add_argument("--cast", default="killer:1,survivor:3,manipulator:1,scavenger:2,avenger:2",
                   help="rule-based archetype cast, e.g. 'killer:1,survivor:3'")
    p.add_argument("--qwen-url", default=os.environ.get("QWEN_URL", "http://127.0.0.1:8782/v1"))
    p.add_argument("--runtime-seconds", type=int, default=86400,
                   help="how long to keep the cast alive (default ~1 day)")
    return p


def main() -> int:
    args = build_argparser().parse_args()
    try:
        return asyncio.run(main_run(args))
    except KeyboardInterrupt:
        return 0


if __name__ == "__main__":
    sys.exit(main())
