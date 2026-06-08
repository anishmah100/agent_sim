"""Shared scaffolding for rule-based archetype bots.

Each archetype is a tiny FSM driving the SDK's observation→action
loop. The FSM lives in the bot process; the engine has no privileged
knowledge that this agent is rule-based.

The shape is deliberately spartan: an `ArchetypeBot` base class with a
single `decide(obs) -> action | None` extension point, a few helpers
shared across archetypes (random walk picker, nearest-of helpers, food
classifier, threat classifier, mental-note transition trace). Each
subclass owns its `state` field and edits it inside `decide`.
"""
from __future__ import annotations

import asyncio
import logging
import random
from dataclasses import dataclass, field
from typing import Optional

from agent_sim_sdk import (
    Action,
    ActionBatch,
    Agent,
    AgentCredentials,
    Step,
    Pickup,
    Observation,
    Pos,
    VisibleEntity,
    VisibleItem,
)
from agents.common.nav import NavGrid
from agents.common.motor import Goal, MotorController


log = logging.getLogger("agents.baselines")


FOOD_KINDS = {"apple", "loaf_bread", "bread_loaf", "cheese_wheel", "fish_cooked", "fish_raw", "berry"}

WEAPON_KINDS = {"dagger", "sword_short", "sword_long", "axe", "club_wood", "hammer", "bow", "crossbow"}

# Monetary item kinds. The engine auto-converts these to gold on
# pickup (inventory.go's coinValues table) so the bot doesn't even
# need a "consume coin" step — pickup IS the deposit.
MONEY_KINDS = {"coin_single", "coins_small_pile", "coin_pouch",
               "coins_large_pile", "coins_jumbo_pile",
               "gem_sapphire", "gem_emerald", "gem_ruby", "gem_diamond"}


def is_money(it) -> bool:
    return item_kind(it) in MONEY_KINDS


# ---------------------------------------------------------------------------
# Helpers that don't depend on FSM state.
# ---------------------------------------------------------------------------

def item_kind(it: VisibleItem) -> str:
    """Strip the ``"item:"`` prefix and ``"#suffix"`` tag from a sprite id."""
    s = it.sprite or ""
    if s.startswith("item:"):
        s = s[5:]
    if "#" in s:
        s = s.split("#", 1)[0]
    return s


def is_food(it: VisibleItem) -> bool:
    return item_kind(it) in FOOD_KINDS


def has_weapon_equipped(ent: VisibleEntity) -> bool:
    """Best-effort: extras_summary may carry ``equipped_slot`` or
    ``equipped_sprite`` after D9. Any non-empty equipped slot signals
    "armed" to the survivor's flee logic."""
    es = ent.extras_summary or {}
    if es.get("equipped_slot"):
        return True
    if es.get("equipped_sprite"):
        return True
    if es.get("equipped_weapon"):
        return True
    return False


def chebyshev(a: Pos, b: Pos) -> int:
    return max(abs(a[0] - b[0]), abs(a[1] - b[1]))


def nearest(items, anchor: Pos):
    """Returns the closest item by Chebyshev distance, or None."""
    closest = None
    best = 10**9
    for x in items:
        d = chebyshev(x.pos, anchor)
        if d < best:
            best, closest = d, x
    return closest


# NOTE: the old greedy `step_toward`/`step_away` helpers (single Chebyshev
# tile toward/away, no obstacle awareness — the ones that froze bots at
# walls) were removed in the movement redesign. All movement now goes
# through the motor/Goal layer (pursue/flee/goto → nav A* → N/S/E/W step).


# ---------------------------------------------------------------------------
# Base class.
# ---------------------------------------------------------------------------

