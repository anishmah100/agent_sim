"""Map an LLM-emitted action dict → a typed SDK Action.

Returns None for unknown verbs or malformed payloads so the caller
can skip them without crashing the decision loop. Grammar-constrained
decoding should make most of these defensive checks unnecessary, but
the agent must NEVER die because the model produced an odd action.
"""
from __future__ import annotations

import logging
from typing import Any, Optional

from agent_sim_sdk import (
    Action, Move, Speak, Whisper, Shout, Eat, Pickup, Equip, Give,
    Pay, Trade, Attack, ProposeTask, AcceptTask, CompleteTask, Wait,
)

log = logging.getLogger("agents.llm.actions")


def to_action(d: dict[str, Any]) -> Optional[Action]:
    if not isinstance(d, dict):
        return None
    verb = d.get("verb")
    try:
        if verb == "move":
            tgt = d.get("target")
            if not (isinstance(tgt, (list, tuple)) and len(tgt) == 2):
                return None
            return Move(target=[int(tgt[0]), int(tgt[1])])
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
        if verb == "wait":
            return Wait(ticks=int(d.get("ticks", 60)))
    except (KeyError, ValueError, TypeError) as e:
        log.warning("could not map action %s: %s", d, e)
        return None
    log.warning("unknown verb: %s", verb)
    return None
