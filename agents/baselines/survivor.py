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
    BuyFood,
    Eat,
    Observation,
    Pay,
    Pickup,
)

# Mirror of the engine's default food_price (money.DefaultFoodPrice). The
# bot can't read world tunings, so it uses the default to decide whether a
# market meal is affordable; an over-optimistic buy just returns
# not_enough_gold harmlessly.
FOOD_PRICE = 6

from agents.common.motor import Goal

from ._common import (
    ArchetypeBot,
    forage_or_roam,
    has_weapon_equipped,
    is_food,
    is_infamous,
    is_money,
    item_kind,
    nearest,
    random_walk,
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

        # Threats = the armed AND the infamous (a known killer is dangerous
        # even unarmed — reputation makes the survivor wary of notorious
        # agents it has heard about / can see).
        threats = [e for e in obs.visible_entities
                   if has_weapon_equipped(e) or is_infamous(e)]
        foods = [it for it in obs.visible_items if is_food(it)]

        # FLEEING wins over everything except ongoing transitions.
        if threats:
            self.state = "FLEEING"
            self.clear_streak = 0
            t = nearest(threats, here)
            self.goal = Goal.flee(t.entity_id)  # motor steps away each tick
            return None

        if self.state == "FLEEING":
            self.clear_streak += 1
            if self.clear_streak >= 5:
                self.state = "IDLE"
                self.goal = Goal.idle()
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
                # (c) walk toward closest food (motor navigates).
                self.goal = Goal.goto(*f.pos)
                return None
            # (d) no food in sight but gold in pocket → buy a meal at the
            # market. This is the gold sink that makes accumulated wealth
            # actually buy survival; a rich survivor no longer starves
            # amid coins it can't eat.
            gold = int(extras.get("gold", 0) or 0)
            if gold >= FOOD_PRICE:
                self.state = "EATING"
                return BuyFood()
            # (e-f) elaborate fallbacks (chop tree, trade at stall) are
            # left for a later pass — without them, broke+DESPERATE → death
            # is the failure mode that flags "food economy is too tight".
            return random_walk(self, here)

        if self.state == "EATING":
            # 1-tick state; next obs will re-evaluate.
            self.state = "IDLE"
            return None

        # IDLE: keep busy. Prefer money (auto-converts to gold, no slot
        # cost), but if there's no coin in view, grab ANY nearby item or
        # roam toward fresh ground — an idle survivor should never just
        # stand among loot. forage_or_roam never returns None.
        money_items = [it for it in obs.visible_items if is_money(it)]
        if money_items:
            target = nearest(money_items, here)
            if max(abs(target.pos[0] - here[0]),
                   abs(target.pos[1] - here[1])) <= 1:
                return Pickup(target=target.entity_id)
            self.goal = Goal.goto(*target.pos)  # motor navigates to the coin
            return None
        return forage_or_roam(self, obs, here)

    def transition_note(self):
        # Default goal is gold accumulation; survival pressure
        # temporarily preempts it.
        if self.state in ("HUNGRY", "EATING", "DESPERATE"):
            goal = "find food"
        elif self.state == "FLEEING":
            goal = "stay alive"
        else:
            goal = "accumulate gold"
        slots = {"goal": goal, "plan": f"state={self.state}"}
        if self.state == "FLEEING":
            slots["emotion"] = "afraid"
        elif self.state == "DESPERATE":
            slots["emotion"] = "desperate"
        return None, slots
