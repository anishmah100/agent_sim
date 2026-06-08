"""Typed observation + action models. Mirrors docs/OBSERVATION_MODEL.md
and docs/VERB_REFERENCE.md. Used by both the SDK and (eventually) the
visual-regression test layer that loads recorded observations off disk.
"""

from __future__ import annotations

from enum import Enum
from typing import Annotated, Any, Literal, NewType, Optional, Union

from pydantic import BaseModel, Field

Pos = tuple[int, int]

# D1 — every action verb that names another agent uses the entity_id
# as the target, NEVER the display name. Display names can collide
# (two agents both named "John") and an LLM hallucinating a name
# should fail loudly, not silently hit the wrong target. EntityID is
# a NewType over str — purely cosmetic at runtime but signals intent
# to IDEs, type checkers, and bot authors. Engine handlers resolve
# targets by direct entity_id map lookup; if the id isn't found, the
# action is rejected with reason="unknown_target".
EntityID = NewType("EntityID", str)


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
    inside_building: Optional[str] = None
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


class VisibleItem(BaseModel):
    """D8 — a pickup-able item entity within the agent's vision radius +
    line of sight. Different from VisibleObject because items support
    the `pickup` verb (whereas decorations only support interact-
    affordance). The sprite carries the kind (e.g. ``"item:apple"``).
    Quantity defaults to 1 for non-stackable items, higher for stacks
    like coin piles."""
    entity_id: str
    sprite: str
    pos: Pos
    quantity: int = 1
    label: Optional[str] = None


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


class LocalView(BaseModel):
    """Egocentric ASCII tile-map — "what the screen shows around me" as text.

    ``rows`` is a square block of glyphs; ``rows[0]`` is the northernmost row.
    ``origin`` is the world (x, y) of ``rows[0][0]``; add a glyph's
    (col, row) to ``origin`` to recover its world coordinate. Terrain is
    fully known out to ``radius``; dynamic entities/items only appear where
    vision+LOS reached, so a blank cell means "no terrain obstacle AND
    nothing seen", not "definitely empty". Use the structured
    ``visible_entities`` / ``visible_items`` lists for exact ids + metadata;
    this view is for spatial reasoning (routes, walls, water).

    Glyphs (also in ``legend``): ``@`` you · ``.`` walkable · ``#`` blocked ·
    ``~`` water · ``(space)`` off-map/unknown · ``P`` person · ``$`` item ·
    ``+`` door.
    """
    radius: int
    origin: Pos
    rows: list[str] = Field(default_factory=list)
    legend: dict[str, str] = Field(default_factory=dict)


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
    # D8 — pickup-able items in vision + LOS. Empty when no items in
    # range; engine returns items as entities of archetype="item".
    visible_items: list[VisibleItem] = Field(default_factory=list)
    audible: list[AudibleEvent] = Field(default_factory=list)
    recent_self_results: list[dict[str, Any]] = Field(default_factory=list)
    # Egocentric ASCII terrain window (radius LocalViewRadius). Present on
    # every live observation; the agent reads it to plan routes the way a
    # human controlling the avatar would (see the lake, route around walls).
    local_view: Optional[LocalView] = None
    world_clock: WorldClock
    view_image: Optional[ViewImage] = None


# === Actions ===

class _Action(BaseModel):
    """Base. Concrete subclasses set their own `verb` literal so the
    discriminated union below picks the right serialization."""
    verb: str
    priority: int = 0


class Step(_Action):
    """Move exactly one tile in a compass direction (N/S/E/W). The AGENT
    owns navigation — compute your route (see agents.common.nav) and feed
    the engine one Step per tick. The engine does NOT pathfind."""
    verb: Literal["step"] = "step"
    dir: Literal["N", "S", "E", "W"]


class Speak(_Action):
    verb: Literal["speak"] = "speak"
    text: str


class Whisper(_Action):
    """Whisper a message to a specific agent (1-tile adjacency required).
    ``target`` is the recipient's ``entity_id`` from
    ``observation.visible_entities[i].entity_id`` — NEVER a display
    name. See D1 in PHASE_SOCIAL_EMERGENCE.md."""
    verb: Literal["whisper"] = "whisper"
    target: EntityID
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


class Eat(_Action):
    """D22 — consume a food item from inventory. Subtracts the food's
    satiety from hunger (clamped at 0). Instant; no cooldown. Reasons
    you might see in last_action_result: ``not_in_inventory``,
    ``not_food`` (item has no satiety value)."""
    verb: Literal["eat"] = "eat"
    item: str


