# Scaling agent_sim to thousands of concurrent bots

Where we are today and what has to change to get from 4 concurrent bots
(current load) to 1,000+ on a single instance and 100,000+ across a
shard fleet.

## Where the bottlenecks live today

The engine is a single Go process. One `sync.RWMutex` (`world.mu`)
serializes every piece of world state:

| Operation | Lock | Frequency | Cost per call |
|---|---|---|---|
| `Tick()` | W | 60 Hz | scans all entities, runs scenario onTick |
| `SubmitAction()` per agent action | W | up to 30/sec per bot | brief — verb handler executes under lock |
| `BuildObservationFor()` per agent obs | W | up to 10/sec per bot | scans entities for visibility + audible |
| `Snapshot()` for viewer broadcast | R | 30 Hz | full entity copy |
| `RecentAudibleAll()` | none | 30 Hz | reads `w.audible` without lock (data race) |

At 4 bots this is fine in principle but in practice we saw the engine
go non-responsive after a few minutes of LLM-driven action. Two
contributing bugs have been fixed; structural changes are still
needed for real scale.

### Fixed in this branch
1. **Agent WS race on teardown.** A new auth on an existing `agent_id`
   stomped the old `h.live` entry; the old `readPump`'s defer could
   delete the NEW entry. Patched via compare-and-delete + a `done`
   channel + atomic `closedAt` flag. Same fix applied to viewer.go.
2. **Blocking sends inside dispatch.** `handleMessage` for `ping`
   used a blocking channel send, deadlocking readPump if writePump
   was slow. Replaced with `trySend`.

### Still structural
3. **`BuildObservationFor` holds `world.mu` while building.** At 4
   bots × 10 Hz that's 40 W-lock acquisitions/sec just for
   observations. Each iterates `w.entities` for visibility, so cost
   grows O(N × M) — agents × visible entities.
4. **`SubmitAction` mutates under W-lock.** Verb handlers can spawn
   entities, mutate inventories, queue events. Long-running handlers
   block the tick.
5. **No async action queue.** Every action is applied synchronously
   in the agent's WS read goroutine. With many bots, each holds the
   lock briefly but the queue can grow unboundedly.
6. **Snapshot encoding allocs.** Each `viewerHub.encodeSnapshot()`
   builds the entire entity slice and JSON-encodes it. At 30 Hz with
   1000 entities that's 30k entity copies/sec.

## The roadmap

### Phase 1 — 1 to ~100 concurrent bots (today, with race fixes)

- ✅ Race-free WS handlers (`agent.go` + `viewer.go`).
- ✅ Per-IP rate limit on `/register` (cheap protection vs. zombie clients).
- ⏳ **Spinlock budget**: stress-test the current architecture at 50, then 100 concurrent bots. Confirm tick rate stays at 60 Hz.
- ⏳ Add a `/debug/pprof` endpoint so we can profile lock contention at the bottleneck.

### Phase 2 — 100 to ~500 concurrent bots

Move observation generation **out of the write lock**.

```
            ┌───────────────────────────┐
   Tick()   │ acquire W-lock            │
            │ mutate entities           │
            │ build SnapshotSlot[next]  │  ← pre-built immutable view
            │ release W-lock            │
            └───────────────────────────┘

         per-agent observation loop
            ┌───────────────────────────┐
            │ read SnapshotSlot[stable] │  ← no lock
            │ filter for self's vision  │
            │ send obs to WS            │
            └───────────────────────────┘
```

Concrete changes:
- Add an `atomic.Pointer[WorldSnapshot]` slot in `World`. Tick writes a
  fresh snapshot to a fresh allocation at the end of each tick.
- `BuildObservationFor` reads the snapshot pointer (lock-free) and
  filters it. No `w.mu` acquisition.
- Same snapshot serves the viewer broadcast — one shared object.
- Trade-off: observations are one tick stale (16 ms at 60 Hz). Acceptable
  for cadences ≥100 ms.

Move actions to an **async queue**.

```
   WS read goroutine
      ↓ (no lock)
   ActionQueue (lock-free MPMC)
      ↓
   Tick():
      W-lock
      drain queue, dispatch verb handlers
      release
```

