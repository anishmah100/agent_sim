// Canvas pointer handling.
//
// Round-1 lesson baked in: classify click vs drag from |up - down|
// Manhattan distance, NOT from cumulative pointermove delta. Mouse
// jitter routinely puts 10-20 px between pointer events even on a
// static click; tight thresholds latch movedFromDown=true and silently
// kill every click. The single source of truth here is the down→up
// distance at pointerup.

import { Viewport } from "pixi-viewport";
import type { EntityState } from "./Entity";
import { TILE_SIZE_PX } from "./tiles";

const CLICK_TOLERANCE_PX = 24;     // |up - down| under this counts as a click

export interface ClickEvent {
  worldX: number;                  // world coords in tile-pixels
  worldY: number;
  tileX: number;
  tileY: number;
  entity: EntityState | null;      // nearest entity within HIT_RADIUS_TILES, if any
}

const HIT_RADIUS_TILES = 1.3;      // forgiving, but not so wide we grab neighbours

export function installClickToInspect(opts: {
  viewport: Viewport;
  getEntities: () => EntityState[];
  onClick: (ev: ClickEvent) => void;
}): () => void {
  const { viewport, getEntities, onClick } = opts;

  let downX = 0;
  let downY = 0;
  let downCaptured = false;

  const onPointerDown = (e: any): void => {
    // pixi-viewport hands us the same event signature as Pixi pointer
    // listeners — pointer.x / pointer.y are screen coords on the canvas.
    downX = e.global?.x ?? e.data?.global?.x ?? 0;
    downY = e.global?.y ?? e.data?.global?.y ?? 0;
    downCaptured = true;
  };

  const onPointerUp = (e: any): void => {
    if (!downCaptured) return;
    downCaptured = false;
    const upX = e.global?.x ?? e.data?.global?.x ?? 0;
    const upY = e.global?.y ?? e.data?.global?.y ?? 0;
    const dist = Math.abs(upX - downX) + Math.abs(upY - downY);
    if (dist > CLICK_TOLERANCE_PX) return;          // it was a drag

    // Convert canvas (screen) coords to world coords via the viewport.
    // pixi-viewport tracks its own transform; .toWorld() is the canon.
    const world = viewport.toWorld(upX, upY);
    const tileX = Math.floor(world.x / TILE_SIZE_PX);
    const tileY = Math.floor(world.y / TILE_SIZE_PX);
    // Hit-test against the PRECISE fractional click point, not the floored
    // tile centre — otherwise every click in a tile is treated as the
    // tile's middle, so two agents one tile apart can't be told apart and
    // the nearest-by-tile pick grabs the wrong (e.g. right-hand) one.
    const fx = world.x / TILE_SIZE_PX;
    const fy = world.y / TILE_SIZE_PX;
    const entity = hitTestEntity(getEntities(), fx, fy);
    onClick({ worldX: world.x, worldY: world.y, tileX, tileY, entity });
  };

  // Attach to the viewport so we get the same pointer events the
  // drag/zoom plugins already see. Bubbling won't help here because the
  // canvas is below the DOM chrome — clicks on panels are caught above.
  viewport.on("pointerdown", onPointerDown);
  viewport.on("pointerup", onPointerUp);
  viewport.on("pointerupoutside", onPointerUp);
  viewport.eventMode = "static";
  viewport.hitArea = {
    contains: () => true,                          // accept clicks anywhere on the viewport
  } as any;

  return () => {
    viewport.off("pointerdown", onPointerDown);
    viewport.off("pointerup", onPointerUp);
    viewport.off("pointerupoutside", onPointerUp);
  };
}

function hitTestEntity(
  entities: EntityState[],
  fx: number,    // fractional click position in tile units (world.x / TILE)
  fy: number,
): EntityState | null {
  let best: { e: EntityState; d2: number } | null = null;
  for (const e of entities) {
    // Entity centre is its tile + (0.5, 0.5). The body sprite renders
    // ABOVE the feet/logical tile, so a click on the visible torso lands
    // ~0.6 tile above the centre; bias the click down by that much so
    // clicking the body matches the entity it belongs to.
    const cx = e.pos[0] + 0.5;
    const cy = e.pos[1] + 0.5;
    const dx = cx - fx;
    const dy = cy - (fy + 0.6);
    const d2 = dx * dx + dy * dy;
    if (d2 < HIT_RADIUS_TILES * HIT_RADIUS_TILES &&
        (best === null || d2 < best.d2)) {
      best = { e, d2 };
    }
  }
  return best?.e ?? null;
}
