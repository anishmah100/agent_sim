"""Typed observation + action models. Mirrors docs/OBSERVATION_MODEL.md
and docs/VERB_REFERENCE.md. Used by both the SDK and (eventually) the
visual-regression test layer that loads recorded observations off disk.
"""

from __future__ import annotations

from enum import Enum
from typing import Annotated, Any, Literal, Optional, Union

from pydantic import BaseModel, Field

Pos = tuple[int, int]


class Facing(str, Enum):
    N = "N"
    S = "S"
    E = "E"
    W = "W"


class VisionMode(str, Enum):
    """How much the engine should render for this agent. `structured`
    is JSON only; `image` includes a per-tick PNG/WebP crop; `both`
    delivers both. Multimodal agents pick `image` or `both`."""
    STRUCTURED = "structured"
    IMAGE = "image"
    BOTH = "both"


class SelfState(BaseModel):
    entity_id: str
    pos: Pos
    facing: Facing
    extras: dict[str, Any] = Field(default_factory=dict)
    current_action: Optional[dict[str, Any]] = None
    last_action_result: Optional[dict[str, Any]] = None


class VisibleEntity(BaseModel):
    entity_id: str
    apparent_label: str
    pos: Pos
    facing: Facing
    archetype: str
    extras_summary: dict[str, Any] = Field(default_factory=dict)
    doing: Optional[str] = None


class VisibleObject(BaseModel):
    object_id: str
    kind: str
    pos: Pos
    affordances: list[str] = Field(default_factory=list)
    state_summary: dict[str, Any] = Field(default_factory=dict)


class AudibleEvent(BaseModel):
    event_id: str
    kind: Literal["speech", "shout", "whisper", "sound"]
    from_entity: str
    from_pos: Pos
    text: Optional[str] = None
    sound_kind: Optional[str] = None
    tick: int


class WorldClock(BaseModel):
    tick: int
    day_phase: Literal["dawn", "morning", "midday", "afternoon", "dusk", "night"]
    weather: str = "clear"


class KnownMap(BaseModel):
    map_id: str
    map_dims: tuple[int, int]
    named_regions: list[dict[str, Any]] = Field(default_factory=list)
    portals: list[dict[str, Any]] = Field(default_factory=list)


class ViewImage(BaseModel):
    """Multimodal observation payload. `data` is raw bytes — base64
    decoded by the WS layer before this model is constructed."""
    format: Literal["png", "webp"]
    width: int
    height: int
    data: bytes
    centered_on_pos: Pos
    facing: Facing


class Observation(BaseModel):
    obs_id: int
    world_tick: int
    self: SelfState
    visible_entities: list[VisibleEntity] = Field(default_factory=list)
    visible_objects: list[VisibleObject] = Field(default_factory=list)
    audible: list[AudibleEvent] = Field(default_factory=list)
    recent_self_results: list[dict[str, Any]] = Field(default_factory=list)
    known_map_summary: Optional[KnownMap] = None
    world_clock: WorldClock
    view_image: Optional[ViewImage] = None


# === Actions ===

class _Action(BaseModel):
    """Base. Concrete subclasses set their own `verb` literal so the
    discriminated union below picks the right serialization."""
    verb: str
    priority: int = 0


class Move(_Action):
    verb: Literal["move"] = "move"
    target: Pos
    jog: bool = False


class Speak(_Action):
    verb: Literal["speak"] = "speak"
    text: str


class Whisper(_Action):
    verb: Literal["whisper"] = "whisper"
    target: str
    text: str


class Shout(_Action):
    verb: Literal["shout"] = "shout"
    text: str


class LookAt(_Action):
    verb: Literal["look_at"] = "look_at"
    target: Union[str, Pos]


class Interact(_Action):
    verb: Literal["interact"] = "interact"
    target: str
    affordance: str


class Pickup(_Action):
    verb: Literal["pickup"] = "pickup"
    target: str


class Drop(_Action):
    verb: Literal["drop"] = "drop"
    item: str


class Equip(_Action):
    verb: Literal["equip"] = "equip"
    item: str
    slot: Optional[str] = None


class Give(_Action):
    verb: Literal["give"] = "give"
    target: str
    item: str


class Attack(_Action):
    verb: Literal["attack"] = "attack"
    target: str


