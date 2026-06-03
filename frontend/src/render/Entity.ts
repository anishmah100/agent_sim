// Entity render layer.
//
// Renders characters / objects as placeholder colored quads with a
// tiny name label until real spritesheets land. The shape is correct
// (16×24, bottom-center anchored, facing-aware) so swapping in
// AnimatedSprite from a real atlas is a single-method change later.

import { Container, Graphics, Text } from "pixi.js";
import { TILE_SIZE_PX } from "./tiles";

export interface EntityState {
  entity_id: string;
  archetype: string;
  pos: [number, number];
  facing: "N" | "S" | "E" | "W";
  display_name?: string;
}

const ENTITY_W = 16;
const ENTITY_H = 24;

interface RenderedEntity {
  state: EntityState;
  container: Container;
  body: Graphics;
  facingMark: Graphics;
  label: Text;
}

export class EntityLayer {
  readonly container: Container;
  private items = new Map<string, RenderedEntity>();
  private selectionRing: Graphics;
  private selectedId: string | null = null;
  private pulsePhase = 0;

  constructor() {
    this.container = new Container();
    this.container.label = "entities";
    // Keep children sorted by Y so entities further south draw on top
    // (proper "depth" for top-down 3/4 view). Built into Pixi when
    // sortableChildren = true + each child's zIndex is set.
    this.container.sortableChildren = true;

    // Selection ring lives on its own. We re-position + redraw each
    // frame in tick(); created once to avoid Graphics churn.
    this.selectionRing = new Graphics();
    this.selectionRing.visible = false;
    this.selectionRing.zIndex = -1;            // under all entity sprites
    this.container.addChild(this.selectionRing);
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
    // Foot position = center of the 16x16 tile footprint.
    const cx = re.container.x + ENTITY_W / 2;
    const cy = re.container.y + ENTITY_H - 1;
    const pulse = 0.7 + 0.3 * Math.sin(this.pulsePhase);
    const rx = ENTITY_W * 0.42;
    const ry = ENTITY_H * 0.11;
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

    // Body: simple human silhouette using palette colors. Drawn at
    // sprite-native size; the viewport scale handles display size.
    const body = new Graphics();
    drawPlaceholderBody(body);

    // Facing mark: tiny triangle on the side the character is facing.
    // Helps validate facing wiring before real sprite directions land.
    const facingMark = new Graphics();
    drawFacingMark(facingMark, e.facing);

    // Floating name label above the head. Drama bubbles will share
    // this anchor pattern later.
    const label = new Text({
      text: e.display_name ?? e.entity_id,
      style: {
        fontFamily: "ui-sans-serif, system-ui, sans-serif",
        fontSize: 7,
        fill: 0xead4aa,
        stroke: { color: 0x181425, width: 2 },
        align: "center",
      },
    });
    label.anchor.set(0.5, 1);
    label.x = ENTITY_W / 2;
    label.y = -2;

    wrap.addChild(body);
    wrap.addChild(facingMark);
    wrap.addChild(label);

    this.applyPos(wrap, e.pos);
    this.container.addChild(wrap);

    return { state: { ...e }, container: wrap, body, facingMark, label };
  }

  private update(re: RenderedEntity, next: EntityState): void {
    const moved = re.state.pos[0] !== next.pos[0] || re.state.pos[1] !== next.pos[1];
    const turned = re.state.facing !== next.facing;
    const renamed = re.state.display_name !== next.display_name;
    if (moved) this.applyPos(re.container, next.pos);
    if (turned) drawFacingMark(re.facingMark, next.facing);
    if (renamed) re.label.text = next.display_name ?? next.entity_id;
    re.state = { ...next };
  }

  private applyPos(c: Container, tile: [number, number]): void {
    // Tile (x,y) → world px coords (top-left of the sprite footprint).
    // Footprint is 16×16; sprite is 16×24 with the extra 8px reaching
    // up above. So sprite top = tile_top - 8.
    c.x = Math.round(tile[0] * TILE_SIZE_PX);
    c.y = Math.round(tile[1] * TILE_SIZE_PX - (ENTITY_H - TILE_SIZE_PX));
    // Sort by bottom-of-sprite Y so entities further south draw on top.
    c.zIndex = c.y + ENTITY_H;
  }

  destroy(): void {
    for (const re of this.items.values()) {
      re.container.destroy({ children: true });
    }
    this.items.clear();
    this.container.destroy({ children: true });
  }
}

/** Placeholder human silhouette in palette colors. Replaced by an
 *  AnimatedSprite from the character atlas in Milestone 2. */
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
