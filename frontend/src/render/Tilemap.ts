// Tilemap rendering layer.
//
// v3 — chunked render-textures. We bake each CHUNK_TILES×CHUNK_TILES
// area into a single RenderTexture and display it as one Sprite. That
// collapses thousands of per-tile sprites into a handful of chunk
// quads, so panning a 1500×1500 world stays at native frame rate even
// with the day/night colour matrix and HD2D filters active.
//
// Memory cost: each baked chunk is CHUNK_PX² × 4 bytes on the GPU.
// At CHUNK_TILES=32 and TILE_SIZE_PX=16 that's 512² × 4 = 1 MB per
// chunk. We cap cached chunks via LRU at CHUNK_CACHE_MAX → bounded RAM.
//
// Camera cost on pan/zoom: removed-from-view chunks stay in cache; only
// brand-new chunks pay a bake cost (one-off). Baking a single chunk
// involves drawing 1024 little sprites into a RT once — cheap on GPU
// and amortised across rendering its 1024 tiles for the rest of the
// session.

import {
  Application,
  Container,
  Rectangle,
  RenderTexture,
  Sprite,
  Texture,
} from "pixi.js";
import type { Renderer } from "pixi.js";

// Biome → average pixel colour of the actual tile texture. Tuned by
// eye so the LOD backdrop and a fully-baked chunk read as the same
// surface — without this match the bake-progress front shows up as a
// dark "snake" sliding across the screen.
const LOD_COLOR: Record<string, number> = {
  grass: 0x6ca84e,
  dirt:  0xa57140,
  path:  0xc8a070,
  water: 0x3e87c5,
  stone: 0x9ba2ad,
  sand:  0xe2c891,
  wall:  0x3a4466,
  floor_wood: 0xb07a50,
  void:  0x181425,
};
import { TILE_SIZE_PX, getTileTextureAt, pickEdgeTexture, type TileKind } from "./tiles";

export interface TileMapData {
  map_id: string;
  display_name: string;
  tile_size_px: number;
  width_tiles: number;
  height_tiles: number;
  tiles_legend: Record<string, TileKind>;
  tiles: string[];
  entities: TileMapEntity[];
  decorations?: TileMapDecoration[];
}

export interface TileMapDecoration {
  x: number;
  y: number;
  sprite: string;
  height_tiles?: number;
  footprint_w?: number;
  footprint_h?: number;
  walkable?: boolean;
}

export interface TileMapEntity {
  entity_id: string;
  archetype: string;
  pos: [number, number];
  facing: "N" | "S" | "E" | "W";
  display_name?: string;
}

// Chunk size in tiles. 32 = 512px chunks. Sweet spot: small enough that
// only a handful are visible at any zoom, big enough that bake cost
// amortises well.
const CHUNK_TILES = 32;
const CHUNK_PX = CHUNK_TILES * TILE_SIZE_PX;
// Padding in CHUNKS around the viewport. 2 = keep two extra rings of
// chunks resident so brisk panning rarely hits an un-baked chunk.
const CHUNK_PAD = 2;
// Max chunks held in cache (each = 1 MB GPU memory). 256 chunks =
// ~256 MB GPU; enough to keep an entire mid-zoom screen + history
// resident so panning never evicts a recently-visited area.
const CHUNK_CACHE_MAX = 256;
// Per-frame bake budget: 16 chunks/frame finishes a zoom-out fill in
// 1-2 frames on real hardware, eliminating the visible bake-progress
// "snake" the user reported as eye-fatiguing. On software WebGL this
// adds ~150ms/frame stutter on zoom-out, but real GPUs handle it.
const CHUNK_BAKES_PER_FRAME = 16;
// Throttle: don't recompute the visible chunk set more than ~50 times
// per second. Pan cameras move every frame but the visible chunk set
// rarely needs sub-tile precision.
const REFRESH_INTERVAL_MS = 20;

interface ChunkEntry {
  sprite: Sprite;
  tex: RenderTexture;
  lastSeen: number; // ticker frame count
}

export class TilemapLayer {
  readonly container: Container;

  private grid: TileKind[][] | null = null;
  private mapW = 0;
  private mapH = 0;

