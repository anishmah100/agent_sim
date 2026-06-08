// Entity render layer.
//
// Renders characters / objects as placeholder colored quads with a
// tiny name label until real spritesheets land. The shape is correct
// (16×24, bottom-center anchored, facing-aware) so swapping in
// AnimatedSprite from a real atlas is a single-method change later.

import { AnimatedSprite, Assets, Container, Graphics, Sprite, Text, Texture } from "pixi.js";
import { OutlineFilter } from "pixi-filters";
import { TILE_SIZE_PX } from "./tiles";
import type { CharacterAtlas, CharacterAnim } from "./CharacterAtlas";
import { artCatalog } from "./ArtCatalog";

// Hover outline filter — applied to the BODY sprite only (not the
// wrap container). Applying it to the wrap pulls the label texture
// bounds into the filter, which produced large floating rectangles.
const HOVER_OUTLINE = new OutlineFilter({
  thickness: 1.5,
  color: 0xfff2a8,
  alpha: 0.85,
  knockout: false,
});

export interface EntityState {
  entity_id: string;
  archetype: string;
  pos: [number, number];
  facing: "N" | "S" | "E" | "W";
  display_name?: string;
  current_action?: "attack" | "interact" | "hit" | null;
  /** When set, the entity is inside a building's interior and should
   *  NOT render on the overworld. The value is the building sprite ID
   *  the entity is currently inside. */
  inside_building?: string;
  /** Public extras the engine elects to expose (progress, kind, hp,
   *  gold, etc.). See engine/internal/world/world.go publicExtraKeys
   *  for the whitelist. Private extras (inventory, contracts) never
   *  arrive here. */
  extras?: Record<string, unknown>;
}

// Map a blueprint's 0..100 progress to the discrete stage index used
// by worldObjectSpriteUrl(). Exported as a free function so update()
// can detect cross-stage transitions cheaply.
function blueprintStage(e: EntityState): number {
  if (e.archetype !== "blueprint") return -1;
  const progress = Number(e.extras?.["progress"] ?? 0);
  // Cottage = 4 build steps → 0/25/50/75/100. Map onto the 6 stage
  // sprites so a freshly placed blueprint shows the ghost outline and
  // advance_construction visibly steps through walls → roof.
  return progress >= 100 ? 5
    : progress >= 75 ? 4
    : progress >= 50 ? 3
    : progress >= 25 ? 2
    : progress > 0   ? 1
    : 0;
}

// Footprint = 16x16 (one tile). Sprite container is anchored at top-left
// of the footprint; the sprite child is centered horizontally and
// bottom-aligned to the footprint bottom row (which corresponds to the
// feet pixel). This lets characters of any sprite height (12 or 24 px)
// render correctly without per-character math.
const FOOTPRINT_W = TILE_SIZE_PX;
const FOOTPRINT_H = TILE_SIZE_PX;

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

// Closed set of world-object archetypes. Mirrors the engine taxonomy
// in internal/core/systems/archetypes.go (these are entities that
// EXIST for the engine's systems to target — Resources targets trees
// and rocks, Construction targets blueprints, etc. — but they're not
// agent bodies).
const WORLD_OBJECT_ARCHETYPES = new Set([
  "tree", "rock", "item", "blueprint",
]);

// Map a world-object entity to a sprite id, then resolve via the art
// catalog. The mapping reads entity_id when an archetype has subtypes
// (tree_oak_1 → tree_oak, rock_iron_1 → boulder_iron_ore).
function worldObjectSpriteId(e: EntityState): string {
  const id = e.entity_id;
  switch (e.archetype) {
    case "tree": {
      const subtype = id.replace(/^tree_/, "").replace(/_\d+$/, "");
      const name = subtype === "" ? "tree_oak" : `tree_${subtype}`;
      return `veg:${name}`;
    }
    case "rock":
      return id.includes("iron") ? "veg:boulder_iron_ore" : "veg:boulder_medium";
    case "blueprint": {
      const stage = blueprintStage(e);
      const stageNames = [
        "cottage_stage_0_blueprint", "cottage_stage_1_foundation",
        "cottage_stage_2_walls_partial", "cottage_stage_3_walls_full",
        "cottage_stage_4_roof_partial", "cottage_stage_5_finished",
      ];
      return `stage:${stageNames[stage]}`;
    }
    case "item": {
      // D8 — read sprite from the entity's public extras. The engine
      // sets entity.Extras["sprite"] = "item:<kind>" at spawn (see
      // promote_scattered_items_to_entities.py + handleDrop). Falls
      // back to wood_log only if extras are completely missing.
      const sprite = (e as any).extras?.sprite as string | undefined;
      if (sprite) return sprite;
      return "item:wood_log";
    }
  }
  return "veg:tree_oak";
}

function worldObjectSpriteUrl(e: EntityState): string {
  const id = worldObjectSpriteId(e);
  const cat = artCatalog();
  const url = cat?.url(id);
  if (url) return url;
  // Legacy fallback while migration finishes. Mirrors the old direct
  // path templates for ids the catalog doesn't yet cover.
  if (id.startsWith("veg:")) {
    return `${ENGINE_URL}/art/processed/v2_resources_world_master/${id.slice(4)}.png`;
  }
  if (id.startsWith("stage:")) {
    return `${ENGINE_URL}/art/processed/v2_construction_stages/${id.slice(6)}.png`;
  }
  if (id.startsWith("item:")) {
    return `${ENGINE_URL}/art/processed/v2_items_master_v2/${id.slice(5)}.png`;
  }
  return `${ENGINE_URL}/art/processed/v2_resources_world_master/tree_oak.png`;
}

