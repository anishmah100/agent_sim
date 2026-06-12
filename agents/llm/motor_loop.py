"""Two-rate execution for LLM focal agents (slice 6).

The bright line from docs/AGENT_MOVEMENT_REDESIGN.md: the harness owns the
MOTOR + PERCEPTION; the LLM owns STRATEGY. This module is the harness side.

Why two rates: an LLM deliberation takes seconds, but movement must happen
every tick or pursuit/fleeing is hopeless. So:

  - FAST reflex (every observation): execute the current standing Goal with
    one N/S/E/W step via the motor (nav A* on known terrain + last-seen
    memory). No LLM involved.
  - SLOW deliberation (every ``deliberate_every_ticks`` of world time, or on
    a salient change): call the LLM IN THE BACKGROUND while the reflex keeps
    moving the body; when it returns, its movement verbs (pursue/flee/goto)
    update the standing Goal and its direct verbs (attack/speak/pickup/…) are
    executed.

`nav_mode`:
  - "harness" (default): movement is a Goal; the motor navigates. This is what
    makes a small local model (Qwen) viable — it never has to path-find.
  - "llm": the model navigates itself by emitting `step` {dir} actions
    (treated as direct verbs); the motor is bypassed. The map-to-LLM arm of
    the experiment — expected to work for a strong model (Claude) only.
"""
from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass, field
from typing import Any, Callable, Optional

from agent_sim_sdk import Action
from agents.common.motor import Goal, MotorController
from agents.common.nav import NavGrid

from .actions import to_action, to_goal

log = logging.getLogger("agents.llm.motor_loop")


@dataclass
class MotorLoop:
    """Per-agent motor + deliberation scheduler. Compose one into a harness
    and drive it from the observation loop."""

    engine_url: str = "http://127.0.0.1:8080"
    # World-tick spacing between LLM deliberations. 120 ticks ≈ 2s at 60Hz.
    # This (NOT the observation cadence) governs LLM cost — a faster obs
    # cadence only makes the reflex movement smoother, not more expensive.
    deliberate_every_ticks: int = 120
    # Minimum tick gap before a salient change (new agent / heard violence)
    # is allowed to trigger an early deliberation — keeps a churny scene from
    # calling the LLM every tick. 30 ticks ≈ 0.5s at 60Hz.
    min_gap_ticks: int = 30
    nav_mode: str = "harness"  # "harness" | "llm"

    goal: Goal = field(default_factory=Goal.idle)
    motor: Optional[MotorController] = None
    _last_deliberate_tick: int = -10 ** 9
    _task: Optional["asyncio.Task[dict]"] = None
    _prev_visible: set = field(default_factory=set)
    _seen_violent: set = field(default_factory=set)  # event_ids of violent audible events already deliberated on

    # ----- setup / perception -----

    def ensure_motor(self) -> bool:
        """Fetch the nav grid + build the motor (once). Returns True if ready."""
        if self.motor is not None:
            return True
        try:
            self.motor = MotorController(nav=NavGrid.fetch(self.engine_url))
            return True
        except Exception:
            log.exception("nav grid fetch failed; will retry next obs")
            return False

    def observe(self, obs: Any) -> None:
        if self.motor is not None:
            self.motor.observe(obs)

    # ----- deliberation scheduling -----

    def should_deliberate(self, obs: Any) -> bool:
        """True when it's time to (re)think: cadence elapsed or a salient
        change (a new agent came into view, or we heard violence). Never
        while a deliberation is already in flight."""
        if self._task is not None:
            return False
        dt = obs.world_tick - self._last_deliberate_tick
        if dt >= self.deliberate_every_ticks:
            return True
        if dt < self.min_gap_ticks:
            return False
        vis = {e.entity_id for e in (obs.visible_entities or [])}
        if vis - self._prev_visible:
            return True
        # Only a NEW violent event is salient. Without this de-dup (mirroring
        # the visible-entity set difference above), the same death_scream re-
        # triggered an LLM deliberation every observation for its whole ~240-
        # tick lifetime in the audible ring — pure token/cost waste (audit).
        for ev in (obs.audible or []):
            if (getattr(ev, "sound_kind", "") or ev.kind) in ("death_scream", "kill_witnessed"):
                eid = getattr(ev, "event_id", None)
                if eid is None or eid not in self._seen_violent:
                    return True
        return False

    def start_deliberation(self, prompt: str, call_llm: Callable[[str], dict]) -> None:
        """Launch the (sync) LLM call in a background thread/task."""
        self._task = asyncio.create_task(asyncio.to_thread(call_llm, prompt))

    def take_decision(self, obs: Any) -> Optional[dict]:
        """If the background LLM call has finished, return its decision dict
        (and record the deliberation tick). Else None."""
        if self._task is None or not self._task.done():
            return None
        task, self._task = self._task, None
        self._last_deliberate_tick = obs.world_tick
        self._prev_visible = {e.entity_id for e in (obs.visible_entities or [])}
        # Snapshot currently-audible violent events as "seen" so they don't
        # re-trigger until a genuinely new one arrives.
        self._seen_violent = {
            getattr(ev, "event_id", None)
            for ev in (obs.audible or [])
            if (getattr(ev, "sound_kind", "") or ev.kind) in ("death_scream", "kill_witnessed")
        }
        try:
            return task.result()
        except Exception as e:
            log.warning("deliberation task failed: %s", e)
            return {}

    # ----- decision → goal + direct actions -----

    def apply_movement(self, action_dicts: list[dict]) -> list[dict]:
        """Update the standing goal from any movement verb (pursue/flee/goto;
        the LAST one wins). In harness nav_mode `step` is left as a direct
        action only if the model is explicitly self-navigating. Returns the
        list of NON-movement (direct) action dicts to execute now."""
        direct: list[dict] = []
        for d in action_dicts:
            g = to_goal(d)
            if g is not None:
                self.goal = g
            else:
                direct.append(d)
        return direct

    def reflex_step(self, obs: Any) -> Optional[Action]:
        """One motor step toward the current goal (harness nav_mode only)."""
        if self.motor is None or self.nav_mode != "harness":
            return None
        return self.motor.next_step(self.goal, obs)

    def target_pos(self, entity_id: str, obs: Any):
        if self.motor is None:
            return None
        return self.motor.target_pos(entity_id, obs)
