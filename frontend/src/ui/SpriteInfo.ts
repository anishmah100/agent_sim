// SpriteInfo — turn a sprite id into a human-readable title + kind +
// description + stat block for the InfoPanel.
//
// The sprite catalog stores paths, sizes, and `enterable` flags — but
// no descriptions. The engine's rulebook only stats 7 items. So this
// file fills the gap: it derives info from sprite ids and pads with
// plausible stat values calibrated to the rulebook's units so an info
// panel feels grounded:
//
//   satiety: 0..1   (apple=0.25, loaf=0.5 in rulebook)
//   damage:  ~5..25 (iron_sword=15)
//   defense: ~2..15 (wooden_shield=5)
//   gold:    coin value
//   weight:  kg
//
// Special cases live in small tables (HEADLINE_DESCRIPTIONS,
// ITEM_STATS) so adding a new sprite is one line, not a code change.

import { artCatalog } from "../render/ArtCatalog";

export interface SpriteStat {
  label: string;
  value: string;
}

export interface SpriteInfo {
  sprite: string;
  title: string;
  kind: string;
  description: string;
  /** Mechanical stats — damage, satiety, gold, etc. Empty if N/A. */
  stats: SpriteStat[];
  thumbUrl: string | null;
  enterable: boolean;
}

const titleCase = (s: string) =>
  s.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());

const MODIFIER_SUFFIXES = new Set([
  "short", "long", "small", "large", "single", "open", "closed",
  "iron", "leather", "wood", "gold", "silver", "stone", "empty",
  "water", "cooked", "raw", "folded", "pile", "emerald", "ruby",
  "sapphire", "medium",
]);

function splitTitleModifier(name: string): [string, string | null] {
  const idx = name.lastIndexOf("_");
  if (idx < 0) return [name, null];
  const tail = name.slice(idx + 1);
  if (MODIFIER_SUFFIXES.has(tail)) return [name.slice(0, idx), tail];
  return [name, null];
}

function prettyTitle(_category: string, name: string): string {
  if (name.startsWith("stall_")) {
    const rest = name.slice("stall_".length);
    const parts = rest.split("_");
    const state = parts[parts.length - 1];
    const goods = parts.slice(0, -1).join(" ");
    if (state === "open" || state === "closed") {
      return `Stall — ${titleCase(goods)} (${titleCase(state)})`;
    }
    return `Stall — ${titleCase(rest)}`;
  }
  const stageMatch = name.match(/^(.+?)_stage_(\d+)_(.+)$/);
  if (stageMatch) {
    return `${titleCase(stageMatch[1])} — Stage ${stageMatch[2]} (${titleCase(stageMatch[3])})`;
  }
  const [base, mod] = splitTitleModifier(name);
  if (mod) return `${titleCase(base)} — ${titleCase(mod)}`;
  return titleCase(name);
}

const HEADLINE_DESCRIPTIONS: Record<string, string> = {
  "bld:well":
    "Stone well. Townsfolk draw water here; an agent can fill a bucket or quench thirst.",
  "bld:granary":
    "Communal granary — stores the town's grain reserves. Bread, flour, and seed sacks are kept inside.",
  "bld:blacksmith":
    "Blacksmith's forge. Iron tools, weapons, and armor are repaired and sold here.",
  "bld:town_hall":
    "Town hall — civic seat. The mayor holds court inside; notices are posted on the door.",
  "bld:watchtower":
    "Watchtower. A guard stands at the top with a clear sightline over the surrounding roads.",
};

const HEADLINE_STATS: Record<string, SpriteStat[]> = {
  "bld:well":      [{ label: "Function", value: "Refills water buckets" }],
  "bld:granary":   [{ label: "Capacity", value: "~500 sacks of grain" }],
  "bld:blacksmith": [{ label: "Function", value: "Repairs + sells weapons & armor" }],
  "bld:town_hall": [{ label: "Function", value: "Civic seat — court + announcements" }],
  "bld:watchtower": [{ label: "Sightline", value: "~30 tiles in all directions" }],
};

const STALL_GOODS_HINT: Record<string, string> = {
  bread: "Fresh loaves and baked goods on offer.",
  fruit: "Apples, pears, and seasonal produce.",
  meat:  "Cured meats and fresh game.",
  fish:  "The day's catch from the river.",
};

