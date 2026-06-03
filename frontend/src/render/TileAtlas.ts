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
  private variantsByKind = new Map<TileKind, Texture[]>();

  has(kind: TileKind): boolean {
    return this.defaultsByKind.has(kind);
  }

  defaultFor(kind: TileKind): Texture | null {
    return this.defaultsByKind.get(kind) ?? null;
  }

  byNameLookup(name: string): Texture | null {
    return this.byName.get(name) ?? null;
  }

  variantsFor(kind: TileKind): Texture[] {
    return this.variantsByKind.get(kind) ?? [];
  }

  static async load(): Promise<TileAtlas> {
    const atlas = new TileAtlas();
    const r = await fetch(MANIFEST_URL);
    if (!r.ok) throw new Error(`tile manifest ${r.status}`);
    const m = (await r.json()) as Manifest;

    for (const t of m.tiles) {
      try {
        const tex = await Assets.load<Texture>(TILE_URL(m.tile_dir, t.name));
        tex.source.scaleMode = "nearest";
        atlas.byName.set(t.name, tex);
      } catch (e) {
        console.warn(`tile load failed: ${t.name}`, e);
      }
    }
    for (const [kind, name] of Object.entries(m.kind_defaults)) {
      const tex = atlas.byName.get(name);
      if (tex) atlas.defaultsByKind.set(kind as TileKind, tex);
    }
    // Group variants by name prefix. e.g. all tiles whose name starts
    // with "grass" or is "grass" → variants for kind "grass". Used by
    // the renderer to randomly pick alternate tiles for visual variety.
    const variantPrefixes: Record<TileKind, string[]> = {
      grass: ["grass"],
      dirt: ["dirt"],
      stone: ["stone", "sand"],
      water: ["water"],
      path: ["stone"],
      wall: ["cliff"],
      floor_wood: ["dirt"],
      void: ["cliff_solid"],
    };
    for (const [kind, prefixes] of Object.entries(variantPrefixes) as Array<[TileKind, string[]]>) {
      const arr: Texture[] = [];
      for (const t of m.tiles) {
        const name = t.name;
        const matches = prefixes.some((p) => name === p || name.startsWith(p + "_"));
        // Filter out edge/corner/transition variants — those are for
        // autotiling, not random ground variety. They have suffixes
        // like _edge_top, _corner_*, _to_*, _tracks_to_grass etc.
        const isPattern = /_edge_|_corner_|_to_/.test(name) || name.endsWith("_inner");
        if (matches && !isPattern) {
          const tex = atlas.byName.get(name);
          if (tex) arr.push(tex);
        }
      }
      if (arr.length > 0) atlas.variantsByKind.set(kind, arr);
    }
    return atlas;
  }
}
