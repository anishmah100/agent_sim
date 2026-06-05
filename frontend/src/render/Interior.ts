// Interior view — Pokemon-style. When the user clicks a building on
// the overworld, this layer fades in showing a room interior. Each
// building has a hand-authored template with three layers:
//
//   tiles[]   — base floor / wall layout (16x16 grid)
//   rugs[]    — optional centered rug pattern that sits on top of floor
//   props{}   — furniture sprites placed at specific tile coords; some
//               span multiple tiles (king bed = 2 tiles wide)
//
// Design goal (per Pokemon HeartGold): rooms should look like real
// rooms. That means:
//   - Top wall row uses a window-decorated wall tile (the iconic
//     blue-square panes you see in HG interiors).
//   - A rug is centered on the floor, with furniture arranged AROUND
//     it pressed against the walls (symmetric where possible).
//   - Furniture is grouped by function: dining cluster, sleeping
//     cluster, cooking cluster, etc. — not scattered free-form.
//   - The door is centered at the bottom and visually framed.
//
// UX:
//   - Fade in over 220 ms
//   - Click the framed door at the bottom to leave (or press ESC)

import { Application, Assets, Container, Graphics, Sprite, Text, Texture } from "pixi.js";
import { OutlineFilter } from "pixi-filters";
import { TILE_SIZE_PX } from "./tiles";
import { artCatalog } from "./ArtCatalog";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

// Sprite-id resolvers — every interior asset goes through the catalog
// first, then falls back to legacy direct paths for assets the catalog
// doesn't yet cover. The catalog's pack-swap mechanism then handles
// alternative interior tilesets too.
const TILE = (name: string) =>
  artCatalog()?.url(`int:${name}`)
  ?? `${ENGINE_URL}/art/processed/v2_interior_tiles_master/${name}.png`;
const LEGACY_TILE = (name: string) =>
  artCatalog()?.url(`prop:${name}`)
  ?? `${ENGINE_URL}/art/processed/tiles/interior/${name}.png`;
const PROP = (name: string) =>
  artCatalog()?.url(`prop:${name}`)
  ?? `${ENGINE_URL}/art/processed/v2_interior_props_master/${name}.png`;
const PROP2W = (name: string) =>
  artCatalog()?.url(`prop:${name}`)
  ?? `${ENGINE_URL}/art/processed/tiles/interior/props_2w/${name}.png`;

// SHEET_PROPS stays: it identifies which prop-name strings live in the
// legacy `tiles/interior/` folder rather than `v2_interior_props_master/`.
// The catalog routes both to the same `prop:` namespace, but the legacy
// fallback below needs to know which path to use.
const SHEET_PROPS = new Set([
  "anvil_sheet", "forge_fire_sheet", "fireplace_stone_sheet",
  "cauldron_sheet", "clock_grandfather", "chandelier_sheet",
  "candelabra_floor_sheet",
]);

// ----------------------------------------------------------------------
// Tile template grammar
// ----------------------------------------------------------------------
//
// Each row is a string of ASCII tile codes — one char per 16-px tile.
//
//   '.' — plain floor (template chooses which: wood/stone/checker)
//   '#' — plain wall
//   'W' — wall with a window (north wall accent)
//   'B' — wall with a banner / picture (north wall accent)
//   'T' — wall with a torch/sconce
//   'A' — stone archway (visual centerpiece on north wall)
//   'D' — exit door (placed BELOW the bottom wall, replaces a wall tile)
//   's' — stairs (no-op for now)
//   'r' — rug, single-tile solid centerpiece
//   'L' — rug, 3-tile-wide LEFT cap (use with 'M' + 'R')
//   'M' — rug, 3-tile-wide MIDDLE
//   'R' — rug, 3-tile-wide RIGHT cap
//
// Props live in a separate `props` map keyed "x,y" → name. Props don't
// block tiles; they layer on top of whatever floor/rug is below.
interface InteriorTemplate {
  width: number;
  height: number;
  /** Theme — picks floor + wall tile palette. */
  theme: "wood" | "stone" | "town_hall";
  /** Base layout (floor / wall / rug / door codes). */
  tiles: string[];
  /** Sparse map of x,y → prop name, with optional width-in-tiles for
   *  multi-tile props (king bed, long table). */
  props: Record<string, { name: string; widthTiles?: number; from2w?: boolean }>;
}

// ----------------------------------------------------------------------
// Theme palette — which tile sprite a code resolves to.
// ----------------------------------------------------------------------

