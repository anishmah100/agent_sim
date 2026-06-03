// Package spatial provides O(1) "what entities are at / near a tile"
// queries via a flat grid index.
//
// Locked by docs/DECISIONS.md Q51 + docs/SYSTEM_ARCHITECTURE_V2.md.
//
// Every entity that has a position registers here at spawn / on every
// move. Every perception query (vision, hearing, AOI) consumes this.
// The Index doesn't know about kinds — that's the caller's job to filter.
package spatial

import "sync"

type Tile = [2]int

// Index is the spatial registry. Concurrent-safe.
type Index struct {
	mu       sync.RWMutex
	cells    map[Tile]map[string]struct{} // tile → set of entity IDs
	location map[string]Tile               // entity ID → current tile
}

func New() *Index {
	return &Index{
		cells:    make(map[Tile]map[string]struct{}),
		location: make(map[string]Tile),
	}
}

// Add records an entity at a tile. No-op if already there.
func (i *Index) Add(id string, t Tile) {
	i.mu.Lock()
	defer i.mu.Unlock()
	// If already tracked at a different tile, remove the old slot.
	if old, ok := i.location[id]; ok && old != t {
		i.removeUnlocked(id, old)
	}
	i.location[id] = t
	if i.cells[t] == nil {
		i.cells[t] = map[string]struct{}{}
	}
	i.cells[t][id] = struct{}{}
}

// Remove drops an entity's record.
func (i *Index) Remove(id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if t, ok := i.location[id]; ok {
		i.removeUnlocked(id, t)
		delete(i.location, id)
	}
}

func (i *Index) removeUnlocked(id string, t Tile) {
	if set, ok := i.cells[t]; ok {
		delete(set, id)
		if len(set) == 0 {
			delete(i.cells, t)
		}
	}
}

// Move updates an entity's position.
func (i *Index) Move(id string, to Tile) {
	i.Add(id, to)
}

// EntityAt returns the IDs of all entities at a tile.
func (i *Index) EntityAt(t Tile) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	set := i.cells[t]
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

// EntitiesInRadius returns the IDs of entities within Chebyshev
// distance r of center (inclusive). r=0 returns only entities at center.
func (i *Index) EntitiesInRadius(center Tile, r int) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := []string{}
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			t := Tile{center[0] + dx, center[1] + dy}
			if set, ok := i.cells[t]; ok {
				for id := range set {
					out = append(out, id)
				}
			}
		}
	}
	return out
}

// EntitiesInRect returns the IDs of entities in [x0,x1) × [y0,y1).
func (i *Index) EntitiesInRect(x0, y0, x1, y1 int) []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := []string{}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if set, ok := i.cells[Tile{x, y}]; ok {
				for id := range set {
					out = append(out, id)
				}
			}
		}
	}
	return out
}

// LocationOf returns the entity's tile + true. Returns zero + false
// if untracked.
func (i *Index) LocationOf(id string) (Tile, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	t, ok := i.location[id]
	return t, ok
}

// Size returns the total number of entities tracked.
func (i *Index) Size() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.location)
}