@dataclass
class ArchetypeBot:
    """Base class: subclass and override `decide`. Override
    `archetype_name` so transition notes are tagged correctly."""

    creds: AgentCredentials
    archetype_name: str = "unknown"
    state: str = "IDLE"
    rng: random.Random = field(default_factory=random.Random)
    _stopped: bool = False
    _agent: Optional[Agent] = None
    _last_state: str = ""
    _last_action: Optional[Action] = None
    # Set from the first observation; lets an experiment runner map this
    # bot to its world entity without racing the /agents endpoint.
    entity_id: Optional[str] = None
    # Agent-side navigation: the static walkability grid, fetched once.
    engine_url: str = "http://127.0.0.1:8080"
    _nav: Optional[NavGrid] = None
    # Two-rate motor layer (see docs/AGENT_MOVEMENT_REDESIGN.md): the FSM
    # (deliberation) sets `goal` + fires direct verbs; the motor (reflex)
    # turns the standing goal into one N/S/E/W step per observation, with
    # last-seen memory so chases survive losing sight of the quarry. decide()
    # returns a direct verb (attack/pickup/speak/…) to act THIS tick, or None
    # to let the motor execute the current goal.
    goal: Goal = field(default_factory=Goal.idle)
    _motor: Optional[MotorController] = None

    def __post_init__(self) -> None:
        # Per-bot RNG seeded by entity id so different bots make
        # different random choices. Deterministic across reruns when
        # the agent_id is held fixed.
        self.rng = random.Random(hash((self.archetype_name, self.creds.agent_id)) & 0xFFFFFFFF)

    # ----- subclass extension point -----

    def decide(self, obs: Observation) -> Optional[Action]:  # pragma: no cover - abstract
        raise NotImplementedError

    def transition_note(self) -> Optional[tuple[str, dict[str, str]]]:
        """Override to attach a goal/plan to the mental note on state
        transition. Default: returns plain "entered <state>"."""
        return None

    # ----- navigation: the harness owns the MOTOR (reflex movement toward a
    #       standing Goal); subclasses own STRATEGY — they set self.goal
    #       (pursue/flee/goto) and the motor steps each tick. See
    #       agents/common/motor.py + docs/AGENT_MOVEMENT_REDESIGN.md. -----

    # ----- runtime -----

    async def run(self) -> None:
        async with Agent(self.creds) as agent:
            self._agent = agent
            # Drain initial observation.
            async for obs in agent.observations():
                if self._stopped:
                    return
                if self.entity_id is None:
                    self.entity_id = obs.self.entity_id
                if self._nav is None:
                    try:
                        self._nav = NavGrid.fetch(self.engine_url)
                        self._motor = MotorController(nav=self._nav)
                    except Exception:
                        log.exception("nav grid fetch failed; will retry next tick")
                # Reflex perception: refresh last-seen memory before deciding.
                if self._motor is not None:
                    self._motor.observe(obs)
                try:
                    act = self.decide(obs)
                except Exception:
                    log.exception("decide() raised; staying put this tick")
                    act = None
                # If the FSM didn't fire a direct verb, the motor executes the
                # standing goal (pursue/flee/goto) — the reflex movement loop.
                if act is None and self._motor is not None:
                    act = self._motor.next_step(self.goal, obs)
                # Emit a mental note on transition.
                if self.state != self._last_state:
                    extra = self.transition_note()
                    text = f"{self.archetype_name}: {self._last_state or '_'} -> {self.state}"
                    slots = {}
                    if extra is not None:
                        goal_extra, slot_extra = extra
                        text = f"{text} ({goal_extra})" if goal_extra else text
                        slots.update(slot_extra)
                    try:
                        await agent.note(text, tag="fsm", slots=slots or None)
                    except Exception:
                        # Mental notes are best-effort; never crash decide.
                        log.exception("mental_note send failed")
                    self._last_state = self.state
                if act is not None:
                    try:
                        await agent.act_batch(ActionBatch(actions=[act]))
                    except Exception:
                        log.exception("act_batch failed")

    def stop(self) -> None:
        """Cooperative stop; run() exits on the next obs."""
        self._stopped = True


# ---------------------------------------------------------------------------
# Default move helper for IDLE-style states.
# ---------------------------------------------------------------------------

def random_walk(bot: ArchetypeBot, here: Pos | None = None) -> Optional[Action]:
    """One random N/S/E/W step. The engine rejects a blocked step harmlessly;
    the next tick tries again. Bot's RNG → deterministic given the seed.

    NOTE: an earlier "home leash" that funneled idle agents toward the hub
    tile was REMOVED — it packed the whole rule-based crowd onto a few cells
    and every step got rejected for occupancy, freezing the town. Agents
    already cluster naturally by chasing the respawning hub items (positive
    attraction), which keeps them lively WITHOUT a forced funnel + gridlock."""
    return Step(dir=bot.rng.choice(["N", "S", "E", "W"]))


def forage_or_roam(bot: "ArchetypeBot", obs, here: Pos) -> Optional[Action]:
    """Default 'something useful to do' for an idle agent so nobody just
    stands around: go grab the nearest visible item (ANY kind — coins auto-
    convert to gold, food/tools fill the pack), picking it up when adjacent;
    if nothing is in view, take a roaming step toward fresh ground. Never
    returns None, so an idle agent is always either gathering or exploring.
    Sets bot.goal for the motor when it needs to walk to the item."""
    from agents.common.motor import Goal
    items = list(getattr(obs, "visible_items", []) or [])
    if items:
        t = nearest(items, here)
        if chebyshev(here, tuple(t.pos)) <= 1:
            return Pickup(target=t.entity_id)
        bot.goal = Goal.goto(*t.pos)
        return None  # motor walks to it
    return random_walk(bot, here)
