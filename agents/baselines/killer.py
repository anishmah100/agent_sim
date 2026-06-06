"""Killer archetype (D16). Predatory — forces focal LLM agents to
think about safety + alliances.

Picks a target each HUNTING tick using a small scoring function
(unarmed + wounded + close-by all push score up). Pursues, attacks
when in range, loots the corpse, and retreats if HP drops low.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from agent_sim_sdk import Action, Attack, Move, Observation, Pickup

from ._common import (
    ArchetypeBot,
    chebyshev,
    has_weapon_equipped,
    item_kind,
    nearest,
    random_walk,
    step_away,
    step_toward,
)


# How many ticks the target may stay off-screen before we give up.
LOSE_TARGET_TICKS = 3


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
                return Move(target=list(step_toward(here, tuple(target.pos))))

        if self.state == "HUNTING":
            choice = self._pick_target(obs.visible_entities, here)
            if choice is not None:
                self.target_id = choice.entity_id
                self.target_last_pos = tuple(choice.pos)
                self.lost_ticks = 0
                self.state = "PURSUING"
            else:
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
                    return Move(target=list(step_toward(here, self.target_last_pos)))
                return random_walk(self, here)
            self.target_last_pos = tuple(target.pos)
            self.lost_ticks = 0
            if chebyshev(here, tuple(target.pos)) <= 1:
                self.state = "ATTACKING"
            else:
                return Move(target=list(step_toward(here, tuple(target.pos))))

        if self.state == "ATTACKING":
            target = self._find_target(obs.visible_entities)
            if target is None:
                self.state = "HUNTING"
                self.target_id = None
                return None
            # Did the target die? Detect via missing entity OR
            # extras_summary's hp_bucket=="dying" then absence.
            if chebyshev(here, tuple(target.pos)) > 1:
                self.state = "PURSUING"
                return Move(target=list(step_toward(here, tuple(target.pos))))
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