// Per-item stats — calibrated to the rulebook's existing scales.
// Numbers are mostly canonical but a few are best-fit estimates.
// Keep this table as the single source of truth for in-panel stats.
const ITEM_STATS: Record<string, SpriteStat[]> = {
  // Food (satiety 0..1)
  apple:         [{ label: "Satiety", value: "+0.25" }, { label: "Weight", value: "0.2 kg" }],
  bread_loaf:    [{ label: "Satiety", value: "+0.50" }, { label: "Weight", value: "0.3 kg" }],
  cheese_wheel:  [{ label: "Satiety", value: "+0.70" }, { label: "Weight", value: "1.5 kg" }],
  fish_raw:      [{ label: "Satiety", value: "+0.20" }, { label: "Note", value: "Cook for full nutrition" }],
  fish_cooked:   [{ label: "Satiety", value: "+0.55" }, { label: "Weight", value: "0.4 kg" }],

  // Weapons (damage matches iron_sword=15 baseline)
  dagger:        [{ label: "Damage", value: "8" }, { label: "Weight", value: "0.8 kg" }, { label: "Style", value: "Quick, light" }],
  sword_short:   [{ label: "Damage", value: "12" }, { label: "Weight", value: "1.8 kg" }, { label: "Style", value: "One-handed" }],
  sword_long:    [{ label: "Damage", value: "18" }, { label: "Weight", value: "2.6 kg" }, { label: "Style", value: "Two-handed" }],
  axe:           [{ label: "Damage", value: "16" }, { label: "Weight", value: "2.4 kg" }, { label: "Style", value: "Heavy chop" }],
  club_wood:     [{ label: "Damage", value: "6" }, { label: "Weight", value: "1.5 kg" }, { label: "Style", value: "Blunt" }],
  hammer:        [{ label: "Damage", value: "10" }, { label: "Weight", value: "2.0 kg" }, { label: "Note", value: "Doubles as a forge tool" }],
  bow:           [{ label: "Damage", value: "14" }, { label: "Range", value: "12 tiles" }, { label: "Note", value: "Needs arrows" }],
  crossbow:      [{ label: "Damage", value: "20" }, { label: "Range", value: "14 tiles" }, { label: "Note", value: "Slow to reload" }],

  // Armor (defense matches wooden_shield=5 baseline)
  helmet_leather:     [{ label: "Defense", value: "+2" }, { label: "Weight", value: "0.7 kg" }],
  helmet_iron:        [{ label: "Defense", value: "+5" }, { label: "Weight", value: "2.5 kg" }],
  chestplate_leather: [{ label: "Defense", value: "+4" }, { label: "Weight", value: "3.0 kg" }],
  chestplate_iron:    [{ label: "Defense", value: "+9" }, { label: "Weight", value: "9.0 kg" }],
  boots_leather:      [{ label: "Defense", value: "+1" }, { label: "Note", value: "Quiet step" }],
  cloak_folded:       [{ label: "Defense", value: "+1" }, { label: "Note", value: "Warm, modest cover" }],

  // Currency (gold = coin value)
  coin_single:        [{ label: "Worth", value: "1 gold" }],
  coins_small_pile:   [{ label: "Worth", value: "~10 gold" }],
  coins_large_pile:   [{ label: "Worth", value: "~50 gold" }],

  // Valuables
  gem_emerald:        [{ label: "Worth", value: "120 gold" }, { label: "Note", value: "Rare" }],
  gem_ruby:           [{ label: "Worth", value: "150 gold" }, { label: "Note", value: "Rare" }],
  gem_sapphire:       [{ label: "Worth", value: "130 gold" }, { label: "Note", value: "Rare" }],
  chalice_gold:       [{ label: "Worth", value: "80 gold" }, { label: "Weight", value: "1.2 kg" }],

  // Tools / utility
  bucket_empty:       [{ label: "Capacity", value: "1 unit of water" }],
  bucket_water:       [{ label: "Holds", value: "Water" }, { label: "Use", value: "Drink, douse, water crops" }],
  bottle_empty:       [{ label: "Capacity", value: "Small (drink/oil/note)" }],
  fishing_rod:        [{ label: "Use", value: "Catch fish at rivers + ponds" }],
  compass:            [{ label: "Use", value: "Points to magnetic north" }],

  // Material
  wood_log:           [{ label: "Worth", value: "2 gold" }, { label: "Use", value: "Building material / fuel" }],
  coal:               [{ label: "Worth", value: "3 gold" }, { label: "Use", value: "Forge / hearth fuel" }],
};

