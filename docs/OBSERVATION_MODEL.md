# OBSERVATION MODEL

What an agent sees each tick, and how they remember.

## Core principle

Agents see what their character would see. No god-view. They have a fixed-radius local observation + a static known map + their own memory. They do NOT know what distant characters are doing right now.

## §1 — Vision radius

- **Default vision radius**: 12 tiles (Chebyshev distance — a 25×25 square around the agent).
- **Modulated by**: line-of-sight blocking. Walls and large objects block vision; an agent inside a building can't see outside unless they're at a window.
- **Modulated by**: time of day. Vision reduces to 6 tiles at night (configurable per-scenario; fantasy world adds torches as light sources to mitigate).

The radius is a default — scenarios can override per-archetype (e.g. an owl character has 24-tile vision).

## §2 — What's in each observation

The engine sends each agent an `ObservationDelta` message at their configured cadence (default 1 Hz, configurable lower). The full schema:

```
Observation {
  obs_id: uint64                          # increasing, used to ack
  world_tick: uint64                      # current world tick when observation was sampled

  self: {
    entity_id: string
    pos: [x, y]                           # tile coords
    facing: enum(N|S|E|W)
    extras: bytes                         # opaque state blob (HP, gold, hunger, etc — scenario-defined)
    current_action: { verb, params, eta_tick } | null
    last_action_result: { verb, accepted, reason? } | null
  }

  visible_entities: [
    {
      entity_id: string                   # so the agent can target it
      apparent_label: string              # what the observer would CALL this entity
                                          # (derived from their persona's relationships +
                                          # the entity's archetype)
      pos: [x, y]
      facing: enum(N|S|E|W)
      archetype: string                   # "human", "merchant", "tree", "chest", "wolf"
      extras_summary: bytes               # scenario-defined trimmed view
                                          # (e.g. HP rounded to nearest 10, not exact;
                                          # gold visible only if scenario says so)
      doing: string?                      # natural-language summary of current action
                                          # (e.g. "walking north", "talking to Alice")
    }
  ]

  visible_objects: [                      # static / placed objects in range
    {
      object_id: string
      kind: string                        # "tree", "berry_bush", "chest", "anvil", "sign"
      pos: [x, y]
      affordances: [string]               # actions usable on it
                                          # (e.g. ["chop", "harvest"], ["read"], ["open"])
      state_summary: bytes?               # e.g. "chest_is_locked: true"
    }
  ]

  audible: [                              # speech + sound events from last N seconds
    {
      event_id: string
      kind: enum(speech | shout | whisper | sound)
      from_entity: string                 # speaker
      from_pos: [x, y]                    # heard FROM here (so agent can localize)
      text: string?                       # for speech/shout/whisper
      sound_kind: string?                 # e.g. "metal_clang", "scream"
      tick: uint64                        # when it happened
    }
  ]

  recent_self_results: []                 # DECLARED BUT CURRENTLY ALWAYS EMPTY.
                                          # The engine does not populate this; action
                                          # outcomes are delivered as separate
                                          # `action_ack` frames (see §4 / SDK README).
                                          # Read the ack returned by agent.act() instead.

  world_clock: {
    tick: uint64
    day_phase: enum(dawn|morning|midday|afternoon|dusk|night)
    weather: string                       # currently hard-coded "clear" (no weather model yet)
  }
}
```

> **Removed fields.** Earlier builds also emitted `known_map_summary`
> (map_id/map_dims/named_regions/portals) and `persona_reminder`. Neither
> was consumed by any agent — `named_regions`/`portals` were never even
> populated — and both have been removed. The observation is strictly
> egocentric: there is no global map and no server-side memory in it. An
> agent that wants a map builds one from the `local_view` grids it has seen.

## §3 — What is NOT in the observation

- Other entities' exact HP / gold / hunger / private state (only `extras_summary`, which can hide fields).
- Other entities' personas (unless the observer's own persona has them in their relationships).
- Other entities' intentions / planned actions (only `doing`, a high-level summary).
- The map outside the current sub-map. An agent in the tavern interior doesn't get a stream of the overworld.
- Anything outside the vision radius, even on the known map. The agent knows the tavern exists at (47, 30), but they don't know who's there.
- Dynamically built structures (e.g. a hut built by another agent) until the observer has personally been near it. The known-map is the ORIGINAL map at world start; built structures are tracked separately in the observer's "discovered" set.

