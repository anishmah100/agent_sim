# AFFORDANCE_MANIFEST

The single source of truth for "what does this world let you do?" Drives bots' verb discovery, the SDK's typed validation, and the UI's World Rulebook page.

Locked in Session 2, Q39 + Q46 + Q47.

## Why it exists

A world is a composition of systems (Q32). Each system contributes verbs, state fields, sounds, and archetypes. We can't ask researchers to memorize what a particular world supports; the manifest lets them query.

The same manifest drives three consumers:
1. **Bot SDK** — fetches at register, codegens / validates action params.
2. **UI World Rulebook** — renders a beautiful rules-of-this-world page.
3. **/api/v1/world/affordances HTTP endpoint** — public, cacheable, free.

## Schema

```ts
type Manifest = {
  world: string;             // map_id
  scenario: string;          // scenario name (e.g. "fantasy_town")
  schema_version: number;    // bump on breaking changes
  systems: SystemDeclaration[];
};

type SystemDeclaration = {
  name: string;              // "combat" / "money" / "construction" / ...
  description: string;       // 1-2 sentence plain-English summary
  verbs: VerbDeclaration[];
  state_fields: StateFieldDeclaration[];
  sounds_emitted: SoundDeclaration[];
  archetypes: ArchetypeDeclaration[];
};

type VerbDeclaration = {
  verb: string;              // "attack" / "vote" / "build" / ...
  description: string;       // plain-English
  params_schema: JSONSchema; // ParamsSchema; validated client + server
  preconditions: string[];   // human-readable, e.g. "target must be adjacent"
  rejection_reasons: string[]; // canonical strings, see AGENT_API.md
  examples: VerbExample[];
  emits_events?: string[];   // names of events emitted (for cross-system docs)
};

type VerbExample = {
  params: any;               // valid params
  result: string;            // plain-English outcome description
};

type StateFieldDeclaration = {
  key: string;               // "hp", "gold", "voting_member", ...
  type: "int" | "float" | "string" | "bool" | "object" | "list";
  owner: "entity.extras" | "world.extras";
  public_at_any_distance: boolean;     // Q43/Q59 — visibility rule for observers
  public_within_distance?: number;     // optional override (e.g. 5 for bios)
  meaning: string;
};

type SoundDeclaration = {
  kind: string;              // "sword_clang" / "door_open" / "howl"
  description: string;
  emitted_by: string;        // which verb / event triggers this
};

type ArchetypeDeclaration = {
  archetype: string;         // "merchant", "guard", "wolf", ...
  description: string;
  default_extras: any;       // initial extras blob for new spawns
  default_verbs_used?: string[]; // typical actions this archetype takes
};
```

## Example manifest entry

```json
{
  "name": "construction",
  "description": "Lets agents build buildings from materials. Composable construction with style-driven procedural assembly.",
  "verbs": [
    {
      "verb": "build",
      "description": "Construct a new building entity at the target position.",
      "params_schema": {
        "type": "object",
        "properties": {
          "style": { "type": "string", "enum": ["cottage","manor","tavern","watchtower","castle"] },
          "footprint": {
            "type": "array",
            "items": { "type": "integer" },
            "minItems": 2, "maxItems": 2
          },
          "room_count": { "type": "integer", "minimum": 1, "maximum": 10 },
          "target_pos": {
            "type": "array", "items": {"type":"integer"}, "minItems":2, "maxItems":2
          }
        },
        "required": ["style","footprint","room_count","target_pos"]
      },
      "preconditions": [
        "agent owns the plot at target_pos",
        "plot is empty (no entities or static blockers)",
        "agent has required materials in inventory"
      ],
      "rejection_reasons": ["not_plot_owner","plot_occupied","not_enough_materials","invalid_style","invalid_footprint"],
      "emits_events": ["StructureBuilt"],
      "examples": [
        {
          "params": { "style":"cottage", "footprint":[5,5], "room_count":3, "target_pos":[40, 22] },
          "result": "Begins constructing a 5×5 three-room cottage at (40,22). Consumes 10 wood + 5 stone from inventory. Building entity spawns after 600 ticks."
        }
      ]
    }
  ],
  "state_fields": [
    {
      "key": "build_in_progress",
      "type": "object",
      "owner": "entity.extras",
      "public_at_any_distance": true,
      "meaning": "Set on the BUILDER entity while they're constructing. Tracks remaining ticks."
    }
  ],
  "sounds_emitted": [
    { "kind": "hammer_strike", "description": "Construction noise.", "emitted_by": "build_in_progress tick" }
  ],
  "archetypes": []
}
```

## Loading + serving

1. At engine boot, each loaded system's package exposes a `Manifest()` method returning its `SystemDeclaration`.
2. Engine aggregates all of them into the full Manifest.
3. Engine exposes `GET /api/v1/world/affordances` returning JSON.
4. Response is cached server-side (immutable until restart for now; mutable when Voting passes rule changes, see Q47 implications).

## Public visibility rules (for state_fields)

Per Q43/Q59:

- `public_at_any_distance: true` — included in `visible_entities[i].extras_summary` for any observer in vision range.
- `public_at_any_distance: false` + `public_within_distance: 5` — included only if observer is within 5 tiles.
- `public_at_any_distance: false` + no override — NEVER in another agent's observation (only the owner sees it via `self.extras`).

The default `hp / max_hp` ships as `public_at_any_distance: true` (Octopath-style HP bars over heads). `gold / inventory` ships as `public_at_any_distance: false` (private).

## Versioning

- Manifest has a `schema_version` field.
- SDK checks at register time. Mismatch = "please upgrade your SDK."
- Internal verb / param changes are signaled by a new system version number (e.g. `"name": "combat", "version": 2`). SDK ships modules pinned to system version pairs.

## What this enables

- Adding a Voting system later = a new package + a Manifest() method + registering verb handlers + event subscribers. **No engine refactor**. Users on the SDK that ships Voting see the new verbs in autocomplete the next time they fetch the manifest.
- The UI World Rulebook automatically lists Voting under "What can I do here?" the moment the system is loaded.
- A documentation site can be generated mechanically from the live manifest.

## Interior entry (click-to-enter buildings)

Front-end specific — kept here because it complements the affordance model.

The interior layer fires only on an **audited allowlist** of sprite IDs;
the legacy `bld:NNN` range (000–030) covers cottages, doors, windows,
lamps, signposts, fences and roof tiles, of which only `bld:000` and
`bld:001` are actually houses. Anything else triggering an interior
would be obviously wrong (lamp-pops-into-cottage).

Current allowlist (`frontend/src/render/Decoration.ts`):

| Sprite ID | What it is | Interior template |
|---|---|---|
| `bld:000` | Red-roof cottage | `cottage` |
| `bld:001` | Brown-roof cottage | `cottage` |
| `bld:blacksmith` | The Forge | `blacksmith` |
| `bld:town_hall` | Town hall | `town_hall` |
| `bld:granary` | Granary | (cottage fallback for now) |
| `bld:watchtower` | Watchtower | (cottage fallback for now) |

Audit process when adding a new building sprite:

1. Open the PNG. Confirm it's an actual building, not a fence/lamp/door.
2. Add an entry to the allowlist with its interior template.
3. Run the audit screenshots and click each new building variant in
   the dev browser — verify the interior opens and the hover-glow
   filter fires.

When `ArtCatalog` lands (see `docs/CLEANUP_PLAN.md`), this allowlist
becomes a property on the sprite entry (`enterable: true`,
`interior_template: "cottage"`) and the hardcoded record in
`Decoration.ts` is removed.