// Map engine archetype string → character_id used by the atlas. As we
// add more characters this grows. v0 starting set rotates the cast so
// the placeholder NPCs become visibly different sprites.
const CHARACTER_ROTATION = [
  "trainer_red", "trainer_lyra_blue", "wizard", "baker",
  "iron_guard", "child", "cloaked_wanderer",
];

function pickCharacterId(state: EntityState): string {
  // If the engine sent a known character archetype, use it directly.
  if (CHARACTER_ROTATION.includes(state.archetype)) return state.archetype;
  // Otherwise hash on entity_id for stable assignment.
  let h = 0;
  for (const c of state.entity_id) h = (h * 31 + c.charCodeAt(0)) | 0;
  const idx = Math.abs(h) % CHARACTER_ROTATION.length;
  return CHARACTER_ROTATION[idx];
}

interface RenderedEntity {
  state: EntityState;
  characterId: string;
  container: Container;
  body: Sprite | AnimatedSprite | Graphics;
  facingMark: Graphics | null;
  label: Text;
  hpBar: Graphics | null;          // floating HP bar; shown only when hurt
  prevPos: [number, number];
  // Interpolation target in PIXELS. tick() eases container.x/y toward this
  // each frame so motion stays smooth even when WS snapshots arrive late or
  // get skipped (which otherwise snapped the sprite across the gap — the
  // "character jumps many tiles" teleport). A jump bigger than SNAP_PX
  // (a respawn / cross-map move) snaps instantly instead of gliding.
  targetX: number;
  targetY: number;
  interp: boolean;                 // true for agents (smooth); false for static items
  movingSince: number;             // ms — for idle detection
  hitFlashUntil: number;           // BLK-1: ms timestamp; red tint while > now
  flinchStart: number;             // ms timestamp of last hit, for flinch nudge
  flinchDir: { x: number; y: number }; // unit kick-back direction
  bodyHomeX: number;               // body sprite's resting x (flinch returns here)
  bodyHomeY: number;               // body sprite's resting y
  prevHp: number | null;           // BLK-1: last seen hp, to detect damage
}

// BLK-1: a removed agent that's fading out (death). Kept rendering for
// FADE_MS so a kill reads as a body falling + dissolving instead of the
// character popping out of existence.
interface DyingEntity {
  re: RenderedEntity;
  start: number;                   // performance.now() when death detected
  baseY: number;                   // container.y at moment of death (slump anchor)
}
const DEATH_FADE_MS = 1400;
const HIT_FLASH_MS = 180;
// How long the flinch nudge lasts. The body kicks back a couple px on a
// hit, then springs home — a small but legible "that hurt" read.
const FLINCH_MS = 170;
const FLINCH_PX = 2.2;
// Position interpolation: ease sprites toward the latest engine position so
// motion stays smooth across slow/skipped WS snapshots (instead of snapping
// the sprite — the multi-tile "teleport"). INTERP_MS is the catch-up window;
// a per-axis jump beyond SNAP_PX is a genuine teleport (respawn / relocate)
// and snaps instantly rather than gliding a body across the map.
const INTERP_MS = 90;
const SNAP_PX = TILE_SIZE_PX * 4;

// BLK-1: read an entity's hp from its public extras, or null if it has
// none (world objects / items). Used to detect damage between snapshots.
function hpOf(e: EntityState): number | null {
  const v = e.extras?.["hp"];
  return typeof v === "number" ? v : null;
}
function maxHpOf(e: EntityState): number | null {
  const v = e.extras?.["max_hp"];
  return typeof v === "number" ? v : null;
}

// Draw a compact RPG-style HP bar (dark frame + green→amber→red fill)
// centered over the footprint. Width scales with the entity tile.
const HP_BAR_W = 14;
const HP_BAR_H = 3;
function drawHpBar(g: Graphics, hp: number, max: number): void {
  g.clear();
  const frac = Math.max(0, Math.min(1, hp / max));
  const x = (FOOTPRINT_W - HP_BAR_W) / 2;
  // color ramps green → amber → red as HP falls (palette-aligned).
  const color = frac > 0.6 ? 0x5ee89a : frac > 0.3 ? 0xfeae34 : 0xe43b44;
  const r = HP_BAR_H / 2;                  // pill radius
  // soft drop shadow under the whole pill for separation from the sprite.
  g.roundRect(x - 1, 1, HP_BAR_W + 2, HP_BAR_H + 2, r + 1)
    .fill({ color: 0x000000, alpha: 0.35 });
  // dark rounded frame.
  g.roundRect(x - 1, -1, HP_BAR_W + 2, HP_BAR_H + 2, r + 1)
    .fill({ color: 0x181425, alpha: 0.92 });
  // empty track.
  g.roundRect(x, 0, HP_BAR_W, HP_BAR_H, r)
    .fill({ color: 0x3a2233, alpha: 0.95 });
  // fill (rounded; clamp width so a tiny sliver still shows as a pill).
  if (frac > 0) {
    const fw = Math.max(HP_BAR_H, HP_BAR_W * frac);
    g.roundRect(x, 0, fw, HP_BAR_H, r).fill({ color, alpha: 1 });
    // top highlight line — subtle glossy sheen.
    g.rect(x + 0.5, 0.4, Math.max(0, fw - 1), 0.7).fill({ color: 0xffffff, alpha: 0.22 });
  }
}

