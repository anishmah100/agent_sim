"""Scavenger archetype (D16). Profit from death without combat.

Listens for `death_scream` audibles. On hearing one, races toward the
scream's tile (rounded to ±5 by the engine's anonymity policy), loots
whatever the corpse dropped, and retreats if an armed agent shows up.
Never attacks.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from agent_sim_sdk import Action, Observation, Pickup

from agents.common.motor import Goal

from ._common import (
    ArchetypeBot,
    has_weapon_equipped,
    is_money,
    item_kind,
    nearest,
    random_walk,
)


@dataclass
class Scavenger(ArchetypeBot):
    archetype_name: str = "scavenger"
    state: str = "IDLE"
    # Most recent scream tile. Reset when LOOTING completes or a fresher
    # scream supersedes.
    target_pos: Optional[tuple[int, int]] = None
    # Cadence throttle for IDLE moves: stay still ~3 of every 4 ticks.
    _idle_tick: int = 0

    def decide(self, obs: Observation) -> Optional[Action]:
        s = obs.self
        here = tuple(s.pos)

        # Update target from the freshest death scream in the audible
        # buffer. Engine anonymizes the position to a 5-tile cell so
        # `target_pos` is intentionally approximate.
        for ev in obs.audible:
            if (ev.sound_kind or "") in ("death_scream", "kill_witnessed"):
                self.target_pos = tuple(ev.from_pos)
                self.state = "RACING"

        # Threat scan applies once we're loitering at a corpse.
        threats = [e for e in obs.visible_entities if has_weapon_equipped(e)]

        if self.state == "RACING":
            if self.target_pos is None:
                self.state = "IDLE"
            else:
                d = max(
                    abs(self.target_pos[0] - here[0]),
                    abs(self.target_pos[1] - here[1]),
                )
                if d <= 2:
                    self.state = "LOOTING"
                else:
                    self.goal = Goal.goto(*self.target_pos)  # race to the scream
                    return None

        if self.state == "LOOTING":
            if threats:
                self.state = "RETREATING"
            else:
                items = list(obs.visible_items)
                if not items:
                    self.state = "IDLE"
                    self.target_pos = None
                else:
                    target = self._pick_loot_target(items)
                    if max(
                        abs(target.pos[0] - here[0]),
                        abs(target.pos[1] - here[1]),
                    ) <= 1:
                        return Pickup(target=target.entity_id)
                    self.goal = Goal.goto(*target.pos)  # walk to the loot
                    return None

        if self.state == "RETREATING":
            if not threats:
                self.state = "IDLE"
                self.target_pos = None
                self.goal = Goal.idle()
            else:
                t = nearest(threats, here)
                self.goal = Goal.flee(t.entity_id)  # motor steps away
                return None

        # IDLE: idle-fallback to grab visible coins/gems when nothing
        # else is going on. Money isn't the scavenger's primary goal
        # (they wait for deaths) — but when no scream has been heard
        # in a while, free coins on the ground are still worth a
        # detour. Killers/manipulators get the same fallback in their
        # own files.
        money_items = [it for it in obs.visible_items if is_money(it)]
        if money_items and not threats:
            target = nearest(money_items, here)
            if max(abs(target.pos[0] - here[0]),
                   abs(target.pos[1] - here[1])) <= 1:
                return Pickup(target=target.entity_id)
            self.goal = Goal.goto(*target.pos)  # motor navigates to the coin
            return None
        self._idle_tick = (self._idle_tick + 1) % 4
        if self._idle_tick == 0:
            return random_walk(self, here)
        return None

    @staticmethod
    def _pick_loot_target(items):
        """Priority: gold piles > weapons > anything else. The sprite
        carries the kind (item:coin_pouch, item:sword_short, ...)."""
        def score(it):
            k = item_kind(it)
            if "coin" in k or "gold" in k:
                return 3
            if k in ("dagger", "sword_short", "sword_long", "axe", "hammer", "club_wood", "bow", "crossbow"):
                return 2
            return 1
        return max(items, key=score)

    def transition_note(self):
        slots = {"goal": "loot dropped items", "plan": f"state={self.state}"}
        if self.state == "RACING" and self.target_pos:
            slots["beliefs"] = f"death near {self.target_pos}"
        return None, slots