interface ItemCategoryRule {
  match: (name: string) => boolean;
  kind: string;
  describe: (name: string) => string;
}
const ITEM_RULES: ItemCategoryRule[] = [
  {
    match: (n) => /sword|dagger|axe|bow|crossbow|club|hammer/.test(n),
    kind: "Item / Weapon",
    describe: (n) => `${titleCase(n.replace(/_/g, " "))}. A weapon — agents can wield it in combat.`,
  },
  {
    match: (n) => /chestplate|helmet|boots|cloak/.test(n),
    kind: "Item / Armor",
    describe: (n) => `${titleCase(n.replace(/_/g, " "))}. Worn for protection.`,
  },
  {
    match: (n) => /apple|bread|cheese|fish_cooked|fish_raw/.test(n),
    kind: "Item / Food",
    describe: (n) => `${titleCase(n.replace(/_/g, " "))}. Edible — restores hunger.`,
  },
  {
    match: (n) => /^coin|coins_/.test(n),
    kind: "Item / Currency",
    describe: (n) =>
      n.includes("large") ? "A heavy purse of coins — significant wealth."
      : n.includes("small") ? "A small pouch of coins — pocket change."
      : "A single gold coin.",
  },
  {
    match: (n) => /^gem_/.test(n),
    kind: "Item / Valuable",
    describe: (n) => {
      const stone = n.replace(/^gem_/, "");
      return `A polished ${stone}. Rare — fetches a high price from a merchant.`;
    },
  },
  {
    match: (n) => /chalice/.test(n),
    kind: "Item / Valuable",
    describe: () => "An ornate drinking vessel. Heavy and valuable.",
  },
  {
    match: (n) => /^bucket/.test(n),
    kind: "Item / Tool",
    describe: (n) =>
      n.includes("water") ? "A bucket of water — for drinking, fighting fires, or watering crops."
      : "An empty bucket. Holds water or grain.",
  },
  {
    match: (n) => /^bottle/.test(n),
    kind: "Item / Tool",
    describe: () => "An empty glass bottle. Could hold a drink, oil, or a written note.",
  },
  {
    match: (n) => /fishing_rod|compass/.test(n),
    kind: "Item / Tool",
    describe: (n) =>
      n.includes("fishing") ? "A fishing rod. Take it to a river or pond."
      : "A compass. Points to magnetic north — useful in the wilderness.",
  },
  {
    match: (n) => /^wood_log/.test(n),
    kind: "Item / Material",
    describe: () => "A felled log. Building material — drag to a construction site.",
  },
  {
    match: (n) => /^coal/.test(n),
    kind: "Item / Material",
    describe: () => "A lump of coal. Fuel for the forge or hearth.",
  },
];

function describeItem(name: string): { kind: string; description: string } {
  for (const rule of ITEM_RULES) {
    if (rule.match(name)) return { kind: rule.kind, description: rule.describe(name) };
  }
  return {
    kind: "Item",
    description: `${titleCase(name.replace(/_/g, " "))}. A miscellaneous item.`,
  };
}

function describeBuilding(name: string): { kind: string; description: string; stats: SpriteStat[] } {
  if (name.startsWith("stall_")) {
    const goodsKey = name.replace(/^stall_/, "").split("_").find((p) => STALL_GOODS_HINT[p]);
    const detail = goodsKey ? STALL_GOODS_HINT[goodsKey] : "An open market stall.";
    const isOpen = name.endsWith("_open");
    return {
      kind: "Building / Market stall",
      description: detail,
      stats: [{ label: "Status", value: isOpen ? "Open for trade" : "Closed" }],
    };
  }
  return {
    kind: "Building",
    description: `A ${titleCase(name)} structure.`,
    stats: [],
  };
}

function describeStage(name: string): { kind: string; description: string; stats: SpriteStat[] } {
  const m = name.match(/^(.+?)_stage_(\d+)_/);
  if (m) {
    return {
      kind: "Construction",
      description: `A ${titleCase(m[1])} at stage ${m[2]} of construction. Agents with carpentry skill can continue building it.`,
      stats: [{ label: "Stage", value: `${m[2]} of 5` }],
    };
  }
  return { kind: "Construction", description: "A structure being built.", stats: [] };
}