- Verb handlers still run under the world lock, but they're batched
  and predictable. Action ack is sent AFTER the next tick processes
  the queue.
- Action rate limit moves from "30/sec per WS" to a per-bot quota
  enforced at dequeue.

### Phase 3 — 500 to ~5,000 concurrent bots

The single process cap is roughly CPU cores × tick capacity. To go
further, **shard the world by region.**

```
   ┌──────────────────────────────────────────┐
   │  World                                   │
   │   ┌────────┐  ┌────────┐  ┌────────┐    │
   │   │Region A│  │Region B│  │Region C│    │
   │   │ Tick   │  │ Tick   │  │ Tick   │    │
   │   │ Lock A │  │ Lock B │  │ Lock C │    │
   │   └────────┘  └────────┘  └────────┘    │
   │      ↑           ↑           ↑          │
   │  cross-region message bus (channel)    │
   └──────────────────────────────────────────┘
```

- Each region is a goroutine with its own `mu`. Agents in region A
  are observed and dispatched by region A's tick.
- An agent moving across a region boundary publishes a `HandOff` event
  on the bus; the destination region picks it up next tick.
- Speech / audible across regions: region tick collects emitted
  events, fans out to neighbors via the bus.
- Observation generation parallelises naturally.

This is the architecture used by EVE Online (system-per-thread) and
WoW (zone-per-server). It scales linearly with cores until inter-region
traffic dominates.

### Phase 4 — 5,000 to 100,000+ concurrent bots

Multi-process. Each world process owns a small number of regions.
A separate **WS gateway** terminates client connections and proxies
to the correct region process.

```
  client ──────► gateway ──► region process 1 (regions A, B)
                          ──► region process 2 (regions C, D)
                          ──► region process N
```

- Gateway: stateless WS multiplexer, no game logic. Handles auth, JWT
  verification, rate limiting. Scales horizontally behind a load
  balancer.
- Region processes: each runs Phase 3 architecture internally, owns
  a fraction of the world.
- Cross-process region handoff via NATS / Redis pubsub.
- Snapshot persistence per region (Postgres-backed or volume-mounted
  JSON like today, sharded by region key).

The unit of scale becomes "regions per process" and "processes per
fleet." 100 processes × 8 regions × 125 agents = 100k concurrent.

## Concrete deltas before we start the work

1. **Profile the current engine under load.** Run the soak harness
   with 100 bots and capture pprof. Confirm the bottleneck is the
   write lock and not, e.g., websocket fan-out or JSON encoding.
2. **Add the snapshot-slot pattern (Phase 2).** This is the highest
   leverage change — many of the symptoms we saw (viewer hangs,
   /metrics stalls during agent load) point at lock contention on
   `Snapshot()` competing with tick.
3. **Action queue + ack-after-tick.** Required for batching and
   for cross-region action ordering.
4. **Per-region locks.** Hard part: defining region boundaries that
   match how speech / vision / combat work. Probably 32×32-tile
   chunks initially.
5. **Gateway extraction.** Last — only after Phase 3 is real.

## Cost guardrails

- **Bots are external clients.** The engine never imports bot code,
  so a buggy bot can crash itself without affecting the world. This
  is what makes the WS-gateway architecture viable.
- **Each bot is one connection.** Go's netpoll handles 10s of thousands
  of WS connections per process; the binding budget is more about
  per-bot ticks and observations than raw connections.
- **LLM cost is the real bottleneck for emergent worlds.** A bot
  generating one LLM call every 8 s at 1000 bots = 125 calls/sec.
  Local inference (Qwen on a 4090) tops out around 8-12 generations/sec
  per slot. Production needs either cheap pooled inference (vLLM with
  speculative decoding) or per-user rate caps.

## Tooling to build alongside

- `engine/cmd/soak` already exists; extend it to spawn 100, 500, 1000
  bots without LLM (rule-based brains) to stress the engine in
  isolation from LLM cost.
- `tools/profile_engine.sh` — start engine with `-cpuprofile`, run
  soak, dump trace, open in `go tool pprof`.
- Per-bot metrics: actions/sec, rejected/sec, observation lag.
  Aggregated by archetype + persona for emergence research.
