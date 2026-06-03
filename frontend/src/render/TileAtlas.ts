// TileAtlas — loads the overworld tileset PNG + manifest and exposes
// per-named textures and a per-TileKind default. Used by TilemapLayer.
//
// Until LDtk lands, the world JSON specifies tiles by short legend
// codes (e.g. "g" = grass). The TilemapLayer maps each legend code →
// TileKind, then looks up the default texture for that kind here.
// Once we ship LDtk maps, the JSON will name the variant directly and
// we drop the kind_defaults layer.

import { Assets, Texture, Rectangle } from "pixi.js";
import type { TileKind } from "./tiles";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";
const MANIFEST_URL = `${ENGINE_URL}/art/manifests/overworld_tileset.json`;
const SHEET_URL = (file: string) => `${ENGINE_URL}/art/processed/${file}`;

interface ManifestCell {
  row: number;
  col: number;
  name: string;
  kind: TileKind;
}

interface Manifest {
  sheet: string;
  native_dims_px: [number, number];
  cell_size_native: number;
  grid_cols: number;
  grid_rows: number;
  cells: ManifestCell[];
  kind_defaults: Record<string, string>;
}

export class TileAtlas {
  private byName = new Map<string, Texture>();
  private defaultsByKind = new Map<TileKind, Texture>();
  /** All textures by kind — used for picking random variants later. */
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

    const base = await Assets.load<Texture>(SHEET_URL(m.sheet));
    base.source.scaleMode = "nearest";
    const s = m.cell_size_native;

    for (const c of m.cells) {
      const rect = new Rectangle(c.col * s, c.row * s, s, s);
      const tex = new Texture({ source: base.source, frame: rect });
      atlas.byName.set(c.name, tex);
      const arr = atlas.variantsByKind.get(c.kind) ?? [];
      arr.push(tex);
      atlas.variantsByKind.set(c.kind, arr);
    }

    for (const [kind, name] of Object.entries(m.kind_defaults)) {
      const tex = atlas.byName.get(name);
      if (tex) atlas.defaultsByKind.set(kind as TileKind, tex);
    }

    return atlas;
  }
}
