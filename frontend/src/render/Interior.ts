// Interior view — Pokemon-style. When the user clicks a building on
// the overworld, this layer fades in showing a small interior tilemap.
// Currently a single hand-authored 10×8 wooden cottage template; later
// each building type points at its own .json layout.
//
// UX:
//   - Fade in over 250 ms
//   - Floor / walls / decor render in the same Pixi viewport
//   - A clickable door tile at the bottom-center exits back to the
//     overworld (fade out, restore overworld layers' visibility)
//   - A floor button at the top-right takes you "up a floor" (no-op
//     placeholder for now)

import { Application, Assets, Container, Graphics, Sprite, Text, Texture } from "pixi.js";
import { OutlineFilter } from "pixi-filters";
import { TILE_SIZE_PX } from "./tiles";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

interface InteriorTemplate {
  width: number;
  height: number;
  /** Char-per-tile, '.' = floor, '#' = wall, 'D' = exit door,
   *  's' = stairs, 't' = table, 'c' = chair, 'b' = bed. */
  tiles: string[];
}

// COTTAGE — uses v2 interior props:
//   t=table  c=chair  b=bed  f=lit-fireplace  r=rug
//   n=barrel h=chest  k=bookshelf  P=painting  l=lantern
//   D=exit door
const COTTAGE: InteriorTemplate = {
  width: 12,
  height: 8,
  tiles: [
    "############",
    "#l........l#",
    "#.f...h..k.#",
    "#..t..b....#",
    "#..c..r....#",
    "#......P...#",
    "#####DD#####",
    "############",
  ],
};

const TAVERN: InteriorTemplate = {
  width: 14,
  height: 10,
  tiles: [
    "##############",
    "#l..........l#",
    "#.t.t.t..p..s#",
    "#.c.c.c..f...#",
    "#............#",
    "#.t.t.t..n.h.#",
    "#.c.c.c..k...#",
    "#............#",
    "######DD######",
    "##############",
  ],
};

// BLACKSMITH interior — anvil + forge fire + storage + bed for the smith
const BLACKSMITH: InteriorTemplate = {
  width: 12,
  height: 8,
  tiles: [
    "############",
    "#f....h..k.#",
    "#......n...#",
    "#..p.......#",
    "#......b...#",
    "#l........l#",
    "#####DD#####",
    "############",
  ],
};

// TOWN HALL interior — large central room with desk, painting, fancy furniture
const TOWN_HALL: InteriorTemplate = {
  width: 16,
  height: 10,
  tiles: [
    "################",
    "#l............l#",
    "#.P..........P.#",
    "#.f....r....k..#",
    "#......r...T...#",
    "#..t.h.r..h....#",
    "#..c....v......#",
    "#..............#",
    "#######DD#######",
    "################",
  ],
};

const TEMPLATES: Record<string, InteriorTemplate> = {
  "bld:000": COTTAGE,
  "bld:001": COTTAGE,
  "bld:004": TAVERN,
  "bld:005": COTTAGE,
  // v2 buildings
  "bld:blacksmith": BLACKSMITH,
  "bld:town_hall": TOWN_HALL,
  "bld:granary": COTTAGE,        // simple storage interior for now
  "bld:watchtower": COTTAGE,     // reuse interior — gameplay later
};

export class InteriorLayer {
  readonly container: Container;
  private exitHandlers: Array<() => void> = [];
  private upFloorHandlers: Array<() => void> = [];

