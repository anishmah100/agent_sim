package world

// LiveSnapshot is an immutable, lock-free view of the world published at
// the END of every Tick(). Observation builders read it without touching
// the world lock; tick mutation never touches it. Cost: observations lag
// by one tick (~16ms at 60Hz) — agents see "the world as of last tick",
// not the absolute live state. Acceptable for LLM agents that react at
// 100ms+ cadence.
//
// Memory model:
//   - Entities is a fresh map; entity values are shallow copies of the
//     live entities. The slice fields inside (walkPath) are reused by
//     reference but are read-only from observation builders.
//   - VisionBlocks and TileKindGrid are static after world load and are
//     SHARED by reference across snapshots.
//   - EntityAtTile + EntitiesByID give us O(k) vision queries instead of
//     O(N entities).
//   - Audible is a fresh slice of recent events.

type LiveSnapshot struct {
	Tick        uint64
	MapID       string
	WidthTiles  int
	HeightTiles int

	Entities      map[string]*Entity   // id → snapshot copy
	EntityAtTile  map[Tile][]string    // tile → entity IDs (spatial idx)
	BuildingDoors map[Tile]buildingRef // immutable since load
	Audible       []AudibleEvent

	VisionBlocks [][]bool   // shared by reference; static
	TileKindGrid [][]string // shared by reference; static
}

// publishSnapshot copies tick-end state into an immutable snapshot and
// atomic-stores the pointer. Must be called WITH the world write-lock
// held (we're reading mutable state). Cost: O(N entities) copy +
// O(N entities) spatial index build per tick.
func (w *World) publishSnapshot() {
	ents := make(map[string]*Entity, len(w.entities))
	atTile := make(map[Tile][]string, len(w.entities))
	for id, e := range w.entities {
		cp := *e
		ents[id] = &cp
		// Index by LogicalTile (the canonical "where they are now" cell).
		// Mid-walk entities still index by LogicalTile — vision and
		// audible care about the destination cell, not the from-cell.
		atTile[e.LogicalTile] = append(atTile[e.LogicalTile], id)
	}
	doors := make(map[Tile]buildingRef, len(w.buildingDoors))
	for k, v := range w.buildingDoors {
		doors[k] = v
	}
	aud := make([]AudibleEvent, len(w.audible))
	copy(aud, w.audible)
	snap := &LiveSnapshot{
		Tick:          w.tick,
		MapID:         w.MapID,
		WidthTiles:    w.WidthTiles,
		HeightTiles:   w.HeightTiles,
		Entities:      ents,
		EntityAtTile:  atTile,
		BuildingDoors: doors,
		Audible:       aud,
		VisionBlocks:  w.visionBlocks,
		TileKindGrid:  w.tileKindGrid,
	}
	w.snapshot.Store(snap)
}

// LoadSnapshot returns the latest published snapshot (nil before first
// tick). Safe to call without any lock.
func (w *World) LoadSnapshot() *LiveSnapshot {
	return w.snapshot.Load()
}

// ── Observation methods on snapshot (lock-free) ───────────────────────

// tileBlocksVision — snapshot variant that reads the shared (static)
// visionBlocks grid. Bounds-checks first.
func (s *LiveSnapshot) tileBlocksVision(t Tile) bool {
	if t[0] < 0 || t[0] >= s.WidthTiles || t[1] < 0 || t[1] >= s.HeightTiles {
		return true
	}
	return s.VisionBlocks[t[1]][t[0]]
}

