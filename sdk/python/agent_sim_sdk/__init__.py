"""agent_sim_sdk — Python client SDK.

Connect an agent to an agent_sim world over WebSocket. Receives typed
observations, sends typed actions. Vision mode (structured / image /
both) is set per connection.
"""

from .client import Agent, register_and_connect, register_agent, AgentCredentials
from .models import (
    Observation, Action, ActionBatch, ActionResult, ReasonCode, Pos, Facing,
    Move, Speak, Whisper, Shout, LookAt, Interact,
    Pickup, Drop, Equip, Give, Attack, Defend, Heal, Wait,
    Pay, WorkForPay, Trade, Loot,
    Chop, Mine,
    Enter, Exit, Lock, Unlock, ClaimOwnership, TransferOwnership,
    PlaceBlueprint, AdvanceConstruction, Demolish,
    ProposeTask, AcceptTask, RejectTask, CompleteTask,
    VisionMode,
)
from .pathfind import Pathfinder, WALKABLE_TILES, BLOCKING_ARCHETYPES

__all__ = [
    "Agent",
    "register_and_connect", "register_agent", "AgentCredentials",
    "Observation",
    "Action", "ActionBatch", "ActionResult", "ReasonCode",
    "Pos",
    "Facing",
    "VisionMode",
    "Move", "Speak", "Whisper", "Shout", "LookAt", "Interact",
    "Pickup", "Drop", "Equip", "Give", "Attack", "Defend", "Heal", "Wait",
    "Pay", "WorkForPay", "Trade", "Loot",
    "Chop", "Mine",
    "Enter", "Exit", "Lock", "Unlock", "ClaimOwnership", "TransferOwnership",
    "PlaceBlueprint", "AdvanceConstruction", "Demolish",
    "ProposeTask", "AcceptTask", "RejectTask", "CompleteTask",
    "Pathfinder", "WALKABLE_TILES", "BLOCKING_ARCHETYPES",
]
