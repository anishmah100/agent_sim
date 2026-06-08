"""Killer archetype (D16). Predatory — forces focal LLM agents to
think about safety + alliances.

Picks a target each HUNTING tick using a small scoring function
(unarmed + wounded + close-by all push score up). Pursues, attacks
when in range, loots the corpse, and retreats if HP drops low.

Movement model (see docs/AGENT_MOVEMENT_REDESIGN.md): the FSM here is the
DELIBERATION layer — it sets a standing ``self.goal`` (pursue/flee/goto)
and fires direct verbs (Attack/Pickup/Equip). The harness MOTOR layer turns
the goal into one N/S/E/W step per tick, with last-seen memory so a chase
survives losing sight of the quarry. decide() returns a direct verb to act
this tick, or ``None`` to let the motor advance the goal.
"""
from __future__ import annotations

from dataclasses import dataclass
from typing import Optional

from agent_sim_sdk import Action, Attack, Equip, Observation, Pickup

from agents.common.motor import Goal

from ._common import (
    ArchetypeBot,
    WEAPON_KINDS,
    chebyshev,
    has_weapon_equipped,
    is_money,
    item_kind,
    nearest,
)


@dataclass
class Killer(ArchetypeBot):
    archetype_name: str = "killer"
    state: str = "HUNTING"
    target_id: Optional[str] = None

    def decide(self, obs: Observation) -> Optional[Action]:
        s = obs.self
        here = tuple(s.pos)
        hp = int((s.extras or {}).get("hp", 100) or 100)

        # Low HP overrides everything except already-LOOTING. Retreat only
        # when badly hurt and re-engage sooner, so the killer spends most of
        # its time actually hunting (a high retreat threshold + slow regen
        # left killers loitering out of the fight for long stretches, which
        # is a big part of why combat went quiet mid-run).
        if hp < 18 and self.state != "LOOTING":
            self.state = "RETREATING"

        if self.state == "RETREATING":
            threats = [e for e in obs.visible_entities if e.entity_id != s.entity_id]
            if not threats and hp > 45:
                self.state = "HUNTING"
                self.target_id = None
                self.goal = Goal.idle()
            elif threats:
                t = nearest(threats, here)
                self.goal = Goal.flee(t.entity_id)
            return None  # motor flees / idles

        if self.state == "LOOTING":
            items = list(obs.visible_items)
            if not items:
                self.state = "HUNTING"
                self.target_id = None
                self.goal = Goal.idle()
            else:
                target = self._pick_loot_target(items)
                if chebyshev(here, tuple(target.pos)) <= 1:
                    return Pickup(target=target.entity_id)
                self.goal = Goal.goto(*target.pos)
                return None

        if self.state == "HUNTING":
            # If unarmed and there's a visible weapon, grab it first —
            # unarmed kills take 25+ uninterrupted hits, but a sword
            # short halves that. Equip before pursuing prey.
            inv = list((s.extras or {}).get("inventory") or [])
            unarmed = not ((s.extras or {}).get("equipped", {}) or {}).get("weapon")
            if unarmed:
                weap_in_inv = next(
                    (i for i in inv if isinstance(i, str)
                     and any(k in i for k in WEAPON_KINDS)),
                    None,
                )
                if weap_in_inv:
                    return Equip(item=weap_in_inv, slot="weapon")
                # Grab a weapon ONLY if it's right here — never detour across
                # the map for one (that read as idle wandering). Unarmed
                # hunting is fine; a found weapon is a bonus.
                weap_items = [
                    it for it in obs.visible_items
                    if item_kind(it) in WEAPON_KINDS
                    and chebyshev(here, tuple(it.pos)) <= 1
                ]
                if weap_items:
                    return Pickup(target=weap_items[0].entity_id)
            choice = self._pick_target(obs.visible_entities, here)
            if choice is not None:
                self.target_id = choice.entity_id
                self.goal = Goal.pursue(choice.entity_id)
                self.state = "PURSUING"
            else:
                # No prey: opportunistic gold grab so the killer doesn't
                # visibly ignore loose coins. Lowest priority.
                money_items = [it for it in obs.visible_items if is_money(it)]
                if money_items:
                    target = nearest(money_items, here)
                    if chebyshev(here, tuple(target.pos)) <= 1:
                        return Pickup(target=target.entity_id)
                    self.goal = Goal.goto(*target.pos)
                    return None
                # Nothing to do — wander so we stumble onto prey. A fresh
                # random goto each idle tick keeps us roaming, not frozen.
                self.goal = Goal.idle()
                from ._common import random_walk
                return random_walk(self, here)

        if self.state == "PURSUING":
            # Keep the pursue goal current; the motor uses last-seen memory
            # if the target slips out of view. If the motor reports the
            # target is truly lost (memory expired), it returns no step and
            # we fall back to re-hunting.
            self.goal = Goal.pursue(self.target_id) if self.target_id else Goal.idle()
            target = self._find_target(obs.visible_entities)
            if target is not None and chebyshev(here, tuple(target.pos)) <= 1:
                self.state = "ATTACKING"
                return Attack(target=target.entity_id)
            # Target not visible AND no usable memory → give up, re-hunt.
            if target is None and self._motor is not None \
                    and self._motor.target_pos(self.target_id, obs) is None:
                self.state = "HUNTING"
                self.target_id = None
                self.goal = Goal.idle()
            return None  # motor pursues (live pos or last-seen)

        if self.state == "ATTACKING":
            target = self._find_target(obs.visible_entities)
            if target is None:
                # Target gone — presumed dead. Grab any loot it dropped.
                self.target_id = None
                self.state = "LOOTING" if obs.visible_items else "HUNTING"
                self.goal = Goal.idle()
                return None
            if chebyshev(here, tuple(target.pos)) > 1:
                self.state = "PURSUING"
                self.goal = Goal.pursue(target.entity_id)
                return None  # motor closes the gap
            return Attack(target=target.entity_id)

        return None

    # ----- target picking -----

    @staticmethod
    def _pick_target(entities, here):
        best, best_score = None, 3.0  # threshold
        for e in entities:
            if e.archetype not in ("trainer", "wanderer", "survivor", "killer", "manipulator", "scavenger"):
                continue
            score = 0.0
            if not has_weapon_equipped(e):
                score += 5.0
            else:
                score += 2.0
            bucket = (e.extras_summary or {}).get("hp_bucket")
            if bucket in ("wounded", "dying"):
                score += 3.0
            d = chebyshev(here, tuple(e.pos))
            if d <= 1:
                score += 2.0
            score -= 0.5 * d
            if score > best_score:
                best_score, best = score, e
        return best

    def _find_target(self, entities):
        if self.target_id is None:
            return None
        for e in entities:
            if e.entity_id == self.target_id:
                return e
        return None

    @staticmethod
    def _pick_loot_target(items):
        def score(it):
            k = item_kind(it)
            if "coin" in k or "gold" in k:
                return 4
            if k in ("dagger", "sword_short", "sword_long", "axe", "hammer", "club_wood", "bow", "crossbow"):
                return 3
            if k in ("apple", "loaf_bread", "bread_loaf", "cheese_wheel", "fish_cooked", "fish_raw"):
                return 2
            return 1
        return max(items, key=score)

    def transition_note(self):
        slots = {"goal": "kill and loot", "plan": f"state={self.state} {self.goal}"}
        if self.target_id:
            slots["beliefs"] = f"target={self.target_id}"
        return None, slots