class Defend(_Action):
    verb: Literal["defend"] = "defend"


class Heal(_Action):
    verb: Literal["heal"] = "heal"
    target: Optional[str] = None  # default = self


class Wait(_Action):
    verb: Literal["wait"] = "wait"
    ticks: int = 60


# === Session-2 composable-system verbs ===

class Pay(_Action):
    """Transfer gold to an adjacent target (Money system)."""
    verb: Literal["pay"] = "pay"
    target: str
    amount: int


class WorkForPay(_Action):
    """Earn a small wage (Money system stub)."""
    verb: Literal["work_for_pay"] = "work_for_pay"


class Trade(_Action):
    """Atomic item-for-gold swap with an adjacent target (Trade system)."""
    verb: Literal["trade"] = "trade"
    target: str
    item: str
    price: int


class Loot(_Action):
    """Take gold + clear inventory from an adjacent corpse (Loot system)."""
    verb: Literal["loot"] = "loot"
    target: str


class Chop(_Action):
    """Chop an adjacent tree entity for wood (Resources system)."""
    verb: Literal["chop"] = "chop"
    target: str


class Mine(_Action):
    """Mine an adjacent rock entity for stone (Resources system)."""
    verb: Literal["mine"] = "mine"
    target: str


class Enter(_Action):
    """Step inside an adjacent building (Property system)."""
    verb: Literal["enter"] = "enter"
    target: str


class Exit(_Action):
    """Leave the building you're currently in (Property system)."""
    verb: Literal["exit"] = "exit"


class Lock(_Action):
    """Lock an owned building (Property system; owner only)."""
    verb: Literal["lock"] = "lock"
    target: str


class Unlock(_Action):
    """Unlock an owned building (Property system; owner only)."""
    verb: Literal["unlock"] = "unlock"
    target: str


class ClaimOwnership(_Action):
    """Claim an unowned adjacent building (Property system)."""
    verb: Literal["claim_ownership"] = "claim_ownership"
    target: str


class TransferOwnership(_Action):
    """Hand an owned building to another entity (Property system)."""
    verb: Literal["transfer_ownership"] = "transfer_ownership"
    target: str
    new_owner: str


class PlaceBlueprint(_Action):
    """Place a building blueprint at an adjacent walkable tile.
    Pays the initial materials up front (Construction system)."""
    verb: Literal["place_blueprint"] = "place_blueprint"
    kind: str       # "cottage" | "shed" | ...
    at: Pos


class AdvanceConstruction(_Action):
    """Spend the next batch of materials on an owned blueprint.
    Completes the build when progress hits 100 (Construction system)."""
    verb: Literal["advance_construction"] = "advance_construction"
    target: str


class Demolish(_Action):
    """Remove an owned blueprint or building (Construction system)."""
    verb: Literal["demolish"] = "demolish"
    target: str


class ProposeTask(_Action):
    """Propose a verbal contract to a known entity (VerbalQuests system).
    The engine records the contract but does NOT enforce completion —
    that's emergent from the agents' behavior."""
    verb: Literal["propose_task"] = "propose_task"
    target: str
    terms: str
    reward: Optional[str] = None


class AcceptTask(_Action):
    """Accept a contract proposed to you (VerbalQuests system)."""
    verb: Literal["accept_task"] = "accept_task"
    id: str


class RejectTask(_Action):
    """Reject a contract proposed to you (VerbalQuests system)."""
    verb: Literal["reject_task"] = "reject_task"
    id: str


class CompleteTask(_Action):
    """Mark a contract complete (from the proposer's PoV — no
    engine verification; VerbalQuests system)."""
    verb: Literal["complete_task"] = "complete_task"
    id: str


Action = Annotated[
    Union[
        # Base verbs.
        Move, Speak, Whisper, Shout, LookAt, Interact,
        Pickup, Drop, Equip, Give, Attack, Defend, Heal, Wait,
        # Composable-system verbs (session 2).
        Pay, WorkForPay, Trade, Loot,
        Chop, Mine,
        Enter, Exit, Lock, Unlock, ClaimOwnership, TransferOwnership,
        PlaceBlueprint, AdvanceConstruction, Demolish,
        ProposeTask, AcceptTask, RejectTask, CompleteTask,
    ],
    Field(discriminator="verb"),
]
