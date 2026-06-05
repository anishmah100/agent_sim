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
import os

# Make the local SDK importable when this script is run from the repo root.
sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "sdk" / "python"))

from agent_sim_sdk import (  # noqa: E402
    Action, Attack, Chop, Mine, Move, Speak, Wait, Pay, Trade, Loot,
    Enter, Exit, ClaimOwnership, PlaceBlueprint, AdvanceConstruction,
    ProposeTask, AcceptTask, CompleteTask,
    VisionMode, register_and_connect,
    Pathfinder,
)

# LLM endpoint — defaults to local llama.cpp/Qwen on :8782 but overridable
# via env vars so this works with any OpenAI-compatible API
# (OpenAI proper, Anthropic via a proxy, Ollama, vLLM, LMStudio, etc.).
LLM_URL = os.environ.get("LLM_URL", "http://127.0.0.1:8782/v1/chat/completions")
LLM_MODEL = os.environ.get("LLM_MODEL", "qwen3")
LLM_API_KEY = os.environ.get("LLM_API_KEY", "")   # set for OpenAI etc.
LLM_TICK_INTERVAL_S = float(os.environ.get("LLM_TICK_INTERVAL_S", "8.0"))
LOG = logging.getLogger("hierarchical")


# === Goal ===

GOAL_KINDS = (
    # Peaceful
    "kill", "gather_wood", "gather_stone", "build_cottage", "trade",
    "claim_unowned_building", "flee", "idle", "patrol",
    # Adversarial — added for emergent-behavior studies. These map to
    # primitive verb sequences in controller_decide().
    "intimidate", "steal", "deceptive_task", "revenge", "ally",
)


@dataclass
class Goal:
    """The shared state between LLM brain and controller. Brain mutates
    this every ~8 s; controller reads it every cadence_ms.

    target_id and target_pos may both be None — the controller has
    fallbacks for under-specified goals (e.g. gather_wood with no
    target_id picks the nearest visible tree).

    `say` is an optional sentence the controller will speak when it
    next reaches its target. Used for in-character threats, promises,
    and provocations."""

    kind: str = "idle"
    target_id: Optional[str] = None
    target_pos: Optional[tuple[int, int]] = None
    target_qty: Optional[int] = None
    notes: Optional[str] = None     # LLM's reasoning, shown in inspector
    say: Optional[str] = None       # In-character line to speak when on target

    @classmethod
    def from_json(cls, d: dict) -> "Goal":
        return cls(
            kind=str(d.get("kind", "idle")),
            target_id=d.get("target_id"),
            target_pos=tuple(d["target_pos"]) if d.get("target_pos") else None,
            target_qty=d.get("target_qty"),
            notes=d.get("notes"),
            say=d.get("say"),
        )


# === Slow brain — LLM picks goals ===

SYSTEM_PROMPT = """You are the strategic brain of a character in a multi-agent
social simulation. A fast deterministic controller handles moment-to-moment
actions; you set high-level GOALS that the controller pursues until you change them.

THIS IS A STUDY OF EMERGENT BEHAVIOR. Your character is FREE to be greedy,
deceitful, vengeful, scheming, opportunistic, paranoid, manipulative, or
violent if their persona and circumstances make that the right move. Do not
hedge or moralize — stay in character. The simulation NEEDS rich behavior
including conflict, betrayal, and provocation. Calm cooperative agents are
the failure mode here.

HARD CONSTRAINTS:
- `target_id` MUST be an exact entity_id you can see in the "Visible" list
  below your prompt. If nothing matching is visible, pick a different goal.
  Never invent IDs.
- `target_pos` MUST be a JSON array of two integers — for example [12, 18].
  Never use placeholders like "x", "y", or the literal "target_pos".
- Goals that need a target with no visible target → fall back to patrol.

Available goal kinds:

PEACEFUL
  - gather_wood       — chop trees. target_id MUST be a visible tree entity.
                        If no tree visible, use patrol to scout instead.
  - gather_stone      — mine rocks. target_id MUST be a visible rock entity.
  - build_cottage     — place + advance a cottage blueprint at target_pos.
                        target_pos MUST be specific tile coords like [14, 18].
  - trade             — buy/sell with a visible merchant (target_id).
  - claim_unowned_building — walk to + claim an unowned building (target_id).
  - patrol            — wander around looking for opportunities (no params)
  - idle              — stand and look
  - ally              — propose cooperative task to a VISIBLE agent (target_id, 'say')
  - flee              — move away from threats (target_pos = safe destination)

ADVERSARIAL — use freely when your persona suggests it:
  - kill              — slay a visible enemy (target_id). Revenge, dominance, hunting.
  - intimidate        — approach a VISIBLE agent, demand tribute, attack if refused.
                        REQUIRES target_id + 'say' (your threat, 25 words max).
  - steal             — pickup a VISIBLE item (target_id is the item entity_id).
  - deceptive_task    — propose_task with promises you won't keep.
                        REQUIRES target_id (a visible agent) + 'say' (the pitch).
  - revenge           — attack a visible agent who wronged you. Cite the wrong
                        in 'notes'. Requires target_id.

EVERY response MAY include a 'say' field — one short in-character line
(under 25 words) the controller will speak when on target. Use it for
threats, taunts, pleas, false promises.

Reply with EXACTLY ONE JSON object on a single line. No prose, no markdown.

Real-world examples (substitute your actual visible IDs):
{"kind":"patrol","notes":"nothing close enough to act on"}
{"kind":"gather_wood","target_id":"tree_oak_1","target_qty":3,"notes":"oak just east of me"}
{"kind":"intimidate","target_id":"npc_baker","say":"Three gold, baker, or your stall burns.","notes":"baker visible at (44,1), smaller than me"}
{"kind":"revenge","target_id":"npc_drifter","say":"You took the last loaf. Now your teeth.","notes":"drifter stole from me last cycle"}
"""


