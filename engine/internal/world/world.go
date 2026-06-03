// Package world owns the authoritative game state and the per-tick
// simulation. Everything else (wire protocol, scenario hooks, viewer
// broadcast) reads or writes through this package.
//
// Per docs/ARCHITECTURE.md §2: the engine knows entity_id, position,
// facing, an opaque state blob, and the registered verb list. Nothing
// scenario-specific lives here. Money, combat, HP — all that lives in
// the scenario pack and rides in entity.Extras.
package world

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
)

type Facing string

const (
	FacingN Facing = "N"
	FacingS Facing = "S"
	FacingE Facing = "E"
	FacingW Facing = "W"
)

// Entity is the authoritative server-side representation of one thing
// in the world. Pos is in tile coordinates (integer logically — we
// store as float for fractional movement during walks).
type Entity struct {
	EntityID    string         `json:"entity_id"`
	Archetype   string         `json:"archetype"`
	Pos         [2]float64     `json:"pos"`
	Facing      Facing         `json:"facing"`
	DisplayName string         `json:"display_name,omitempty"`
	Extras      map[string]any `json:"extras,omitempty"` // scenario-defined, opaque to engine
}

// World holds the live state for one world (one process = one world,
// per docs/ARCHITECTURE.md §5). Concurrency model: all writes happen on
// the tick goroutine; reads can happen from broadcast goroutines under
// a RLock.
type World struct {
	MapID       string
	WidthTiles  int
	HeightTiles int

	mu       sync.RWMutex
	tick     uint64
	entities map[string]*Entity
	rng      *rand.Rand
}

// fileWorld is the on-disk shape of worlds/<name>.json. Matches the
// in-house tile format described in worlds/dev_test.json. We don't read
// tile data here — the engine doesn't need it for movement (collision
// is layered in once the scenario pack is wired). The frontend loads
// the static JSON directly for now.
type fileWorld struct {
	MapID       string            `json:"map_id"`
	WidthTiles  int               `json:"width_tiles"`
	HeightTiles int               `json:"height_tiles"`
	Entities    []json.RawMessage `json:"entities"`
}

type fileEntity struct {
	EntityID    string     `json:"entity_id"`
	Archetype   string     `json:"archetype"`
	Pos         [2]float64 `json:"pos"`
	Facing      Facing     `json:"facing"`
	DisplayName string     `json:"display_name"`
}

// Load parses the world's JSON file and constructs a populated World.
func Load(path string) (*World, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var fw fileWorld
	if err := json.Unmarshal(data, &fw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	w := &World{
		MapID:       fw.MapID,
		WidthTiles:  fw.WidthTiles,
		HeightTiles: fw.HeightTiles,
		entities:    make(map[string]*Entity, len(fw.Entities)),
		// Deterministic-ish seed for v0. Real seeds come from scenario
		// config later.
		rng: rand.New(rand.NewPCG(1, 2)),
	}
	for _, raw := range fw.Entities {
		var fe fileEntity
		if err := json.Unmarshal(raw, &fe); err != nil {
			return nil, fmt.Errorf("parse entity: %w", err)
		}
		w.entities[fe.EntityID] = &Entity{
			EntityID:    fe.EntityID,
			Archetype:   fe.Archetype,
			Pos:         fe.Pos,
			Facing:      fe.Facing,
			DisplayName: fe.DisplayName,
		}
	}
	return w, nil
}

// Tick advances simulation by one step. Called from the engine's main
// loop at 60Hz. For v0 we just have a placeholder "wander" behaviour
// so the frontend can see live state updates while we wire the rest.
// Real verb dispatch + scenario rules land in milestone 3+.
func (w *World) Tick() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.tick++

	// Move each NPC a tiny bit every tick. The pace is fractional so
	// it reads as smooth walking on the client (no engine-side
	// interpolation needed). Every ~1 second (60 ticks) the NPC picks
	// a new direction.
	for _, e := range w.entities {
		if w.tick%60 == 0 || w.rng.IntN(240) == 0 {
			e.Facing = randFacing(w.rng)
		}
		const speedTilesPerTick = 0.04
		switch e.Facing {
		case FacingN:
			e.Pos[1] -= speedTilesPerTick
		case FacingS:
			e.Pos[1] += speedTilesPerTick
		case FacingE:
			e.Pos[0] += speedTilesPerTick
		case FacingW:
			e.Pos[0] -= speedTilesPerTick
		}
		// Clamp to world bounds; flip facing if we hit an edge so the
		// NPC bounces off instead of pinning to the wall.
		if e.Pos[0] < 0 {
			e.Pos[0] = 0
			e.Facing = FacingE
		} else if float64(w.WidthTiles-1) < e.Pos[0] {
			e.Pos[0] = float64(w.WidthTiles - 1)
			e.Facing = FacingW
		}
		if e.Pos[1] < 0 {
			e.Pos[1] = 0
			e.Facing = FacingS
		} else if float64(w.HeightTiles-1) < e.Pos[1] {
			e.Pos[1] = float64(w.HeightTiles - 1)
			e.Facing = FacingN
		}
	}
}

func randFacing(r *rand.Rand) Facing {
	switch r.IntN(4) {
	case 0:
		return FacingN
	case 1:
		return FacingS
	case 2:
		return FacingE
	default:
		return FacingW
	}
}

// Snapshot returns a read-only copy of the current entity list. Safe
// to call from any goroutine; takes the read lock.
//
// For the v0 broadcast we send a full snapshot every push. Delta
// encoding comes in milestone 5+ alongside AOI.
func (w *World) Snapshot() WorldSnapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	ents := make([]Entity, 0, len(w.entities))
	for _, e := range w.entities {
		ents = append(ents, *e)
	}
	return WorldSnapshot{
		Tick:     w.tick,
		MapID:    w.MapID,
		Entities: ents,
	}
}

// Tick returns the current tick number (cheap; readlock).
func (w *World) Tick0() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.tick
}

// WorldSnapshot is the wire-shape we send to viewers. Will be replaced
// by a delta payload once AOI subscription lands.
type WorldSnapshot struct {
	Tick     uint64   `json:"tick"`
	MapID    string   `json:"map_id"`
	Entities []Entity `json:"entities"`
}