interface ThemePalette {
  floor: string;
  wall: string;
  wallWindow: string;
  wallBanner: string;
  wallTorch: string;
  wallArch: string;
  rugSolid: string;
  rugLeft: string;
  rugMid: string;
  rugRight: string;
  door: string;
}

const THEME_WOOD: ThemePalette = {
  floor: "floor_wood_medium",
  wall: "wall_wood_light",
  wallWindow: "wall_wood_window",
  wallBanner: "wall_wood_picture",
  wallTorch: "wall_wood_picture",
  wallArch: "wall_wood_picture",
  rugSolid: "rug_red_solid",
  rugLeft: "rug_red_l",
  rugMid: "rug_red_m",
  rugRight: "rug_red_r",
  door: "door_wood_plain",
};

const THEME_STONE: ThemePalette = {
  ...THEME_WOOD,
  floor: "floor_stone_large",
  wall: "wall_stone",
  wallWindow: "wall_stone_arch_window",
  wallBanner: "wall_stone_banner",
  wallTorch: "wall_stone_torch",
  wallArch: "wall_stone_arch_window",
  door: "door_iron_bound",
};

const THEME_TOWN_HALL: ThemePalette = {
  ...THEME_WOOD,
  floor: "floor_checker",
  door: "door_wood_double",
};

const THEME: Record<InteriorTemplate["theme"], ThemePalette> = {
  wood: THEME_WOOD,
  stone: THEME_STONE,
  town_hall: THEME_TOWN_HALL,
};

// ----------------------------------------------------------------------
// Hand-authored rooms.
// ----------------------------------------------------------------------
//
// Layout convention:
//   - Outer walls: top row and sides are '#' / 'W' / 'B' / 'T' / 'A'
//   - Top wall has WINDOWS spaced rhythmically (every 2-3 tiles), with
//     a banner or arch as a centerpiece
//   - Door 'D' sits at the bottom-center, IN the wall row
//   - A 3-tile-wide rug ('L','M','R') runs across the middle of the
//     room with the dining table centered on it
//   - Furniture clusters pressed against walls, symmetric L-R where
//     reasonable, props layered on top via the props map
//
// Coordinates in the props map are (x, y) measured in tile units from
// the top-left of the template grid — same coordinate system as tiles.

const COTTAGE: InteriorTemplate = {
  width: 14, height: 10, theme: "wood",
  tiles: [
    "##WW##BB##WW##",  // top wall: windows / centerpiece picture / windows
    "#............#",
    "#............#",
    "#............#",
    "#...LMMMR....#",  // 5-tile rug centered horizontally
    "#............#",
    "#............#",
    "#............#",
    "#............#",
    "#####DD#######",  // door at bottom-center
  ],
  props: {
    // North-west corner: cooking cluster (fireplace + cauldron + barrel)
    "1,1": { name: "fireplace_lit" },
    "2,2": { name: "cooking_pot_fire" },
    "1,4": { name: "barrel" },
    "1,5": { name: "flour_sack" },
    // North-east corner: sleeping cluster (bed + chest + nightstand)
    "12,1": { name: "bed_red" },
    "11,1": { name: "side_table_candlestick" },
    "12,3": { name: "chest_closed" },
    // West wall: living (bookshelf + reading chair)
    "1,7": { name: "bookshelf" },
    "2,7": { name: "padded_stool" },
    // East wall: storage / dressing (cabinet + mirror)
    "12,6": { name: "cabinet_closed" },
    "12,7": { name: "mirror_oval" },
    // Center on rug: dining table + symmetric chairs
    "6,4": { name: "round_table_cloth" },
    "5,4": { name: "chair_backrest" },
    "8,4": { name: "chair_backrest" },
    // Vase + lanterns for warmth
    "7,1": { name: "vase_flowers" },
    "1,8": { name: "lantern_hanging_lit" },
    "12,8": { name: "lantern_hanging_lit" },
  },
};