export interface ItemHoverEvent {
  /** Entity id of the hovered item. */
  entity_id: string;
  /** Sprite (e.g. "item:apple") from entity.extras.sprite, or
   *  "item:unknown" if missing. */
  sprite: string;
  /** Tile position. */
  pos: [number, number];
}

/** Pointer-enter on a non-item / non-world-object entity (i.e. an
 *  agent or character). Carries screen coords from the originating
 *  pointer event so the App layer can position a floating hover-card
 *  next to the cursor. */
export interface AgentHoverEvent {
  entity_id: string;
  archetype: string;
  display_name?: string;
  /** Window-space coords of the pointer at the time of the event. */
  screen_x: number;
  screen_y: number;
}

export class EntityLayer {
  readonly container: Container;
  /** FX hook: called with (tile, amount) when an entity's hp drops, so
   *  the FxLayer can float a damage number. Wired by PixiApp. */
  onDamage?: (tile: [number, number], amount: number) => void;
  private items = new Map<string, RenderedEntity>();
  private dying: DyingEntity[] = [];   // BLK-1: agents fading out on death
  private selectionRing: Graphics;
  private selectedId: string | null = null;
  private pulsePhase = 0;
  private atlas: CharacterAtlas | null = null;
  private itemHoverEnterHandlers: Array<(ev: ItemHoverEvent) => void> = [];
  private itemHoverExitHandlers: Array<(ev: ItemHoverEvent) => void> = [];
  private agentHoverEnterHandlers: Array<(ev: AgentHoverEvent) => void> = [];
  private agentHoverExitHandlers: Array<(ev: AgentHoverEvent) => void> = [];

  /** Subscribe to pointer-enter on an item-archetype entity. Used by
   *  the App layer to drive the InfoPanel (D8 + D17). */
  onItemHoverEnter(h: (ev: ItemHoverEvent) => void): () => void {
    this.itemHoverEnterHandlers.push(h);
    return () => {
      const i = this.itemHoverEnterHandlers.indexOf(h);
      if (i >= 0) this.itemHoverEnterHandlers.splice(i, 1);
    };
  }

  /** Subscribe to pointer-exit on an item-archetype entity. */
  onItemHoverExit(h: (ev: ItemHoverEvent) => void): () => void {
    this.itemHoverExitHandlers.push(h);
    return () => {
      const i = this.itemHoverExitHandlers.indexOf(h);
      if (i >= 0) this.itemHoverExitHandlers.splice(i, 1);
    };
  }

  /** Subscribe to pointer-enter on an agent / character entity (i.e.
   *  NOT items, trees, rocks, blueprints). Drives the floating
   *  hover-card preview in App (D17 task 6.2). */
  onAgentHoverEnter(h: (ev: AgentHoverEvent) => void): () => void {
    this.agentHoverEnterHandlers.push(h);
    return () => {
      const i = this.agentHoverEnterHandlers.indexOf(h);
      if (i >= 0) this.agentHoverEnterHandlers.splice(i, 1);
    };
  }

  /** Subscribe to pointer-exit on an agent / character entity. */
  onAgentHoverExit(h: (ev: AgentHoverEvent) => void): () => void {
    this.agentHoverExitHandlers.push(h);
    return () => {
      const i = this.agentHoverExitHandlers.indexOf(h);
      if (i >= 0) this.agentHoverExitHandlers.splice(i, 1);
    };
  }

  constructor() {
    this.container = new Container();
    this.container.label = "entities";
    this.container.sortableChildren = true;

    // Selection ring lives on its own. We re-position + redraw each
    // frame in tick(); created once to avoid Graphics churn.
    this.selectionRing = new Graphics();
    this.selectionRing.visible = false;
    this.selectionRing.zIndex = -1;            // under all entity sprites
    this.container.addChild(this.selectionRing);
  }

  /** Inject the character atlas once it's loaded. Existing rendered
   *  entities are torn down and rebuilt — necessary because the old
   *  placeholder body is a Graphics, but the new body is an
   *  AnimatedSprite. update() can't swap between these shapes. */
  setAtlas(atlas: CharacterAtlas | null): void {
    this.atlas = atlas;
    if (this.items.size === 0) return;
    const states = this.getAll();
    for (const re of this.items.values()) {
      re.container.destroy({ children: true });
    }
    this.items.clear();
    for (const s of states) {
      this.items.set(s.entity_id, this.create(s));
    }
  }

  getAll(): EntityState[] {
    return Array.from(this.items.values()).map((re) => ({ ...re.state }));
  }

  // Debug bridge: report whether the entity's sprite container is
  // currently visible. Used by the building visual probe to assert the
  // "hide while inside_building" rule. Returns null if the entity has
  // not been tracked yet.
  spriteVisible(id: string): boolean | null {
    const re = this.items.get(id);
    if (!re) return null;
    return re.container.visible !== false;
  }

  /** Society-Pulse bridge: world-space center point of a live entity's
   *  body, or null if it isn't tracked / is hidden inside a building.
   *  Used by RelationshipOverlay to anchor relationship lines. The point
   *  is the mid-body (footprint center), matching where the eye reads a
   *  character's "location". */
  posOf(id: string): { x: number; y: number } | null {
    const re = this.items.get(id);
    if (!re) return null;
    if (re.container.visible === false) return null; // inside building / hidden
    return {
      x: re.container.x + FOOTPRINT_W / 2,
      y: re.container.y + FOOTPRINT_H / 2,
    };
  }