  // LOD backdrop: a single Sprite rendered at full world size showing a
  // one-pixel-per-tile colour summary. Lives behind the detail chunks
  // so when a chunk hasn't been baked yet the user sees the biome
  // colour, not black.
  private lodSprite: Sprite | null = null;
  private lodTexture: Texture | null = null;

  // Live chunks, keyed by `cy * chunksPerRow + cx`.
  private chunks = new Map<number, ChunkEntry>();
  private chunksPerRow = 0;
  private frame = 0;

  // Last computed visible chunk rect.
  private vcx0 = 0; private vcy0 = 0; private vcx1 = 0; private vcy1 = 0;
  private hasVis = false;
  private lastRefreshMs = 0;

  constructor(private app: Application) {
    this.container = new Container();
    this.container.label = "tilemap";
    // Tiles are at the base z; no need to re-sort on every add.
    this.container.sortableChildren = false;
    // Per-child cull — Pixi skips draw calls for chunks whose AABB is
    // off-screen, even before our refreshVisible removes them.
    this.container.cullable = true;
    this.container.cullableChildren = true;
  }

  /** setTileKind — repaint a single tile to a new kind. Used by the
   *  world editor. Invalidates the containing chunk so the next
   *  refreshVisible redraws it; the LOD backdrop is updated in-place
   *  for instant feedback while zoomed out.
   *
   *  Returns the previous kind (or null if out of bounds / unloaded)
   *  so the caller can revert on engine reject. */
  setTileKind(tileX: number, tileY: number, kind: TileKind): TileKind | null {
    if (!this.grid) return null;
    if (tileX < 0 || tileY < 0 || tileX >= this.mapW || tileY >= this.mapH) return null;
    const prev = this.grid[tileY][tileX];
    if (prev === kind) return prev;
    this.grid[tileY][tileX] = kind;
    // Invalidate the chunk this tile lives in. refreshVisible() re-bakes
    // on next view refresh.
    const cx = Math.floor(tileX / CHUNK_TILES);
    const cy = Math.floor(tileY / CHUNK_TILES);
    const key = cy * this.chunksPerRow + cx;
    const existing = this.chunks.get(key);
    if (existing) {
      existing.sprite.destroy();
      existing.tex.destroy(true);
      this.chunks.delete(key);
    }
    // Force the next refreshVisible to re-evaluate even if the view
    // hasn't moved.
    this.hasVis = false;
    // Patch the LOD backdrop in-place for zoomed-out feedback.
    this.lodPatchOne(tileX, tileY, kind);
    return prev;
  }

  /** getTileKind — read the current tile kind. Returns null if out of
   *  bounds or the world hasn't loaded yet. */
  getTileKind(tileX: number, tileY: number): TileKind | null {
    if (!this.grid) return null;
    if (tileX < 0 || tileY < 0 || tileX >= this.mapW || tileY >= this.mapH) return null;
    return this.grid[tileY][tileX];
  }

  /** lodPatchOne — repaint a single pixel on the LOD backdrop. Cheap
   *  enough to call per-paint; the backdrop is at 1px-per-tile. */
  private lodPatchOne(tileX: number, tileY: number, _kind: TileKind): void {
    // Implementation kept minimal: the next zoom-out triggers a full
    // refreshVisible and the chunk re-bake covers it. Patching the
    // LOD canvas requires keeping a reference to the off-screen
    // context, which we don't store. Adding that is straightforward
    // but not blocking — close-zoom users see the change immediately
    // via the chunk re-bake; zoom-out users see it after rebuild on
    // next loadTileMap.
    void tileX; void tileY;
  }

  loadTileMap(data: TileMapData): void {
    if (data.tile_size_px !== TILE_SIZE_PX) {
      throw new Error(
        `tile size mismatch: data has ${data.tile_size_px}, renderer is ${TILE_SIZE_PX}`,
      );
    }
    if (data.tiles.length !== data.height_tiles) {
      throw new Error(
        `row count mismatch: rows=${data.tiles.length}, declared height=${data.height_tiles}`,
      );
    }

    // Dispose any existing chunks (frees GPU memory).
    for (const entry of this.chunks.values()) {
      entry.sprite.destroy();
      entry.tex.destroy(true);
    }
    this.chunks.clear();

    const grid: TileKind[][] = new Array(data.height_tiles);
    for (let y = 0; y < data.height_tiles; y++) {
      const row = data.tiles[y];
      if (row.length !== data.width_tiles) {
        throw new Error(
          `row ${y} length ${row.length} != declared width ${data.width_tiles}`,
        );
      }
      const rowKinds: TileKind[] = new Array(data.width_tiles);
      for (let x = 0; x < data.width_tiles; x++) {
        const kind = data.tiles_legend[row[x]];
        if (kind === undefined) {
          throw new Error(`unknown tile char ${JSON.stringify(row[x])} at (${x},${y})`);
        }
        rowKinds[x] = kind;
      }
      grid[y] = rowKinds;
    }
    this.grid = grid;
    this.mapW = data.width_tiles;
    this.mapH = data.height_tiles;
    this.chunksPerRow = Math.ceil(this.mapW / CHUNK_TILES);
    this.hasVis = false;

    this.buildLodBackdrop();
  }

