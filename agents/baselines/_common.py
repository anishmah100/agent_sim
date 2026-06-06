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
    Move,
    Observation,
    Pos,
    VisibleEntity,
    VisibleItem,
)


log = logging.getLogger("agents.baselines")


FOOD_KINDS = {"apple", "loaf_bread", "bread_loaf", "cheese_wheel", "fish_cooked", "fish_raw", "berry"}

WEAPON_KINDS = {"dagger", "sword_short", "sword_long", "axe", "club_wood", "hammer", "bow", "crossbow"}

# Monetary item kinds. The engine auto-converts these to gold on
# pickup (inventory.go's coinValues table) so the bot doesn't even
# need a "consume coin" step — pickup IS the deposit.
MONEY_KINDS = {"coin_single", "coins_small_pile", "coin_pouch",
               "gem_emerald", "gem_ruby", "gem_diamond"}


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


def step_toward(here: Pos, there: Pos) -> Pos:
    """One-tile step from `here` toward `there` along the Chebyshev path.
    Returns the same tile if already there."""
    dx = (1 if there[0] > here[0] else -1 if there[0] < here[0] else 0)
    dy = (1 if there[1] > here[1] else -1 if there[1] < here[1] else 0)
    return (here[0] + dx, here[1] + dy)


def step_away(here: Pos, threat: Pos) -> Pos:
    """One-tile step from `here` away from `threat`."""
    dx = (1 if here[0] >= threat[0] else -1)
    dy = (1 if here[1] >= threat[1] else -1)
    return (here[0] + dx, here[1] + dy)


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

    # ----- runtime -----

    async def run(self) -> None:
        async with Agent(self.creds) as agent:
            self._agent = agent
            # Drain initial observation.
            async for obs in agent.observations():
                if self._stopped:
                    return
                try:
                    act = self.decide(obs)
                except Exception:
                    log.exception("decide() raised; staying put this tick")
                    act = None
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

def random_walk(bot: ArchetypeBot, here: Pos) -> Optional[Action]:
    """Pick a random 8-direction neighbor and move there. Bot's RNG
    so trajectory is deterministic given the seed."""
    dx, dy = bot.rng.choice([(1, 0), (-1, 0), (0, 1), (0, -1), (1, 1), (-1, -1), (1, -1), (-1, 1)])
    return Move(target=[here[0] + dx, here[1] + dy])
