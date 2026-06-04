// Entity render layer.
//
// Renders characters / objects as placeholder colored quads with a
// tiny name label until real spritesheets land. The shape is correct
// (16×24, bottom-center anchored, facing-aware) so swapping in
// AnimatedSprite from a real atlas is a single-method change later.

import { AnimatedSprite, Container, Graphics, Sprite, Text } from "pixi.js";
import { OutlineFilter } from "pixi-filters";
import { TILE_SIZE_PX } from "./tiles";
import type { CharacterAtlas, CharacterAnim } from "./CharacterAtlas";

// Shared hover outline filter for entities — same look as decorations'
// building hover (Decoration.ts). One instance is fine because Pixi
// re-evaluates the filter each frame against whichever container's
// .filters array currently points to it.
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
}

// Footprint = 16x16 (one tile). Sprite container is anchored at top-left
// of the footprint; the sprite child is centered horizontally and
// bottom-aligned to the footprint bottom row (which corresponds to the
// feet pixel). This lets characters of any sprite height (12 or 24 px)
// render correctly without per-character math.
const FOOTPRINT_W = TILE_SIZE_PX;
const FOOTPRINT_H = TILE_SIZE_PX;

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
  prevPos: [number, number];
  movingSince: number;             // ms — for idle detection
}

export class EntityLayer {
  readonly container: Container;
  private items = new Map<string, RenderedEntity>();
  private selectionRing: Graphics;
  private selectedId: string | null = null;
  private pulsePhase = 0;
  private atlas: CharacterAtlas | null = null;

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
    // Filter out non-character archetypes. Resource entities like
    // trees/rocks live in the engine so the Resources system can
    // target them via chop/mine — they should NOT be drawn as
    // character sprites by this layer (the placeholder character
    // would render in place of the building/tree visual). The
    // Decoration layer handles their visuals when present.
    const NON_CHARACTER = new Set([
      "tree", "rock", "item", "blueprint", "building", "decoration",
    ]);
    const visible = entities.filter((e) => !NON_CHARACTER.has(e.archetype));
    const incoming = new Set(visible.map((e) => e.entity_id));
    // Remove anything that disappeared or became invisible.
    for (const [id, re] of this.items) {
      if (!incoming.has(id)) {
        re.container.destroy({ children: true });
        this.items.delete(id);
      }
    }
    // Add or update incoming.
    for (const e of visible) {
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

    this.applyPos(wrap, e.pos);
    this.container.addChild(wrap);

    // Hover affordance — match the building hover in Decoration.ts so
    // users learn one interaction model: pointer-over outlines the
    // clickable thing, click selects/enters. Items + decorations stay
    // un-hoverable to keep the visual signal meaningful.
    if (e.archetype !== "item" && e.archetype !== "decoration") {
      wrap.eventMode = "static";
      wrap.cursor = "pointer";
      wrap.on("pointerover", () => {
        wrap.filters = [HOVER_OUTLINE];
      });
      wrap.on("pointerout", () => {
        wrap.filters = [];
      });
    }

    return {
      state: { ...e },
      characterId,
      container: wrap,
      body,
      facingMark,
      label,
      prevPos: [e.pos[0], e.pos[1]],
      movingSince: performance.now(),
    };
  }

  private update(re: RenderedEntity, next: EntityState): void {
    const moved = re.state.pos[0] !== next.pos[0] || re.state.pos[1] !== next.pos[1];
    const turned = re.state.facing !== next.facing;
    const renamed = re.state.display_name !== next.display_name;
    if (moved) {
      this.applyPos(re.container, next.pos);
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
    re.state = { ...next };
  }

  private applyPos(c: Container, tile: [number, number]): void {
    // Container origin sits at the top-left of the 16x16 footprint.
    // The body sprite was positioned with its anchor at footprint
    // bottom-center, so head/cap extends up automatically.
    c.x = Math.round(tile[0] * TILE_SIZE_PX);
    c.y = Math.round(tile[1] * TILE_SIZE_PX);
    // Sort by foot pixel Y so entities further south draw on top.
    c.zIndex = c.y + FOOTPRINT_H;
  }

  destroy(): void {
    for (const re of this.items.values()) {
      re.container.destroy({ children: true });
    }
    this.items.clear();
    this.container.destroy({ children: true });
  }
}

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