  setSelected(id: string | null): void {
    this.selectedId = id;
    if (id === null) {
      this.selectionRing.visible = false;
    }
  }

  /** Per-frame update — drives the selection ring's position + pulse.
   *  Called from PixiApp's ticker. */
  tick(deltaMs: number): void {
    this.pulsePhase = (this.pulsePhase + deltaMs / 230) % (Math.PI * 2);

    // BLK-1: damage flash + flinch — bodies whose hp just dropped flash
    // bright red, then quickly cool to white; simultaneously the body
    // kicks back a couple px and springs home so the hit visibly reads.
    const now = performance.now();
    for (const re of this.items.values()) {
      const b = re.body as unknown as { tint?: number };
      // Flash: bright red at impact, easing back to white over the flash
      // window — a single-frame full-red looked harsh / strobed.
      if (b.tint !== undefined) {
        if (re.hitFlashUntil > now) {
          const f = (re.hitFlashUntil - now) / HIT_FLASH_MS; // 1 → 0
          b.tint = lerpTint(0xffffff, 0xff3b3b, f);
        } else {
          b.tint = 0xffffff;
        }
      }
      // Flinch nudge: a damped spring-out-and-back on the body sprite.
      const ft = (now - re.flinchStart) / FLINCH_MS;
      if (re.flinchStart > 0 && ft <= 1) {
        // sin curve: 0 → peak at ft=0.5 → 0. Eases out + back.
        const k = Math.sin(ft * Math.PI) * FLINCH_PX;
        re.body.x = re.bodyHomeX + re.flinchDir.x * k;
        re.body.y = re.bodyHomeY + re.flinchDir.y * k;
      } else if (re.flinchStart > 0) {
        re.body.x = re.bodyHomeX;
        re.body.y = re.bodyHomeY;
        re.flinchStart = 0;                  // settle; stop touching it
      }
      // Smooth position interpolation toward the latest engine position.
      // Easing across frames hides slow/skipped WS snapshots that would
      // otherwise snap the sprite (the multi-tile "teleport"). A jump
      // larger than SNAP_PX (respawn / cross-map) is a real teleport — snap
      // it instantly rather than gliding a body across the whole map.
      if (re.interp) {
        const dx = re.targetX - re.container.x;
        const dy = re.targetY - re.container.y;
        if (dx !== 0 || dy !== 0) {
          if (Math.abs(dx) > SNAP_PX || Math.abs(dy) > SNAP_PX) {
            re.container.x = re.targetX;
            re.container.y = re.targetY;
          } else {
            const k = Math.min(1, (deltaMs || 16) / INTERP_MS);
            re.container.x += dx * k;
            re.container.y += dy * k;
            if (Math.abs(re.targetX - re.container.x) < 0.5) re.container.x = re.targetX;
            if (Math.abs(re.targetY - re.container.y) < 0.5) re.container.y = re.targetY;
          }
          re.container.zIndex = re.container.y + FOOTPRINT_H;
        }
      }
    }
    // BLK-1: death dissolve — a readable kill beat: brief white flash →
    // grayscale → slump downward + fade out. The body doesn't just pop.
    if (this.dying.length > 0) {
      for (let i = this.dying.length - 1; i >= 0; i--) {
        const d = this.dying[i];
        const t01 = Math.min(1, (now - d.start) / DEATH_FADE_MS);
        const b = d.re.body as unknown as { tint?: number };
        if (t01 < 0.12) {
          // opening white flash — "the killing blow lands".
          d.re.container.alpha = 1;
          if (b.tint !== undefined) b.tint = 0xffffff;
        } else {
          const dt = (t01 - 0.12) / 0.88;     // 0 → 1 over the dissolve
          // grayscale-ish fade by tinting toward a cold gray, then fading.
          if (b.tint !== undefined) b.tint = lerpTint(0xffffff, 0x5a5a6e, Math.min(1, dt * 1.5));
          d.re.container.alpha = 1 - dt;
          // slump: ease the body downward a few px as it falls.
          d.re.container.y = d.baseY + easeOutCubic(dt) * 3;
        }
        if (t01 >= 1) {
          d.re.container.destroy({ children: true });
          this.dying.splice(i, 1);
        }
      }
    }

    if (this.selectedId === null) return;
    const re = this.items.get(this.selectedId);
    if (!re) {
      this.selectionRing.visible = false;
      return;
    }
    // Foot position = center-bottom of the 16x16 tile footprint.
    const cx = re.container.x + FOOTPRINT_W / 2;
    const cy = re.container.y + FOOTPRINT_H - 1;
    const pulse = 0.7 + 0.3 * Math.sin(this.pulsePhase);
    const rx = FOOTPRINT_W * 0.42;
    const ry = FOOTPRINT_H * 0.18;
    this.selectionRing.clear();
    // Dark frame.
    this.selectionRing.ellipse(cx, cy, rx + 1.2, ry + 1.2)
      .stroke({ color: 0x181425, width: 1.2, alpha: 0.7 });
    // Bright gold ring.
    this.selectionRing.ellipse(cx, cy, rx, ry)
      .stroke({ color: 0xfee761, width: 0.8, alpha: 0.95 * pulse });
    // Soft outer halo for visibility against any tile.
    this.selectionRing.ellipse(cx, cy, rx + 0.8, ry + 0.8)
      .stroke({ color: 0xffd24a, width: 1.5, alpha: 0.3 * pulse });
    this.selectionRing.zIndex = re.container.zIndex - 0.5;
    this.selectionRing.visible = true;
  }