const TAVERN: InteriorTemplate = {
  width: 16, height: 12, theme: "wood",
  tiles: [
    "##WW##BB##BB##WW",
    "#..............#",
    "#..............#",
    "#..............#",
    "#..............#",
    "#...LMMMMMR....#",
    "#..............#",
    "#..............#",
    "#..............#",
    "#..............#",
    "#..............#",
    "#######DD#######",
  ],
  props: {
    // North wall: stocked storage (barrels of ale, bottle cabinet)
    "2,1": { name: "barrel" }, "3,1": { name: "barrel" },
    "12,1": { name: "wine_bottle_goblet" }, "13,1": { name: "cabinet_open_bottles" },
    // Continuous bar counter — 6 tiles of bar_counter_mug reads as ONE long bar
    "5,2": { name: "bar_counter_mug" }, "6,2": { name: "bar_counter_mug" },
    "7,2": { name: "bar_counter_mug" }, "8,2": { name: "bar_counter_mug" },
    "9,2": { name: "bar_counter_mug" }, "10,2": { name: "bar_counter_mug" },
    // Padded stools tucked in front of the bar
    "5,3": { name: "padded_stool" }, "7,3": { name: "padded_stool" }, "9,3": { name: "padded_stool" },
    // Symmetric dining clusters on the runner side
    "2,5": { name: "round_table_cloth" }, "1,5": { name: "chair_backrest" }, "3,5": { name: "chair_backrest" },
    "13,5": { name: "round_table_cloth" }, "12,5": { name: "chair_backrest" }, "14,5": { name: "chair_backrest" },
    // Shared long-bench centered ON the runner
    "7,5": { name: "long_bench" }, "8,5": { name: "long_bench" },
    // Two more dining clusters lower down
    "3,8": { name: "round_table_cloth" }, "2,8": { name: "chair_backrest" }, "4,8": { name: "chair_backrest" },
    "12,8": { name: "round_table_cloth" }, "11,8": { name: "chair_backrest" }, "13,8": { name: "chair_backrest" },
    // Fireplace nook east, archive nook west
    "14,10": { name: "fireplace_lit" },
    "1,8": { name: "bookshelf" }, "1,10": { name: "chest_closed" },
    // Lanterns flanking
    "1,1": { name: "lantern_hanging_lit" }, "14,1": { name: "lantern_hanging_lit" },
  },
};

const BLACKSMITH: InteriorTemplate = {
  width: 14, height: 10, theme: "stone",
  tiles: [
    "##TT##BB##TT##",
    "#............#",
    "#............#",
    "#............#",
    "#....LMMMR...#",
    "#............#",
    "#............#",
    "#............#",
    "#............#",
    "#####DD#######",
  ],
  props: {
    // North wall: 4-section forge with stone fireplaces flanked by material barrels
    "5,1": { name: "fireplace_lit" }, "6,1": { name: "fireplace_lit" },
    "7,1": { name: "fireplace_lit" }, "8,1": { name: "fireplace_lit" },
    "3,1": { name: "barrel" }, "4,1": { name: "barrel" },
    "9,1": { name: "barrel" }, "10,1": { name: "barrel" },
    // Work area: dual anvils on the rug with a continuous 4-tile workbench behind
    "6,4": { name: "anvil_sheet" }, "8,4": { name: "anvil_sheet" },
    "5,5": { name: "long_bench" }, "6,5": { name: "long_bench" },
    "7,5": { name: "long_bench" }, "8,5": { name: "long_bench" },
    // Smith's living quarters along west wall
    "1,2": { name: "cot_straw" }, "1,4": { name: "chest_closed" },
    "1,7": { name: "bookshelf" }, "1,8": { name: "chest_closed" },
    // Workshop tools along east wall
    "12,2": { name: "writing_desk_quill" },
    "12,4": { name: "cabinet_closed" },
    "12,6": { name: "crate_rope" }, "12,7": { name: "crate_rope" }, "12,8": { name: "crate_rope" },
    // Floor lanterns flanking the door
    "3,8": { name: "lantern_hanging_lit" }, "10,8": { name: "lantern_hanging_lit" },
  },
};

