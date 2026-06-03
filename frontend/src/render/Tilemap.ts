// Tilemap rendering layer.
//
// v1 implementation: one PixiJS Sprite per tile in a Container. This
// scales fine to dev_test's 640 tiles and avoids the @pixi/tilemap
// vs PixiJS v8 sub-rectangle texture bug we hit when trying its
// CompositeTilemap path. Once we move to chunked 1000x1000 worlds
// we'll swap to @pixi/tilemap or our own batched draw — the interface
// of TilemapLayer stays the same.

import { Application, Container, Sprite, Texture } from "pixi.js";
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
  /** Render height in tiles. Width comes from the sprite's aspect
   *  ratio unless footprint_w is given. */
  height_tiles?: number;
  /** Render width in tiles. For multi-tile buildings; falls back to
   *  aspect-based width when omitted. */
  footprint_w?: number;
  /** Number of tiles the engine treats as blocking. The footprint is
   *  anchored at (x, y) at the SOUTH edge — y is the southernmost
   *  blocked row, x is the WEST edge. Defaults to 1. */
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

    // Build a TileKind grid so the autotiler can read neighbors.
    const grid: TileKind[][] = [];
    for (let y = 0; y < data.height_tiles; y++) {
      const row = data.tiles[y];
      if (row.length !== data.width_tiles) {
        throw new Error(
          `row ${y} length ${row.length} != declared width ${data.width_tiles}`,
        );
      }
      const rowKinds: TileKind[] = [];
      for (let x = 0; x < data.width_tiles; x++) {
        const ch = row[x];
        const kind = data.tiles_legend[ch];
        if (kind === undefined) {
          throw new Error(`unknown tile char ${JSON.stringify(ch)} at (${x},${y})`);
        }
        rowKinds.push(kind);
      }
      grid.push(rowKinds);
    }

    const kindAt = (x: number, y: number): TileKind | null => {
      if (x < 0 || y < 0 || x >= data.width_tiles || y >= data.height_tiles) return null;
      return grid[y][x];
    };

    for (let y = 0; y < data.height_tiles; y++) {
      for (let x = 0; x < data.width_tiles; x++) {
        const kind = grid[y][x];
        let tex: Texture | null = pickEdgeTexture(kind, x, y, kindAt);
        if (!tex) tex = getTileTextureAt(this.app, kind, x, y);
        const sp = new Sprite(tex);
        sp.x = x * TILE_SIZE_PX;
        sp.y = y * TILE_SIZE_PX;
        // 1px overlap hides subpixel seams at non-integer zoom.
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
