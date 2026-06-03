// Tilemap rendering layer.
//
// v1 implementation: one PixiJS Sprite per tile in a Container. This
// scales fine to dev_test's 640 tiles and avoids the @pixi/tilemap
// vs PixiJS v8 sub-rectangle texture bug we hit when trying its
// CompositeTilemap path. Once we move to chunked 1000x1000 worlds
// we'll swap to @pixi/tilemap or our own batched draw — the interface
// of TilemapLayer stays the same.

import { Application, Container, Sprite } from "pixi.js";
import { TILE_SIZE_PX, getTileTexture, type TileKind } from "./tiles";

export interface TileMapData {
  map_id: string;
  display_name: string;
  tile_size_px: number;
  width_tiles: number;
  height_tiles: number;
  tiles_legend: Record<string, TileKind>;
  tiles: string[];
  entities: TileMapEntity[];
}

export interface TileMapEntity {
  entity_id: string;
  archetype: string;
  pos: [number, number];
  facing: "N" | "S" | "E" | "W";
  display_name?: string;
}

export class TilemapLayer {
  readonly container: Container;

  constructor(private app: Application) {
    this.container = new Container();
    this.container.label = "tilemap";
  }

  /** Replace the rendered tilemap with a new map. Destroys + rebuilds
   *  all child sprites. Cheap up to a few thousand tiles. */
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

    // Tear down existing sprites.
    for (const child of [...this.container.children]) {
      child.destroy();
    }

    for (let y = 0; y < data.height_tiles; y++) {
      const row = data.tiles[y];
      if (row.length !== data.width_tiles) {
        throw new Error(
          `row ${y} length ${row.length} != declared width ${data.width_tiles}`,
        );
      }
      for (let x = 0; x < data.width_tiles; x++) {
        const ch = row[x];
        const kind = data.tiles_legend[ch];
        if (kind === undefined) {
          throw new Error(`unknown tile char ${JSON.stringify(ch)} at (${x},${y})`);
        }
        const tex = getTileTexture(this.app, kind);
        const sp = new Sprite(tex);
        sp.x = x * TILE_SIZE_PX;
        sp.y = y * TILE_SIZE_PX;
        // Source tiles are ~117×111 px (full DALL-E detail preserved).
        // Scale so each tile fills its 16×16 footprint. We render each
        // tile 1 world-pixel WIDER and TALLER than its grid slot to
        // overlap the next tile — this hides any subpixel seam that
        // would otherwise show as a thin dark line at non-integer
        // viewport zoom levels (the standard tilemap extrusion trick).
        sp.width = TILE_SIZE_PX + 1;
        sp.height = TILE_SIZE_PX + 1;
        this.container.addChild(sp);
      }
    }
  }

  destroy(): void {
    this.container.destroy({ children: true });
  }
}