const TOWN_HALL: InteriorTemplate = {
  width: 18, height: 12, theme: "town_hall",
  tiles: [
    "##WW##BB##BB##WW##",
    "#................#",
    "#................#",
    "#................#",
    "#................#",
    "#....LMMMMMMMR...#",
    "#................#",
    "#................#",
    "#................#",
    "#................#",
    "#................#",
    "########DD########",
  ],
  props: {
    // Mayor's desk centered on the north wall
    "8,1": { name: "writing_desk_quill" }, "9,1": { name: "writing_desk_quill" },
    "7,1": { name: "candelabra" }, "10,1": { name: "candelabra" },
    "6,1": { name: "vase_flowers" }, "11,1": { name: "vase_flowers" },
    "4,1": { name: "painting" }, "13,1": { name: "painting" },
    // Mayor's chair-of-state behind the desk
    "8,2": { name: "chair_backrest" }, "9,2": { name: "chair_backrest" },
    // Assembly seating — chairs tucked immediately above and below the runner
    "6,4": { name: "chair_backrest" }, "8,4": { name: "chair_backrest" },
    "10,4": { name: "chair_backrest" }, "12,4": { name: "chair_backrest" },
    "5,5": { name: "long_bench" }, "12,5": { name: "long_bench" },
    "6,5": { name: "round_table_cloth" }, "8,5": { name: "round_table_cloth" }, "10,5": { name: "round_table_cloth" },
    "6,6": { name: "chair_backrest" }, "8,6": { name: "chair_backrest" },
    "10,6": { name: "chair_backrest" }, "12,6": { name: "chair_backrest" },
    // Archive west wall
    "1,3": { name: "bookshelf" }, "1,5": { name: "bookshelf" },
    "1,7": { name: "scroll_stand" }, "1,9": { name: "chest_closed" },
    // Ceremonial east wall
    "16,3": { name: "tapestry" }, "16,5": { name: "tapestry" },
    "16,7": { name: "sconce_lit" }, "16,9": { name: "chest_closed" },
    // Hearth + cabinet flanking the door area
    "3,10": { name: "fireplace_lit" }, "14,10": { name: "cabinet_closed" },
  },
};

const GRANARY: InteriorTemplate = {
  width: 12, height: 8, theme: "wood",
  tiles: [
    "##WW##BB##WW",
    "#..........#",
    "#..........#",
    "#..........#",
    "#..........#",
    "#..........#",
    "#..........#",
    "####DD######",
  ],
  props: {
    "1,1": { name: "barrel" }, "2,1": { name: "barrel" }, "3,1": { name: "barrel" },
    "8,1": { name: "barrel" }, "9,1": { name: "barrel" }, "10,1": { name: "barrel" },
    "1,3": { name: "crate_rope" }, "2,3": { name: "crate_rope" }, "3,3": { name: "crate_rope" },
    "8,3": { name: "crate_rope" }, "9,3": { name: "crate_rope" }, "10,3": { name: "crate_rope" },
    "5,3": { name: "flour_sack" }, "6,3": { name: "flour_sack" },
    "1,5": { name: "writing_desk_quill" }, "2,5": { name: "padded_stool" },
    "10,5": { name: "chest_closed" }, "9,5": { name: "lantern_hanging_lit" },
    "5,5": { name: "round_table_cloth" }, "6,5": { name: "round_table_cloth" },
  },
};

const TEMPLATES: Record<string, InteriorTemplate> = {
  "bld:000": COTTAGE,
  "bld:001": COTTAGE,
  "bld:004": TAVERN,
  "bld:005": COTTAGE,
  "bld:blacksmith": BLACKSMITH,
  "bld:town_hall": TOWN_HALL,
  "bld:granary": GRANARY,
  "bld:watchtower": COTTAGE,
};

export class InteriorLayer {
  readonly container: Container;
  private exitHandlers: Array<() => void> = [];
  private upFloorHandlers: Array<() => void> = [];

  private keyHandler: ((e: KeyboardEvent) => void) | null = null;

  constructor(private app: Application) {
    this.container = new Container();
    this.container.label = "interior";
    this.container.visible = false;
    this.container.eventMode = "static";
    this.keyHandler = (e: KeyboardEvent) => {
      if (e.key === "Escape" && this.container.visible) {
        e.stopPropagation();
        for (const h of this.exitHandlers) h();
      }
    };
    window.addEventListener("keydown", this.keyHandler, true);
  }

  destroy(): void {
    if (this.keyHandler) {
      window.removeEventListener("keydown", this.keyHandler, true);
      this.keyHandler = null;
    }
  }

  onExit(h: () => void): () => void {
    this.exitHandlers.push(h);
    return () => {
      const i = this.exitHandlers.indexOf(h);
      if (i >= 0) this.exitHandlers.splice(i, 1);
    };
  }

  onGoUpFloor(h: () => void): () => void {
    this.upFloorHandlers.push(h);
    return () => {
      const i = this.upFloorHandlers.indexOf(h);
      if (i >= 0) this.upFloorHandlers.splice(i, 1);
    };
  }

