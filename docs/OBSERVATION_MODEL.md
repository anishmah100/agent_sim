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

  recent_self_results: [                  # results of THIS agent's own recent actions
    {
      verb: string
      accepted: bool
      reason: string?                     # e.g. "target_too_far", "not_enough_gold"
      tick: uint64
    }
  ]

  known_map_summary: {                    # the static world map the agent "knows"
    map_id: string                        # which sub-map they're currently in
    map_dims: [w, h]
    named_regions: [                      # places they could navigate to
      { name: string, center: [x, y], kind: string }
    ]
    portals: [                            # door/transition tiles they're aware of
      { at: [x, y], to_map: string }
    ]
  }

  world_clock: {
    tick: uint64
    day_phase: enum(dawn|morning|midday|afternoon|dusk|night)
    weather: string                       # scenario-defined, e.g. "clear", "rain"
  }

  persona_reminder: bytes                 # the agent's own persona block, repeated here
                                          # so the prompt template doesn't need to
                                          # remember it separately. (Engine just echoes
                                          # what the agent set at register time.)
}
```

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