class Cook(_Action):
    """Turn a raw food item in inventory into its cooked form (higher
    satiety), e.g. fish_raw -> fish_cooked (Inventory system). Reasons:
    not_in_inventory / not_cookable."""
    verb: Literal["cook"] = "cook"
    item: str


class MentalNote(BaseModel):
    """D14 — generic, architecture-agnostic mental-state record.
    PRIVATE: never relayed to other agents; visible only to the
    emitting agent + the researcher inspector. Bot author chooses
    cadence (per-decision / per-minute / never).

    Slots are RECOMMENDED, not required. Goal/plan/beliefs/emotion
    are the canonical four; the inspector renders them prominently
    when populated. Other keys are accepted but ignored by the UI.

    Routed through the SDK as a session-meta message, not an
    ActionBatch action (mental_note is a private channel, not a
    world-affecting action). See ``Agent.note()`` for the helper."""
    text: str
    tag: Optional[str] = None
    slots: Optional[dict[str, str]] = None


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


class BuyFood(_Action):
    """Buy a meal: spend food_price gold to cut hunger (Money system).
    The economy's gold sink + survival loop."""
    verb: Literal["buy_food"] = "buy_food"


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


class Forage(_Action):
    """Gather fruit (a food item) from an adjacent tree/bush without
    felling it (Resources system). Renewable: the source ripens again
    after a cooldown. Reasons: not_forageable / not_ripe / target_too_far."""
    verb: Literal["forage"] = "forage"
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
        Step, Speak, Whisper, Shout, LookAt, Interact,
        Pickup, Drop, Eat, Cook, Equip, Give, Attack, Defend, Heal, Wait,
        # Composable-system verbs (session 2).
        Pay, WorkForPay, BuyFood, Trade, Loot,
        Chop, Mine, Forage,
        Enter, Exit, Lock, Unlock, ClaimOwnership, TransferOwnership,
        PlaceBlueprint, AdvanceConstruction, Demolish,
        ProposeTask, AcceptTask, RejectTask, CompleteTask,
    ],
    Field(discriminator="verb"),
]


# === Action batch + result (Phase AGENT-A1) ===
#
# The 4-layer agent brain (see docs/AGENT_ARCHITECTURE_PLAN.md) emits
# 1–3 actions per tactical cycle alongside a free-text `reasoning`
# trace. The engine consumes them serially under per-tick ordering and
# pushes one action_ack per action.


class ActionResult(BaseModel):
    """Result of a single submitted action.

    The engine currently emits `accepted` + `reason` (legacy). The
    architecture plan extends this to a structured triple — `reason_code`
    (enum the harness branches on), `context` (machine-readable details
    like the blocker entity ID), and `human_text` (the LLM-friendly
    explanation). The SDK accepts BOTH shapes: if the engine still
    sends the legacy format, we expose `reason` and leave the new
    fields as None; when the engine ships richer acks the SDK already
    knows how to read them.
    """

    action_id: str
    verb: str
    accepted: bool
    # Legacy field. Kept for backwards-compat.
    reason: Optional[str] = None
    # New, richer fields (populated when engine sends them).
    reason_code: Optional[str] = None
    context: Optional[dict[str, Any]] = None
    human_text: Optional[str] = None


# Reason codes the engine currently emits (and the rich ones it will
# emit once the new ActionResult shape lands engine-side). Harness
# code can branch on these constants rather than substring matching
# free-form strings.
class ReasonCode(str, Enum):
    BLOCKED_BY_ENTITY     = "blocked_by_entity"
    BLOCKED_BY_TERRAIN    = "blocked_by_terrain"
    OUT_OF_RANGE          = "out_of_range"
    PRECONDITION_FAILED   = "precondition_failed"
    INSUFFICIENT_RESOURCE = "insufficient_resource"
    TARGET_GONE           = "target_gone"
    WORLD_RULE_VIOLATED   = "world_rule_violated"
    UNKNOWN_ENTITY        = "unknown_entity"
    UNKNOWN_TARGET        = "unknown_target"
    RATE_LIMITED          = "rate_limited"


class ActionBatch(BaseModel):
    """A receding-horizon batch of 1–3 actions plus a free-text
    reasoning trace, as the tactical brain emits each cycle.

    The engine consumes actions serially; if any fails, the rest are
    dropped — the brain re-plans next cycle. See
    docs/AGENT_ARCHITECTURE_PLAN.md §"Layer 3 / Tactical brain".
    """

    actions: list[Action]
    reasoning: Optional[str] = None  # free-text, layered opt-in for capture
    # Optional per-action interrupts; if interrupt_if fires on a later
    # observation, the queue is dropped and tactical re-runs.
    interrupt_if: Optional[dict[str, Any]] = None