  async show(buildingSprite: string): Promise<void> {
    this.clear();
    const tpl = TEMPLATES[buildingSprite] ?? COTTAGE;
    await this.render(tpl, buildingSprite);

    // Center the interior in the screen. Pick the largest integer scale
    // that still fits the room + name banner + hint text comfortably.
    const cx = this.app.screen.width / 2;
    const cy = this.app.screen.height / 2;
    const scale = Math.min(
      this.app.screen.width / (tpl.width * TILE_SIZE_PX + 64),
      this.app.screen.height / (tpl.height * TILE_SIZE_PX + 140),
    );
    this.container.scale.set(Math.max(2, Math.floor(scale)));
    this.container.x = cx - (tpl.width * TILE_SIZE_PX * this.container.scale.x) / 2;
    this.container.y = cy - (tpl.height * TILE_SIZE_PX * this.container.scale.y) / 2;

    this.container.visible = true;
    this.container.alpha = 0;
    this.fadeIn(220);
  }

  hide(): void {
    this.fadeOut(180, () => {
      this.container.visible = false;
      this.clear();
    });
  }

  private fadeIn(ms: number): void {
    const start = performance.now();
    const tick = () => {
      const t = Math.min(1, (performance.now() - start) / ms);
      this.container.alpha = t;
      if (t < 1) requestAnimationFrame(tick);
    };
    tick();
  }

  private fadeOut(ms: number, then: () => void): void {
    const start = performance.now();
    const begin = this.container.alpha;
    const tick = () => {
      const t = Math.min(1, (performance.now() - start) / ms);
      this.container.alpha = begin * (1 - t);
      if (t < 1) requestAnimationFrame(tick);
      else then();
    };
    tick();
  }

  private clear(): void {
    for (const c of [...this.container.children]) c.destroy();
  }

