# CONSTRUCTION_PROCEDURAL

How the Construction system produces beautiful agent-built structures. Locked in Session 2, Q60.

## The bar

Agents can build cottages, manors, taverns, watchtowers, eventually castles. The result must **always look beautiful** — Octopath-grade. We cannot accept a player-built version that looks like a Sims McMansion.

Reference inspirations: Townscaper (constraint-based always-beautiful), Manor Lords (zoning-driven procedural village houses), Animal Crossing (predefined-exterior + free interior).

The hedged plan: 2-week sprint on the procedural approach for one style (Cottage). Design-critic loop converges. If it clears, the approach is locked + we add more styles. If not, fall back to fixed hand-authored blueprints + free-form interior.

## The procedural approach

Agent calls one verb:

```
Build(style="cottage", footprint=[5,5], room_count=3, target_pos=[40,22])
```

Five inputs: style, footprint dimensions, room count, target plot, (implicit) builder identity.

Five things happen:

### 1. Validate
- Plot at target_pos is empty (no entities, no static blockers).
- Plot is owned by the builder (or unowned + claimable per scenario rules).
- Builder has the required materials in inventory (from style's material recipe).
- Footprint dimensions are within style's range (cottage = 4×4 to 7×7; manor 8×8 to 12×12; castle 15×15+).

### 2. Procedurally generate floor plan
- **Binary space partition** on the footprint, with constraints from `room_count`.
- For each room: assign a role (kitchen / bedroom / living / hall / utility) based on size + position.
- Place doors at room boundaries; ensure every room is reachable.
- Place an exterior door (south-facing by default).
- Result: a 2D grid of `wall | floor | door` cells.

### 3. Auto-tile assembly (exterior)
- Apply the style's wall tileset to the wall cells. Auto-tile based on neighbor pattern (same machinery as terrain autotile).
- Generate roof from footprint outline using the style's roof tileset.
- Place windows along outer walls at archetypal positions (every 3-4 tiles).
- Place chimney + door visuals.
- Apply the style's color palette + decorative trim.

### 4. Spawn building entity + interior sub-map
- Building entity created (archetype="building").
  - `extras.style = "cottage"`.
  - `extras.footprint = [5,5]`.
  - `extras.owner = builder.entity_id`.
  - `extras.interior_map_id = "interior:bld:NEW_UUID"`.
- A new interior sub-map is created in the MultiMapHub.
- Interior comes pre-furnished with the style's BARE essentials (floor tiles, exterior walls, internal walls, doors).
- Empty for the agent to decorate.

### 5. Materials consumed; build duration
- N-tick build progress; sound `hammer_strike` emits during.
- `extras.build_in_progress` is set on the BUILDER (not the building), so observers see "X is building."
- On completion, emits `StructureBuilt` event.

## Per-style component library

Per Q60: we hand-author one library per style.

Cottage library (~50 hand-painted tiles + props):
- Wall tiles: stone + plaster + corner variants + window-cut wall + door-cut wall
- Roof tiles: thatched + corner + ridge + chimney connector
- Window: 16×16 wood-frame closed + 16×16 open
- Door: 16×16 wooden + 16×16 ajar
- Decorative trim: hanging flower box, vine, lantern, sign
- Floor tiles (interior): wood plank + corner
- Internal wall: thinner stucco

Each tile authored once. Auto-tile rules handle adjacency. Procedural placement handles position.

Manor library: + slate roof, ornamental columns, balconies.
Tavern library: + signpost, awning, big door, central hearth.
Watchtower library: + stone foundation, crenellations, narrow windows, ladder access.
Castle library: + curtain walls, gatehouses, drawbridge, courtyard tiles, multiple floors.

## Interior decoration (always free-form)

After the building is built, the OWNER agent can decorate:

```
Place_furniture(item="oak_table", target_pos=[3, 4], rotation=0)
```

Furniture is inventory items (you bought them or built them). Verb owns by **Furniture system** (a small sub-system of Construction):
- `Place_furniture` — adds furniture entity to the interior sub-map.
- `Remove_furniture` — back to inventory.
- `Rotate_furniture` — change orientation.

Free-form here is fine because:
- Interiors are private (only people you invite see them).
- The MESSY-Sims problem doesn't hit the exterior, which is the public face of the world.

## Material sources (Q60 answer: both gather AND buy)

Adds two systems:

### Forestry system
- Trees become harvestable (when Forestry loaded, they convert from decorations to entities, archetype="tree").
- `Chop` verb: agent uses an axe (item), tree HP decreases, on 0 the tree drops 3-5 wood items.
- Tree regrows after N ticks (configurable).

### Mining system
- Rocks become harvestable similarly.
- `Mine` verb with pickaxe → stone items.

Or wrap both into a **Resources** system if we want one less plugin.

## Multi-floor (Q47 implications)

Multi-floor buildings = the interior sub-map has a stair tile (portal) leading to a floor-2 sub-map.

```
Build(style="manor", footprint=[8,8], floors=2, ...)
```

The procedural generator places stairs in the interior; on completion, two sub-maps exist:
- `interior:bld:NEW_UUID:floor1`
- `interior:bld:NEW_UUID:floor2`

A stair portal in floor1 warps to floor2. Same Warp mechanism as exterior↔interior.

## Time-box + fallback (Q60)

**Week 1**: prototype the Cottage style end-to-end. Component library art (50 tiles) + floor-plan generator + auto-tile + Build verb + interior spawn.

**Week 2**: design-critic loop. Build cottages with random parameters; let the critic compare them to Octopath cottage references.

**Decision at end of week 2**:
- Critic clears: lock the approach. Add Manor (week 3), Watchtower (week 4), Tavern (week 5), Castle (week 6+).
- Critic does NOT clear: pivot to **fixed blueprints**. Predefined cottage / manor / tavern / etc., each hand-authored as one specific template. Build verb still exists but `style` selects a fixed template, `footprint` is fixed, `room_count` is fixed. Interior decoration stays free-form.

## Open questions deferred to implementation

- How long does a build take? Probably scenario-tunable. Cottage = 30 sec real-time; castle = several minutes.
- Can a build be sabotaged mid-construction (Combat attacks the half-built site)?
- Can buildings be REPAIRED after damage? (Probably yes — a `Repair` verb in Construction.)
- What's the room-role inference (kitchen vs bedroom)? — depends on size + position heuristics, refine later.
- Furniture price + crafting + style matching — Furniture system mini-design.
