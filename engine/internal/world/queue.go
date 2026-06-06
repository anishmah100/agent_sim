package world

import "time"

// Async action queue. Agent WS goroutines enqueue actions without ever
// taking the world write lock; Tick drains the queue at the start of
// each tick, in FIFO (per-tick submission order). Each action gets a
// reply channel that fires with the ActionResult once applied.
//
// Compromise: actions are applied at next tick boundary instead of
// inline. Latency: 0–16ms (one tick) per action. Strict per-tick
// ordering is preserved (no out-of-order races between agents).
//
// If two agents queue actions on the same target the same tick, the one
// that enqueued first wins. Within a single tick, the queue drain order
// is the order of select-receive on a buffered channel; Go's channel
// FIFO guarantee covers this.

type pendingAction struct {
	entityID string
	env      *ActionEnvelope
	reply    chan ActionResult
}

// QueueAction enqueues an action. Returns a buffered (cap 1) reply
// channel. The reply is sent exactly once after the action is applied
// (or rejected at queue boundary).
//
// Never blocks. If the queue is full, the action is rejected immediately
// with reason "queue_full" — this is a backpressure signal to the agent
// to slow down its action rate.
func (w *World) QueueAction(entityID string, env *ActionEnvelope) <-chan ActionResult {
	pa := &pendingAction{
		entityID: entityID,
		env:      env,
		reply:    make(chan ActionResult, 1),
	}
	select {
	case w.actionQueue <- pa:
		// queued
	default:
		pa.reply <- ActionResult{
			ActionID: env.ActionID,
			Verb:     env.Verb,
			Accepted: false,
			Reason:   "queue_full",
		}
	}
	return pa.reply
}

// drainActionQueue applies queued actions until EITHER the per-tick cap
// is hit OR the per-tick time budget is exhausted. Caller holds the
// world write lock; each reply channel is cap 1 so the send never blocks.
//
// Time budget: an action like move() can do BFS (bounded) plus scenario
// callbacks. At 60Hz the tick has 16.67ms total; we leave 60% for
// actions, 40% for the rest of the tick (entity loop, snapshot publish).
// Excess actions remain in the queue and apply next tick.
const drainTimeBudget = 10 * time.Millisecond

func (w *World) drainActionQueue(maxPerTick int) {
	deadline := time.Now().Add(drainTimeBudget)
	for i := 0; i < maxPerTick; i++ {
		select {
		case pa := <-w.actionQueue:
			res := w.applyQueuedAction(pa.entityID, pa.env)
			pa.reply <- res
		default:
			return
		}
		if time.Now().After(deadline) {
			return
		}
	}
}

// applyQueuedAction — same shape as the old SubmitAction body, minus the
// outer lock acquire (already held by Tick).
func (w *World) applyQueuedAction(entityID string, env *ActionEnvelope) ActionResult {
	e := w.entities[entityID]
	if e == nil {
		return ActionResult{
			ActionID: env.ActionID,
			Verb:     env.Verb,
			Accepted: false,
			Reason:   "unknown_entity",
		}
	}
	res := w.Dispatch(e, env)
	// Fire historian hook on accepted actions so native engine verbs
	// (move, speak, …) land in the run log + smoke scorer. SystemHost
	// wires this to bus.Queue(ActionAccepted{...}); bare engine no-ops.
	// Pass w.tick through so the callback doesn't have to read it back
	// via CurrentTick — which would deadlock since we already hold
	// the write lock here.
	if res.Accepted && w.onActionAccepted != nil {
		w.onActionAccepted(entityID, env.Verb, w.tick, env.Raw)
	}
	return res
}

// SpawnAgentEntity creates a fresh agent-archetype entity at a random
// walkable tile. Used by the register handler when no bind_entity is
// given and the world has no free agent-eligible body. Returns the new
// entity ID. Thread-safe (takes the write lock).
//
// D5 clustered spawn: if the rule set declares `spawn_hub_x`,
// `spawn_hub_y`, and `spawn_radius` (> 0), the random tile is drawn
// from within that disc. This forces every joining agent into mutual
// vision range during the first few in-game minutes so direct
// reciprocity (Nowak's `w > c/b`) can take hold. Falls back to the
// full-world random search when no hub is declared OR no walkable
// tile is found within the disc after the per-call budget.
func (w *World) SpawnAgentEntity(archetype, displayName string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if archetype == "" {
		archetype = "wanderer"
	}

	hubX := w.Rules.GetInt("spawn_hub_x", -1)
	hubY := w.Rules.GetInt("spawn_hub_y", -1)
	radius := w.Rules.GetInt("spawn_radius", 0)
	clustered := hubX >= 0 && hubY >= 0 && radius > 0

	// Find a random walkable, unoccupied tile. Try up to 64 times before
	// scanning the whole grid.
	var pos Tile
	found := false
	for i := 0; i < 64; i++ {
		var x, y int
		if clustered {
			// Uniform square within the disc bounding box, then
			// reject-sample to the disc. Square is `2*radius + 1`
			// wide / tall — for radius 15 that's 31 candidate tiles
			// per axis, plenty to find a free one.
			x = hubX + w.rng.IntN(2*radius+1) - radius
			y = hubY + w.rng.IntN(2*radius+1) - radius
			dx, dy := x-hubX, y-hubY
			if dx*dx+dy*dy > radius*radius {
				continue
			}
		} else {
			x = w.rng.IntN(w.WidthTiles)
			y = w.rng.IntN(w.HeightTiles)
		}
		if x < 0 || y < 0 || x >= w.WidthTiles || y >= w.HeightTiles {
			continue
		}
		t := Tile{x, y}
		if w.walkable[y][x] && w.occupants[t] == "" {
			pos = t
			found = true
			break
		}
	}
	if !found {
		// Disc was full / blocked / off-map; fall back to full-grid
		// scan so we never wedge registrations.
		for y := 0; y < w.HeightTiles && !found; y++ {
			for x := 0; x < w.WidthTiles; x++ {
				t := Tile{x, y}
				if w.walkable[y][x] && w.occupants[t] == "" {
					pos, found = t, true
					break
				}
			}
		}
	}
	if !found {
		return "", errNoFreeTile
	}
	id := nextEntityID(&w.eventSeq)
	e := &Entity{
		EntityID:     id,
		Archetype:    archetype,
		DisplayName:  displayName,
		LogicalTile:  pos,
		WalkFromTile: pos,
		WalkProgress: 1,
		Facing:       FacingS,
		Extras:       map[string]any{},
	}
	w.entities[id] = e
	w.occupants[pos] = id
	return id, nil
}

func nextEntityID(seq *uint64) string {
	*seq++
	return "spawn_" + formatUint64(*seq)
}

var errNoFreeTile = &simpleErr{"no_free_tile"}

type simpleErr struct{ s string }

func (e *simpleErr) Error() string { return e.s }