  setAll(entities: EntityState[]): void {
    // Render every entity (characters + world objects). Characters
    // use animated character sprites; world-object archetypes get a
    // static sprite from the v2 master sheets via worldObjectSprite().
    const incoming = new Set(entities.map((e) => e.entity_id));
    // Remove anything that disappeared.
    for (const [id, re] of this.items) {
      if (!incoming.has(id)) {
        // BLK-1: a vanished CHARACTER (has hp + wasn't inside a building)
        // most likely just died — fade it out as a falling body instead
        // of popping it out instantly. Items (no hp) and agents that
        // merely entered a building are destroyed immediately as before.
        const wasChar = re.state.extras != null &&
          re.state.extras["hp"] !== undefined &&
          !re.state.inside_building;
        if (wasChar) {
          re.container.visible = true;
          this.dying.push({ re, start: performance.now(), baseY: re.container.y });
        } else {
          re.container.destroy({ children: true });
        }
        this.items.delete(id);
        if (this.selectedId === id) this.setSelected(null);
      }
    }
    // Add or update incoming.
    for (const e of entities) {
      const existing = this.items.get(e.entity_id);
      if (existing) {
        this.update(existing, e);
      } else {
        this.items.set(e.entity_id, this.create(e));
      }
      // Hide entities currently inside a building. Selection ring
      // tick will skip them via the visible flag.
      const r = this.items.get(e.entity_id);
      if (r) r.container.visible = !e.inside_building;
    }
  }

  private create(e: EntityState): RenderedEntity {
    const wrap = new Container();
    wrap.label = `entity:${e.entity_id}`;

    // World-object archetypes (trees / rocks / items / blueprints) get
    // a static sprite from the v2 master sheets. They're entities so
    // the engine can target them by ID; they render as their proper
    // sprite so users can SEE them.
    if (WORLD_OBJECT_ARCHETYPES.has(e.archetype)) {
      return this.createWorldObject(e, wrap);
    }

    const characterId = pickCharacterId(e);
    let body: Sprite | AnimatedSprite | Graphics;
    let facingMark: Graphics | null = null;

    const spec = this.atlas?.get(characterId);
    // Drop shadow under the character — classic JRPG polish detail. A
    // small dark ellipse at the feet grounds the character so they
    // don't look like they're floating above the tiles.
    if (spec) {
      const shadow = new Graphics();
      shadow.ellipse(FOOTPRINT_W / 2, FOOTPRINT_H - 2, 5, 1.6).fill({ color: 0x000000, alpha: 0.32 });
      wrap.addChild(shadow);
    }
    if (spec) {
      const sprite = new AnimatedSprite(spec.anims.walk_down);
      sprite.animationSpeed = 0.13;
      sprite.loop = true;
      // Bottom-center anchor: every frame was tight-cropped, bottom
      // pixel = foot.
      sprite.anchor.set(0.5, 1);
      // HeartGold standard: character is 1.5 tiles tall (24 world px
      // on our 16 px tiles), centered horizontally on the footprint,
      // foot pixel at the bottom row of the footprint tile (y = 15
      // is the last pixel of a 16 px tile, not y = 16 which is the
      // next tile).
      const TARGET_DISPLAY_HEIGHT = 24;
      const scale = TARGET_DISPLAY_HEIGHT / spec.ref_frame_h;
      sprite.scale.set(scale);
      sprite.x = FOOTPRINT_W / 2;
      sprite.y = FOOTPRINT_H - 1;
      sprite.texture.source.scaleMode = "nearest";
      sprite.stop();
      body = sprite;
    } else {
      const g = new Graphics();
      drawPlaceholderBody(g);
      facingMark = new Graphics();
      drawFacingMark(facingMark, e.facing);
      body = g;
    }

    // Labels render at HIGH resolution then scale DOWN. Renders the
    // text at e.g. 32 px font size into a texture, then we scale the
    // sprite to fit. Avoids the blurry small-font-size problem because
    // PixiJS rasterizes Text once at the configured fontSize, and
    // scaling down with NEAREST keeps the rasterization crisp.
    const label = new Text({
      text: e.display_name ?? e.entity_id,
      style: {
        fontFamily: "ui-sans-serif, system-ui, sans-serif",
        fontSize: 14,                       // raster at clean modern size
        fontWeight: "600",
        fill: 0xfee761,                     // Endesga warm yellow
        stroke: { color: 0x181425, width: 3 },
        align: "center",
      },
      resolution: 2,                        // 2× crispness
    });
    // Scale-down so it fits over the character at world scale (~tile px).
    label.scale.set(0.4);
    label.anchor.set(0.5, 1);
    label.x = FOOTPRINT_W / 2;
    if (spec) {
      const TARGET_DISPLAY_HEIGHT = 24;
      const headY = (FOOTPRINT_H - 1) - TARGET_DISPLAY_HEIGHT;
      label.y = headY - 2;
    } else {
      label.y = -10;
    }

    wrap.addChild(body);
    if (facingMark) wrap.addChild(facingMark);
    wrap.addChild(label);

    // Floating HP bar — sits just above the name label, hidden until the
    // entity has actually taken damage (full-HP agents stay uncluttered).
    const hpBar = new Graphics();
    hpBar.y = (spec ? ((FOOTPRINT_H - 1) - 24) - 6 : -16);
    hpBar.visible = false;
    wrap.addChild(hpBar);
    {
      const hp = hpOf(e), mx = maxHpOf(e);
      if (hp !== null && mx !== null && hp < mx) {
        drawHpBar(hpBar, hp, mx);
        hpBar.visible = true;
      }
    }

    this.applyPos(wrap, e.pos, e.archetype);
    this.container.addChild(wrap);

    // Hover outline on the BODY sprite only. Clicks still flow through
    // input.ts's viewport-level hit-test; the eventMode on `wrap` is
    // just so pointerover/pointerout fire reliably for the affordance.
    if (e.archetype !== "item" && e.archetype !== "decoration") {
      wrap.eventMode = "static";
      wrap.cursor = "pointer";
      // Capture entity identity at create-time so the closures don't
      // rely on later mutations of `e` (the engine sends a fresh
      // EntityState every tick; `re.state` is what we keep).
      const agentEv = (screenX: number, screenY: number): AgentHoverEvent => ({
        entity_id: e.entity_id,
        archetype: e.archetype,
        display_name: e.display_name,
        screen_x: screenX,
        screen_y: screenY,
      });
      wrap.on("pointerover", (ev) => {
        body.filters = [HOVER_OUTLINE];
        // Pixi8 FederatedPointerEvent — global is page-space.
        const g = (ev as { global?: { x: number; y: number } }).global;
        const sx = g?.x ?? 0;
        const sy = g?.y ?? 0;
        for (const h of this.agentHoverEnterHandlers) h(agentEv(sx, sy));
      });
      wrap.on("pointerout", (ev) => {
        body.filters = [];
        const g = (ev as { global?: { x: number; y: number } }).global;
        const sx = g?.x ?? 0;
        const sy = g?.y ?? 0;
        for (const h of this.agentHoverExitHandlers) h(agentEv(sx, sy));
      });
    }

    return {
      state: { ...e },
      characterId,
      container: wrap,
      body,
      facingMark,
      label,
      hpBar,
      prevPos: [e.pos[0], e.pos[1]],
      targetX: wrap.x,
      targetY: wrap.y,
      interp: true,
      movingSince: performance.now(),
      hitFlashUntil: 0,
      flinchStart: 0,
      flinchDir: { x: 0, y: 1 },
      bodyHomeX: body.x,
      bodyHomeY: body.y,
      prevHp: hpOf(e),
    };
  }

