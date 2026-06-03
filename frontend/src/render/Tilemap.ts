// Tilemap rendering layer.
//
// Wraps @pixi/tilemap. Given a TileMapData (the in-house v0 format —
// see worlds/dev_test.json), builds a CompositeTilemap that PixiJS
// renders as one batched draw call regardless of map size.
//
// When we swap to LDtk, this class stays — only loadTileMap() changes
// to consume the LDtk parser's output. The render path is unchanged.

import { Application, Container } from "pixi.js";
import { CompositeTilemap } from "@pixi/tilemap";
import { TILE_SIZE_PX, getTileTexture, type TileKind } from "./tiles";

export interface TileMapData {
  map_id: string;
  display_name: string;
  tile_size_px: number;
  width_tiles: number;
  height_tiles: number;
  tiles_legend: Record<string, TileKind>;
  tiles: string[];            // rows of single-char legend keys
  entities: TileMapEntity[];  // shipped with the world JSON for now;
                              // will come from engine WS once that lands
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
  private composite: CompositeTilemap;

  constructor(private app: Application) {
    this.container = new Container();
    this.container.label = "tilemap";
    this.composite = new CompositeTilemap();
    this.container.addChild(this.composite);
  }

  /** Replace the rendered tilemap with a new map. Cheap: clears the
   *  composite and re-adds one tile per cell. Even at 1000x1000 this
   *  is well under a frame. */
  loadTileMap(data: TileMapData): void {
    if (data.tile_size_px !== TILE_SIZE_PX) {
      // Hard-fail rather than silently misrender. Tile size is a
      // bedrock invariant — see docs/ART_STYLE_GUIDE.md.
      throw new Error(
        `tile size mismatch: data has ${data.tile_size_px}, renderer is ${TILE_SIZE_PX}`,
      );
    }
    if (data.tiles.length !== data.height_tiles) {
      throw new Error(
        `row count mismatch: rows=${data.tiles.length}, declared height=${data.height_tiles}`,
      );
    }

    this.composite.clear();
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
        this.composite.tile(tex, x * TILE_SIZE_PX, y * TILE_SIZE_PX);
      }
    }
  }

  destroy(): void {
    this.composite.destroy();
    this.container.destroy({ children: true });
  }
}