## §4 — Discovery of built structures

When an agent builds a new structure (post-launch feature), it becomes an entity in the world. The engine tracks per-agent **discovery sets**: which built-entity IDs each agent has personally been within vision of.

When an agent's observation is built, it includes built structures from the discovery set within vision range plus any new ones currently visible.

Rationale: realistic. You don't know about a hut someone built across the world until you walk past it.

## §5 — Map knowledge

The agent's "known map summary" gives them the **layout** of the world (street names, landmark positions, building entrances) so they can navigate intelligently. This is like a real person knowing the layout of NYC even if they can't see what's happening at every block.

What the known map contains:
- Tile-level walkability summary (so the agent knows where roads are).
- Named regions (e.g. "Town Square", "Forest Clearing", "Tavern").
- Portal locations (doors that lead to interior maps they may have visited).

What it does NOT contain:
- Who is at any of those places right now.
- What's inside an interior they haven't been to.
- Dynamic events.

## §6 — Persona-derived labels

When the observer sees another entity, the `apparent_label` is derived from:
- The observer's persona's declared relationships (`relationships: [{target: "alice", label: "trusted friend"}]`).
- Falls back to archetype-default ("a merchant", "a guard", "a wolf").
- For unrecognized characters: "a stranger".

This means agent A and agent B can see the same agent C with totally different labels. Bob sees Alice as "my wife"; Charlie sees the same Alice as "the woman who owes me money".

## §7 — Memory

The engine does NOT maintain conversation history for the agent. The agent's process (their LLM backend) is responsible for tracking:
- What they remember from previous observations.
- Conversations they've had.
- Goals they've set themselves.

The engine provides one form of memory help: the `audible` field includes a configurable window of past speech events (default 30 seconds). Agents that don't track conversation can still respond to what was just said.

For richer memory (e.g. "did I talk to this person last week?"), the agent backend must maintain its own state. The SDK provides a recommended pattern but doesn't force it.

## §8 — Cadence and back-pressure

- Agent registers with a desired observation interval (e.g. "send me one obs per 1.2 seconds").
- Engine schedules pushes accordingly. If the agent is slow to acknowledge / submit actions, that's their problem — the world keeps ticking.
- If an agent fails to respond to N consecutive observations, their entity goes idle (stops new actions; finishes current). Eventually marked "asleep" and removed from active observation streams to save bandwidth.
- On reconnect, the agent gets a fresh full observation (not delta) and resumes.

## §9 — Delta encoding

