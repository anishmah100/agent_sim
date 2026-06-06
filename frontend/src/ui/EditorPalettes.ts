// Curated editor palettes for the world editor's decoration tab.
//
// Each entry is a sprite the user can drop on the world. Footprint +
// height come baked in so a click on the canvas POSTs an engine-ready
// decoration record. Walkability defaults match the spirit of the
// sprite (buildings block, items don't).

export interface PaletteEntry {
  sprite: string;
  label: string;
  /** Render height in TILES (matches DecorationSpec.height_tiles). */
  height_tiles: number;
  footprint_w: number;
  footprint_h: number;
  walkable: boolean;
}

// Buildings — taken from the catalog's bld:* and stall:* namespaces.
// Order is rough "common → rare" for editor scan-ability.
export const BUILDING_PALETTE: PaletteEntry[] = [
  { sprite: "bld:000",        label: "Cottage A",   height_tiles: 4, footprint_w: 5, footprint_h: 2, walkable: false },
  { sprite: "bld:001",        label: "Cottage B",   height_tiles: 4, footprint_w: 5, footprint_h: 2, walkable: false },
  { sprite: "bld:blacksmith", label: "Blacksmith",  height_tiles: 4, footprint_w: 5, footprint_h: 2, walkable: false },
  { sprite: "bld:granary",    label: "Granary",     height_tiles: 4, footprint_w: 4, footprint_h: 2, walkable: false },
  { sprite: "bld:town_hall",  label: "Town hall",   height_tiles: 4, footprint_w: 6, footprint_h: 2, walkable: false },
  { sprite: "bld:watchtower", label: "Watchtower",  height_tiles: 5, footprint_w: 2, footprint_h: 2, walkable: false },
  { sprite: "bld:well",       label: "Well",        height_tiles: 1.5, footprint_w: 1, footprint_h: 1, walkable: false },
  // Stalls — single-tile footprint, low height (open-air market goods)
  { sprite: "bld:stall_red_bread_open",   label: "Stall — bread",  height_tiles: 2, footprint_w: 2, footprint_h: 1, walkable: false },
  { sprite: "bld:stall_blue_fruit_open",  label: "Stall — fruit",  height_tiles: 2, footprint_w: 2, footprint_h: 1, walkable: false },
  { sprite: "bld:stall_green_meat_open",  label: "Stall — meat",   height_tiles: 2, footprint_w: 2, footprint_h: 1, walkable: false },
];

// Items — pulled from the v2_items_master_v2 sheet via the sprite
// catalog's item:NAME ids. Walkable=true so they don't block agents.
export const ITEM_PALETTE: PaletteEntry[] = [
  // Food
  { sprite: "item:apple",         label: "Apple",         height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:bread_loaf",    label: "Bread loaf",    height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:cheese_wheel",  label: "Cheese wheel",  height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:fish_cooked",   label: "Fish (cooked)", height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  // Weapons
  { sprite: "item:dagger",        label: "Dagger",        height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:sword_short",   label: "Sword (short)", height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:axe",           label: "Axe",           height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:hammer",        label: "Hammer",        height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:bow",           label: "Bow",           height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  // Armor
  { sprite: "item:helmet_iron",        label: "Iron helmet",   height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:chestplate_iron",    label: "Iron plate",    height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:boots_leather",      label: "Leather boots", height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:cloak_folded",       label: "Cloak",         height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  // Currency / valuables
  { sprite: "item:coin_single",        label: "Coin",          height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:coins_small_pile",   label: "Coin pile",     height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:gem_emerald",        label: "Emerald",       height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:gem_ruby",           label: "Ruby",          height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:gem_sapphire",       label: "Sapphire",      height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:chalice_gold",       label: "Gold chalice",  height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  // Tools / material
  { sprite: "item:bucket_water",       label: "Water bucket",  height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:fishing_rod",        label: "Fishing rod",   height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:compass",            label: "Compass",       height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:wood_log",           label: "Wood log",      height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
  { sprite: "item:coal",               label: "Coal",          height_tiles: 1, footprint_w: 1, footprint_h: 1, walkable: true },
];

export type EditorCategory = "tile" | "building" | "item";