  private createWorldObject(e: EntityState, wrap: Container): RenderedEntity {
    // Static sprite from the master sheets. Loaded async; until then,
    // a small placeholder so the entity still occupies space.
    const placeholder = new Graphics()
      .rect(2, 2, FOOTPRINT_W - 4, FOOTPRINT_H - 4)
      .fill({ color: 0x265c42, alpha: 0.6 });
    wrap.addChild(placeholder);

    const url = worldObjectSpriteUrl(e);
    // Swap the dark placeholder rect for a real sprite once the texture
    // loads. Factored so the error path can retry with a fallback.
    const place = (tex: Texture) => {
      tex.source.scaleMode = "nearest";
      const sp = new Sprite(tex);
      // Match the decoration-tree visual scale (~3 tiles tall) so v2
      // tree entities don't look like saplings next to the v1
      // decoration trees. Saplings stay 1 tile; rocks are tile-sized.
      const isSapling = /sapling/i.test(e.entity_id);
      const targetHeightTiles =
        e.archetype === "tree" ? (isSapling ? 1 : 3)
        : e.archetype === "blueprint" ? 3
        : 1.2;   // rocks slightly bigger than 1 tile so they read
      const targetH = targetHeightTiles * TILE_SIZE_PX;
      const aspect = tex.width / tex.height;
      const targetW = targetH * aspect;
      sp.width = targetW;
      sp.height = targetH;
      sp.x = (FOOTPRINT_W - targetW) / 2;
      sp.y = FOOTPRINT_H - targetH;
      if (placeholder.parent) wrap.removeChild(placeholder);
      placeholder.destroy();
      wrap.addChild(sp);
    };
    void Assets.load<Texture>(url).then(place).catch(() => {
      // The sprite 404'd (an item kind with no art file). Don't leave the
      // dark-green placeholder rect forever (the "item renders as a dark
      // square" bug) — fall back to a known-good generic item sprite, and
      // if THAT fails too, recolor the placeholder to a small neutral
      // item dot so it never reads as a broken black box.
      const fallback = `${ENGINE_URL}/art/processed/v2_items_master_v2/wood_log.png`;
      Assets.load<Texture>(fallback).then(place).catch((err) => {
        console.warn(`world-object sprite + fallback failed for ${e.entity_id}:`, err);
        placeholder.clear();
        placeholder.circle(FOOTPRINT_W / 2, FOOTPRINT_H / 2, 3)
          .fill({ color: 0xc9a227, alpha: 0.9 });  // small gold "item" dot
      });
    });

    // Subtle drop-shadow ellipse at the base, like characters.
    const shadow = new Graphics();
    shadow.ellipse(FOOTPRINT_W / 2, FOOTPRINT_H - 2, 5, 1.6)
      .fill({ color: 0x000000, alpha: 0.28 });
    wrap.addChildAt(shadow, 0);

    // Items: hover outline + emit hover events for the InfoPanel.
    // The InfoPanel describes them via SpriteInfo.describeSprite(sprite),
    // same path used by buildings/wells/stalls. Clicks on items are
    // intentionally NOT forwarded to the Inspector — items aren't
    // agents, they don't have Mind/Speech/Trace, and a click-to-open
    // inspector was confusing per user feedback during P2 build.
    if (e.archetype === "item") {
      wrap.eventMode = "static";
      wrap.cursor = "help";
      const sprite = worldObjectSpriteId(e);
      const evShape: ItemHoverEvent = {
        entity_id: e.entity_id,
        sprite,
        pos: [e.pos[0], e.pos[1]],
      };
      wrap.on("pointerover", () => {
        wrap.filters = [HOVER_OUTLINE];
        for (const h of this.itemHoverEnterHandlers) h(evShape);
      });
      wrap.on("pointerout", () => {
        wrap.filters = [];
        for (const h of this.itemHoverExitHandlers) h(evShape);
      });
      // Block click bubble — input.ts's viewport-level hit-test
      // turns canvas clicks into an entity click, which App opens
      // the inspector for. Item entities should NOT open the
      // inspector; stop the click on the sprite so it never reaches
      // the viewport listener.
      wrap.on("pointertap", (ev) => { ev.stopPropagation(); });
    }

    this.applyPos(wrap, e.pos, e.archetype);
    this.container.addChild(wrap);

    return {
      state: { ...e },
      characterId: e.archetype,    // placeholder — unused for world objects
      container: wrap,
      body: placeholder,           // initial body (replaced on tex load)
      facingMark: null,
      label: null as unknown as Text,  // no label for world objects
      hpBar: null,
      prevPos: [e.pos[0], e.pos[1]],
      targetX: wrap.x,
      targetY: wrap.y,
      interp: false,                   // items don't walk; snap in place
      movingSince: performance.now(),
      hitFlashUntil: 0,
      flinchStart: 0,
      flinchDir: { x: 0, y: 1 },
      bodyHomeX: placeholder.x,
      bodyHomeY: placeholder.y,
      prevHp: hpOf(e),
    };
  }

