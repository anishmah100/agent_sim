package world

import "sync/atomic"

// Observation builder.
//
// Composes the per-agent payload from the engine's authoritative state:
// the self block, visible entities (vision-radius + LOS), visible
// objects (decorations with affordances), audible events (last few
// ticks), recent action results, a known-map summary, and the world
// clock. Excludes a view_image — that's added by the rasterizer when
// the agent's vision_mode includes it.

type AgentObservationOpts struct {
	Radius           int            // override default vision radius
	IncludeOccupants bool           // include passersby in visible_entities
	LastSinceTick    uint64         // window for audible events
}

func defaultObsOpts() AgentObservationOpts {
	return AgentObservationOpts{
		Radius:           VisionRadius,
		IncludeOccupants: true,
		LastSinceTick:    0, // engine fills in (tick - 240) at build time
	}
}

// BuildObservation returns a fresh Observation for the given entity.
// Caller holds the world lock.
func (w *World) BuildObservation(e *Entity, obsID uint64, opts *AgentObservationOpts) *Observation {
	o := opts
	if o == nil {
		def := defaultObsOpts()
		o = &def
	}
	if o.Radius <= 0 {
		o.Radius = VisionRadius
	}
	if o.LastSinceTick == 0 && w.tick > 240 {
		o.LastSinceTick = w.tick - 240
	}

	obs := &Observation{
		ObsID:     obsID,
		WorldTick: w.tick,
		Self: SelfState{
			EntityID:       e.EntityID,
			Pos:            e.LogicalTile,
			Facing:         string(e.Facing),
			Extras:         copyExtras(e.Extras),
			InsideBuilding: e.InsideBuilding,
		},
		WorldClock: WorldClockState{
			Tick:      w.tick,
			DayPhase:  dayPhaseFromTick(w.tick),
			Weather:   "clear",
		},
		KnownMap: &KnownMapSummary{
			MapID:    w.MapID,
			MapDims:  [2]int{w.WidthTiles, w.HeightTiles},
		},
	}
	if e.CurrentAction != "" {
		obs.Self.CurrentAction = map[string]interface{}{
			"verb": e.CurrentAction,
		}
	}
	for _, other := range w.entities {
		if other.EntityID == e.EntityID || other.InsideBuilding != "" {
			continue
		}
		if !w.SeesEntity(e, other, o.Radius) {
			continue
		}
		// D8 — items split off into visible_items, not visible_entities.
		// Different consumer affordance (pickup, not negotiate).
		if other.Archetype == "item" {
			sprite, _ := other.Extras["sprite"].(string)
			if sprite == "" {
				sprite = "item:" + other.EntityID
			}
			qty := 1
			if q, ok := other.Extras["quantity"].(int); ok && q > 0 {
				qty = q
			}
			obs.VisibleItems = append(obs.VisibleItems, VisibleItemState{
				EntityID: other.EntityID,
				Sprite:   sprite,
				Pos:      other.LogicalTile,
				Quantity: qty,
				Label:    other.DisplayName,
			})
			continue
		}
		obs.VisibleEntities = append(obs.VisibleEntities, VisibleEntityState{
			EntityID:      other.EntityID,
			ApparentLabel: apparentLabel(other),
			Pos:           other.LogicalTile,
			Facing:        string(other.Facing),
			Archetype:     other.Archetype,
			ExtrasSummary: buildExtrasSummary(other),
		})
	}
	for door, ref := range w.buildingDoors {
		if !w.SeesTile(e, door, o.Radius) {
			continue
		}
		obs.VisibleObjects = append(obs.VisibleObjects, VisibleObjectState{
			ObjectID:    "door:" + ref.Sprite + ":" + tileKey(door),
			Kind:        "door",
			Pos:         door,
			Affordances: []string{"enter"},
			StateSummary: map[string]interface{}{
				"building_sprite": ref.Sprite,
			},
		})
	}
	obs.Audible = w.VisibleAudible(e, o.LastSinceTick)
	return obs
}

type Observation struct {
	ObsID             uint64                `json:"obs_id"`
	WorldTick         uint64                `json:"world_tick"`
	Self              SelfState             `json:"self"`
	VisibleEntities   []VisibleEntityState  `json:"visible_entities"`
	VisibleObjects    []VisibleObjectState  `json:"visible_objects"`
	// D8 — items on the ground within vision + LOS. Populated from
	// entities with Archetype="item". Different field from VisibleObjects
	// because items are pickup-able (you can pickup verb them) whereas
	// VisibleObjects (doors/decorations) only support interact afford.
	VisibleItems      []VisibleItemState    `json:"visible_items,omitempty"`
	Audible           []AudibleEvent        `json:"audible"`
	RecentSelfResults []ActionResult        `json:"recent_self_results,omitempty"`
	KnownMap          *KnownMapSummary      `json:"known_map_summary,omitempty"`
	WorldClock        WorldClockState       `json:"world_clock"`
	ViewImage         *ViewImage            `json:"view_image,omitempty"`
}