  constructor(private app: Application) {
    this.container = new Container();
    this.container.label = "interior";
    this.container.visible = false;
    this.container.eventMode = "static";
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

    // Center the interior in the screen.
    const cx = this.app.screen.width / 2;
    const cy = this.app.screen.height / 2;
    const scale = Math.min(
      this.app.screen.width / (tpl.width * TILE_SIZE_PX + 64),
      this.app.screen.height / (tpl.height * TILE_SIZE_PX + 120),
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
    // Background dim of the overworld behind.
    const dim = new Graphics();
    dim.rect(
      -this.app.screen.width, -this.app.screen.height,
      this.app.screen.width * 3, this.app.screen.height * 3,
    ).fill({ color: 0x000000, alpha: 0.55 });
    this.container.addChild(dim);

    // Tile container — gets its own scale so the interior renders at
    // crisp integer pixels regardless of the overworld zoom.
    const tileBox = new Container();
    this.container.addChild(tileBox);

    // Load the floor + wall textures from the tile atlas the engine
    // serves. floor_wood = our wooden floor tile; cliff_face = wall.
    const floorTex = await Assets.load<Texture>(
      `${ENGINE_URL}/art/processed/tiles/overworld/dirt.png`,
    );
    const wallTex = await Assets.load<Texture>(
      `${ENGINE_URL}/art/processed/tiles/overworld/cliff_face.png`,
    );
    floorTex.source.scaleMode = "nearest";
    wallTex.source.scaleMode = "nearest";

    for (let y = 0; y < tpl.height; y++) {
      const row = tpl.tiles[y];
      for (let x = 0; x < row.length; x++) {
        const ch = row[x];
        let tex: Texture | null = null;
        if (ch === ".") tex = floorTex;
        else if (ch === "#") tex = wallTex;
        else if (ch === "D") tex = floorTex;  // door floor underneath
        else if (ch === "s") tex = floorTex;
        else tex = floorTex;
        if (!tex) continue;
        const sp = new Sprite(tex);
        sp.x = x * TILE_SIZE_PX;
        sp.y = y * TILE_SIZE_PX;
        sp.width = TILE_SIZE_PX;
        sp.height = TILE_SIZE_PX;
        tileBox.addChild(sp);
      }
    }

    // Props — v2: real pixel-art sprites from the interior_props_master
    // sheet. Each ASCII char maps to one sliced sprite, scaled to fit
    // within the tile bounds with the natural aspect of the source.
    // The processed sprites live at /art/processed/v2_interior_props_master/<name>.png.
    const propMap: Record<string, string> = {
      "t": "small_table",          // small wooden table
      "c": "chair_backrest",       // chair with backrest
      "b": "bed_red",              // red-blanket single bed
      "B": "bed_blue",             // blue-blanket variant
      "r": "rug_wool",             // wool rug
      "f": "fireplace_lit",        // lit fireplace
      "F": "fireplace_unlit",      // unlit fireplace
      "n": "barrel",               // wooden barrel
      "h": "chest_closed",         // treasure chest
      "k": "bookshelf",            // bookshelf
      "p": "cooking_pot_fire",     // cooking pot
      "v": "vase_flowers",         // vase with flowers
      "l": "lantern_hanging_lit",  // lit hanging lantern
      "L": "lantern_hanging_unlit",
      "S": "padded_stool",         // round padded stool
      "m": "mirror_oval",          // mirror
      "P": "painting",             // wall painting
      "T": "tapestry",             // tapestry
    };
    const propPromises: Promise<void>[] = [];
    for (let y = 0; y < tpl.height; y++) {
      const row = tpl.tiles[y];
      for (let x = 0; x < row.length; x++) {
        const ch = row[x];
        const propName = propMap[ch];
        if (!propName) continue;
        const propUrl =
          `${ENGINE_URL}/art/processed/v2_interior_props_master/${propName}.png`;
        const px = x * TILE_SIZE_PX;
        const py = y * TILE_SIZE_PX;
        propPromises.push(
          Assets.load<Texture>(propUrl).then((tex) => {
            tex.source.scaleMode = "nearest";
            const sp = new Sprite(tex);
            // Fit each prop within the tile; preserve aspect ratio.
            // Furniture tends to be taller than wide; let height fill
            // the tile and width auto-scale. The prop's natural
            // anchor is bottom-center on the tile floor.
            const aspect = tex.width / tex.height;
            sp.height = TILE_SIZE_PX;
            sp.width = TILE_SIZE_PX * aspect;
            sp.x = px + (TILE_SIZE_PX - sp.width) / 2;
            sp.y = py;
            tileBox.addChild(sp);
          }).catch((e) => {
            console.warn(`interior prop ${propName} failed:`, e);
          })
        );
      }
    }
    await Promise.all(propPromises);

    // Exit door — clickable interactive tile at the bottom of the
    // interior. Visually a dark wooden door with a gold highlight.
    for (let y = 0; y < tpl.height; y++) {
      const row = tpl.tiles[y];
      for (let x = 0; x < row.length; x++) {
        if (row[x] === "D") {
          const door = new Graphics();
          door.rect(x*16+1, y*16+2, 14, 14)
            .fill(0x4d2e10).stroke({color:0x2a1607, width:1});
          door.circle(x*16+12, y*16+10, 1).fill(0xffcc44);
          door.eventMode = "static";
          door.cursor = "pointer";
          const glow = new OutlineFilter({ thickness: 2, color: 0xfff2a8, alpha: 0.85 });
          door.on("pointerover", () => { door.filters = [glow]; });
          door.on("pointerout", () => { door.filters = []; });
          door.on("pointertap", () => {
            for (const h of this.exitHandlers) h();
          });
          tileBox.addChild(door);
        } else if (row[x] === "s") {
          const stair = new Graphics();
          stair.rect(x*16+1, y*16+1, 14, 14)
            .fill(0xa0a0a0).stroke({color:0x707070, width:1});
          stair.moveTo(x*16+3, y*16+13).lineTo(x*16+13, y*16+3)
            .stroke({color:0x404040, width:2});
          stair.eventMode = "static";
          stair.cursor = "pointer";
          const glow = new OutlineFilter({ thickness: 2, color: 0xfff2a8, alpha: 0.85 });
          stair.on("pointerover", () => { stair.filters = [glow]; });
          stair.on("pointerout", () => { stair.filters = []; });
          stair.on("pointertap", () => {
            for (const h of this.upFloorHandlers) h();
          });
          tileBox.addChild(stair);
        }
      }
    }

    // Building name banner across the top.
    const banner = new Text({
      text: prettyName(buildingSprite),
      style: {
        fontFamily: "monospace",
        fontSize: 6,
        fill: 0xfff2a8,
        align: "center",
      },
    });
    banner.resolution = 4;
    banner.x = (tpl.width * TILE_SIZE_PX) / 2 - banner.width / 2;
    banner.y = -10;
    tileBox.addChild(banner);

    // Hint text bottom.
    const hint = new Text({
      text: "click the door to leave",
      style: {
        fontFamily: "monospace",
        fontSize: 5,
        fill: 0xeeeeee,
      },
    });
    hint.resolution = 4;
    hint.x = (tpl.width * TILE_SIZE_PX) / 2 - hint.width / 2;
    hint.y = tpl.height * TILE_SIZE_PX + 4;
    tileBox.addChild(hint);
  }
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