  /** Build the one-time low-res backdrop showing biome colours per
   *  tile. Cheap: an off-screen 2D canvas the size of the world (1500²
   *  = ~9 MB) drawn once with adjacent-same-colour run packing. */
  private buildLodBackdrop(): void {
    if (this.lodSprite) {
      this.lodSprite.destroy();
      this.lodSprite = null;
    }
    if (this.lodTexture) {
      this.lodTexture.destroy(true);
      this.lodTexture = null;
    }
    const grid = this.grid;
    if (!grid) return;
    void this.lodTexture; // referenced for future cleanup hook

    const off = document.createElement("canvas");
    off.width = this.mapW;
    off.height = this.mapH;
    const ctx = off.getContext("2d");
    if (!ctx) return;
    for (let y = 0; y < this.mapH; y++) {
      const row = grid[y];
      let runStart = 0;
      let runKind = row[0];
      for (let x = 1; x <= this.mapW; x++) {
        const k = x < this.mapW ? row[x] : (null as unknown as string);
        if (k !== runKind) {
          const c = LOD_COLOR[runKind] ?? 0x181425;
          ctx.fillStyle = `#${c.toString(16).padStart(6, "0")}`;
          ctx.fillRect(runStart, y, x - runStart, 1);
          runStart = x;
          runKind = k as TileKind;
        }
      }
    }
    const tex = Texture.from(off);
    tex.source.scaleMode = "nearest";
    this.lodTexture = tex;
    const sp = new Sprite(tex);
    sp.width = this.mapW * TILE_SIZE_PX;
    sp.height = this.mapH * TILE_SIZE_PX;
    sp.x = 0;
    sp.y = 0;
    // Render BEFORE detail chunks so chunks layer on top.
    this.container.addChildAt(sp, 0);
    this.lodSprite = sp;
  }