type SelfState struct {
	EntityID         string                 `json:"entity_id"`
	Pos              Tile                   `json:"pos"`
	Facing           string                 `json:"facing"`
	Extras           map[string]interface{} `json:"extras,omitempty"`
	// InsideBuilding — non-empty when this entity is currently inside a
	// building (set by the property system's enter handler). Surfaces
	// in observations so the SDK can offer Exit, and so the brain
	// knows it's indoors.
	InsideBuilding   string                 `json:"inside_building,omitempty"`
	CurrentAction    map[string]interface{} `json:"current_action,omitempty"`
	LastActionResult *ActionResult          `json:"last_action_result,omitempty"`
}

// copyExtras shallow-copies an entity's extras map so the observation
// builder hands the agent a snapshot that isn't aliased to the live
// state (which the tick may mutate while the observation is queued).
// Caller is expected to hold whatever lock is appropriate for reading
// e.Extras (write lock in the live path, none in the snapshot path).
func copyExtras(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

type VisibleEntityState struct {
	EntityID      string                 `json:"entity_id"`
	ApparentLabel string                 `json:"apparent_label"`
	Pos           Tile                   `json:"pos"`
	Facing        string                 `json:"facing"`
	Archetype     string                 `json:"archetype"`
	ExtrasSummary map[string]interface{} `json:"extras_summary,omitempty"`
	Doing         string                 `json:"doing,omitempty"`
}

type VisibleObjectState struct {
	ObjectID     string                 `json:"object_id"`
	Kind         string                 `json:"kind"`
	Pos          Tile                   `json:"pos"`
	Affordances  []string               `json:"affordances,omitempty"`
	StateSummary map[string]interface{} `json:"state_summary,omitempty"`
}

// VisibleItemState — an item-archetype entity in vision + LOS. Items
// are pickup-able via the `pickup` verb. `Sprite` carries the kind
// (e.g. "item:apple"); `Quantity` defaults to 1 for non-stackable
// items, higher for stacks like coin piles. `Label` is the human-
// readable name if available.
type VisibleItemState struct {
	EntityID string `json:"entity_id"`
	Sprite   string `json:"sprite"`
	Pos      Tile   `json:"pos"`
	Quantity int    `json:"quantity,omitempty"`
	Label    string `json:"label,omitempty"`
}

type WorldClockState struct {
	Tick     uint64 `json:"tick"`
	DayPhase string `json:"day_phase"`
	Weather  string `json:"weather"`
}

type KnownMapSummary struct {
	MapID    string   `json:"map_id"`
	MapDims  [2]int   `json:"map_dims"`
}

func dayPhaseFromTick(tick uint64) string {
	// One in-engine day = 14400 ticks (4 min @ 60Hz). Phase by sextiles.
	const dayLen = 14400
	t := tick % dayLen
	switch {
	case t < dayLen/12:
		return "dawn"
	case t < dayLen/4:
		return "morning"
	case t < dayLen*5/12:
		return "midday"
	case t < dayLen*7/12:
		return "midday"
	case t < dayLen*9/12:
		return "afternoon"
	case t < dayLen*11/12:
		return "dusk"
	default:
		return "night"
	}
}

// apparentLabel — for v0 we expose the display name OR the archetype
// when no name is set. Persona-relationship-driven labels land later.
func apparentLabel(e *Entity) string {
	if e.DisplayName != "" {
		return e.DisplayName
	}
	return e.Archetype
}

// buildExtrasSummary — D9. What other agents see about you.
// Combat-relevant state is public; resource/economic state is private.
// Concretely surfaces: equipped_slot, equipped_sprite (so threats are
// readable at a glance), hp_bucket ("full"/"wounded"/"dying" — coarse
// bins to avoid leaking exact HP). Does NOT surface inventory, gold,
// hunger — those stay opaque so wealth + needs must be inferred from
// behavior + dialogue.
func buildExtrasSummary(e *Entity) map[string]interface{} {
	if len(e.Extras) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	// HP bucket: full (≥80%), wounded (30-80%), dying (<30%).
	hp, hpOK := numericExtra(e.Extras, "hp")
	maxHP, maxOK := numericExtra(e.Extras, "max_hp")
	if hpOK && maxOK && maxHP > 0 {
		frac := hp / maxHP
		switch {
		case frac >= 0.8:
			out["hp_bucket"] = "full"
		case frac >= 0.3:
			out["hp_bucket"] = "wounded"
		default:
			out["hp_bucket"] = "dying"
		}
	}
	// Equipped — surface the FIRST populated slot (typically "weapon"
	// or "hand"). Carries slot name + sprite so threat assessment is
	// possible at a glance ("they're wielding an axe").
	if eq, ok := e.Extras["equipped"].(map[string]interface{}); ok {
		for slot, raw := range eq {
			itemID, _ := raw.(string)
			if itemID == "" {
				continue
			}
			out["equipped_slot"] = slot
			out["equipped_sprite"] = spriteFromItemID(itemID)
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// numericExtra reads either an int, float64, or float32 from an
// Extras key. Returns (value, ok).
func numericExtra(extras map[string]interface{}, key string) (float64, bool) {
	v, ok := extras[key]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case float32:
		return float64(x), true
	}
	return 0, false
}

// spriteFromItemID maps an inventory/equipped item_id back to its
// sprite. Convention: item ids are formatted as "item:<kind>#<seq>"
// (e.g. "item:sword_short#42"). Strip the suffix to recover the
// sprite. If the id has no colon at all, assume it IS the kind and
// prefix with "item:".
func spriteFromItemID(id string) string {
	if id == "" {
		return ""
	}
	// Strip "#suffix" if present.
	for i := 0; i < len(id); i++ {
		if id[i] == '#' {
			id = id[:i]
			break
		}
	}
	if hasColon := func(s string) bool {
		for i := 0; i < len(s); i++ {
			if s[i] == ':' {
				return true
			}
		}
		return false
	}(id); !hasColon {
		return "item:" + id
	}
	return id
}

func tileKey(t Tile) string {
	return formatUint64(uint64(t[0])) + "," + formatUint64(uint64(t[1]))
}

// === Public agent-facing API. The wire package uses these. ===

// EntityByID returns a snapshot copy of the entity (safe to inspect
// without holding the world lock). Returns nil if not found.
//
// Scenario callbacks invoked from inside Dispatch / Tick (where the
// world write lock is already held) MUST call EntityByIDUnlocked
// instead — calling this one re-enters the lock and deadlocks.
func (w *World) EntityByID(id string) *Entity {
	w.mu.RLock()
	defer w.mu.RUnlock()
	e := w.entities[id]
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}

// EntityByIDUnlocked — caller must already hold the world lock.
func (w *World) EntityByIDUnlocked(id string) *Entity {
	return w.entities[id]
}

// EntityIDs returns the live entity ID set.
func (w *World) EntityIDs() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]string, 0, len(w.entities))
	for id := range w.entities {
		out = append(out, id)
	}
	return out
}

