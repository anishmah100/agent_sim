"""Hierarchical agent — slow LLM brain + fast deterministic controller.

The HFT-market-maker analogy:
  - SLOW brain (Qwen3.6 via llama.cpp on :8782) is the "fair value
    algorithm". It ticks every 8 seconds with a compressed observation
    summary and sets a high-level Goal: kill X, gather wood, build a
    cottage at (10,10), trade for bread.
  - FAST controller is the "quoter / FPGA". It ticks every cadence_ms
    (default 600ms) and executes one primitive Action based on the
    current Goal — pathfind toward a target, attack when adjacent,
    chop trees, advance construction, etc.

Goal is the ONLY shared state between layers. Easy to test, easy to
log, easy to render in the inspector.

Run after `./start.sh` (and start llama-server on 8782):
    python examples/hierarchical_agent.py --server http://127.0.0.1:8080 --token dev
"""

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import sys
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Optional

import httpx

# Make the local SDK importable when this script is run from the repo root.
sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "sdk" / "python"))

from agent_sim_sdk import (  # noqa: E402
    Action, Attack, Chop, Mine, Move, Speak, Wait, Pay, Trade, Loot,
    Enter, Exit, ClaimOwnership, PlaceBlueprint, AdvanceConstruction,
    ProposeTask, AcceptTask, CompleteTask,
    VisionMode, register_and_connect,
)

LLM_URL = "http://127.0.0.1:8782/v1/chat/completions"
LLM_MODEL = "qwen3"
LLM_TICK_INTERVAL_S = 8.0
LOG = logging.getLogger("hierarchical")


# === Goal ===

GOAL_KINDS = (
    "kill", "gather_wood", "gather_stone", "build_cottage", "trade",
    "claim_unowned_building", "flee", "idle", "patrol",
)


@dataclass
class Goal:
    """The shared state between LLM brain and controller. Brain mutates
    this every ~8 s; controller reads it every cadence_ms.

    target_id and target_pos may both be None — the controller has
    fallbacks for under-specified goals (e.g. gather_wood with no
    target_id picks the nearest visible tree)."""

    kind: str = "idle"
    target_id: Optional[str] = None
    target_pos: Optional[tuple[int, int]] = None
    target_qty: Optional[int] = None
    notes: Optional[str] = None     # LLM's reasoning, shown in inspector

    @classmethod
    def from_json(cls, d: dict) -> "Goal":
        return cls(
            kind=str(d.get("kind", "idle")),
            target_id=d.get("target_id"),
            target_pos=tuple(d["target_pos"]) if d.get("target_pos") else None,
            target_qty=d.get("target_qty"),
            notes=d.get("notes"),
        )


# === Slow brain — LLM picks goals ===

SYSTEM_PROMPT = """You are the strategic brain of an NPC in a top-down RPG.
A fast deterministic controller handles moment-to-moment actions.
You set high-level GOALS that the controller pursues until you change them.

Available goal kinds:
  - kill              — slay a target (set target_id)
  - gather_wood       — chop trees until you have N wood (target_qty, optional target_id)
  - gather_stone      — mine rocks until you have N stone (target_qty, optional target_id)
  - build_cottage     — place + advance a cottage blueprint at target_pos
  - trade             — buy/sell with a merchant (set target_id)
  - claim_unowned_building — walk to + claim an unowned building (target_id)
  - flee              — move away from threats (target_pos = safe direction)
  - patrol            — wander around (no params)
  - idle              — stand and look

Reply with EXACTLY ONE JSON object on a single line. No prose.
Example: {"kind":"gather_wood","target_qty":5,"notes":"need wood for cottage"}
Example: {"kind":"kill","target_id":"goblin_2","notes":"goblin near village"}
Example: {"kind":"build_cottage","target_pos":[14,18],"notes":"open lot near plaza"}
"""


def _summarize_obs(obs, current_goal: Goal) -> str:
    me = obs.self
    parts = [
        f"You ({me.entity_id}) at {me.pos}, facing {me.facing}.",
        f"HP={me.extras.get('hp','?')}  gold={me.extras.get('gold','?')}",
        f"Inventory: {me.extras.get('inventory', [])}",
        f"Current goal: {asdict(current_goal)}",
        f"World tick: {obs.world_tick}, phase: {obs.world_clock.day_phase}",
        "Visible:" if obs.visible_entities else "Visible: (none)",
    ]
    for v in obs.visible_entities[:10]:
        hp = v.extras_summary.get("hp", "?")
        parts.append(
            f"  - {v.entity_id} ({v.archetype}, {v.apparent_label}) at {v.pos}, hp={hp}"
        )
    if me.last_action_result:
        parts.append(f"Last action: {me.last_action_result}")
    return "\n".join(parts)


