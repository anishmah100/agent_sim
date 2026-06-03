// CharacterAtlas — loads every character animation frame as a separate
// texture from /art/processed/frames/<character>/<row>_<frame>.png.
//
// We DO NOT slice from a packed spritesheet — every DALL-E sheet has
// slightly different cell sizes and irregular frame positions. Loading
// each frame as its own file removes all slicing math.

import { Assets, Texture } from "pixi.js";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";
const MANIFEST_URL = `${ENGINE_URL}/art/manifests/characters.json`;
const FRAME_URL = (charId: string, anim: string, idx: number) =>
  `${ENGINE_URL}/art/processed/frames/${charId}/${anim}_${idx}.png`;

export type CharacterAnim =
  | "walk_down" | "walk_up" | "walk_left" | "walk_right"
  | "attack_windup" | "attack_release" | "hit" | "interact";

export interface CharacterSpec {
  id: string;
  display_name: string;
  /** Per-anim frame textures. Each texture is its own image — sizes
   *  may vary slightly between frames in the same animation. */
  anims: Record<CharacterAnim, Texture[]>;
  /** First frame of each direction's walk — also the idle pose. */
  idle: Record<"N" | "S" | "E" | "W", Texture>;
  /** Tallest frame across all 20 animations. Used to compute a uniform
   *  display scale so the character doesn't grow/shrink between frames. */
  ref_frame_h: number;
}

interface ManifestCharacter {
  id: string;
  sheet: string;
  display_name: string;
}

interface Manifest {
  characters: ManifestCharacter[];
}

/** Maps the 5 sheet rows to the 8 animation slots. */
const ROW_NAMES: Array<"walk_down" | "walk_up" | "walk_left" | "walk_right" | "action"> = [
  "walk_down", "walk_up", "walk_left", "walk_right", "action",
];
const ACTION_FRAMES: Array<{ idx: number; anim: CharacterAnim }> = [
  { idx: 0, anim: "attack_windup" },
  { idx: 1, anim: "attack_release" },
  { idx: 2, anim: "hit" },
  { idx: 3, anim: "interact" },
];

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
    const r = await fetch(MANIFEST_URL);
    if (!r.ok) throw new Error(`character manifest ${r.status}`);
    const manifest = (await r.json()) as Manifest;

    for (const c of manifest.characters) {
      const anims: Record<CharacterAnim, Texture[]> = {
        walk_down: [], walk_up: [], walk_left: [], walk_right: [],
        attack_windup: [], attack_release: [], hit: [], interact: [],
      };
      let maxH = 0;
      let ok = true;

      // Walk rows: load 4 frames each.
      for (const row of ROW_NAMES) {
        if (row === "action") continue;
        for (let i = 0; i < 4; i++) {
          try {
            const tex = await Assets.load<Texture>(FRAME_URL(c.id, row, i));
            tex.source.scaleMode = "nearest";
            anims[row].push(tex);
            if (tex.height > maxH) maxH = tex.height;
          } catch (e) {
            console.warn(`frame load failed for ${c.id}/${row}_${i}:`, e);
            ok = false;
          }
        }
      }

      // Action row: 4 frames mapped to attack/hit/interact.
      for (const af of ACTION_FRAMES) {
        try {
          const tex = await Assets.load<Texture>(FRAME_URL(c.id, "action", af.idx));
          tex.source.scaleMode = "nearest";
          anims[af.anim].push(tex);
          if (tex.height > maxH) maxH = tex.height;
        } catch (e) {
          console.warn(`action frame load failed for ${c.id}/${af.idx}:`, e);
          ok = false;
        }
      }

      if (!ok) {
        console.warn(`skipping ${c.id} — frames missing`);
        continue;
      }

      const spec: CharacterSpec = {
        id: c.id,
        display_name: c.display_name,
        anims,
        idle: {
          N: anims.walk_up[0],
          S: anims.walk_down[0],
          E: anims.walk_right[0],
          W: anims.walk_left[0],
        },
        ref_frame_h: maxH,
      };
      atlas.characters.set(c.id, spec);
      if (atlas.fallback === null) atlas.fallback = spec;
    }
    return atlas;
  }
}
