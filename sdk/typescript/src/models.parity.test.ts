import { describe, it, expect } from "vitest";
import { Observation } from "./models";

// A representative observation frame matching the engine's WS wire shape
// (captured live from Eldoria; see docs/ENVIRONMENT_AUDIT_PLAN.md S7). This
// guards SDK<->engine parity: the TS Observation must parse exactly what the
// engine sends, including visible_items (which was previously missing) and
// WITHOUT the removed known_map_summary / recent_self_results / weather / doing.
const LIVE_FRAME = {
  obs_id: 1780946790179,
  world_tick: 30421,
  self: {
    entity_id: "spawn_14",
    pos: [769, 862],
    facing: "S",
    extras: { hp: 100, max_hp: 100, gold: 25, hunger: 0.04, inventory: [], equipped: {}, contracts: [], defending: false, reputation: 0 },
  },
  visible_entities: [
    { entity_id: "spawn_9", apparent_label: "wanderer", pos: [764, 863], facing: "S", archetype: "wanderer", extras_summary: { hp_bucket: "full" } },
  ],
  visible_objects: [
    { object_id: "door:bld:000:767,867", kind: "door", pos: [767, 867], affordances: ["enter"], state_summary: { building_sprite: "bld:000" } },
  ],
  visible_items: [
    { entity_id: "item_221", sprite: "item:coins_large_pile", pos: [763, 854], quantity: 43, label: "coins_large_pile" },
    { entity_id: "spawn_3", sprite: "item:sword_short", pos: [764, 852], quantity: 1, label: "sword_short" },
  ],
  audible: [],
  local_view: { radius: 20, origin: [749, 842], rows: ["....."], legend: { "@": "you" } },
  world_clock: { tick: 30421, day_phase: "afternoon" },
  view_image: null,
};

describe("Observation wire parity", () => {
  it("parses a live engine frame", () => {
    const o = Observation.parse(LIVE_FRAME);
    expect(o.obs_id).toBe(1780946790179);
    expect(o.world_clock.day_phase).toBe("afternoon");
  });

  it("carries visible_items (regression: was missing from TS SDK)", () => {
    const o = Observation.parse(LIVE_FRAME);
    expect(o.visible_items).toHaveLength(2);
    expect(o.visible_items[0].sprite).toBe("item:coins_large_pile");
    expect(o.visible_items[0].quantity).toBe(43);
  });

  it("defaults visible_items to [] when absent", () => {
    const f: Record<string, unknown> = { ...LIVE_FRAME };
    delete f.visible_items;
    const o = Observation.parse(f);
    expect(o.visible_items).toEqual([]);
  });

  it("has no removed fields on the parsed type", () => {
    const o = Observation.parse(LIVE_FRAME) as Record<string, unknown>;
    // zod strips unknown keys; these must not be part of the schema output.
    expect("known_map_summary" in o).toBe(false);
    expect("recent_self_results" in o).toBe(false);
    expect("weather" in (o.world_clock as Record<string, unknown>)).toBe(false);
  });
});