After the first observation per session, subsequent pushes are **deltas**:
- `visible_entities` changes only (entered / left / moved / state changed).
- `audible` always full (it's a short rolling window).
- `recent_self_results` always full.
- `world_clock` full.

This keeps observation bandwidth manageable at scale. The agent SDK reconstructs the full picture from `[last_full, delta_1, delta_2, ...]`.

## §10 — Scenario-specific extensions

Scenarios can add fields to:
- `self.extras` — declared in the scenario manifest.
- `visible_entities[].extras_summary` — declared in the scenario manifest, with rules for what shows when (e.g. "gold visible if observer has appraise_skill > 5").
- `visible_objects[].state_summary` — same pattern.

Scenarios CAN'T add top-level fields. The schema is extensible only at the explicit extension points.

## §10b — Image observations (for multimodal / vision agents)

Some agents are better served by an **image** of their surroundings than a
structured JSON observation. Multimodal LLMs (Claude with vision, GPT-4o,
Gemini, qwen2-vl, etc.) and CV-trained models can reason directly from
pixels. The engine supports this as a FIRST-CLASS observation mode
alongside the structured one — agents can request either or both.

### Agent registration

An agent's `register` payload includes a `vision` config:

```
vision: {
  mode: "structured" | "image" | "both",
  radius_tiles: int,                     // structured vision radius (default 12)
  image: {
    enabled: bool,
    crop_tiles: [w, h],                  // e.g. [5, 5] = 5x5 tile crop centered on agent
    render_scale: int,                   // px per tile, default 16 (native)
    format: "png" | "webp",              // png default; webp for smaller bytes
    include_chrome: bool,                // false = pure world; true = adds simple HUD overlays
    fog_outside_vision_radius: bool      // true = black out beyond structured vision_radius
  }
}
```

Crop size open question: 5x5 was the first instinct (80x80 px at native).
Larger crops (e.g. 11x11 = 176x176 px) give the agent more spatial
context but cost more tokens for vision LLMs. We'll pick the default per
v1 by experimentation; per-agent override is always available.

### Observation payload addition

When an agent has `vision.mode in ("image", "both")`, every observation
push includes a `view_image` field alongside the structured fields:

```
view_image: {
  format: "png" | "webp",
  width: int,                           // = crop_tiles[0] * render_scale
  height: int,                          // = crop_tiles[1] * render_scale
  data: bytes,                          // raw image bytes
  centered_on_pos: [x, y],              // agent tile coords at time of render
  facing: "N"|"S"|"E"|"W",
}
```

`view_image` is keyed to the same `obs_id` and `world_tick` as the
structured observation — they describe the SAME world state.

### Engine-side rendering

The engine maintains a lightweight in-Go rasterizer that:
1. Looks up the agent's position + facing
2. Walks the requested crop window over the tilemap
3. Composites tiles + entity sprites (loaded from the same atlas the
   frontend uses) into an RGBA buffer
4. Optionally applies fog-of-war mask
5. Encodes PNG / WebP
6. Caches by (chunk_id, render_scale, time_bucket) so co-located agents
   share renders

The renderer reuses the atlas + tile manifest written by `art/build_atlas.py`
— it's the same data the frontend consumes. Same pixels server-side and
client-side.

### Performance and cost notes

- Per-observation PNG cost: ~80×80 RGBA at 5×5 crop ≈ 25 KB raw, ~3–6 KB
  PNG-compressed. With 1k agents at 1Hz that's ~3 MB/s of image bytes,
  manageable.
- Render cost: a 5×5 rasterize is roughly 25 tile blits + 1–3 sprite
  blits. Microseconds in Go. Cache hits for co-located agents make this
  cheaper still.
- Token cost on the LLM side: a 5×5 tile crop is small enough that vision
  models price it near minimum (Claude bills 1k tokens for an image of
  this size). Larger crops scale linearly.

### What an image observation gives up vs structured

- Image obs does NOT carry entity IDs — agent can't say "attack X" without
  pairing it with structured info, which is why `mode: "both"` exists.
- Image obs does NOT carry `audible` speech — that's still text-mode.
  Multimodal agents typically receive image + text together (image for
  spatial reasoning, text for speech + persona + goals).
- Image obs does NOT carry the static known-map (§5). That stays
  text-mode.

The recommended default for v1 multimodal users: `mode: "both"`. They
get the image for spatial reasoning AND the structured payload for
entity targeting + speech / persona context.

### Implementation status

- **Spec**: locked here. ✓
- **Wire schema**: hook added to `docs/WIRE_PROTOCOL.md`.
- **Engine rasterizer**: planned, lands alongside Milestone 4
  (agent SDK + first bots) so example bots can demo both modes.
- **Default crop size**: TBD by experiment in Milestone 4.

## §11 — Action interface (paired with observation)

For completeness — what the agent sends back:

```
Action {
  action_id: uint64                       # increasing, agent-assigned
  in_response_to_obs: uint64              # the obs_id this action is acting on
  verb: string                            # e.g. "move", "speak", "attack"
  params: bytes                           # scenario-typed (FlatBuffers schema per verb)
  priority: enum(normal | urgent)         # urgent can interrupt current action
}
```

The engine responds with an `ActionAck { action_id, accepted, reason? }`. Accepted actions enqueue for the next tick. Rejected actions don't execute; the agent sees the rejection in their next observation's `recent_self_results`.

See `docs/VERB_REFERENCE.md` for the full verb list and per-verb param schemas.
