# WIRE PROTOCOL

How the browser, the agents, and the engine talk to each other.

## Transport

- **WebSocket** for all bi-directional streams (agents and viewers).
- **HTTP** for one-shot operations: registration, snapshot fetch, asset downloads, auth.
- **FlatBuffers** as the binary serialization for hot-path messages (observations, world deltas).
- **JSON** for cold-path messages (registration, auth, snapshot index) — easier to debug, perf doesn't matter.

## Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `POST /api/v1/agent/register` | HTTP+JSON | Agent registers with the world. Receives `agent_id`, `auth_token`. |
| `GET /api/v1/world/info` | HTTP+JSON | Static world info (map list, scenario manifest, verb list, palette). |
| `WS /ws/agent` | WebSocket | Agent's main stream. Receives obs, sends actions. Auth via subprotocol or first message. |
| `WS /ws/viewer` | WebSocket | Browser's stream. Receives world deltas. Sends camera/click cmds. Auth via auth.js session cookie. |
| `GET /assets/...` | HTTP | Static art atlas, manifest, sound files (none in v1). |

## Authentication

### Agent

1. Agent POSTs to `/api/v1/agent/register` with `{agent_id?, user_token, persona_blob, ...}`.
   - `user_token` is the user's session (issued by Auth.js after login).
   - `persona_blob` is the form data (name, bio, etc.).
2. Engine validates `user_token` with Auth.js, looks up or creates the agent record in Postgres.
3. Returns `{agent_id, agent_secret, ws_url}`.
4. Agent connects to `ws_url`. First message is `{auth: agent_secret}`. Engine validates, switches the stream to binary.

### Viewer

1. User logs in via Auth.js (cookie session).
2. Browser connects to `/ws/viewer`. Engine reads the cookie, looks up the user, assigns the WS to their viewer session.
3. Anonymous viewing (not logged in) is allowed but restricted: can spectate but not control any agent.

## Image observations

Agents register with a `vision.mode` of `structured`, `image`, or
`both`. When images are requested, every `observation` message includes
a `view_image` field. See `docs/OBSERVATION_MODEL.md` §10b for the
schema and rationale.

Wire-level shape:

```
ObservationDelta {
  ... structured fields ...
  view_image: ViewImage?
}

ViewImage {
  format: ImageFormat (enum: PNG, WEBP)
  width: uint16
  height: uint16
  data: [ubyte]
  centered_on_pos: Pos
  facing: Facing
}
```

The image bytes live in the same WS frame as the structured payload —
no separate fetch. For multimodal agents this is the lowest-latency
path; for structured-only agents the field is absent and there's zero
overhead.

## Message types

### Engine → Agent

| Type | When | Contents |
|---|---|---|
| `observation` | At agent's configured cadence | See `docs/OBSERVATION_MODEL.md` |
| `action_ack` | Immediately after each agent action | `{action_id, accepted: bool, reason: string?}` |
| `world_event_notify` | When something high-priority happens to the agent (taking damage, being addressed) | Same fields as audible event |
| `disconnect_notice` | Before disconnecting | `{reason: string}` |

### Agent → Engine

| Type | When | Contents |
|---|---|---|
| `action` | Whenever the agent decides | `{action_id, in_response_to_obs, verb, params, priority}` |
| `set_cadence` | Initial connect + can be re-sent | `{interval_ms: int}` — how often to push observations to me |
| `ping` | Heartbeat (every 30s) | `{ping_id, sent_at_ms}` |

### Engine → Viewer

| Type | When | Contents |
|---|---|---|
| `world_delta` | At 30Hz, AOI-filtered | Tile + entity + object diffs in the subscribed chunks |
| `full_state` | On connect or chunk subscribe | Full state of the requested chunks |
| `event_stream` | When notable events happen in subscribed area | speech, deaths, structure-build events for live drama feed |
| `agent_summary` | On request, batched | For story-feed and leaderboards |

### Viewer → Engine

| Type | When | Contents |
|---|---|---|
| `subscribe_chunks` | On camera change | `{chunks: [(x, y), ...]}` — which chunk ids to subscribe to |
| `unsubscribe_chunks` | Implicit via re-subscribe | (we send full list, engine diffs) |
| `inspect_entity` | On click | `{entity_id}` — returns rich inspector data |
| `request_story_feed` | On opening story view | `{user_id, since_tick}` — paginated story events |
| `ping` | Heartbeat (every 30s) | `{ping_id, sent_at_ms}` |

## FlatBuffers schemas

Schemas live in `schemas/*.fbs`. Build script generates language bindings:

- `gen/go/` — Go bindings for the engine.
- `gen/ts/` — TS bindings for the frontend.
- `gen/py/` — Python bindings for the Python SDK.

Sample schema (sketch):

```fbs
// observation.fbs
namespace agent_sim.proto;

table Pos { x: float; y: float; }

table SelfState {
  entity_id: string;
  pos: Pos;
  facing: byte;
  extras: [ubyte];                      // opaque blob
  current_action: ActionInProgress;
  last_action_result: ActionResult;
}

table VisibleEntity {
  entity_id: string;
  apparent_label: string;
  pos: Pos;
  facing: byte;
  archetype: string;
  extras_summary: [ubyte];
  doing: string;
}

table Observation {
  obs_id: uint64;
  world_tick: uint64;
  self_state: SelfState;
  visible_entities: [VisibleEntity];
  // ... visible_objects, visible_items, audible, local_view, world_clock, view_image
  //     (recent_self_results is present in the schema but currently always empty)
}

root_type Observation;
```

Versioning: every schema has a `schema_version: uint16` field. The engine and SDK both check; mismatch → connection rejected with a "please upgrade your SDK" message.

## Delta encoding

For `observation` (engine → agent) and `world_delta` (engine → viewer):

First message in a session is `full_state`. Subsequent messages are `delta` with:
- Entries added since last message.
- Entries that changed (with the changed fields only).
- IDs of entries removed.

The receiver maintains a local snapshot and applies deltas to reconstruct the current full state.

A `re-sync` message can be requested by either side to ask for a fresh full state (e.g. if the receiver detects an inconsistency).

## Rate limits and back-pressure

- Agents are rate-limited at the engine: max action submission rate is `1.5 × their observation rate` (so they can submit slightly faster than they observe, but not spam).
- Viewers are rate-limited on `inspect_entity` and `request_story_feed` (100/min).
- Slow agents (haven't acked observations in 5+ pushes) get backed off — engine doubles their effective interval until they catch up.

## Error handling

Engine sends a typed error message for any protocol violation:

```
ProtoError {
  code: enum(BAD_VERB | INVALID_PARAMS | RATE_LIMIT | UNAUTHED | SCHEMA_MISMATCH | INTERNAL)
  detail: string
}
```

Clients log and (if recoverable) retry. Schema mismatch is fatal — disconnect.

## Pings and disconnects

- Both sides ping every 30s. If no pong in 60s, disconnect.
- On graceful disconnect, agent sends a `disconnect_intent` and waits for engine `ack` before closing. Engine flushes pending events.
- On unexpected disconnect, the agent's entity goes idle. Reconnect within 5 minutes preserves state; after that, the entity may be despawned per scenario rules.
