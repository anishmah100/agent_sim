"""Motor layer — the harness's reflex/perception primitive.

This is the "body + senses" half of the bright line from
docs/AGENT_MOVEMENT_REDESIGN.md: the harness owns MOTION and PERCEPTION,
the mind (LLM, or a rule-based FSM) owns STRATEGY. The mind sets a single
standing **goal**; the motor turns that goal into one N/S/E/W step per
observation by running A* on the known terrain — re-planning every tick so
moving targets and transient blockers are handled for free.

Goals:
  - ``Goal.goto(x, y)``        walk onto a fixed tile
  - ``Goal.pursue(entity_id)`` close in on (and stay adjacent to) a creature
  - ``Goal.flee(entity_id)``   maximise distance from a creature
  - ``Goal.idle()``            do nothing (motor returns no step)

Last-seen tracking: every observation updates a memory of where each named
entity was last seen. Pursuit/flee fall back to that remembered tile when
the target steps out of vision, so a chase survives brief line-of-sight
breaks and the agent keeps heading to where the quarry was — exactly what a
human controlling the avatar would do — instead of instantly forgetting.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Literal, Optional

from agent_sim_sdk import Action, Observation, Pos, Step
from agents.common.nav import NavGrid

GoalKind = Literal["idle", "goto", "pursue", "flee"]


@dataclass
class Goal:
    kind: GoalKind = "idle"
    # For goto: the destination tile. For pursue/flee: the target entity.
    pos: Optional[Pos] = None
    entity_id: Optional[str] = None

    @staticmethod
    def goto(x: int, y: int) -> "Goal":
        return Goal(kind="goto", pos=(x, y))

    @staticmethod
    def pursue(entity_id: str) -> "Goal":
        return Goal(kind="pursue", entity_id=entity_id)

    @staticmethod
    def flee(entity_id: str) -> "Goal":
        return Goal(kind="flee", entity_id=entity_id)

    @staticmethod
    def idle() -> "Goal":
        return Goal(kind="idle")

    def __str__(self) -> str:
        if self.kind == "goto":
            return f"goto{self.pos}"
        if self.kind in ("pursue", "flee"):
            return f"{self.kind}({self.entity_id})"
        return "idle"


@dataclass
class _Seen:
    pos: Pos
    tick: int


@dataclass
class MotorController:
    """Holds the nav grid + last-seen memory and converts a Goal into a step.

    One per agent. Call ``observe(obs)`` every observation to refresh memory,
    then ``next_step(goal, obs)`` to get the reflex action.
    """

    nav: NavGrid
    # How long (in ticks) a last-seen position stays usable before we treat
    # the target as truly lost. 60Hz engine → 1800 ticks ≈ 30s of memory.
    memory_ticks: int = 1800
    last_seen: dict[str, _Seen] = field(default_factory=dict)

    # ----- perception -----

    def observe(self, obs: Observation) -> None:
        """Refresh last-seen memory from the current observation."""
        tick = obs.world_tick
        for e in obs.visible_entities or []:
            self.last_seen[e.entity_id] = _Seen(tuple(e.pos), tick)

    def target_pos(self, entity_id: str, obs: Observation) -> Optional[Pos]:
        """Best estimate of an entity's tile: its live position if visible,
        else the remembered last-seen tile (until it goes stale)."""
        for e in obs.visible_entities or []:
            if e.entity_id == entity_id:
                return tuple(e.pos)
        seen = self.last_seen.get(entity_id)
        if seen is None:
            return None
        if obs.world_tick - seen.tick > self.memory_ticks:
            return None
        return seen.pos

    def is_visible(self, entity_id: str, obs: Observation) -> bool:
        return any(e.entity_id == entity_id for e in (obs.visible_entities or []))

    # ----- motor -----

    def _blockers(self, obs: Observation, exclude: Optional[str] = None) -> list[Pos]:
        me = obs.self.entity_id
        return [tuple(e.pos) for e in (obs.visible_entities or [])
                if e.entity_id != me and e.entity_id != exclude]

    def next_step(self, goal: Goal, obs: Observation) -> Optional[Action]:
        """One reflex step toward satisfying `goal`, or None if nothing to do
        (already there / target lost / no route)."""
        here: Pos = tuple(obs.self.pos)

        if goal.kind == "goto" and goal.pos is not None:
            d = self.nav.next_dir(here, tuple(goal.pos),
                                  dynamic_blocked=self._blockers(obs),
                                  stop_adjacent=False)
            return Step(dir=d) if d else None

        if goal.kind == "pursue" and goal.entity_id is not None:
            tp = self.target_pos(goal.entity_id, obs)
            if tp is None:
                return None  # target lost — mind decides what's next
            # Don't treat the target's own tile as a blocker (we want to
            # route up to it); stop_adjacent so we end one tile away, in
            # attack reach, rather than fighting for its cell.
            d = self.nav.next_dir(here, tp,
                                  dynamic_blocked=self._blockers(obs, exclude=goal.entity_id),
                                  stop_adjacent=True)
            return Step(dir=d) if d else None

        if goal.kind == "flee" and goal.entity_id is not None:
            tp = self.target_pos(goal.entity_id, obs)
            if tp is None:
                return None
            return self._flee_step(here, tp)

        return None

    def _flee_step(self, here: Pos, threat: Pos) -> Optional[Action]:
        """The one walkable N/S/E/W step that most increases Chebyshev
        distance from the threat. Terrain eventually corners a fleer — which
        is what lets a determined pursuer win."""
        best_dir, best_dist = None, -1
        for d, (dx, dy) in (("N", (0, -1)), ("S", (0, 1)),
                            ("E", (1, 0)), ("W", (-1, 0))):
            nx, ny = here[0] + dx, here[1] + dy
            if not self.nav.walkable(nx, ny):
                continue
            dist = max(abs(nx - threat[0]), abs(ny - threat[1]))
            if dist > best_dist:
                best_dist, best_dir = dist, d
        return Step(dir=best_dir) if best_dir else None