// lineOfSight — bresenham, same as world.lineOfSight but reads snapshot.
func (s *LiveSnapshot) lineOfSight(a, b Tile) bool {
	x0, y0 := a[0], a[1]
	x1, y1 := b[0], b[1]
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if x0 == x1 && y0 == y1 {
			return true
		}
		if !(x0 == a[0] && y0 == a[1]) {
			if s.tileBlocksVision(Tile{x0, y0}) {
				return false
			}
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func (s *LiveSnapshot) seesEntity(eTile Tile, otherTile Tile, radius int) bool {
	if chebyshev(eTile, otherTile) > radius {
		return false
	}
	return s.lineOfSight(eTile, otherTile)
}

func (s *LiveSnapshot) seesTile(eTile, t Tile, radius int) bool {
	if chebyshev(eTile, t) > radius {
		return false
	}
	return s.lineOfSight(eTile, t)
}

// visibleAudible — snapshot variant of world.VisibleAudible.
func (s *LiveSnapshot) visibleAudible(eID string, eTile Tile, sinceTick uint64) []AudibleEvent {
	out := make([]AudibleEvent, 0, 4)
	for _, ev := range s.Audible {
		if ev.Tick < sinceTick {
			continue
		}
		if ev.whisperTo != "" && ev.whisperTo != eID {
			continue
		}
		if chebyshev(eTile, ev.FromPos) > ev.radius {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// buildObservationSnap — lock-free observation builder. The CALLER does
// not need any world lock; the snapshot is immutable.
//
// Uses the EntityAtTile spatial index to iterate only entities in the
// vision-radius box (O(r² + k)) instead of all entities (O(N)).
func (s *LiveSnapshot) buildObservationSnap(e *Entity, obsID uint64, opts *AgentObservationOpts) *Observation {
	o := opts
	if o == nil {
		def := defaultObsOpts()
		o = &def
	}
	if o.Radius <= 0 {
		o.Radius = VisionRadius
	}
	if o.LastSinceTick == 0 && s.Tick > 240 {
		o.LastSinceTick = s.Tick - 240
	}

	obs := &Observation{
		ObsID:     obsID,
		WorldTick: s.Tick,
		Self: SelfState{
			EntityID: e.EntityID,
			Pos:      e.LogicalTile,
			Facing:   string(e.Facing),
		},
		// Initialize collection fields as empty slices, NOT nil. Go's
		// json.Marshal renders nil as `null`, which the SDK's pydantic
		// Observation model rejects (these are typed list[...], not
		// Optional). Empty slices serialize to `[]` and clients can
		// iterate without null-guards.
		VisibleEntities: []VisibleEntityState{},
		VisibleObjects:  []VisibleObjectState{},
		Audible:         []AudibleEvent{},
		WorldClock: WorldClockState{
			Tick:     s.Tick,
			DayPhase: dayPhaseFromTick(s.Tick),
			Weather:  "clear",
		},
		KnownMap: &KnownMapSummary{
			MapID:   s.MapID,
			MapDims: [2]int{s.WidthTiles, s.HeightTiles},
		},
	}
	if e.CurrentAction != "" {
		obs.Self.CurrentAction = map[string]interface{}{
			"verb": e.CurrentAction,
		}
	}

	// Spatial-index walk: scan only the bounding box around the self tile.
	cx, cy := e.LogicalTile[0], e.LogicalTile[1]
	r := o.Radius
	x0, y0 := cx-r, cy-r
	x1, y1 := cx+r, cy+r
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			ids := s.EntityAtTile[Tile{x, y}]
			for _, id := range ids {
				if id == e.EntityID {
					continue
				}
				other := s.Entities[id]
				if other == nil || other.InsideBuilding != "" {
					continue
				}
				if !s.seesEntity(e.LogicalTile, other.LogicalTile, r) {
					continue
				}
				obs.VisibleEntities = append(obs.VisibleEntities, VisibleEntityState{
					EntityID:      other.EntityID,
					ApparentLabel: apparentLabel(other),
					Pos:           other.LogicalTile,
					Facing:        string(other.Facing),
					Archetype:     other.Archetype,
				})
			}
		}
	}

	// Building doors — small set per map; iterating all is fine.
	for door, ref := range s.BuildingDoors {
		if !s.seesTile(e.LogicalTile, door, r) {
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

	obs.Audible = s.visibleAudible(e.EntityID, e.LogicalTile, o.LastSinceTick)
	return obs
}