async def _ask_llm(prompt: str) -> dict:
    body = {
        "model": LLM_MODEL,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": prompt},
        ],
        "temperature": 0.4,
        "max_tokens": 120,
        "response_format": {"type": "json_object"},
    }
    async with httpx.AsyncClient(timeout=30.0) as h:
        r = await h.post(LLM_URL, json=body)
        r.raise_for_status()
        text = r.json()["choices"][0]["message"]["content"].strip()
    s, e = text.find("{"), text.rfind("}")
    if s < 0 or e < 0:
        return {"kind": "idle"}
    try:
        return json.loads(text[s : e + 1])
    except json.JSONDecodeError:
        return {"kind": "idle"}


# === Fast controller — deterministic action picker ===


def _chebyshev(a, b) -> int:
    return max(abs(a[0] - b[0]), abs(a[1] - b[1]))


def _step_toward(me_pos, target_pos) -> tuple[int, int]:
    dx = (1 if target_pos[0] > me_pos[0] else
          -1 if target_pos[0] < me_pos[0] else 0)
    dy = (1 if target_pos[1] > me_pos[1] else
          -1 if target_pos[1] < me_pos[1] else 0)
    return (me_pos[0] + dx, me_pos[1] + dy)


def _step_away(me_pos, threat_pos) -> tuple[int, int]:
    dx = (-1 if threat_pos[0] > me_pos[0] else
          1 if threat_pos[0] < me_pos[0] else 0)
    dy = (-1 if threat_pos[1] > me_pos[1] else
          1 if threat_pos[1] < me_pos[1] else 0)
    return (me_pos[0] + dx, me_pos[1] + dy)


def _find_by_archetype(obs, archetype: str):
    for v in obs.visible_entities:
        if v.archetype == archetype:
            return v
    return None


def _find_by_id(obs, eid: str):
    for v in obs.visible_entities:
        if v.entity_id == eid:
            return v
    return None


def _inventory_count(obs, prefix: str) -> int:
    inv = obs.self.extras.get("inventory", []) or []
    return sum(1 for x in inv if isinstance(x, str) and x.startswith(prefix))


