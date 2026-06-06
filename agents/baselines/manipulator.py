"""Manipulator archetype (D16). Builds fake trust, proposes a soft
contract, then defects. Tests D13's verbal-contracts substrate from
the adversarial side.

The most behaviourally complex archetype — but the FSM itself is
small. Hardness comes from the scripted speech templates and the
betrayal-timing logic.

Coin-flip defection: when WAITING long enough, ~50% transition to
DEFECTING_SILENT (walk away) and ~50% to DEFECTING_ACTIVE (attack).
The flip is seeded by the bot's RNG → deterministic across re-runs.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Optional

from agent_sim_sdk import (
    Action,
    Attack,
    Give,
    Move,
    Observation,
    ProposeTask,
    Speak,
)

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


# How many ticks to stay in WAITING before flipping the betrayal coin.
WAITING_DEADLINE_TICKS = 30
# How many ticks of vision-clear before transitioning FLEEING → SCOUTING.
FLEE_CLEAR_TICKS = 10


# Scripted speech templates. Index by state for clarity.
TRUST_LINES = [
    "Hello, friend. Strange to find someone trustworthy out here.",
    "Take this — a small token of goodwill.",
    "We should look out for each other in these dangerous times.",
]
PROPOSE_LINES = [
    "I have a proposition. Bring me food and I'll pay you well.",
    "Deal?",
]
BETRAYAL_LINE = "Sorry, friend. The arrangement is off."


@dataclass
class Manipulator(ArchetypeBot):
    archetype_name: str = "manipulator"
    state: str = "SCOUTING"
    target_id: Optional[str] = None
    last_target_pos: Optional[tuple[int, int]] = None
    waiting_ticks: int = 0
    flee_clear_ticks: int = 0
    # 0 = haven't run the BUILDING_TRUST sub-script yet, 1 = gift given,
    # 2 = friendly line spoken.
    trust_step: int = 0
    propose_step: int = 0

    def decide(self, obs: Observation) -> Optional[Action]:
        s = obs.self
        here = tuple(s.pos)
        threats = [
            e for e in obs.visible_entities
            if e.entity_id != s.entity_id and has_weapon_equipped(e)
            and e.entity_id != self.target_id
        ]

        # Retaliation → FLEEING wins over everything except already-fleeing.
        if self.state in ("DEFECTING_ACTIVE",) and threats:
            self.state = "FLEEING"

        if self.state == "FLEEING":
            if not threats:
                self.flee_clear_ticks += 1
                if self.flee_clear_ticks >= FLEE_CLEAR_TICKS:
                    self.state = "SCOUTING"
                    self.target_id = None
                    self.flee_clear_ticks = 0
                return random_walk(self, here)
            self.flee_clear_ticks = 0
            t = nearest(threats, here)
            return Move(target=list(step_away(here, tuple(t.pos))))

        # SCOUTING — pick a target by score.
        if self.state == "SCOUTING":
            target = self._pick_target(obs.visible_entities, here, s.entity_id)
            if target is not None:
                self.target_id = target.entity_id
                self.last_target_pos = tuple(target.pos)
                self.state = "APPROACHING"
                self.trust_step = 0
                self.propose_step = 0
                self.waiting_ticks = 0
            else:
                return random_walk(self, here)

        # APPROACHING — walk to target until adjacent.
        if self.state == "APPROACHING":
            target = self._find_target(obs.visible_entities)
            if target is None:
                # Target faded out of vision. Walk to last known.
                if self.last_target_pos is not None:
                    return Move(target=list(step_toward(here, self.last_target_pos)))
                self.state = "SCOUTING"
                self.target_id = None
                return random_walk(self, here)
            self.last_target_pos = tuple(target.pos)
            if chebyshev(here, tuple(target.pos)) <= 1:
                self.state = "BUILDING_TRUST"
            else:
                return Move(target=list(step_toward(here, tuple(target.pos))))

        # BUILDING_TRUST — gift then speak friendly.
        if self.state == "BUILDING_TRUST":
            target = self._find_target(obs.visible_entities)
            if target is None:
                self.state = "SCOUTING"
                return None
            if self.trust_step == 0:
                # Pick a low-value inventory item to gift.
                inv = list((s.extras or {}).get("inventory") or [])
                gift = next(
                    (i for i in inv if isinstance(i, str) and "apple" in i),
                    inv[0] if inv else None,
                )
                self.trust_step = 1
                if gift:
                    return Give(target=target.entity_id, item=gift)
                # No inventory? Skip ahead.
            if self.trust_step == 1:
                self.trust_step = 2
                return Speak(text=TRUST_LINES[0])
            # Trust script done; move to PROPOSING.
            self.state = "PROPOSING"

        if self.state == "PROPOSING":
            target = self._find_target(obs.visible_entities)
            if target is None:
                self.state = "SCOUTING"
                return None
            if self.propose_step == 0:
                self.propose_step = 1
                return ProposeTask(
                    target=target.entity_id,
                    terms="bring me food",
                    reward="20 gold",
                )
            # Already proposed: wait for accept/reject by tick.
            # We approximate "accepted" by looking for a TaskAccepted-ish
            # signal. Since we don't have direct access to contracts
            # from the SDK obs, we transition to WAITING optimistically
            # and let timing drive the betrayal.
            self.state = "WAITING"
            self.waiting_ticks = 0
            return Speak(text=PROPOSE_LINES[1])

        if self.state == "WAITING":
            self.waiting_ticks += 1
            if self.waiting_ticks >= WAITING_DEADLINE_TICKS:
                # Coin flip via the bot's deterministic RNG.
                if self.rng.random() < 0.5:
                    self.state = "DEFECTING_SILENT"
                else:
                    self.state = "DEFECTING_ACTIVE"
            elif self.waiting_ticks % 8 == 0:
                return Speak(text="Looking forward to our arrangement.")
            else:
                # Stay near the target (random local walk).
                return random_walk(self, here)

        if self.state == "DEFECTING_SILENT":
            target = self._find_target(obs.visible_entities)
            if target is not None:
                return Move(target=list(step_away(here, tuple(target.pos))))
            # Target gone; head back to scouting.
            self.state = "SCOUTING"
            self.target_id = None
            return random_walk(self, here)

        if self.state == "DEFECTING_ACTIVE":
            target = self._find_target(obs.visible_entities)
            if target is None:
                self.state = "SCOUTING"
                self.target_id = None
                return None
            if chebyshev(here, tuple(target.pos)) > 1:
                return Move(target=list(step_toward(here, tuple(target.pos))))
            # First tick in this state: speak the betrayal line; next
            # tick: attack. The `_betrayal_spoken` instance bit tracks
            # whether the warning line landed already.
            if not getattr(self, "_betrayal_spoken", False):
                self._betrayal_spoken = True
                return Speak(text=BETRAYAL_LINE)
            return Attack(target=target.entity_id)

        return None

    # ----- target picking -----

    @staticmethod
    def _pick_target(entities, here, self_id):
        # Any visible agent is fair game; ranking prefers closer targets.
        best, best_score = None, 0.0
        for e in entities:
            if e.entity_id == self_id:
                continue
            if e.archetype not in (
                "trainer", "wanderer", "survivor", "killer",
                "manipulator", "scavenger",
            ):
                continue
            # No combat indicator → safe target. (Combat flag is not
            # currently in extras_summary; without it everyone is
            # eligible.)
            d = chebyshev(here, tuple(e.pos))
            # The mental_state-derived gold heuristic isn't available in
            # the obs; treat every agent as equally desirable, weighted
            # by proximity.
            score = 1.0 / max(d, 1)
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

    def transition_note(self):
        slots = {"goal": "extract value via fake trust", "plan": f"state={self.state}"}
        if self.target_id:
            slots["beliefs"] = f"mark={self.target_id}"
        if self.state in ("DEFECTING_SILENT", "DEFECTING_ACTIVE"):
            slots["emotion"] = "satisfied"
        return None, slots
