"""Avenger archetype (slice 7). The "gang up" half of the deranged-killer
set-piece.

Normally forages/roams like any townsfolk. But the instant it WITNESSES a
kill — the engine delivers a targeted ``kill_witnessed`` audible to every
agent with line-of-sight to the killing tile, carrying the killer's id —
it holds a grudge: arm up, hunt the named killer, and attack on contact.
Because every witness to the same kill names the same killer, a cluster of
avengers converges on the murderer at once: the mob forms emergently from
shared perception, not from any central coordinator.

The grudge decays: if the killer slips out of sight for a while the avenger
cools off and returns to ordinary life, so the town doesn't stay locked in
a permanent manhunt.

Movement follows the standard two-rate model (docs/AGENT_MOVEMENT_REDESIGN.md):
decide() sets a standing ``self.goal`` (pursue/flee) + fires direct verbs
(Attack/Equip/Pickup); the harness motor turns the goal into one step/tick.
"""
from __future__ import annotations

import json
from dataclasses import dataclass
from typing import Optional

from agent_sim_sdk import Action, Attack, Equip, Observation, Pickup

from agents.common.motor import Goal

from ._common import (
    ArchetypeBot,
    WEAPON_KINDS,
    chebyshev,
    forage_or_roam,
    is_infamous,
    is_money,
    item_kind,
    nearest,
    random_walk,
)

# How long (ticks) a grudge survives without seeing the killer before the
# avenger gives up and goes back to ordinary life. ~Several seconds at the
# rule cadence — long enough to chase across a plaza, short enough that the
# town isn't a permanent lynch mob.
GRUDGE_TTL = 80


@dataclass
class Avenger(ArchetypeBot):
    archetype_name: str = "avenger"
    state: str = "IDLE"
    grudge_target: Optional[str] = None
    grudge_pos: Optional[tuple[int, int]] = None
    _grudge_ticks: int = 0

    def decide(self, obs: Observation) -> Optional[Action]:
        s = obs.self
        here = tuple(s.pos)
        hp = int((s.extras or {}).get("hp", 100) or 100)

        # 1) Did I just witness a murder? The engine hands witnesses a
        #    `kill_witnessed` sound whose text names killer + victim.
        for ev in obs.audible:
            if (ev.sound_kind or "") == "kill_witnessed" and ev.text:
                try:
                    payload = json.loads(ev.text)
                except (ValueError, TypeError):
                    continue
                killer = payload.get("killer")
                if killer and killer != s.entity_id:
                    self.grudge_target = killer
                    self.grudge_pos = tuple(ev.from_pos)
                    self._grudge_ticks = GRUDGE_TTL
                    self.state = "AVENGING"

        # 2) Badly hurt: break off and flee whoever's nearest. A dead
        #    avenger avenges nobody.
        if hp < 20:
            self.state = "RETREATING"
            threats = [e for e in obs.visible_entities if e.entity_id != s.entity_id]
            if threats:
                self.goal = Goal.flee(nearest(threats, here).entity_id)
                return None
            self.goal = Goal.idle()
            return random_walk(self, here)

        # 3) Grudge held: arm up, then hunt the named killer.
        if self.state == "AVENGING" and self.grudge_target is not None:
            self._grudge_ticks -= 1
            if self._grudge_ticks <= 0:
                # Cooled off — back to ordinary life.
                self.grudge_target = None
                self.grudge_pos = None
                self.state = "IDLE"
            else:
                # Equip a carried weapon / grab one underfoot so the mob
                # actually has teeth.
                armed = self._arm_up(s, obs, here)
                if armed is not None:
                    return armed
                # Killer in view? Close and strike.
                tgt = next((e for e in obs.visible_entities
                            if e.entity_id == self.grudge_target), None)
                if tgt is not None:
                    self.grudge_pos = tuple(tgt.pos)
                    if chebyshev(here, tuple(tgt.pos)) <= 1:
                        return Attack(target=self.grudge_target)
                    self.goal = Goal.pursue(self.grudge_target)
                    return None
                # Lost sight: march on the last place I saw the killing.
                if self.grudge_pos is not None:
                    self.goal = Goal.goto(*self.grudge_pos)
                    return None

        # 3b) No grudge from a witnessed kill, but a known-infamous agent
        #     (notorious killer, by reputation) is in sight — move against it
        #     anyway. This is the gossip/reputation channel: an avenger acts
        #     on an agent's standing even without personally seeing the deed.
        if self.state != "AVENGING":
            infamous = [e for e in obs.visible_entities
                        if e.entity_id != s.entity_id and is_infamous(e)]
            if infamous:
                tgt = nearest(infamous, here)
                self.grudge_target = tgt.entity_id
                self.grudge_pos = tuple(tgt.pos)
                self._grudge_ticks = GRUDGE_TTL
                self.state = "AVENGING"
                armed = self._arm_up(s, obs, here)
                if armed is not None:
                    return armed
                if chebyshev(here, tuple(tgt.pos)) <= 1:
                    return Attack(target=self.grudge_target)
                self.goal = Goal.pursue(self.grudge_target)
                return None

        # 4) Nothing to avenge: behave like ordinary townsfolk — grab loose
        #    gold, forage, or roam. Never just stand.
        money_items = [it for it in obs.visible_items if is_money(it)]
        if money_items:
            t = nearest(money_items, here)
            if chebyshev(here, tuple(t.pos)) <= 1:
                return Pickup(target=t.entity_id)
            self.goal = Goal.goto(*t.pos)
            return None
        return forage_or_roam(self, obs, here)

    def _arm_up(self, s, obs, here) -> Optional[Action]:
        """Equip a carried weapon, or pick up one within reach. Returns the
        action to take this tick, or None if already armed / nothing handy."""
        equipped = ((s.extras or {}).get("equipped", {}) or {}).get("weapon")
        if equipped:
            return None
        inv = list((s.extras or {}).get("inventory") or [])
        win = next((i for i in inv if isinstance(i, str)
                    and any(k in i for k in WEAPON_KINDS)), None)
        if win:
            return Equip(item=win, slot="weapon")
        whot = [it for it in obs.visible_items if item_kind(it) in WEAPON_KINDS
                and chebyshev(here, tuple(it.pos)) <= 1]
        if whot:
            return Pickup(target=whot[0].entity_id)
        return None

    def transition_note(self):
        slots = {"goal": "avenge the slain", "plan": f"state={self.state}"}
        if self.grudge_target:
            slots["beliefs"] = f"hunting killer {self.grudge_target}"
        return None, slots
