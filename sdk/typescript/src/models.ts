// Typed observation + action models. Mirrors docs/OBSERVATION_MODEL.md
// and docs/VERB_REFERENCE.md. Compatible with the Python SDK shape.

import { z } from "zod";

export const Pos = z.tuple([z.number().int(), z.number().int()]);
export type Pos = z.infer<typeof Pos>;

export const Facing = z.enum(["N", "S", "E", "W"]);
export type Facing = z.infer<typeof Facing>;

export const VisionMode = z.enum(["structured", "image", "both"]);
export type VisionMode = z.infer<typeof VisionMode>;

export const DayPhase = z.enum([
  "dawn", "morning", "midday", "afternoon", "dusk", "night",
]);

export const SelfState = z.object({
  entity_id: z.string(),
  pos: Pos,
  facing: Facing,
  extras: z.record(z.unknown()).default({}),
  current_action: z.record(z.unknown()).nullable().optional(),
  last_action_result: z.record(z.unknown()).nullable().optional(),
});

export const VisibleEntity = z.object({
  entity_id: z.string(),
  apparent_label: z.string(),
  pos: Pos,
  facing: Facing,
  archetype: z.string(),
  extras_summary: z.record(z.unknown()).default({}),
});

export const VisibleObject = z.object({
  object_id: z.string(),
  kind: z.string(),
  pos: Pos,
  affordances: z.array(z.string()).default([]),
  state_summary: z.record(z.unknown()).default({}),
});

// A pickup-able item entity within vision + line-of-sight. Mirrors the
// Python SDK VisibleItem. `sprite` carries the kind (e.g. "item:apple");
// `quantity` is 1 for non-stackables, higher for stacks like coin piles.
export const VisibleItem = z.object({
  entity_id: z.string(),
  sprite: z.string(),
  pos: Pos,
  quantity: z.number().int().default(1),
  label: z.string().nullable().optional(),
});

export const AudibleEvent = z.object({
  event_id: z.string(),
  kind: z.enum(["speech", "shout", "whisper", "sound"]),
  from_entity: z.string(),
  from_pos: Pos,
  text: z.string().nullable().optional(),
  sound_kind: z.string().nullable().optional(),
  tick: z.number().int(),
});

export const WorldClock = z.object({
  tick: z.number().int(),
  day_phase: DayPhase,
});

// Egocentric ASCII tile-map. rows[0] is the northernmost row; origin is
// the world (x,y) of rows[0][0]. Glyphs: @ you, . walkable, # blocked,
// ~ water, (space) off-map, P person, $ item, + door. Terrain is known to
// `radius`; entities/items only appear where vision reached.
export const LocalView = z.object({
  radius: z.number().int(),
  origin: Pos,
  rows: z.array(z.string()).default([]),
  legend: z.record(z.string()).default({}),
});

export const ViewImage = z.object({
  format: z.enum(["png", "webp"]),
  width: z.number().int(),
  height: z.number().int(),
  data: z.instanceof(Uint8Array),
  centered_on_pos: Pos,
  facing: Facing,
});

export const Observation = z.object({
  obs_id: z.number().int(),
  world_tick: z.number().int(),
  self: SelfState,
  visible_entities: z.array(VisibleEntity).default([]),
  visible_objects: z.array(VisibleObject).default([]),
  visible_items: z.array(VisibleItem).default([]),
  audible: z.array(AudibleEvent).default([]),
  local_view: LocalView.nullable().optional(),
  world_clock: WorldClock,
  view_image: ViewImage.nullable().optional(),
});
export type Observation = z.infer<typeof Observation>;

// === Actions ===

const ActionBase = { priority: z.number().int().default(0) };

// Move exactly ONE tile in a compass direction. The AGENT owns navigation
// (the engine does not pathfind) — the old multi-tile `move` verb was removed.
export const Step = z.object({
  verb: z.literal("step"),
  dir: z.enum(["N", "S", "E", "W"]),
  ...ActionBase,
});

export const Speak = z.object({
  verb: z.literal("speak"),
  text: z.string(),
  ...ActionBase,
});

export const Whisper = z.object({
  verb: z.literal("whisper"),
  target: z.string(),
  text: z.string(),
  ...ActionBase,
});

export const Shout = z.object({
  verb: z.literal("shout"),
  text: z.string(),
  ...ActionBase,
});

export const LookAt = z.object({
  verb: z.literal("look_at"),
  target: z.union([z.string(), Pos]),
  ...ActionBase,
});

