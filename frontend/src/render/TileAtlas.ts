// TileAtlas — loads each overworld tile as its own source-resolution
// PNG. NO downsampling. The renderer scales sprites to tile size at
// draw time. Mirrors the per-frame approach we use for characters.

import { Assets, Texture } from "pixi.js";
import type { TileKind } from "./tiles";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";
const MANIFEST_URL = `${ENGINE_URL}/art/manifests/overworld_tileset.json`;
const TILE_URL = (dir: string, name: string) =>
  `${ENGINE_URL}/art/processed/${dir}/${name}.png`;

interface ManifestTile {
  name: string;
  row: number;
  col: number;
  src_size: [number, number];
}

interface Manifest {
  tile_dir: string;
  tiles: ManifestTile[];
  kind_defaults: Record<string, string>;
}

export class TileAtlas {
  private byName = new Map<string, Texture>();
  private defaultsByKind = new Map<TileKind, Texture>();

  has(kind: TileKind): boolean {
    return this.defaultsByKind.has(kind);
  }

  defaultFor(kind: TileKind): Texture | null {
    return this.defaultsByKind.get(kind) ?? null;
  }

  byNameLookup(name: string): Texture | null {
    return this.byName.get(name) ?? null;
  }

  static async load(): Promise<TileAtlas> {
    const atlas = new TileAtlas();
    console.log("[atlas] fetching manifest", MANIFEST_URL);
    const r = await fetch(MANIFEST_URL);
    if (!r.ok) throw new Error(`tile manifest ${r.status}`);
    const m = (await r.json()) as Manifest;
    console.log(`[atlas] manifest ok: ${m.tiles.length} tiles, dir=${m.tile_dir}`);

    // Parallel-load tiles. Sequential await was costing ~50ms per tile
    // and could hang the whole pipeline if a single load stalled.
    const results = await Promise.allSettled(
      m.tiles.map(async (t) => {
        const url = TILE_URL(m.tile_dir, t.name);
        const tex = await Assets.load<Texture>(url);
        tex.source.scaleMode = "nearest";
        return { name: t.name, tex };
      }),
    );
    let okCount = 0;
    for (const res of results) {
      if (res.status === "fulfilled") {
        atlas.byName.set(res.value.name, res.value.tex);
        okCount++;
      } else {
        console.warn("[atlas] tile load failed:", res.reason);
      }
    }
    console.log(`[atlas] ${okCount}/${m.tiles.length} tile textures loaded`);

    for (const [kind, name] of Object.entries(m.kind_defaults)) {
      const tex = atlas.byName.get(name);
      if (tex) atlas.defaultsByKind.set(kind as TileKind, tex);
    }
    console.log(`[atlas] kind defaults registered: ${atlas.defaultsByKind.size}`);
    return atlas;
  }
}