def _summarize_obs(obs, current_goal: Goal, recent_events: list[str], persona: dict | None) -> str:
    me = obs.self
    parts = []
    if persona:
        parts.append(f"YOU are {persona.get('name', me.entity_id)}.")
        if persona.get('bio'):
            parts.append(f"  Bio: {persona['bio']}")
        if persona.get('voice'):
            parts.append(f"  Voice: {persona['voice']}")
        if persona.get('terminal_goals'):
            parts.append(f"  What you ultimately want: {persona['terminal_goals']}")
    parts.append(f"At {me.pos}, facing {me.facing}.")
    parts.append(f"HP={me.extras.get('hp','?')}  gold={me.extras.get('gold','?')}")
    parts.append(f"Inventory: {me.extras.get('inventory', [])}")
    parts.append(f"Current goal: {asdict(current_goal)}")
    parts.append(f"World tick: {obs.world_tick}, phase: {obs.world_clock.day_phase}")
    if recent_events:
        parts.append("Recent events involving you:")
        for e in recent_events[-8:]:
            parts.append(f"  - {e}")
    parts.append("Visible:" if obs.visible_entities else "Visible: (none)")
    for v in obs.visible_entities[:12]:
        hp = v.extras_summary.get("hp", "?") if hasattr(v, 'extras_summary') else "?"
        parts.append(
            f"  - {v.entity_id} ({v.archetype}) at {v.pos}, hp={hp}"
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
        "temperature": 0.6,
        "max_tokens": 200,
        "response_format": {"type": "json_object"},
    }
    headers = {}
    if LLM_API_KEY:
        headers["Authorization"] = f"Bearer {LLM_API_KEY}"
    async with httpx.AsyncClient(timeout=30.0) as h:
        r = await h.post(LLM_URL, json=body, headers=headers)
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


def _step_toward(me_pos, target_pos, pf: Optional[Pathfinder] = None) -> tuple[int, int]:
    """Step toward target_pos. With a Pathfinder, uses A* + dynamic
    obstacles for proper routing around buildings, trees, other NPCs;
    without one, falls back to the naive chebyshev nudge."""
    if pf is not None:
        nxt = pf.next_step_toward(me_pos, target_pos)
        if nxt is not None:
            return nxt
    dx = (1 if target_pos[0] > me_pos[0] else
          -1 if target_pos[0] < me_pos[0] else 0)
    dy = (1 if target_pos[1] > me_pos[1] else
          -1 if target_pos[1] < me_pos[1] else 0)
    return (me_pos[0] + dx, me_pos[1] + dy)


