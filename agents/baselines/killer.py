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
    random_walk,
)

# Archetypes the killer treats as prey. Registered agents are "wanderer";
# the rest cover scenario-spawned actors. Items/objects are excluded.
_AGENTS = ("trainer", "wanderer", "survivor", "killer", "manipulator", "scavenger")


@dataclass
class Killer(ArchetypeBot):
    archetype_name: str = "killer"
    state: str = "HUNTING"
    target_id: Optional[str] = None

    def decide(self, obs: Observation) -> Optional[Action]:
        s = obs.self
        here = tuple(s.pos)
        hp = int((s.extras or {}).get("hp", 100) or 100)

        # Badly hurt: flee the nearest agent until patched up. Never just
        # stand — flee or roam.
        if hp < 18:
            self.state = "RETREATING"
            threats = [e for e in obs.visible_entities if e.entity_id != s.entity_id]
            if threats:
                self.goal = Goal.flee(nearest(threats, here).entity_id)
                return None
            self.goal = Goal.idle()
            return random_walk(self, here)
        if self.state == "RETREATING" and hp < 45:
            # still recovering
            threats = [e for e in obs.visible_entities if e.entity_id != s.entity_id]
            if threats and hp < 30:
                self.goal = Goal.flee(nearest(threats, here).entity_id)
                return None

        # Free lethality: equip a carried weapon, or grab one underfoot.
        inv = list((s.extras or {}).get("inventory") or [])
        if not ((s.extras or {}).get("equipped", {}) or {}).get("weapon"):
            win = next((i for i in inv if isinstance(i, str)
                        and any(k in i for k in WEAPON_KINDS)), None)
            if win:
                return Equip(item=win, slot="weapon")
            whot = [it for it in obs.visible_items if item_kind(it) in WEAPON_KINDS
                    and chebyshev(here, tuple(it.pos)) <= 1]
            if whot:
                return Pickup(target=whot[0].entity_id)

        # RELENTLESS HUNT: re-pick the nearest visible agent EVERY tick and
        # chase it. No stale target-lock — when a victim flees out of sight
        # the killer immediately switches to the next-nearest agent instead
        # of walking to an empty last-seen tile and sitting there (the "once
        # the victim is far they just sit" bug). Attack the instant adjacent.
        prey = [e for e in obs.visible_entities
                if e.entity_id != s.entity_id and e.archetype in _AGENTS]
        if prey:
            t = nearest(prey, here)
            self.target_id = t.entity_id
            if chebyshev(here, tuple(t.pos)) <= 1:
                self.state = "ATTACKING"
                return Attack(target=t.entity_id)
            self.state = "PURSUING"
            self.goal = Goal.pursue(t.entity_id)
            return None  # motor closes on the nearest prey

        # No agent in sight: grab loose gold, else roam to find someone.
        money_items = [it for it in obs.visible_items if is_money(it)]
        if money_items:
            t = nearest(money_items, here)
            if chebyshev(here, tuple(t.pos)) <= 1:
                return Pickup(target=t.entity_id)
            self.goal = Goal.goto(*t.pos)
            return None
        self.state = "PROWL"
        self.goal = Goal.idle()
        return random_walk(self, here)

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
