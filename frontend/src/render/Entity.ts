// Entity render layer.
//
// Renders characters / objects as placeholder colored quads with a
// tiny name label until real spritesheets land. The shape is correct
// (16×24, bottom-center anchored, facing-aware) so swapping in
// AnimatedSprite from a real atlas is a single-method change later.

import { AnimatedSprite, Container, Graphics, Sprite, Text } from "pixi.js";
import { TILE_SIZE_PX } from "./tiles";
import type { CharacterAtlas, CharacterAnim } from "./CharacterAtlas";

export interface EntityState {
  entity_id: string;
  archetype: string;
  pos: [number, number];
  facing: "N" | "S" | "E" | "W";
  display_name?: string;
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
const ARCHETYPE_TO_CHARACTER: Record<string, string> = {
  // dev_test world uses "human" + display_name to differentiate. Until
  // engine sends a real character_id field, we hash on entity_id to
  // pick from the rotation.
  human: "trainer_red",
};
const CHARACTER_ROTATION = [
  "trainer_red", "trainer_lyra_blue", "wizard", "baker",
  "iron_guard", "child", "cloaked_wanderer",
];

function pickCharacterId(state: EntityState): string {
  const mapped = ARCHETYPE_TO_CHARACTER[state.archetype];
  if (mapped !== undefined && state.archetype !== "human") return mapped;
  // Deterministic hash on entity_id so the same NPC always gets the
  // same sprite across reloads.
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
    const incoming = new Set(entities.map((e) => e.entity_id));
    // Remove anything that disappeared.
    for (const [id, re] of this.items) {
      if (!incoming.has(id)) {
        re.container.destroy({ children: true });
        this.items.delete(id);
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
    }
  }

  private create(e: EntityState): RenderedEntity {
    const wrap = new Container();
    wrap.label = `entity:${e.entity_id}`;
    const characterId = pickCharacterId(e);

    let body: Sprite | AnimatedSprite | Graphics;
    let facingMark: Graphics | null = null;

    const spec = this.atlas?.get(characterId);
    if (spec) {
      // Real sprite. AnimatedSprite handles per-anim frame cycling; we
      // swap the textures field when facing changes. Default to idle
      // (frame 0 of the appropriate walk direction).
      const sprite = new AnimatedSprite(spec.anims.walk_down);
      sprite.animationSpeed = 0.13;        // ~8 fps walk
      sprite.loop = true;
      sprite.anchor.set(spec.anchor_px[0] / spec.frame_w,
                        spec.anchor_px[1] / spec.frame_h);
      // 12-tall native sprite scaled 2x = 24-tall display, ~1.5 tiles
      // tall, matching HeartGold's character-to-tile ratio.
      sprite.scale.set(2);
      // Sit on the bottom-center of the 16x16 footprint.
      sprite.x = FOOTPRINT_W / 2;
      sprite.y = FOOTPRINT_H;
      sprite.texture.source.scaleMode = "nearest";
      sprite.stop();                       // start in idle
      body = sprite;
    } else {
      const g = new Graphics();
      drawPlaceholderBody(g);
      facingMark = new Graphics();
      drawFacingMark(facingMark, e.facing);
      body = g;
    }

    const label = new Text({
      text: e.display_name ?? e.entity_id,
      style: {
        fontFamily: "ui-sans-serif, system-ui, sans-serif",
        fontSize: 6,
        fill: 0xead4aa,
        stroke: { color: 0x181425, width: 2 },
        align: "center",
      },
    });
    label.anchor.set(0.5, 1);
    label.x = FOOTPRINT_W / 2;
    // Label sits 2 px above the sprite's head. Sprite head pixel is
    // computed from spec.anchor_px (foot pixel in native) and scale.
    // For a 12-tall sprite anchored at y=11 with 2× scale, head is at
    // worldY = FOOTPRINT_H - frame_h*scale*(anchor_y/frame_h) = 16 - 22 = -6.
    label.y = spec
      ? FOOTPRINT_H - spec.anchor_px[1] * 2 - 2
      : -10;

    wrap.addChild(body);
    if (facingMark) wrap.addChild(facingMark);
    wrap.addChild(label);

    this.applyPos(wrap, e.pos);
    this.container.addChild(wrap);

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
        const animKey = facingToAnim(next.facing);
        const desired = spec.anims[animKey];
        // Swap the textures sequence only when facing actually changed
        // — re-assigning textures resets the playhead which would
        // otherwise jitter every tick.
        if (turned || re.body.textures !== desired) {
          re.body.textures = desired;
          re.body.play();
        }
        // Idle: if we haven't moved in ~250ms, freeze on frame 0 of the
        // current direction. Looks like a HeartGold idle.
        const idleNow = performance.now() - re.movingSince > 250;
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