  private update(re: RenderedEntity, next: EntityState): void {
    // World objects: their archetype is stable, but blueprint sprites
    // need to advance through construction stages as extras.progress
    // climbs. Detect the stage change and tear down + recreate so the
    // new texture loads. Same for archetype flips (blueprint → building
    // is engine-emitted as a remove+spawn, but defensive-style).
    if (WORLD_OBJECT_ARCHETYPES.has(re.state.archetype)) {
      const stageChanged =
        re.state.archetype === "blueprint" &&
        next.archetype === "blueprint" &&
        blueprintStage(re.state) !== blueprintStage(next);
      const archChanged = re.state.archetype !== next.archetype;
      if (stageChanged || archChanged) {
        const id = next.entity_id;
        re.container.destroy({ children: true });
        const recreated = this.create(next);
        this.items.set(id, recreated);
        return;
      }
      // Still need to track pos changes (rare — most world objects
      // don't move, but items dropped at character feet do).
      const moved = re.state.pos[0] !== next.pos[0] || re.state.pos[1] !== next.pos[1];
      if (moved) this.applyPos(re.container, next.pos, next.archetype);
      re.state = { ...next };
      return;
    }

    const moved = re.state.pos[0] !== next.pos[0] || re.state.pos[1] !== next.pos[1];
    const turned = re.state.facing !== next.facing;
    const renamed = re.state.display_name !== next.display_name;
    if (moved) {
      // Set the interpolation TARGET (px); tick() eases the sprite there so
      // motion is smooth across slow/skipped snapshots instead of snapping
      // (the multi-tile teleport). Big jumps (respawn) snap in tick().
      re.targetX = Math.round(next.pos[0] * TILE_SIZE_PX);
      re.targetY = Math.round(next.pos[1] * TILE_SIZE_PX);
      re.movingSince = performance.now();
    }
    if (renamed) re.label.text = next.display_name ?? next.entity_id;

    if (re.body instanceof AnimatedSprite) {
      const spec = this.atlas?.get(re.characterId);
      if (spec) {
        // Action animations take priority. When current_action is set,
        // play that anim's textures once. Otherwise default to walk
        // direction.
        let animKey: CharacterAnim;
        if (next.current_action === "attack") {
          animKey = "attack_release";
        } else if (next.current_action === "interact") {
          animKey = "interact";
        } else if (next.current_action === "hit") {
          animKey = "hit";
        } else {
          animKey = facingToAnim(next.facing);
        }
        const desired = spec.anims[animKey];
        const actionChanged = re.state.current_action !== next.current_action;
        if (turned || actionChanged || re.body.textures !== desired) {
          re.body.textures = desired;
          re.body.play();
        }
        // Idle (for walk anims only): freeze on frame 0 after 250ms idle.
        const isWalk = animKey.startsWith("walk_");
        const idleNow = isWalk && performance.now() - re.movingSince > 250;
        if (idleNow && re.body.playing) {
          re.body.gotoAndStop(0);
        } else if (!idleNow && !re.body.playing) {
          re.body.play();
        }
      }
    } else if (re.facingMark && turned) {
      drawFacingMark(re.facingMark, next.facing);
    }
    // BLK-1: damage flash — if hp dropped since the last snapshot, flash
    // the body red briefly so combat is visible (the victim flinches).
    const nh = hpOf(next);
    if (nh !== null && re.prevHp !== null && nh < re.prevHp) {
      re.hitFlashUntil = performance.now() + HIT_FLASH_MS;
      re.flinchStart = performance.now();
      // Recoil AWAY from where the victim is facing (i.e. away from the
      // attacker in front of them) so the hit reads as a knockback.
      re.flinchDir = flinchVector(next.facing);
      // BLK-1/FX: emit a floating damage number at the victim.
      // Anchor the damage number to the sprite's LIVE (interpolated) tile,
      // not the raw engine tile — otherwise it floats ahead of a moving/
      // lagging body. container.x/y are pixels; convert back to a fractional
      // tile so tilePx() re-centers exactly on the rendered sprite.
      this.onDamage?.(
        [re.container.x / TILE_SIZE_PX, re.container.y / TILE_SIZE_PX],
        re.prevHp - nh);
    }
    // Floating HP bar — show whenever the entity is below max HP.
    if (re.hpBar) {
      const mx = maxHpOf(next);
      if (nh !== null && mx !== null && nh < mx && nh > 0) {
        drawHpBar(re.hpBar, nh, mx);
        re.hpBar.visible = true;
      } else {
        re.hpBar.visible = false;
      }
    }
    re.prevHp = nh;
    re.state = { ...next };
  }