function describeProp(name: string): { kind: string; description: string; stats: SpriteStat[] } {
  if (/bed|cot|bedroll/.test(name))
    return { kind: "Furniture", description: "A bed. Agents rest here to recover stamina.", stats: [{ label: "Use", value: "Restores stamina + sleep" }] };
  if (/chair|stool|bench/.test(name))
    return { kind: "Furniture", description: "A seat.", stats: [] };
  if (/table|desk/.test(name))
    return { kind: "Furniture", description: "A flat surface for work or meals.", stats: [] };
  if (/cabinet|chest|wardrobe/.test(name))
    return { kind: "Furniture / Storage", description: "Closed storage. May contain items inside.", stats: [{ label: "Capacity", value: "~10 items" }] };
  if (/anvil/.test(name))
    return { kind: "Workstation", description: "An anvil — the heart of the smithy. Used to shape iron.", stats: [{ label: "Use", value: "Craft / repair iron items" }] };
  if (/forge|fire|fireplace|cauldron/.test(name))
    return { kind: "Workstation", description: "An open flame. Cooks, heats, or smelts.", stats: [{ label: "Use", value: "Cook food, smelt ore" }] };
  if (/clock/.test(name))
    return { kind: "Furniture", description: "A timepiece.", stats: [] };
  if (/chandelier|candelabra|sconce|torch/.test(name))
    return { kind: "Lighting", description: "A light source — keeps the room visible after dusk.", stats: [{ label: "Range", value: "~3 tiles" }] };
  if (/rug|carpet/.test(name))
    return { kind: "Furnishing", description: "A floor covering.", stats: [] };
  return { kind: "Furnishing", description: `A ${titleCase(name)}.`, stats: [] };
}

function describeFx(name: string): { kind: string; description: string; stats: SpriteStat[] } {
  if (/glow|window/.test(name))
    return { kind: "Visual effect", description: "A glow effect — usually marks a lit window from outside.", stats: [] };
  if (/smoke|steam/.test(name))
    return { kind: "Visual effect", description: "Smoke or steam.", stats: [] };
  return { kind: "Visual effect", description: `A ${titleCase(name)} effect.`, stats: [] };
}

export function describeSprite(sprite: string): SpriteInfo | null {
  if (!sprite) return null;
  const colon = sprite.indexOf(":");
  if (colon < 0) return null;
  const category = sprite.slice(0, colon);
  const name = sprite.slice(colon + 1);

  if (category === "veg") return null;

  const cat = artCatalog();
  const meta = cat?.meta(sprite) ?? null;
  const thumbUrl = cat?.url(sprite) ?? null;
  const enterable = cat?.enterable(sprite) ?? false;

  const headline = HEADLINE_DESCRIPTIONS[sprite];
  if (headline) {
    return {
      sprite,
      title: prettyTitle(category, name),
      kind: meta?.kind ? `Building / ${titleCase(meta.kind)}` : "Building",
      description: headline,
      stats: HEADLINE_STATS[sprite] ?? [],
      thumbUrl,
      enterable,
    };
  }

  let kind: string;
  let description: string;
  let stats: SpriteStat[] = [];
  switch (category) {
    case "bld": {
      const built = describeBuilding(name);
      kind = built.kind;
      description = built.description;
      stats = built.stats;
      break;
    }
    case "stall": {
      const built = describeBuilding(`stall_${name}`);
      kind = built.kind;
      description = built.description;
      stats = built.stats;
      break;
    }
    case "item": {
      const i = describeItem(name);
      kind = i.kind;
      description = i.description;
      stats = ITEM_STATS[name] ?? [];
      break;
    }
    case "prop":
    case "props":
    case "int": {
      const p = describeProp(name);
      kind = p.kind;
      description = p.description;
      stats = p.stats;
      break;
    }
    case "fx": {
      const f = describeFx(name);
      kind = f.kind;
      description = f.description;
      stats = f.stats;
      break;
    }
    case "stage": {
      const s = describeStage(name);
      kind = s.kind;
      description = s.description;
      stats = s.stats;
      break;
    }
    case "ui":
      return null;
    default:
      kind = titleCase(category);
      description = `A ${titleCase(name)}.`;
  }

  return {
    sprite,
    title: prettyTitle(category, name),
    kind,
    description,
    stats,
    thumbUrl,
    enterable,
  };
}
