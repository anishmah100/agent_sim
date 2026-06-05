// ArtCatalog — single source of truth for every sprite reference in
// the frontend. Loads a sprite manifest once at boot. Every sprite-URL
// resolver (Decoration, Entity, Interior, …) delegates here so adding
// a new sprite is one manifest entry, not a sweep through scattered
// allowlists + path templates.
//
// PACK SWAPPING: which manifest to load is configurable via
// `VITE_ART_MANIFEST` (default: `sprites.json`). To ship a different
// sprite set:
//   1. Put the new PNGs under `art/packs/<name>/...`.
//   2. Write `art/manifests/sprites.<name>.json` referencing them.
//   3. Set `VITE_ART_MANIFEST=sprites.<name>.json` and reload.
// The manifest can also `"extends": "sprites.json"` to layer on top of
// another manifest — handy when an alt pack only wants to replace a
// subset of sprites. The catalog walks the extends chain bottom-up,
// later entries winning.
//
// During the migration, resolvers fall through to their legacy paths
// when the catalog has no entry for a sprite id. Once the manifest
// covers everything in use, the legacy paths get deleted.

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

// The default manifest. Override via env to swap entire sprite sets.
const MANIFEST_NAME: string =
  (import.meta.env.VITE_ART_MANIFEST as string | undefined) ?? "sprites.json";

export interface SpriteFrames {
  dir: string;
  by_action?: Record<string, string[]>;
  ref_height_px?: number;
}

export interface SpriteMeta {
  /** Path under art/processed/. URL = `${ENGINE_URL}/art/processed/${path}`. */
  path: string;
  label?: string;
  kind?: string;
  /** [width, height] of the source PNG in pixels. */
  native_size_px: [number, number];
  /** Render footprint in TILES, if the sprite is a building / structure. */
  footprint_tiles?: [number, number];
  render_height_tiles?: number;
  /** True for buildings the interior layer should open on click. */
  enterable?: boolean;
  /** Which interior template the interior layer should render. */
  interior_template?: string;
  /** For character-style animated entities. */
  frames?: SpriteFrames;
}

interface CategoryMeta {
  label: string;
  default_height_tiles: number;
}

interface Manifest {
  $schema: string;
  /** If present, every sprite.path resolves under `processed/<base_path>/`
   *  instead of directly under `processed/`. Lets a pack drop its
   *  PNGs in `processed/packs/medieval/` without restating that prefix
   *  on every entry. */
  base_path?: string;
  /** If present, the named manifest is loaded first and this one is
   *  merged on top — later entries override earlier ones. Path is
   *  relative to `/art/manifests/`. Chain depth is capped at 4. */
  extends?: string;
  categories?: Record<string, CategoryMeta>;
  sprites?: Record<string, SpriteMeta>;
}

function manifestUrl(name: string): string {
  return `${ENGINE_URL}/art/manifests/${name}`;
}

export class ArtCatalog {
  private sprites = new Map<string, SpriteMeta>();
  private categories = new Map<string, CategoryMeta>();
  /** Path prefix prepended to each sprite's `path` field when building
   *  the URL. Default is empty, so paths resolve directly under
   *  `art/processed/`. A pack manifest can set this to e.g. "packs/medieval/"
   *  and only restate the file name in each sprite entry. */
  private basePath = "";

  static async load(name: string = MANIFEST_NAME): Promise<ArtCatalog> {
    const cat = new ArtCatalog();
    await cat.loadInto(name, 4);
    return cat;
  }

  /** Walk the extends chain bottom-up so later manifests override
   *  earlier ones, then layer this manifest's entries on top. */
  private async loadInto(name: string, depthBudget: number): Promise<void> {
    if (depthBudget <= 0) {
      throw new Error(`art manifest extends chain too deep at ${name}`);
    }
    const r = await fetch(manifestUrl(name));
    if (!r.ok) throw new Error(`sprites manifest ${name}: ${r.status}`);
    const m = (await r.json()) as Manifest;
    if (m.extends) {
      await this.loadInto(m.extends, depthBudget - 1);
    }
    if (m.base_path) this.basePath = m.base_path.replace(/\/+$/, "") + "/";
    if (m.categories) {
      for (const [id, c] of Object.entries(m.categories)) {
        this.categories.set(id, c);
      }
    }
    if (m.sprites) {
      for (const [id, sp] of Object.entries(m.sprites)) {
        this.sprites.set(id, sp);
      }
    }
  }

  has(id: string): boolean {
    return this.sprites.has(id);
  }

  meta(id: string): SpriteMeta | null {
    return this.sprites.get(id) ?? null;
  }

  /** Resolve a sprite id to a render-ready URL, or null if not in catalog. */
  url(id: string): string | null {
    const sp = this.sprites.get(id);
    if (!sp) return null;
    return `${ENGINE_URL}/art/processed/${this.basePath}${sp.path}`;
  }

  /** True if click-to-enter should fire an interior view for this sprite. */
  enterable(id: string): boolean {
    return this.sprites.get(id)?.enterable === true;
  }

  /** Which interior template to open on entry. */
  interiorTemplate(id: string): string | null {
    return this.sprites.get(id)?.interior_template ?? null;
  }

  /** Native aspect ratio (w / h). null if sprite missing or has no size. */
  nativeAspect(id: string): number | null {
    const sp = this.sprites.get(id);
    if (!sp || !sp.native_size_px) return null;
    const [w, h] = sp.native_size_px;
    if (h === 0) return null;
    return w / h;
  }

  categoryDefaultHeight(id: string): number | null {
    const cat = id.split(":")[0];
    return this.categories.get(cat)?.default_height_tiles ?? null;
  }

  /** Count for boot diagnostics. */
  size(): number {
    return this.sprites.size;
  }
}

// Module-level singleton, populated by setArtCatalog() from PixiApp
// after the load promise resolves. Resolvers grab this and fall back
// to their legacy logic when it's null or the id isn't present.
let _catalog: ArtCatalog | null = null;

export function setArtCatalog(c: ArtCatalog | null): void {
  _catalog = c;
}

export function artCatalog(): ArtCatalog | null {
  return _catalog;
}