  private async render(tpl: InteriorTemplate, buildingSprite: string): Promise<void> {
    const theme = THEME[tpl.theme];

    // Backdrop dim of the overworld.
    const dim = new Graphics();
    dim.rect(
      -this.app.screen.width, -this.app.screen.height,
      this.app.screen.width * 3, this.app.screen.height * 3,
    ).fill({ color: 0x000000, alpha: 0.6 });
    this.container.addChild(dim);

    const tileBox = new Container();
    this.container.addChild(tileBox);

    // Resolve every distinct tile texture we need up front, in parallel.
    const tileNames = new Set<string>([
      theme.floor, theme.wall, theme.wallWindow, theme.wallBanner,
      theme.wallTorch, theme.wallArch,
      theme.rugSolid, theme.rugLeft, theme.rugMid, theme.rugRight,
      theme.door,
    ]);
    const tileTex: Record<string, Texture> = {};
    await Promise.all(
      [...tileNames].map(async (name) => {
        const t = await Assets.load<Texture>(TILE(name));
        t.source.scaleMode = "nearest";
        tileTex[name] = t;
      }),
    );

    // Tile layer pass.
    for (let y = 0; y < tpl.height; y++) {
      const row = tpl.tiles[y] ?? "";
      for (let x = 0; x < row.length; x++) {
        const ch = row[x];
        // Always paint floor underneath everything except solid walls,
        // so rugs/doors get a continuous floor backdrop and props don't
        // sit on raw black.
        const isWall = ch === "#" || ch === "W" || ch === "B" || ch === "T" || ch === "A";
        if (!isWall) {
          paintTile(tileBox, tileTex[theme.floor], x, y);
        }
        let tileName: string | null = null;
        switch (ch) {
          case "#": tileName = theme.wall; break;
          case "W": tileName = theme.wallWindow; break;
          case "B": tileName = theme.wallBanner; break;
          case "T": tileName = theme.wallTorch; break;
          case "A": tileName = theme.wallArch; break;
          case "L": tileName = theme.rugLeft; break;
          case "M": tileName = theme.rugMid; break;
          case "R": tileName = theme.rugRight; break;
          case "r": tileName = theme.rugSolid; break;
          case "D": tileName = theme.floor; break;  // door tile gets a floor underneath, door drawn later
        }
        if (tileName && tileName !== theme.floor) {
          paintTile(tileBox, tileTex[tileName], x, y);
        }
      }
    }

    // Prop pass — load each unique prop once, then place every instance.
    const propNames = new Set<string>();
    for (const meta of Object.values(tpl.props)) propNames.add(meta.name);
    const propTex: Record<string, Texture> = {};
    await Promise.all(
      [...propNames].map(async (name) => {
        const url =
          isProp2W(name) ? PROP2W(name) :
          SHEET_PROPS.has(name) ? LEGACY_TILE(name) :
          PROP(name);
        try {
          const t = await Assets.load<Texture>(url);
          t.source.scaleMode = "nearest";
          propTex[name] = t;
        } catch (e) {
          console.warn(`interior prop load failed: ${name}`, e);
        }
      }),
    );

    for (const [coord, meta] of Object.entries(tpl.props)) {
      const tex = propTex[meta.name];
      if (!tex) continue;
      const [xs, ys] = coord.split(",");
      const x = Number(xs);
      const y = Number(ys);
      const widthTiles = meta.widthTiles ?? 1;
      const sp = new Sprite(tex);
      // Bottom-center anchor on its tile footprint so props look like
      // they're "sitting on" the floor; tall furniture (cabinets, beds)
      // extends upward, never downward into adjacent floor.
      const targetW = widthTiles * TILE_SIZE_PX;
      const aspect = tex.width / tex.height;
      const targetH = targetW / aspect;
      sp.width = targetW;
      sp.height = targetH;
      sp.x = x * TILE_SIZE_PX;
      sp.y = (y + 1) * TILE_SIZE_PX - targetH;
      tileBox.addChild(sp);
    }

    // Door pass — the 'D' tile renders the theme door sprite over the
    // floor backdrop. Clicking it (or pressing Esc) exits the interior.
    for (let y = 0; y < tpl.height; y++) {
      const row = tpl.tiles[y] ?? "";
      for (let x = 0; x < row.length; x++) {
        if (row[x] !== "D") continue;
        const tex = tileTex[theme.door];
        const door = new Sprite(tex);
        door.x = x * TILE_SIZE_PX;
        door.y = y * TILE_SIZE_PX;
        door.width = TILE_SIZE_PX;
        door.height = TILE_SIZE_PX;
        door.eventMode = "static";
        door.cursor = "pointer";
        const glow = new OutlineFilter({ thickness: 1.5, color: 0xfff2a8, alpha: 0.9 });
        door.on("pointerover", () => { door.filters = [glow]; });
        door.on("pointerout", () => { door.filters = []; });
        door.on("pointertap", () => {
          for (const h of this.exitHandlers) h();
        });
        tileBox.addChild(door);
      }
    }

    // Building name banner and exit hint.
    const banner = new Text({
      text: prettyName(buildingSprite),
      style: {
        fontFamily: "ui-sans-serif, system-ui, sans-serif",
        fontSize: 9, fill: 0xfff2a8, align: "center",
        stroke: { color: 0x181425, width: 3 },
        fontWeight: "700",
      },
    });
    banner.resolution = 4;
    banner.x = (tpl.width * TILE_SIZE_PX) / 2 - banner.width / 2;
    banner.y = -14;
    tileBox.addChild(banner);

    const hint = new Text({
      text: "click the door to leave  ·  press esc",
      style: {
        fontFamily: "ui-sans-serif, system-ui, sans-serif",
        fontSize: 6, fill: 0xead4aa,
      },
    });
    hint.resolution = 4;
    hint.x = (tpl.width * TILE_SIZE_PX) / 2 - hint.width / 2;
    hint.y = tpl.height * TILE_SIZE_PX + 4;
    tileBox.addChild(hint);
  }
}

function paintTile(parent: Container, tex: Texture, x: number, y: number): void {
  const sp = new Sprite(tex);
  sp.x = x * TILE_SIZE_PX;
  sp.y = y * TILE_SIZE_PX;
  sp.width = TILE_SIZE_PX;
  sp.height = TILE_SIZE_PX;
  parent.addChild(sp);
}

// The 2-tile-wide furniture pieces sliced from interior_tileset.png live
// in their own subfolder and use a different loader path.
function isProp2W(name: string): boolean {
  return name === "bed_blue_2w" || name === "bed_red_king" || name === "table_food_2w";
}

function prettyName(sprite: string): string {
  switch (sprite) {
    case "bld:000": return "Cottage";
    case "bld:001": return "Cottage";
    case "bld:004": return "The Hollow Tankard";
    case "bld:005": return "Market Stall";
    case "bld:blacksmith": return "The Forge";
    case "bld:town_hall": return "Town Hall";
    case "bld:granary": return "Granary";
    case "bld:watchtower": return "Watchtower";
    default: return "Interior";
  }
}