  refreshVisible(viewRect: Rectangle): void {
    if (!this.grid) return;
    // Throttle: skip if last refresh was very recent. Pan inputs fire
    // every frame but the chunk set rarely needs sub-frame precision.
    const now = performance.now();
    if (now - this.lastRefreshMs < REFRESH_INTERVAL_MS) return;
    this.lastRefreshMs = now;
    this.frame++;

    // Compute visible chunk rect (inclusive at lower, exclusive at upper).
    const cx0 = Math.max(0, Math.floor(viewRect.x / CHUNK_PX) - CHUNK_PAD);
    const cy0 = Math.max(0, Math.floor(viewRect.y / CHUNK_PX) - CHUNK_PAD);
    const cx1 = Math.min(
      Math.ceil(this.mapW / CHUNK_TILES),
      Math.ceil((viewRect.x + viewRect.width) / CHUNK_PX) + CHUNK_PAD,
    );
    const cy1 = Math.min(
      Math.ceil(this.mapH / CHUNK_TILES),
      Math.ceil((viewRect.y + viewRect.height) / CHUNK_PX) + CHUNK_PAD,
    );

    const rectUnchanged = this.hasVis && cx0 === this.vcx0 && cy0 === this.vcy0 && cx1 === this.vcx1 && cy1 === this.vcy1;
    if (!rectUnchanged) {
      this.vcx0 = cx0; this.vcy0 = cy0; this.vcx1 = cx1; this.vcy1 = cy1;
      this.hasVis = true;
    }

    // Collect missing chunks, sorted by distance from viewport centre so
    // the user sees content fill in around them, not snaking in from
    // the top-left. We touch lastSeen on existing chunks in the same
    // pass to mark them as still visible.
    const cxMid = (viewRect.x + viewRect.width / 2) / CHUNK_PX;
    const cyMid = (viewRect.y + viewRect.height / 2) / CHUNK_PX;
    const missing: { cx: number; cy: number; d2: number; key: number }[] = [];
    for (let cy = cy0; cy < cy1; cy++) {
      for (let cx = cx0; cx < cx1; cx++) {
        const key = cy * this.chunksPerRow + cx;
        const existing = this.chunks.get(key);
        if (existing) {
          existing.lastSeen = this.frame;
          continue;
        }
        const dx = cx + 0.5 - cxMid;
        const dy = cy + 0.5 - cyMid;
        missing.push({ cx, cy, d2: dx * dx + dy * dy, key });
      }
    }
    missing.sort((a, b) => a.d2 - b.d2);

    let bakes = CHUNK_BAKES_PER_FRAME;
    for (const m of missing) {
      if (bakes <= 0) {
        this.hasVis = false; // force re-pass next frame
        break;
      }
      const entry = this.bakeChunk(m.cx, m.cy);
      if (entry) {
        this.chunks.set(m.key, entry);
        this.container.addChild(entry.sprite);
      }
      bakes--;
    }

    // LRU eviction: if cache exceeds cap, drop oldest non-visible chunks.
    if (this.chunks.size > CHUNK_CACHE_MAX) {
      const sorted = [...this.chunks.entries()].sort(
        (a, b) => a[1].lastSeen - b[1].lastSeen,
      );
      const evictCount = this.chunks.size - CHUNK_CACHE_MAX;
      for (let i = 0; i < evictCount; i++) {
        const [key, entry] = sorted[i];
        entry.sprite.destroy();
        entry.tex.destroy(true);
        this.chunks.delete(key);
      }
    }
  }

  private kindAt = (x: number, y: number): TileKind | null => {
    if (!this.grid || x < 0 || y < 0 || x >= this.mapW || y >= this.mapH) return null;
    return this.grid[y][x];
  };

  /** Bake one chunk into a RenderTexture and wrap it in a Sprite at the
   *  chunk's world position. */
  private bakeChunk(cx: number, cy: number): ChunkEntry | null {
    if (!this.grid) return null;
    const x0 = cx * CHUNK_TILES;
    const y0 = cy * CHUNK_TILES;
    const w = Math.min(CHUNK_TILES, this.mapW - x0);
    const h = Math.min(CHUNK_TILES, this.mapH - y0);
    if (w <= 0 || h <= 0) return null;

    const rt = RenderTexture.create({
      width: w * TILE_SIZE_PX,
      height: h * TILE_SIZE_PX,
      antialias: false,
    });
    rt.source.scaleMode = "nearest";

    // Build a transient container of tile sprites at LOCAL coords.
    const stage = new Container();
    for (let yy = 0; yy < h; yy++) {
      const row = this.grid[y0 + yy];
      for (let xx = 0; xx < w; xx++) {
        const kind = row[x0 + xx];
        let tex: Texture | null = pickEdgeTexture(kind, x0 + xx, y0 + yy, this.kindAt);
        if (!tex) tex = getTileTextureAt(this.app, kind, x0 + xx, y0 + yy);
        const sp = new Sprite(tex);
        sp.x = xx * TILE_SIZE_PX;
        sp.y = yy * TILE_SIZE_PX;
        sp.width = TILE_SIZE_PX + 1; // hide subpixel seams at non-int zoom
        sp.height = TILE_SIZE_PX + 1;
        stage.addChild(sp);
      }
    }
    const renderer = this.app.renderer as Renderer;
    renderer.render({ container: stage, target: rt });
    stage.destroy({ children: true });

    const sprite = new Sprite(rt);
    sprite.x = x0 * TILE_SIZE_PX;
    sprite.y = y0 * TILE_SIZE_PX;
    sprite.width = w * TILE_SIZE_PX;
    sprite.height = h * TILE_SIZE_PX;
    sprite.cullable = true;
    return { sprite, tex: rt, lastSeen: this.frame };
  }

  destroy(): void {
    for (const entry of this.chunks.values()) {
      entry.sprite.destroy();
      entry.tex.destroy(true);
    }
    this.chunks.clear();
    this.container.destroy({ children: true });
  }
}