  private applyPos(c: Container, tile: [number, number], archetype?: string): void {
    // Container origin sits at the top-left of the 16x16 footprint.
    // The body sprite was positioned with its anchor at footprint
    // bottom-center, so head/cap extends up automatically.
    c.x = Math.round(tile[0] * TILE_SIZE_PX);
    c.y = Math.round(tile[1] * TILE_SIZE_PX);
    // Sort by foot pixel Y so entities further south draw on top.
    // BUT items (coins, gems, dropped loot) sit on the ground and
    // must ALWAYS render below characters — even when a character
    // walks onto an item's tile. Push items down by a big constant
    // so any non-item Y-sorted zIndex still beats them.
    const groundOffset = archetype === "item" ? -100000 : 0;
    c.zIndex = c.y + FOOTPRINT_H + groundOffset;
  }

  destroy(): void {
    for (const re of this.items.values()) {
      re.container.destroy({ children: true });
    }
    this.items.clear();
    this.container.destroy({ children: true });
  }
}

// Knockback direction for a flinch: the victim recoils AWAY from the
// direction it's facing (the attacker is in front of it). Y grows
// downward in screen space.
function flinchVector(f: "N" | "S" | "E" | "W"): { x: number; y: number } {
  switch (f) {
    case "N": return { x: 0, y: 1 };   // facing up → kicked down
    case "S": return { x: 0, y: -1 };  // facing down → kicked up
    case "E": return { x: -1, y: 0 };  // facing right → kicked left
    case "W": return { x: 1, y: 0 };   // facing left → kicked right
  }
}

// Lerp between two 0xRRGGBB colors. f=0 → a, f=1 → b. Used for the smooth
// hit-flash decay and the death grayscale fade.
function lerpTint(a: number, b: number, f: number): number {
  f = Math.max(0, Math.min(1, f));
  const ar = (a >> 16) & 0xff, ag = (a >> 8) & 0xff, ab = a & 0xff;
  const br = (b >> 16) & 0xff, bg = (b >> 8) & 0xff, bb = b & 0xff;
  const r = Math.round(ar + (br - ar) * f);
  const g = Math.round(ag + (bg - ag) * f);
  const bl = Math.round(ab + (bb - ab) * f);
  return (r << 16) | (g << 8) | bl;
}

function easeOutCubic(t: number): number { return 1 - Math.pow(1 - t, 3); }

function facingToAnim(f: "N"|"S"|"E"|"W"): CharacterAnim {
  switch (f) {
    case "N": return "walk_up";
    case "S": return "walk_down";
    case "E": return "walk_right";
    case "W": return "walk_left";
  }
}

/** Placeholder human silhouette in palette colors. Used only when the
 *  character atlas hasn't loaded yet (or for archetypes with no real
 *  sprite assigned). */
function drawPlaceholderBody(g: Graphics): void {
  g.clear();
  // Boots
  g.rect(4, 21, 8, 3).fill(0x181425);
  // Pants
  g.rect(4, 14, 8, 8).fill(0x3e2731);
  // Tunic
  g.rect(3, 9, 10, 6).fill(0x733e39);
  // Head
  g.rect(5, 2, 6, 7).fill(0xe8b796);
  // Hair top
  g.rect(5, 2, 6, 2).fill(0x3e2731);
  // 1px outline (we draw the outline last as a stroked rect)
  g.rect(3, 2, 10, 22).stroke({ color: 0x181425, width: 1, alignment: 1 });
}

/** Small mark indicating facing direction. Will be removed once the
 *  real character sprites have per-direction frames. */
function drawFacingMark(g: Graphics, facing: "N" | "S" | "E" | "W"): void {
  g.clear();
  g.beginPath();
  switch (facing) {
    case "S": g.moveTo(6, 24).lineTo(10, 24).lineTo(8, 26); break;
    case "N": g.moveTo(6, 0).lineTo(10, 0).lineTo(8, -2); break;
    case "E": g.moveTo(13, 12).lineTo(13, 16).lineTo(16, 14); break;
    case "W": g.moveTo(3, 12).lineTo(3, 16).lineTo(0, 14); break;
  }
  g.closePath();
  g.fill(0xfee761);                       // Endesga sun yellow — pops against any tile
}