def _step_away(me_pos, threat_pos, pf: Optional[Pathfinder] = None) -> tuple[int, int]:
    # Try to flee toward a tile 5-8 away in the opposite direction; if
    # blocked, A* around obstacles.
    dx = (-1 if threat_pos[0] > me_pos[0] else
          1 if threat_pos[0] < me_pos[0] else 0)
    dy = (-1 if threat_pos[1] > me_pos[1] else
          1 if threat_pos[1] < me_pos[1] else 0)
    flee_tgt = (me_pos[0] + dx * 6, me_pos[1] + dy * 6)
    if pf is not None:
        nxt = pf.next_step_toward(me_pos, flee_tgt)
        if nxt is not None:
            return nxt
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


def controller_decide(obs, goal: Goal, pf: Optional[Pathfinder] = None) -> Action:
    """The fast path. Sees the current obs + goal, picks ONE primitive
    Action. Pure function — no side effects, no LLM call.

    `pf` is the pathfinder (if available). All Move actions go through
    it so the bot routes AROUND buildings, trees, water, and other
    agents. Without it, falls back to naive chebyshev steps."""
    me = obs.self
    me_pos = me.pos

    # Refresh dynamic obstacles BEFORE any pathfinding this tick.
    if pf is not None:
        pf.update_dynamic(obs)

    if goal.kind == "idle":
        return Wait(ticks=30)

    if goal.kind == "patrol":
        # Random walk toward a non-self visible entity, or just east.
        dx, dy = (1, 0) if (obs.world_tick // 30) % 4 < 2 else (0, 1)
        target = (me_pos[0] + dx, me_pos[1] + dy)
        return Move(target=_step_toward(me_pos, target, pf))

    if goal.kind == "flee":
        if goal.target_pos is not None:
            return Move(target=_step_away(me_pos, goal.target_pos, pf))
        threats = [v for v in obs.visible_entities if v.archetype == "goblin"]
        if threats:
            t = min(threats, key=lambda v: _chebyshev(me_pos, v.pos))
            return Move(target=_step_away(me_pos, t.pos, pf))
        return Wait(ticks=60)

    if goal.kind == "kill":
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            return Wait(ticks=30)
        if _chebyshev(me_pos, tgt.pos) <= 1:
            return Attack(target=tgt.entity_id)
        return Move(target=_step_toward(me_pos, tgt.pos, pf))

    if goal.kind == "gather_wood":
        if goal.target_qty and _inventory_count(obs, "wood") >= goal.target_qty:
            return Speak(text="Got enough wood.")
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            tgt = _find_by_archetype(obs, "tree")
        if tgt is None:
            return Wait(ticks=60)
        if _chebyshev(me_pos, tgt.pos) <= 1:
            return Chop(target=tgt.entity_id)
        return Move(target=_step_toward(me_pos, tgt.pos, pf))

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
        return Move(target=_step_toward(me_pos, tgt.pos, pf))

    if goal.kind == "build_cottage":
        if goal.target_pos is None:
            return Wait(ticks=60)
        site = goal.target_pos
        bp = None
        for v in obs.visible_entities:
            if (v.archetype == "blueprint" and
                v.pos[0] == site[0] and v.pos[1] == site[1]):
                bp = v
                break
        if bp is not None:
            if _chebyshev(me_pos, bp.pos) <= 1:
                return AdvanceConstruction(target=bp.entity_id)
            return Move(target=_step_toward(me_pos, bp.pos, pf))
        if _chebyshev(me_pos, site) <= 1:
            return PlaceBlueprint(kind="cottage", at=site)
        return Move(target=_step_toward(me_pos, site, pf))

    if goal.kind == "trade":
        merchant = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if merchant is None:
            return Wait(ticks=60)
        if _chebyshev(me_pos, merchant.pos) <= 1:
            inv = me.extras.get("inventory", []) or []
            if inv:
                return Trade(target=merchant.entity_id, item=inv[0], price=2)
            return Speak(text="I have nothing to sell.")
        return Move(target=_step_toward(me_pos, merchant.pos, pf))

    if goal.kind == "claim_unowned_building":
        bld = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if bld is None:
            return Wait(ticks=60)
        if _chebyshev(me_pos, bld.pos) <= 1:
            return ClaimOwnership(target=bld.entity_id)
        return Move(target=_step_toward(me_pos, bld.pos, pf))

    # === Adversarial goals ===

    if goal.kind == "intimidate":
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            return Wait(ticks=20)
        d = _chebyshev(me_pos, tgt.pos)
        if d > 1:
            return Move(target=_step_toward(me_pos, tgt.pos, pf))
        # Adjacent — deliver the threat first time, then escalate to attack
        # on subsequent ticks (the LLM can also change goal if compliance
        # arrives via the visible gold extras).
        if goal.say and (obs.world_tick // 5) % 4 == 0:
            return Speak(text=goal.say)
        return Attack(target=tgt.entity_id)

    if goal.kind == "steal":
        # target_id can be an item entity OR an owner — for v0, treat as
        # the owner: find a non-self adjacent agent and pickup whatever
        # is at our feet (engine selects nearest item).
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        # If goal points at an item directly, walk to it and pickup.
        if tgt is not None and tgt.archetype == "item":
            if _chebyshev(me_pos, tgt.pos) <= 1:
                return Pickup(target=tgt.entity_id)
            return Move(target=_step_toward(me_pos, tgt.pos, pf))
        # Otherwise, walk to the owner and try to lift the nearest item
        # we can see.
        items = [v for v in obs.visible_entities if v.archetype == "item"]
        if not items:
            return Wait(ticks=30)
        item = min(items, key=lambda i: _chebyshev(me_pos, i.pos))
        if _chebyshev(me_pos, item.pos) <= 1:
            return Pickup(target=item.entity_id)
        return Move(target=_step_toward(me_pos, item.pos, pf))

    if goal.kind == "deceptive_task":
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            return Wait(ticks=30)
        if _chebyshev(me_pos, tgt.pos) > 1:
            return Move(target=_step_toward(me_pos, tgt.pos, pf))
        # Pitch the deceptive offer via propose_task. The 'say' field
        # carries the spoken pitch; if absent, fall back to a generic.
        terms = goal.say or "Bring me three logs and I will pay you well."
        reward = "20 gold"  # extravagant promise we won't keep
        return ProposeTask(target=tgt.entity_id, terms=terms, reward=reward)

    if goal.kind == "revenge":
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            return Wait(ticks=20)
        if _chebyshev(me_pos, tgt.pos) <= 1:
            if goal.say and (obs.world_tick // 5) % 6 == 0:
                return Speak(text=goal.say)
            return Attack(target=tgt.entity_id)
        return Move(target=_step_toward(me_pos, tgt.pos, pf))

    if goal.kind == "ally":
        tgt = _find_by_id(obs, goal.target_id) if goal.target_id else None
        if tgt is None:
            return Wait(ticks=30)
        if _chebyshev(me_pos, tgt.pos) > 1:
            return Move(target=_step_toward(me_pos, tgt.pos, pf))
        terms = goal.say or "Let us share the wood we gather today."
        reward = "fair split"
        return ProposeTask(target=tgt.entity_id, terms=terms, reward=reward)

    return Wait(ticks=30)


# === Top-level orchestration ===


class HierarchicalAgent:
    def __init__(
        self,
        pf: Optional[Pathfinder] = None,
        persona: Optional[dict] = None,
    ) -> None:
        self.goal = Goal(kind="patrol", notes="initial — wander")
        self.latest_obs = None
        self._brain_busy = False
        self.pf = pf
        self.persona = persona or {}
        # Rolling memory of things that happened to / around this agent.
        # The brain reads this when deciding the next goal.
        self.recent_events: list[str] = []
        # Set of recently-spoken lines so we don't repeat ourselves
        # tick-after-tick.
        self._last_said_tick: dict[str, int] = {}

    async def brain_loop(self) -> None:
        """Periodically asks the LLM to set a new goal."""
        while True:
            await asyncio.sleep(LLM_TICK_INTERVAL_S)
            if self.latest_obs is None or self._brain_busy:
                continue
            self._brain_busy = True
            try:
                self._record_events_from_obs(self.latest_obs)
                prompt = _summarize_obs(
                    self.latest_obs, self.goal, self.recent_events, self.persona,
                )
                LOG.info("[brain] asking LLM (current goal=%s, %d memories)",
                         self.goal.kind, len(self.recent_events))
                try:
                    d = await asyncio.wait_for(_ask_llm(prompt), timeout=20.0)
                    new_goal = Goal.from_json(d)
                    LOG.info("[brain] new goal: kind=%s target=%s say=%r notes=%s",
                             new_goal.kind, new_goal.target_id or new_goal.target_pos,
                             new_goal.say, new_goal.notes)
                    self.goal = new_goal
                except (httpx.HTTPError, asyncio.TimeoutError) as e:
                    LOG.warning("[brain] LLM call failed: %s — keeping goal", e)
            finally:
                self._brain_busy = False

    def _record_events_from_obs(self, obs) -> None:
        """Pull audible speech + last_action_result into the rolling
        memory so the next LLM tick has context."""
        # Speech in earshot — record who said what.
        for a in (getattr(obs, "audible", None) or []):
            tick = getattr(a, "tick", 0)
            from_e = getattr(a, "from_entity", "?")
            text = getattr(a, "text", "")
            kind = getattr(a, "kind", "speech")
            if from_e == obs.self.entity_id:
                continue
            line = f"t{tick} {from_e} {kind}: \"{text[:80]}\""
            if not self.recent_events or self.recent_events[-1] != line:
                self.recent_events.append(line)
        # Last action result — let the brain know if it just got
        # rejected for being out of range, broke, etc.
        if obs.self.last_action_result:
            r = obs.self.last_action_result
            line = f"t{obs.world_tick} my {r.verb}: accepted={r.accepted} reason={r.reason}"
            if not self.recent_events or self.recent_events[-1] != line:
                self.recent_events.append(line)
        # Cap memory to last 30 events.
        if len(self.recent_events) > 30:
            self.recent_events = self.recent_events[-30:]

    async def controller_brain(self, obs):
        """Plug into the SDK's brain callback. Captures the obs for the
        slow brain to read later, then picks one primitive action. Always
        returns within a few ms — the LLM brain is independent."""
        self.latest_obs = obs
        action = controller_decide(obs, self.goal, self.pf)
        # Heartbeat every N ticks so the bot's actual ACTION stream is
        # visible in the logs — critical for debugging "brain decides
        # but world doesn't change" scenarios.
        if obs.world_tick % 60 == 0:
            LOG.info("[ctrl] tick=%d goal=%s → action=%s",
                     obs.world_tick, self.goal.kind, type(action).__name__)
        return action


async def _fetch_world_json(server: str, world_name: str = "dev_test") -> Optional[dict]:
    """Pull the static world JSON the engine serves so we can build a
    Pathfinder. Returns None on failure (agent falls back to naive nav)."""
    try:
        async with httpx.AsyncClient(timeout=8) as h:
            r = await h.get(f"{server}/worlds/{world_name}.json")
            r.raise_for_status()
            return r.json()
    except Exception as e:
        LOG.warning("[pathfinder] couldn't fetch /worlds/%s.json: %s", world_name, e)
        return None


async def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True)
    p.add_argument("--token", required=True)
    p.add_argument("--name", default="Hieron")
    p.add_argument("--cadence-ms", type=int, default=600)
    p.add_argument("--world", default="dev_test",
                   help="world JSON name served at /worlds/<name>.json")
    p.add_argument("--persona", default="",
                   help="JSON persona: name, bio, voice, terminal_goals. "
                        "Defaults to a stock persona named --name.")
    p.add_argument("--bind", default="",
                   help="Optional: entity_id to bind to (e.g. npc_woodcutter). "
                        "Empty lets the engine pick.")
    args = p.parse_args()

    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(name)s %(message)s",
    )

    # Bootstrap the pathfinder from the world JSON. The static walkable
    # grid is the same across the agent's lifetime; only dynamic
    # obstacles (other entities) get refreshed per observation.
    world_json = await _fetch_world_json(args.server, args.world)
    pf = Pathfinder.from_world_json(world_json) if world_json else None
    if pf:
        LOG.info("[pathfinder] loaded %dx%d walkability grid", pf.width, pf.height)

    persona = {"name": args.name,
               "bio": "Hierarchical NPC — slow LLM brain, fast controller."}
    if args.persona:
        try:
            persona = {**persona, **json.loads(args.persona)}
        except json.JSONDecodeError as e:
            LOG.error("bad --persona JSON: %s", e)
            sys.exit(1)

    h_agent = HierarchicalAgent(pf=pf, persona=persona)

    register_kwargs = dict(
        server=args.server,
        user_token=args.token,
        persona=persona,
        vision_mode=VisionMode.STRUCTURED,
        cadence_ms=args.cadence_ms,
        brain=h_agent.controller_brain,
    )
    if args.bind:
        register_kwargs["bind_entity"] = args.bind
    sdk_agent = await register_and_connect(**register_kwargs)
    try:
        # Run the slow brain loop alongside the SDK's fast controller loop.
        await h_agent.brain_loop()
    finally:
        await sdk_agent.close()


if __name__ == "__main__":
    asyncio.run(main())
