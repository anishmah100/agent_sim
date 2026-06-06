"""Survivor archetype (D16). Peaceful. Stay alive by eating food.

States: IDLE → HUNGRY → EATING; IDLE/HUNGRY → FLEEING when an armed
agent appears in vision; HUNGRY → DESPERATE when starving and no food
visible.

Never attacks. Provides a victim class for killers + a quantitative
target for whether the food economy is balanced (a survivor that
starves means food is too scarce or too costly).
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from agent_sim_sdk import (
    Action,
    Eat,
    Move,
    Observation,
    Pay,
    Pickup,
)

from ._common import (
    ArchetypeBot,
    has_weapon_equipped,
    is_food,
    item_kind,
    nearest,
    random_walk,
    step_away,
    step_toward,
)


@dataclass
class Survivor(ArchetypeBot):
    archetype_name: str = "survivor"
    state: str = "IDLE"
    # FLEEING → IDLE requires 5 consecutive ticks with no armed agent
    # in vision. Reset whenever a threat reappears.
    clear_streak: int = 0

    def decide(self, obs: Observation) -> Optional[Action]:
        s = obs.self
        here = tuple(s.pos)
        extras = s.extras or {}
        hunger = float(extras.get("hunger", 0.0) or 0.0)
        inv = list(extras.get("inventory") or [])

        threats = [e for e in obs.visible_entities if has_weapon_equipped(e)]
        foods = [it for it in obs.visible_items if is_food(it)]

        # FLEEING wins over everything except ongoing transitions.
        if threats:
            self.state = "FLEEING"
            self.clear_streak = 0
            t = nearest(threats, here)
            return Move(target=list(step_away(here, tuple(t.pos))))

        if self.state == "FLEEING":
            self.clear_streak += 1
            if self.clear_streak >= 5:
                self.state = "IDLE"
            else:
                # No threat in vision this tick, but be cautious: random walk.
                return random_walk(self, here)

        # Hunger arbitration.
        food_in_inv = next((it for it in inv if isinstance(it, str) and any(
            k in it for k in ("apple", "bread", "cheese", "fish", "berry"))), None)

        if hunger > 0.85 and not foods and not food_in_inv:
            self.state = "DESPERATE"
        elif hunger > 0.5:
            self.state = "HUNGRY"
        elif hunger < 0.3 and self.state in ("HUNGRY", "DESPERATE", "EATING"):
            self.state = "IDLE"

        if self.state in ("HUNGRY", "DESPERATE"):
            # (a) inventory food → eat.
            if food_in_inv:
                self.state = "EATING"
                return Eat(item=food_in_inv)
            # (b) adjacent food entity → pickup.
            if foods:
                f = nearest(foods, here)
                if max(abs(f.pos[0] - here[0]), abs(f.pos[1] - here[1])) <= 1:
                    return Pickup(target=f.entity_id)
                # (c) walk toward closest food.
                return Move(target=list(step_toward(here, tuple(f.pos))))
            # (d-f) elaborate fallbacks (chop tree, trade at stall) are
            # left for a later pass — without them, DESPERATE → death
            # is the failure mode that flags "food economy is too tight".
            return random_walk(self, here)

        if self.state == "EATING":
            # 1-tick state; next obs will re-evaluate.
            self.state = "IDLE"
            return None

        # IDLE: random walk ~50% of the time.
        if self.rng.random() < 0.5:
            return random_walk(self, here)
        return None

    def transition_note(self):
        slots = {"goal": "stay alive", "plan": f"state={self.state}"}
        if self.state == "FLEEING":
            slots["emotion"] = "afraid"
        elif self.state == "DESPERATE":
            slots["emotion"] = "desperate"
        return None, slots
