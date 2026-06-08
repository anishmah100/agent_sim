"""Map an LLM-emitted action dict → either a typed SDK Action or a motor Goal.

Movement is expressed as a STANDING GOAL (pursue/flee/goto), not a one-shot
coordinate move: the LLM (deliberation) sets the goal and the harness motor
(reflex) executes it one N/S/E/W step per tick until it changes (see
docs/AGENT_MOVEMENT_REDESIGN.md). ``to_goal`` recognises the movement verbs;
``to_action`` handles the direct verbs (speak/attack/pickup/…). A direct
``step`` is also supported for the "let the LLM navigate" experiment mode.

Both return None for unknown/malformed payloads so the caller can skip them
without crashing — the agent must NEVER die because the model produced an odd
action.
"""
from __future__ import annotations

import logging
from typing import Any, Optional

from agent_sim_sdk import (
    Action, Step, Speak, Whisper, Shout, Eat, Pickup, Equip, Give,
    Pay, BuyFood, Trade, Attack, ProposeTask, AcceptTask, CompleteTask, RejectTask, Wait,
    Enter, Exit,
)
from agents.common.motor import Goal

log = logging.getLogger("agents.llm.actions")

# Verbs that set the standing motor goal rather than acting this instant.
MOVEMENT_VERBS = {"pursue", "flee", "goto"}
_DIRS = {"N", "S", "E", "W"}


def to_goal(d: dict[str, Any]) -> Optional[Goal]:
    """If `d` is a movement verb, return the Goal it sets; else None."""
    if not isinstance(d, dict):
        return None
    verb = d.get("verb")
    try:
        if verb == "pursue":
            return Goal.pursue(str(d["target"]))
        if verb == "flee":
            return Goal.flee(str(d["target"]))
        if verb == "goto":
            tgt = d.get("target")
            if not (isinstance(tgt, (list, tuple)) and len(tgt) == 2):
                return None
            return Goal.goto(int(tgt[0]), int(tgt[1]))
    except (KeyError, ValueError, TypeError) as e:
        log.warning("could not map goal %s: %s", d, e)
        return None
    return None


def to_action(d: dict[str, Any]) -> Optional[Action]:
    """Map a non-movement (direct) verb to an SDK Action. Movement verbs
    (pursue/flee/goto) are handled by to_goal and return None here."""
    if not isinstance(d, dict):
        return None
    verb = d.get("verb")
    try:
        if verb == "step":
            dr = str(d.get("dir", "")).upper()[:1]
            return Step(dir=dr) if dr in _DIRS else None
        if verb == "speak":
            return Speak(text=str(d.get("text", "")))
        if verb == "whisper":
            return Whisper(target=str(d["target"]), text=str(d.get("text", "")))
        if verb == "shout":
            return Shout(text=str(d.get("text", "")))
        if verb == "eat":
            return Eat(item=str(d["item"]))
        if verb == "pickup":
            return Pickup(target=str(d["target"]))
        if verb == "equip":
            return Equip(item=str(d["item"]), slot=d.get("slot"))
        if verb == "give":
            return Give(target=str(d["target"]), item=str(d["item"]))
        if verb == "pay":
            return Pay(target=str(d["target"]), amount=int(d["amount"]))
        if verb == "buy_food":
            return BuyFood()
        if verb == "trade":
            return Trade(target=str(d["target"]), item=str(d["item"]),
                         price=int(d["price"]))
        if verb == "attack":
            return Attack(target=str(d["target"]))
        if verb == "propose_task":
            return ProposeTask(target=str(d["target"]),
                               terms=str(d.get("terms", "")),
                               reward=d.get("reward"))
        if verb == "accept_task":
            return AcceptTask(id=str(d["id"]))
        if verb == "complete_task":
            return CompleteTask(id=str(d["id"]))
        if verb == "reject_task":
            return RejectTask(id=str(d["id"]))
        if verb == "enter":
            return Enter(target=str(d["target"]))
        if verb == "exit":
            return Exit()
        if verb == "wait":
            return Wait(ticks=int(d.get("ticks", 60)))
    except (KeyError, ValueError, TypeError) as e:
        log.warning("could not map action %s: %s", d, e)
        return None
    if verb in MOVEMENT_VERBS:
        return None  # handled by to_goal
    log.warning("unknown verb: %s", verb)
    return None
