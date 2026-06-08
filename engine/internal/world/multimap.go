package world

// Multi-map support.
//
// One engine process can host multiple maps. The "overworld" map is
// the user-visible top-level; each building's interior is a separate
// sub-map loaded from a small JSON template. Entities track their
// CurrentMap; portal tiles (doors, stairs) move them between maps.
//
// All gameplay state (entities, decorations, walkable, vision) lives
// on the World struct that represents a single map. The engine main
// loop ticks one map per goroutine. The MultiMapHub owns the set.

import (
	"path/filepath"
	"sync"
)

// MultiMapHub — holds the engine's loaded maps and routes warp events.
type MultiMapHub struct {
	mu      sync.RWMutex
	maps    map[string]*World        // map_id → world
	primary string                   // the overworld's map_id
}

func NewMultiMapHub(primary *World) *MultiMapHub {
	h := &MultiMapHub{
		maps:    make(map[string]*World),
		primary: primary.MapID,
	}
	h.maps[primary.MapID] = primary
	return h
}

// LoadInterior loads an interior sub-map (small JSON file) and
// registers it under its map_id. Returns the loaded World.
func (h *MultiMapHub) LoadInterior(path string) (*World, error) {
	w, err := Load(path)
	if err != nil {
		return nil, err
	}
	w.MapID = "interior:" + w.MapID + ":" + filepath.Base(path)
	h.mu.Lock()
	h.maps[w.MapID] = w
	h.mu.Unlock()
	return w, nil
}

// Maps returns a snapshot of loaded map IDs.
func (h *MultiMapHub) Maps() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]string, 0, len(h.maps))
	for id := range h.maps {
		out = append(out, id)
	}
	return out
}

// Get returns a loaded world by ID.
func (h *MultiMapHub) Get(id string) *World {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.maps[id]
}

// Primary returns the overworld (top-level) map.
func (h *MultiMapHub) Primary() *World {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.maps[h.primary]
}

// WorldOf returns the map that currently holds the given entity, or nil if
// no loaded map contains it. Used to route an agent's observations/actions to
// whichever map it's standing on (overworld or a building interior).
func (h *MultiMapHub) WorldOf(entityID string) *World {
	h.mu.RLock()
	ms := make([]*World, 0, len(h.maps))
	for _, w := range h.maps {
		ms = append(ms, w)
	}
	h.mu.RUnlock()
	for _, w := range ms {
		if w.EntityByID(entityID) != nil {
			return w
		}
	}
	return nil
}

// Warp moves an entity from one map to another. The entity is removed
// from `from` and added to `to` at the target tile. Returns false if
// either map is unknown or the entity isn't on `from`.
func (h *MultiMapHub) Warp(entityID, fromID, toID string, target Tile) bool {
	h.mu.RLock()
	from := h.maps[fromID]
	to := h.maps[toID]
	h.mu.RUnlock()
	if from == nil || to == nil {
		return false
	}
	// Pull the entity off `from`.
	from.mu.Lock()
	e := from.entities[entityID]
	if e == nil {
		from.mu.Unlock()
		return false
	}
	delete(from.occupants, e.LogicalTile)
	delete(from.entities, entityID)
	from.mu.Unlock()
	// Push onto `to`.
	to.mu.Lock()
	e.LogicalTile = target
	e.WalkFromTile = target
	e.WalkProgress = 1
	e.CurrentAction = ""
	e.CurrentMap = toID
	to.entities[entityID] = e
	to.occupants[target] = entityID
	to.mu.Unlock()
	return true
}

// TickAll ticks every loaded map. Maps are independent so we can fan
// out across goroutines later; v1 is sequential to keep ordering
// deterministic.
func (h *MultiMapHub) TickAll() {
	h.mu.RLock()
	ms := make([]*World, 0, len(h.maps))
	for _, w := range h.maps {
		ms = append(ms, w)
	}
	h.mu.RUnlock()
	for _, w := range ms {
		w.Tick()
	}
}