// EntityIDsUnlocked — caller must already hold the world lock.
func (w *World) EntityIDsUnlocked() []string {
	out := make([]string, 0, len(w.entities))
	for id := range w.entities {
		out = append(out, id)
	}
	return out
}

// SubmitAction enqueues an action and waits for it to be applied at the
// next tick. Latency 0–16ms. Strict per-tick ordering across all agents.
//
// Callers that want non-blocking semantics should use World.QueueAction
// directly and drive the reply channel themselves.
func (w *World) SubmitAction(entityID string, env *ActionEnvelope) ActionResult {
	return <-w.QueueAction(entityID, env)
}

// BuildObservationFor — LOCK-FREE observation builder. Reads the latest
// published snapshot; if none yet (cold start before first tick) falls
// back to the locked path for a one-shot bootstrap obs.
func (w *World) BuildObservationFor(entityID string, obsID uint64, opts *AgentObservationOpts) *Observation {
	if snap := w.snapshot.Load(); snap != nil {
		e := snap.Entities[entityID]
		if e == nil {
			return nil
		}
		return snap.buildObservationSnap(e, obsID, opts)
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	e := w.entities[entityID]
	if e == nil {
		return nil
	}
	return w.BuildObservation(e, obsID, opts)
}

// DebugObsAtTile — synthetic observation at the given tile using the
// CURRENT live snapshot. The probe is fake (not a real entity); used
// only by /api/v1/debug/vision to diagnose what the engine's
// observation pipeline would return for that position. Returns nil
// if no snapshot has been published yet.
func (w *World) DebugObsAtTile(at [2]int) *Observation {
	snap := w.snapshot.Load()
	if snap == nil {
		return nil
	}
	probe := &Entity{
		EntityID:    "_debug_probe",
		Archetype:   "trainer",
		LogicalTile: Tile{at[0], at[1]},
		Facing:      FacingS,
		Extras:      map[string]any{},
	}
	return snap.buildObservationSnap(probe, 0, nil)
}

// CurrentTick returns the latest engine tick (for cadence calc).
//
// Lock-free: reads w.tick atomically. Writes happen under w.mu.Lock()
// at the top of every Tick() via atomic.AddUint64. Concurrent atomic
// reads of an aligned uint64 are safe; the worst case is a stale tick
// by one — fine for callers (event stamping, /world/info), and the
// correct shape for any reader that might be inside Bus.Drain (which
// runs under the world write lock — re-acquiring a read lock there
// would deadlock since sync.RWMutex doesn't allow write→read re-entry).
func (w *World) CurrentTick() uint64 {
	return atomic.LoadUint64(&w.tick)
}

// MutateEntity runs `f` against the live entity holding the world
// write-lock. Use this from a verb handler that's already inside a
// locked Dispatch — but be aware Dispatch holds the lock, so callers
// from within Dispatch should NOT take the lock again. This wrapper
// is safe to call from EITHER inside an existing locked section OR
// from outside, because it uses sync.Mutex's TryLock pattern: only
// locks if not already held by us. For v1 simplicity we just do the
// non-blocking path — scenario handlers are always called while the
// world lock is held.
func (w *World) MutateEntity(id string, f func(*Entity)) {
	e := w.entities[id]
	if e == nil {
		return
	}
	if e.Extras == nil {
		e.Extras = map[string]any{}
	}
	f(e)
}

// RemoveEntity deletes the entity from the world. Caller must hold
// the world lock (called from scenario handlers).
func (w *World) RemoveEntity(id string) {
	e := w.entities[id]
	if e == nil {
		return
	}
	delete(w.occupants, e.LogicalTile)
	delete(w.entities, id)
}

// SpawnEntity adds a new entity. Caller must hold the world lock.
// Fires the scenario's onSpawn hook so system seedSpawn callbacks
// (combat.hp, money.gold, vitals.hunger, …) initialize stats on
// runtime-spawned entities the same way they do at world boot.
func (w *World) SpawnEntity(e *Entity) {
	if e.Extras == nil {
		e.Extras = map[string]any{}
	}
	w.entities[e.EntityID] = e
	w.fireSpawnHook(e)
}

// MutateExtra is a convenience for setting a single key without the
// closure dance.
func (w *World) MutateExtra(id, key string, value any) {
	if e := w.entities[id]; e != nil {
		if e.Extras == nil {
			e.Extras = map[string]any{}
		}
		e.Extras[key] = value
	}
}

// TileKindAt returns the kind name at the given tile, or "" if off-map.
// Used by the rasterizer to pick the right tile texture.
func (w *World) TileKindAt(x, y int) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if x < 0 || x >= w.WidthTiles || y < 0 || y >= w.HeightTiles {
		return ""
	}
	return w.tileKindGrid[y][x]
}

