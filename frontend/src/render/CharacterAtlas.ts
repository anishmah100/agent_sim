// CharacterAtlas — loads processed character spritesheets + manifest
// once, exposes per-character named animations.
//
// Asset URLs come from the engine (/art/manifests/characters.json and
// /art/processed/<sheet>.png) so the frontend and the multimodal
// agent rasterizer read the SAME files.

import { Assets, Texture, Rectangle } from "pixi.js";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

const MANIFEST_URL = `${ENGINE_URL}/art/manifests/characters.json`;
const SHEET_URL = (file: string) => `${ENGINE_URL}/art/processed/${file}`;

export type CharacterAnim =
  | "walk_down" | "walk_up" | "walk_left" | "walk_right"
  | "attack_windup" | "attack_release" | "hit" | "interact";

export interface CharacterSpec {
  id: string;
  display_name: string;
  /** Per-anim frame textures. The renderer feeds these into AnimatedSprite. */
  anims: Record<CharacterAnim, Texture[]>;
  /** First frame of each direction's walk — also serves as the idle pose. */
  idle: Record<"N" | "S" | "E" | "W", Texture>;
  /** Pixel dims used so the renderer can scale uniformly. */
  frame_w: number;
  frame_h: number;
  /** Anchor in pixels (bottom-center by default). */
  anchor_px: [number, number];
}

interface ManifestSheetShape {
  native_dims_px: [number, number];
  frame_box_px: [number, number];
  frame_anchor_px: [number, number];
  rows: Array<{ name: string; row_index: number; frames: number }>;
}

interface ManifestCharacter {
  id: string;
  sheet: string;
  display_name: string;
}

interface Manifest {
  sheet_shape: ManifestSheetShape;
  characters: ManifestCharacter[];
}

/** Loaded atlas: one entry per character. Populated by loadCharacterAtlas(). */
export class CharacterAtlas {
  private characters = new Map<string, CharacterSpec>();
  private fallback: CharacterSpec | null = null;

  has(id: string): boolean { return this.characters.has(id); }

  get(id: string): CharacterSpec | null {
    return this.characters.get(id) ?? this.fallback;
  }

  list(): CharacterSpec[] {
    return Array.from(this.characters.values());
  }

  static async load(): Promise<CharacterAtlas> {
    const atlas = new CharacterAtlas();
    const manifest = await fetchManifest();
    const shape = manifest.sheet_shape;
    const [fw, fh] = shape.frame_box_px;
    const anchor = shape.frame_anchor_px;

    // Each character: load its sheet PNG and slice out the rows × frames.
    for (const c of manifest.characters) {
      let baseTex: Texture;
      try {
        baseTex = await Assets.load<Texture>(SHEET_URL(c.sheet));
      } catch (e) {
        console.warn(`character sheet load failed for ${c.id}: ${e}`);
        continue;
      }
      // Disable smoothing — pixel art must render crisp.
      baseTex.source.scaleMode = "nearest";

      const anims = sliceAnims(baseTex, shape, fw, fh);
      const idle = {
        N: anims.walk_up[0],
        S: anims.walk_down[0],
        E: anims.walk_right[0],
        W: anims.walk_left[0],
      };
      const spec: CharacterSpec = {
        id: c.id,
        display_name: c.display_name,
        anims, idle,
        frame_w: fw, frame_h: fh,
        anchor_px: [anchor[0], anchor[1]],
      };
      atlas.characters.set(c.id, spec);
      if (atlas.fallback === null) atlas.fallback = spec;
    }
    return atlas;
  }
}

async function fetchManifest(): Promise<Manifest> {
  const r = await fetch(MANIFEST_URL);
  if (!r.ok) throw new Error(`manifest fetch ${r.status}`);
  return (await r.json()) as Manifest;
}

function sliceAnims(
  base: Texture, _shape: ManifestSheetShape, fw: number, fh: number,
): Record<CharacterAnim, Texture[]> {
  const cellW = 32;                      // matches style.json -> cell_w_native
  const cellPad = 8;                     // = (cellW - fw) / 2
  const out: Record<CharacterAnim, Texture[]> = {
    walk_down: [], walk_up: [], walk_left: [], walk_right: [],
    attack_windup: [], attack_release: [], hit: [], interact: [],
  };

  const cut = (anim: CharacterAnim, row: number, frame: number) => {
    const x = frame * cellW + cellPad;
    const y = row * fh;
    const rect = new Rectangle(x, y, fw, fh);
    out[anim].push(new Texture({ source: base.source, frame: rect }));
  };

  // Walk rows: 0=down, 1=up, 2=left, 3=right, each with 4 frames.
  const walkRows: Array<{ row: number; key: CharacterAnim }> = [
    { row: 0, key: "walk_down" },
    { row: 1, key: "walk_up" },
    { row: 2, key: "walk_left" },
    { row: 3, key: "walk_right" },
  ];
  for (const { row, key } of walkRows) {
    for (let f = 0; f < 4; f++) cut(key, row, f);
  }

  // Action row (4): col 0 = attack_windup, 1 = attack_release, 2 = hit, 3 = interact.
  cut("attack_windup", 4, 0);
  cut("attack_release", 4, 1);
  cut("hit", 4, 2);
  cut("interact", 4, 3);

  // Verify slice integrity — fail fast if any frame ended up empty.
  for (const k of Object.keys(out) as CharacterAnim[]) {
    if (out[k].length === 0) {
      throw new Error(`character atlas slice missing anim ${k}`);
    }
  }
  return out;
}

