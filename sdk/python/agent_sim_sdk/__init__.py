"""agent_sim_sdk — Python client SDK.

Connect an agent to an agent_sim world over WebSocket. Receives typed
observations, sends typed actions. Vision mode (structured / image /
both) is set per connection.
"""

from .client import Agent, register_and_connect, register_agent, AgentCredentials
from .models import (
    Observation, SelfState, VisibleEntity, VisibleObject, VisibleItem, AudibleEvent, WorldClock,
    LocalView,
    Action, ActionBatch, ActionResult, ReasonCode, Pos, Facing,
    Move, Step, Speak, Whisper, Shout, LookAt, Interact,
    Pickup, Drop, Eat, Equip, Give, Attack, Defend, Heal, Wait,
    MentalNote,
    Pay, WorkForPay, Trade, Loot,
    Chop, Mine,
    Enter, Exit, Lock, Unlock, ClaimOwnership, TransferOwnership,
    PlaceBlueprint, AdvanceConstruction, Demolish,
    ProposeTask, AcceptTask, RejectTask, CompleteTask,
    VisionMode,
)
from .pathfind import Pathfinder, WALKABLE_TILES, BLOCKING_ARCHETYPES
from .observation_render import (
    render_layered_observation, render_self, render_nearby,
    render_audible, render_minimap, rank_nearby, relative_compass,
)

__all__ = [
    "Agent",
    "register_and_connect", "register_agent", "AgentCredentials",
    "Observation", "SelfState", "VisibleEntity", "VisibleObject", "VisibleItem",
    "AudibleEvent", "WorldClock", "LocalView",
    "Action", "ActionBatch", "ActionResult", "ReasonCode",
    "Pos",
    "Facing",
    "VisionMode",
    "Move", "Step", "Speak", "Whisper", "Shout", "LookAt", "Interact",
    "Pickup", "Drop", "Eat", "Equip", "Give", "Attack", "Defend", "Heal", "Wait",
    "MentalNote",
    "Pay", "WorkForPay", "Trade", "Loot",
    "Chop", "Mine",
    "Enter", "Exit", "Lock", "Unlock", "ClaimOwnership", "TransferOwnership",
    "PlaceBlueprint", "AdvanceConstruction", "Demolish",
    "ProposeTask", "AcceptTask", "RejectTask", "CompleteTask",
    "Pathfinder", "WALKABLE_TILES", "BLOCKING_ARCHETYPES",
    "render_layered_observation", "render_self", "render_nearby",
    "render_audible", "render_minimap", "rank_nearby", "relative_compass",
]