export const Interact = z.object({
  verb: z.literal("interact"),
  target: z.string(),
  affordance: z.string(),
  ...ActionBase,
});

export const Pickup = z.object({
  verb: z.literal("pickup"),
  target: z.string(),
  ...ActionBase,
});

export const Drop = z.object({
  verb: z.literal("drop"),
  item: z.string(),
  ...ActionBase,
});

export const Equip = z.object({
  verb: z.literal("equip"),
  item: z.string(),
  slot: z.string().nullable().optional(),
  ...ActionBase,
});

export const Give = z.object({
  verb: z.literal("give"),
  target: z.string(),
  item: z.string(),
  ...ActionBase,
});

export const Attack = z.object({
  verb: z.literal("attack"),
  target: z.string(),
  ...ActionBase,
});

export const Defend = z.object({
  verb: z.literal("defend"),
  ...ActionBase,
});

export const Heal = z.object({
  verb: z.literal("heal"),
  target: z.string().nullable().optional(),
  ...ActionBase,
});

export const Wait = z.object({
  verb: z.literal("wait"),
  ticks: z.number().int().default(60),
  ...ActionBase,
});

export const Eat = z.object({ verb: z.literal("eat"), item: z.string(), ...ActionBase });
export const Cook = z.object({ verb: z.literal("cook"), item: z.string(), ...ActionBase });

// === Composable-system verbs (mirror the Python SDK + engine manifest) ===
export const Pay = z.object({ verb: z.literal("pay"), target: z.string(), amount: z.number().int(), ...ActionBase });
export const WorkForPay = z.object({ verb: z.literal("work_for_pay"), ...ActionBase });
export const BuyFood = z.object({ verb: z.literal("buy_food"), ...ActionBase });
export const Trade = z.object({ verb: z.literal("trade"), target: z.string(), item: z.string(), price: z.number().int(), ...ActionBase });
export const Loot = z.object({ verb: z.literal("loot"), target: z.string(), ...ActionBase });
export const Chop = z.object({ verb: z.literal("chop"), target: z.string(), ...ActionBase });
export const Mine = z.object({ verb: z.literal("mine"), target: z.string(), ...ActionBase });
export const Forage = z.object({ verb: z.literal("forage"), target: z.string(), ...ActionBase });
export const Enter = z.object({ verb: z.literal("enter"), target: z.string(), ...ActionBase });
export const Exit = z.object({ verb: z.literal("exit"), ...ActionBase });
export const Lock = z.object({ verb: z.literal("lock"), target: z.string(), ...ActionBase });
export const Unlock = z.object({ verb: z.literal("unlock"), target: z.string(), ...ActionBase });
export const ClaimOwnership = z.object({ verb: z.literal("claim_ownership"), target: z.string(), ...ActionBase });
export const TransferOwnership = z.object({ verb: z.literal("transfer_ownership"), target: z.string(), new_owner: z.string(), ...ActionBase });
export const PlaceBlueprint = z.object({ verb: z.literal("place_blueprint"), kind: z.string(), at: Pos, ...ActionBase });
export const AdvanceConstruction = z.object({ verb: z.literal("advance_construction"), target: z.string(), ...ActionBase });
export const Demolish = z.object({ verb: z.literal("demolish"), target: z.string(), ...ActionBase });
export const ProposeTask = z.object({ verb: z.literal("propose_task"), target: z.string(), terms: z.string(), reward: z.string().nullable().optional(), ...ActionBase });
export const AcceptTask = z.object({ verb: z.literal("accept_task"), id: z.string(), ...ActionBase });
export const RejectTask = z.object({ verb: z.literal("reject_task"), id: z.string(), ...ActionBase });
export const CompleteTask = z.object({ verb: z.literal("complete_task"), id: z.string(), ...ActionBase });

export const Action = z.discriminatedUnion("verb", [
  // Base verbs.
  Step, Speak, Whisper, Shout, LookAt, Interact,
  Pickup, Drop, Equip, Give, Attack, Defend, Heal, Wait, Eat, Cook,
  // Composable-system verbs.
  Pay, WorkForPay, BuyFood, Trade, Loot, Chop, Mine, Forage,
  Enter, Exit, Lock, Unlock, ClaimOwnership, TransferOwnership,
  PlaceBlueprint, AdvanceConstruction, Demolish,
  ProposeTask, AcceptTask, RejectTask, CompleteTask,
]);
export type Action = z.infer<typeof Action>;