def controller_decide(obs, goal: Goal) -> Action:
    """The fast path. Sees the current obs + goal, picks ONE primitive
    Action. Pure function — no side effects, no LLM call."""
    me = obs.self
    me_pos = me.pos

    if goal.kind == "idle":
        return Wait(ticks=30)

    if goal.kind == "patrol":
        # Random walk toward a non-self visible entity, or just east.
        dx, dy = (1, 0) if (obs.world_tick // 30) % 4 < 2 else (0, 1)
        return Move(target=(me_pos[0] + dx, me_pos[1] + dy))

    if goal.kind == "flee":
        if goal.target_pos is not None:
            return Move(target=_step_away(me_pos, goal.target_pos))
        threats = [v for v in obs.visible_entities if v.archetype == "goblin"]
        if threats:
            t = min(threats, key=lambda v: _chebyshev(me_pos, v.pos))
            return Move(target=_step_away(me_pos, t.pos))
        return Wait(ticks=60)

    if goal.kind == "kill":
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            # No target visible — wander.
            return Wait(ticks=30)
        if _chebyshev(me_pos, tgt.pos) <= 1:
            return Attack(target=tgt.entity_id)
        return Move(target=_step_toward(me_pos, tgt.pos))

    if goal.kind == "gather_wood":
        if goal.target_qty and _inventory_count(obs, "wood") >= goal.target_qty:
            return Speak(text="Got enough wood.")
        # Find a tree — prefer the goal's target_id, fall back to nearest.
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            tgt = _find_by_archetype(obs, "tree")
        if tgt is None:
            return Wait(ticks=60)
        if _chebyshev(me_pos, tgt.pos) <= 1:
            return Chop(target=tgt.entity_id)
        return Move(target=_step_toward(me_pos, tgt.pos))

    if goal.kind == "gather_stone":
        if goal.target_qty and _inventory_count(obs, "stone") >= goal.target_qty:
            return Speak(text="Got enough stone.")
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            tgt = _find_by_archetype(obs, "rock")
        if tgt is None:
            return Wait(ticks=60)
        if _chebyshev(me_pos, tgt.pos) <= 1:
            return Mine(target=tgt.entity_id)
        return Move(target=_step_toward(me_pos, tgt.pos))

    if goal.kind == "build_cottage":
        if goal.target_pos is None:
            return Wait(ticks=60)
        site = goal.target_pos
        # If a blueprint already exists at site, advance it.
        bp = None
        for v in obs.visible_entities:
            if (v.archetype == "blueprint" and
                v.pos[0] == site[0] and v.pos[1] == site[1]):
                bp = v
                break
        if bp is not None:
            if _chebyshev(me_pos, bp.pos) <= 1:
                return AdvanceConstruction(target=bp.entity_id)
            return Move(target=_step_toward(me_pos, bp.pos))
        # No blueprint yet — get adjacent + place one.
        if _chebyshev(me_pos, site) <= 1:
            return PlaceBlueprint(kind="cottage", at=site)
        return Move(target=_step_toward(me_pos, site))

    if goal.kind == "trade":
        merchant = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if merchant is None:
            return Wait(ticks=60)
        if _chebyshev(me_pos, merchant.pos) <= 1:
            inv = me.extras.get("inventory", []) or []
            if inv:
                return Trade(target=merchant.entity_id, item=inv[0], price=2)
            return Speak(text="I have nothing to sell.")
        return Move(target=_step_toward(me_pos, merchant.pos))

    if goal.kind == "claim_unowned_building":
        bld = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if bld is None:
            return Wait(ticks=60)
        if _chebyshev(me_pos, bld.pos) <= 1:
            return ClaimOwnership(target=bld.entity_id)
        return Move(target=_step_toward(me_pos, bld.pos))

    # Unknown goal — idle.
    return Wait(ticks=30)


# === Top-level orchestration ===


class HierarchicalAgent:
    def __init__(self) -> None:
        self.goal = Goal(kind="patrol", notes="initial — wander")
        # Latest obs is captured by the controller_brain callback. The
        # slow brain_loop reads from it without ever needing to pull
        # from the SDK's observation stream itself.
        self.latest_obs = None
        self._brain_busy = False

    async def brain_loop(self) -> None:
        """Periodically asks the LLM to set a new goal."""
        while True:
            await asyncio.sleep(LLM_TICK_INTERVAL_S)
            if self.latest_obs is None or self._brain_busy:
                continue
            self._brain_busy = True
            try:
                prompt = _summarize_obs(self.latest_obs, self.goal)
                LOG.info("[brain] asking LLM (current goal=%s)", self.goal.kind)
                try:
                    d = await asyncio.wait_for(_ask_llm(prompt), timeout=20.0)
                    new_goal = Goal.from_json(d)
                    LOG.info("[brain] new goal: kind=%s target=%s notes=%s",
                             new_goal.kind, new_goal.target_id or new_goal.target_pos,
                             new_goal.notes)
                    self.goal = new_goal
                except (httpx.HTTPError, asyncio.TimeoutError) as e:
                    LOG.warning("[brain] LLM call failed: %s — keeping goal", e)
            finally:
                self._brain_busy = False

    async def controller_brain(self, obs):
        """Plug into the SDK's brain callback. Captures the obs for the
        slow brain to read later, then picks one primitive action. Always
        returns within a few ms — the LLM brain is independent."""
        self.latest_obs = obs
        return controller_decide(obs, self.goal)


async def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True)
    p.add_argument("--token", required=True)
    p.add_argument("--name", default="Hieron")
    p.add_argument("--cadence-ms", type=int, default=600)
    args = p.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(name)s %(message)s",
    )

    h_agent = HierarchicalAgent()

    sdk_agent = await register_and_connect(
        args.server,
        user_token=args.token,
        persona={
            "display_name": args.name,
            "bio": "Hierarchical NPC — slow LLM brain, fast controller.",
        },
        vision_mode=VisionMode.STRUCTURED,
        cadence_ms=args.cadence_ms,
        brain=h_agent.controller_brain,
    )
    try:
        # Run the slow brain loop alongside the SDK's fast controller loop.
        await h_agent.brain_loop()
    finally:
        await sdk_agent.close()


if __name__ == "__main__":
    asyncio.run(main())
