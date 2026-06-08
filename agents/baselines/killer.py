"""Killer archetype (D16). Predatory — forces focal LLM agents to
think about safety + alliances.

Picks a target each HUNTING tick using a small scoring function
(unarmed + wounded + close-by all push score up). Pursues, attacks
when in range, loots the corpse, and retreats if HP drops low.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from agent_sim_sdk import Action, Attack, Equip, Move, Observation, Pickup

from ._common import (
    ArchetypeBot,
    WEAPON_KINDS,
    chebyshev,
    has_weapon_equipped,
    is_money,
    item_kind,
    nearest,
    random_walk,
    step_away,
    step_toward,
)


# How many ticks the target may stay off-screen before we give up.
# Generous so the killer commits to a chase instead of constantly
# re-picking and orbiting (which read as idle wandering).
LOSE_TARGET_TICKS = 12


@dataclass
class Killer(ArchetypeBot):
    archetype_name: str = "killer"
    state: str = "HUNTING"
    target_id: Optional[str] = None
    target_last_pos: Optional[tuple[int, int]] = None
    lost_ticks: int = 0

    def decide(self, obs: Observation) -> Optional[Action]:
        s = obs.self
        here = tuple(s.pos)
        hp = int((s.extras or {}).get("hp", 100) or 100)

        # Low HP overrides everything except already-RETREATING.
        if hp < 30 and self.state != "LOOTING":
            self.state = "RETREATING"

        if self.state == "RETREATING":
            threats = [e for e in obs.visible_entities if e.entity_id != s.entity_id]
            if not threats and hp > 70:
                self.state = "HUNTING"
                self.target_id = None
            elif threats:
                t = nearest(threats, here)
                return Move(target=list(step_away(here, tuple(t.pos))))
            return random_walk(self, here)

        if self.state == "LOOTING":
            items = list(obs.visible_items)
            if not items:
                self.state = "HUNTING"
                self.target_id = None
                self.target_last_pos = None
            else:
                target = self._pick_loot_target(items)
                if max(
                    abs(target.pos[0] - here[0]),
                    abs(target.pos[1] - here[1]),
                ) <= 1:
                    return Pickup(target=target.entity_id)
                return Move(target=list(target.pos))

        if self.state == "HUNTING":
            # If unarmed and there's a visible weapon, grab it first —
            # unarmed kills take 25+ uninterrupted hits, but a sword
            # short halves that. The killer prioritizes equipping
            # before pursuing prey.
            inv = list((s.extras or {}).get("inventory") or [])
            unarmed = not (
                (s.extras or {}).get("equipped", {}) or {}
            ).get("weapon")
            if unarmed:
                # Inventory weapon already? Equip it.
                weap_in_inv = next(
                    (i for i in inv if isinstance(i, str)
                     and any(k in i for k in WEAPON_KINDS)),
                    None,
                )
                if weap_in_inv:
                    return Equip(item=weap_in_inv, slot="weapon")
                # Grab a weapon ONLY if it's right here — never detour across
                # the map for one. Detouring made killers wander after distant
                # weapons forever and never actually hunt (they read as idle).
                # Unarmed hunting is fine; a found weapon is a bonus.
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
                self.target_last_pos = tuple(choice.pos)
                self.lost_ticks = 0
                self.state = "PURSUING"
            else:
                # No target + no weapon need: opportunistic gold grab
                # so the killer doesn't visibly ignore loose coins.
                # Money is the LOWEST priority — only fires when
                # nothing more interesting is around.
                money_items = [it for it in obs.visible_items if is_money(it)]
                if money_items:
                    target = nearest(money_items, here)
                    if chebyshev(here, tuple(target.pos)) <= 1:
                        return Pickup(target=target.entity_id)
                    return Move(target=list(target.pos))
                return random_walk(self, here)

        if self.state == "PURSUING":
            target = self._find_target(obs.visible_entities)
            if target is None:
                self.lost_ticks += 1
                if self.lost_ticks >= LOSE_TARGET_TICKS:
                    self.state = "HUNTING"
                    self.target_id = None
                    self.target_last_pos = None
                    return random_walk(self, here)
                # Walk toward the last known position.
                if self.target_last_pos is not None:
                    return Move(target=list(self.target_last_pos))
                return random_walk(self, here)
            self.target_last_pos = tuple(target.pos)
            self.lost_ticks = 0
            if chebyshev(here, tuple(target.pos)) <= 1:
                self.state = "ATTACKING"
            else:
                return Move(target=list(target.pos))

        if self.state == "ATTACKING":
            target = self._find_target(obs.visible_entities)
            if target is None:
                # MAJ-9: target gone — presumed dead. If it dropped loot
                # nearby, switch to LOOTING to grab it (the kill→loot beat
                # the docstring promised). The LOOTING handler walks to and
                # picks up the nearest visible item, then returns to HUNTING.
                self.target_id = None
                if obs.visible_items:
                    self.state = "LOOTING"
                else:
                    self.state = "HUNTING"
                return None
            # Did the target die? Detect via missing entity OR
            # extras_summary's hp_bucket=="dying" then absence.
            if chebyshev(here, tuple(target.pos)) > 1:
                self.state = "PURSUING"
                return Move(target=list(target.pos))
            return Attack(target=target.entity_id)

        return None

    # ----- target picking -----

    @staticmethod
    def _pick_target(entities, here):
        best, best_score = None, 3.0  # threshold
        for e in entities:
            if e.archetype not in ("trainer", "wanderer", "survivor", "killer", "manipulator", "scavenger"):
                # Skip non-agent entities (decorations etc).
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
        slots = {"goal": "kill and loot", "plan": f"state={self.state}"}
        if self.target_id:
            slots["beliefs"] = f"target={self.target_id}"
        return None, slots