// DecorationsInRect returns all decorations whose footprint overlaps
// the [x0,x1) × [y0,y1) rectangle. Returned in placement order.
func (w *World) DecorationsInRect(x0, y0, x1, y1 int) []DecorationRef {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]DecorationRef, 0, 32)
	for _, d := range w.decorations {
		// d.X / d.Y is the SW corner; footprint extends N by fpH-1.
		fpW := d.FootprintW
		if fpW < 1 {
			fpW = 1
		}
		fpH := d.FootprintH
		if fpH < 1 {
			fpH = 1
		}
		minY := d.Y - fpH + 1
		if d.X+fpW <= x0 || d.X >= x1 || d.Y+1 <= y0 || minY >= y1 {
			continue
		}
		out = append(out, d)
	}
	return out
}

// ApplySnapshot overlays previously-saved per-entity state. Currently
// applies Extras + InsideBuilding only — position is kept where the
// world JSON defines it for now (interiors-as-real-maps lands later).
func (w *World) ApplySnapshot(entityID string, extras map[string]any, insideBuilding string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.entities[entityID]
	if e == nil {
		return
	}
	if extras != nil {
		if e.Extras == nil {
			e.Extras = map[string]any{}
		}
		for k, v := range extras {
			e.Extras[k] = v
		}
	}
	if insideBuilding != "" {
		e.InsideBuilding = insideBuilding
	}
}

// DecorationRef — public copy of a decoration record for accessor use.
type DecorationRef struct {
	X           int
	Y           int
	Sprite      string
	HeightTiles float64
	FootprintW  int
	FootprintH  int
	Walkable    bool
}
