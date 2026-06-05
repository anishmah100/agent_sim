// Package world owns the authoritative game state and the per-tick
// simulation. See docs/MOVEMENT_AND_COLLISION.md for the position
// model: logical tile is discrete (integer), render position is a
// float lerp the engine computes for the wire payload.
package world

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/anishmah100/agent_sim/engine/internal/world/rules"
)

type Facing string

const (
	FacingN Facing = "N"
	FacingS Facing = "S"
	FacingE Facing = "E"
	FacingW Facing = "W"
)

// TICKS_PER_STEP — number of engine ticks for one tile-to-tile walk.
// At 60Hz, 24 ticks ≈ 400 ms per tile, similar to HeartGold walk speed.
const TicksPerStep = 24

type Tile = [2]int

// Entity is the authoritative server-side representation. Position is
// stored discretely as LogicalTile + WalkProgress; the client lerps for
// smooth rendering.
type Entity struct {
	EntityID    string         `json:"entity_id"`
	Archetype   string         `json:"archetype"`
	DisplayName string         `json:"display_name,omitempty"`
	Extras      map[string]any `json:"extras,omitempty"`

	LogicalTile  Tile    `json:"-"`
	WalkFromTile Tile    `json:"-"`
	WalkProgress float64 `json:"-"`
	walkPath     []Tile  // queued path; first elem = next tile to enter

	Facing        Facing `json:"facing"`
	CurrentAction string `json:"current_action,omitempty"`
	actionTicks   int    // demo random action timer

	// PlayerControlled — true when an external bot has bound to this
	// entity. The engine's autonomous wander / demo-action loop SKIPS
	// these entities so the bot's intent isn't overridden every few
	// hundred ms by a random direction change. Toggled by SetPlayerControlled.
	PlayerControlled bool `json:"player_controlled,omitempty"`

	// InsideBuilding — non-empty when the entity is inside a building's
	// interior. While set, the entity is hidden from the overworld
	// render. Set to building sprite ID (e.g. "bld:000") + footprint
	// origin so the frontend can route renders to the right interior.
	InsideBuilding string `json:"inside_building,omitempty"`
	insideTicks    int    // remaining ticks until automatic exit

	// targetTile is the original move-target; emitted in observations so
	// the agent can confirm what it's walking toward.
	targetTile Tile

	// Computed render position broadcast on the wire.
	renderPos [2]float64
}

// publicExtraKeys — extras whitelist exposed to viewers. Anything not in
// this set stays server-side (inventory, contracts, owner tokens, etc.).
// Keep render-relevant + leaderboard-relevant keys here; private state
// must NEVER leak through the viewer snapshot.
var publicExtraKeys = map[string]bool{
	"progress":    true, // construction 0..100 — drives stage sprite
	"steps_done":  true,
	"steps_total": true,
	"kind":        true, // blueprint kind, item kind, etc.
	"hp":          true,
	"max_hp":      true,
	"gold":        true,
	"locked":      true, // building lock state
}

// MarshalJSON emits the render-friendly fields the frontend expects.
func (e *Entity) MarshalJSON() ([]byte, error) {
	var publicExtras map[string]any
	if len(e.Extras) > 0 {
		for k, v := range e.Extras {
			if !publicExtraKeys[k] {
				continue
			}
			if publicExtras == nil {
				publicExtras = make(map[string]any, 4)
			}
			publicExtras[k] = v
		}
	}
	return json.Marshal(struct {
		EntityID      string         `json:"entity_id"`
		Archetype     string         `json:"archetype"`
		DisplayName   string         `json:"display_name,omitempty"`
		Pos            [2]float64    `json:"pos"`
		Facing         Facing        `json:"facing"`
		CurrentAction  string        `json:"current_action,omitempty"`
		LogicalTile    [2]int        `json:"logical_tile"`
		InsideBuilding string        `json:"inside_building,omitempty"`
		Extras         map[string]any `json:"extras,omitempty"`
	}{
		EntityID:       e.EntityID,
		Archetype:      e.Archetype,
		DisplayName:    e.DisplayName,
		Pos:            e.renderPos,
		Facing:         e.Facing,
		CurrentAction:  e.CurrentAction,
		LogicalTile:    e.LogicalTile,
		InsideBuilding: e.InsideBuilding,
		Extras:         publicExtras,
	})
}

// recomputeRenderPos updates renderPos based on the walk state. Called
// every tick.
func (e *Entity) recomputeRenderPos() {
	if e.WalkProgress >= 1 {
		e.renderPos[0] = float64(e.LogicalTile[0])
		e.renderPos[1] = float64(e.LogicalTile[1])
		return
	}
	e.renderPos[0] = float64(e.WalkFromTile[0]) +
		(float64(e.LogicalTile[0])-float64(e.WalkFromTile[0]))*e.WalkProgress
	e.renderPos[1] = float64(e.WalkFromTile[1]) +
		(float64(e.LogicalTile[1])-float64(e.WalkFromTile[1]))*e.WalkProgress
}

type World struct {
	MapID       string
	WidthTiles  int
	HeightTiles int

	// Rules is the declarative ruleset for this world, parsed from
	// worlds/<name>/rules.star at load time. Nil if the bundle did not
	// declare [rules.file] (or if loaded via the legacy Load path).
	// Callers should read tunings via Rules.GetFloat(...) etc — those
	// methods are nil-safe and return the supplied default.
	Rules *rules.RuleSet

	mu       sync.RWMutex
	tick     uint64
	entities map[string]*Entity
	rng      *rand.Rand

	// Async action queue. WS goroutines enqueue without locking; tick
	// drains at start of each tick under the write lock.
	actionQueue chan *pendingAction

	// Lock-free snapshot for observation builders. Replaced atomically
	// at end of each tick.
	snapshot atomic.Pointer[LiveSnapshot]

	// Static walkability map (terrain only). True = walkable terrain.
	walkable [][]bool

	// Per-tile dynamic occupancy by entities. Mid-walk an entity claims
	// BOTH its WalkFromTile and LogicalTile.
	occupants map[Tile]string

	// Buildings the entity can enter. Key is the SOUTH-edge door tile
	// (the tile DIRECTLY south of the building's footprint, at its
	// horizontal centre). Value is the building's sprite ID + SW corner
	// — the same fields the frontend uses to pick which interior to
	// render. Entities at a door tile may switch to InsideBuilding.
	buildingDoors map[Tile]buildingRef

	// Tile-level vision blockers. true = tile blocks vision (walls,
	// tall blocking decorations). false = clear.
	visionBlocks [][]bool

	// Per-tile kind name (grass, dirt, water, ...). Used by the
	// rasterizer to pick the right tile texture. Same shape as walkable.
	tileKindGrid [][]string

	// Decoration list as loaded (read-only after init).
	decorations []DecorationRef

	// Audible event ring buffer + monotonic event ID counter.
	audible  []AudibleEvent
	eventSeq uint64

	// Scenario hooks — installed via InstallScenario. Both are nil for
	// a bare engine; when a scenario is installed, the dispatcher
	// consults verbHandlers and Tick() calls onTick.
	verbHandlers map[string]func(*World, *Entity, *ActionEnvelope) ActionResult
	onTick       func(*World, uint64)
}

// SetPlayerControlled toggles the PlayerControlled flag on an entity.
// Called when an external WS agent binds/unbinds to an entity. Idempotent.
// Returns false if the entity doesn't exist.
func (w *World) SetPlayerControlled(entityID string, on bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	e := w.entities[entityID]
	if e == nil {
		return false
	}
	e.PlayerControlled = on
	return true
}

// InstallScenario wires verb handlers and per-tick callback into the
// world. Call once at startup, before Tick() begins.
func (w *World) InstallScenario(verbs map[string]func(*World, *Entity, *ActionEnvelope) ActionResult, onTick func(*World, uint64), onSpawn func(*Entity)) {
	w.mu.Lock()
	w.verbHandlers = verbs
	w.onTick = onTick
	if onSpawn != nil {
		for _, e := range w.entities {
			onSpawn(e)
		}
	}
	w.mu.Unlock()
}

func (w *World) scenarioHandler(verb string) func(*World, *Entity, *ActionEnvelope) ActionResult {
	return w.verbHandlers[verb]
}

// VisionRadius / NightRadius — engine defaults; scenarios can override.
const (
	VisionRadius = 12
	NightRadius  = 6
)

type buildingRef struct {
	Sprite string
	X, Y   int
}

type fileWorld struct {
	MapID        string             `json:"map_id"`
	WidthTiles   int                `json:"width_tiles"`
	HeightTiles  int                `json:"height_tiles"`
	TilesLegend  map[string]string  `json:"tiles_legend"`
	Tiles        []string           `json:"tiles"`
	Entities     []json.RawMessage  `json:"entities"`
	Decorations  []fileDecoration   `json:"decorations,omitempty"`
}

type fileDecoration struct {
	X           int     `json:"x"`
	Y           int     `json:"y"`
	Sprite      string  `json:"sprite"`
	HeightTiles float64 `json:"height_tiles,omitempty"`
	FootprintW  int     `json:"footprint_w,omitempty"`
	FootprintH  int     `json:"footprint_h,omitempty"`
	Walkable    *bool   `json:"walkable,omitempty"`
}

type fileEntity struct {
	EntityID    string `json:"entity_id"`
	Archetype   string `json:"archetype"`
	Pos         [2]int `json:"pos"`
	Facing      Facing `json:"facing"`
	DisplayName string `json:"display_name"`
}

// objectArchetypes mirrors the closed set in
// internal/core/systems/archetypes.go — kept duplicated here to avoid
// importing systems from world (the cleaner direction is world →
// systems, not the other way, and this taxonomy is engine-wide).
// Keep these in sync.
var objectArchetypes = map[string]bool{
	"item":       true,
	"tree":       true,
	"rock":       true,
	"building":   true,
	"blueprint":  true,
	"decoration": true,
}

func isAgentArchetype(a string) bool { return !objectArchetypes[a] }

// Walkable tile kinds. Anything not in this set blocks movement.
var walkableKinds = map[string]bool{
	"grass":      true,
	"dirt":       true,
	"path":       true,
	"floor_wood": true,
	"stone":      true,
	"sand":       true,
}

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
		MapID:         fw.MapID,
		WidthTiles:    fw.WidthTiles,
		HeightTiles:   fw.HeightTiles,
		entities:      make(map[string]*Entity, len(fw.Entities)),
		rng:           rand.New(rand.NewPCG(1, 2)),
		occupants:     make(map[Tile]string),
		buildingDoors: make(map[Tile]buildingRef),
		// Cap at 16384 → at 1000 agents that's ~16 pending actions/agent.
		// Excess yields a "queue_full" reject (backpressure signal).
		actionQueue: make(chan *pendingAction, 16384),
	}
	w.visionBlocks = make([][]bool, fw.HeightTiles)
	w.tileKindGrid = make([][]string, fw.HeightTiles)
	for y := 0; y < fw.HeightTiles; y++ {
		w.visionBlocks[y] = make([]bool, fw.WidthTiles)
		w.tileKindGrid[y] = make([]string, fw.WidthTiles)
	}

	// Build walkability map from the tile legend.
	w.walkable = make([][]bool, w.HeightTiles)
	for y := 0; y < w.HeightTiles; y++ {
		w.walkable[y] = make([]bool, w.WidthTiles)
		row := ""
		if y < len(fw.Tiles) {
			row = fw.Tiles[y]
		}
		for x := 0; x < w.WidthTiles; x++ {
			var kind string
			if x < len(row) {
				ch := string(row[x])
				kind = fw.TilesLegend[ch]
			}
			w.walkable[y][x] = walkableKinds[kind]
			w.tileKindGrid[y][x] = kind
			// Walls block vision. Water/void don't (you can see across).
			if kind == "wall" {
				w.visionBlocks[y][x] = true
			}
		}
	}

	// Decorations can declare themselves blocking (trees, stumps, rocks).
	// Walkable defaults to FALSE if the field is missing — most veg in
	// our sheet is solid scenery. Bushes/flowers/groundcover get
	// walkable=true at placement time in the world generator.
	//
	// For decorations TALLER than 1.5 tiles (mostly trees), we also
	// block the tile north of the footprint — the canopy renders into
	// that tile and a character walking through it would visually be
	// inside the tree's leaves. Block forces the path to go around.
	for _, d := range fw.Decorations {
		// Record into the public read-only list.
		ref := DecorationRef{
			X: d.X, Y: d.Y, Sprite: d.Sprite,
			HeightTiles: d.HeightTiles,
			FootprintW:  d.FootprintW,
			FootprintH:  d.FootprintH,
		}
		if d.Walkable != nil {
			ref.Walkable = *d.Walkable
		}
		w.decorations = append(w.decorations, ref)

		if d.X < 0 || d.X >= w.WidthTiles || d.Y < 0 || d.Y >= w.HeightTiles {
			continue
		}
		walkable := false
		if d.Walkable != nil {
			walkable = *d.Walkable
		}
		if walkable {
			continue
		}
		fpW := d.FootprintW
		if fpW < 1 {
			fpW = 1
		}
		fpH := d.FootprintH
		if fpH < 1 {
			fpH = 1
		}
		// (X, Y) is the SW corner of the footprint. Block every tile in
		// the FPW × FPH slab anchored there.
		for dy := 0; dy < fpH; dy++ {
			for dx := 0; dx < fpW; dx++ {
				ny := d.Y - dy
				nx := d.X + dx
				if nx < 0 || nx >= w.WidthTiles || ny < 0 || ny >= w.HeightTiles {
					continue
				}
				w.walkable[ny][nx] = false
				// Tall blocking decorations also block vision through them.
				if d.HeightTiles >= 1.5 {
					w.visionBlocks[ny][nx] = true
				}
			}
		}
		// Tall objects (trees, buildings ≥ 1.5 tiles) also block the
		// rows ABOVE their footprint that the visual sprite covers —
		// otherwise a character can walk to a tile that's drawn-over by
		// the roof and end up apparently standing on top of the
		// building. We block ceil(height_tiles - fpH) additional rows.
		if d.HeightTiles >= 1.5 {
			extraRows := int(d.HeightTiles) - fpH
			if d.HeightTiles-float64(int(d.HeightTiles)) > 1e-9 {
				extraRows++   // ceil — covers fractional roof
			}
			if extraRows < 1 {
				extraRows = 1
			}
			for k := 1; k <= extraRows; k++ {
				ny := d.Y - fpH - (k - 1)
				if ny < 0 {
					continue
				}
				for dx := 0; dx < fpW; dx++ {
					nx := d.X + dx
					if nx >= 0 && nx < w.WidthTiles {
						w.walkable[ny][nx] = false
					}
				}
			}
		}
		// Buildings (multi-tile, blocking) register a door tile DIRECTLY
		// south of the footprint at its horizontal centre. Entities at
		// that tile can enter the interior.
		if fpW >= 2 && strings.HasPrefix(d.Sprite, "bld:") {
			doorX := d.X + fpW/2
			doorY := d.Y + 1
			if doorY < w.HeightTiles {
				w.buildingDoors[Tile{doorX, doorY}] = buildingRef{
					Sprite: d.Sprite, X: d.X, Y: d.Y,
				}
				// Make sure the door tile itself is walkable.
				w.walkable[doorY][doorX] = true
			}
		}
	}

	for _, raw := range fw.Entities {
		var fe fileEntity
		if err := json.Unmarshal(raw, &fe); err != nil {
			return nil, fmt.Errorf("parse entity: %w", err)
		}
		tile := Tile{fe.Pos[0], fe.Pos[1]}
		e := &Entity{
			EntityID:     fe.EntityID,
			Archetype:    fe.Archetype,
			DisplayName:  fe.DisplayName,
			Facing:       fe.Facing,
			LogicalTile:  tile,
			WalkFromTile: tile,
			WalkProgress: 1,
		}
		e.recomputeRenderPos()
		w.entities[fe.EntityID] = e
		w.occupants[tile] = fe.EntityID
	}
	return w, nil
}

// IsWalkable returns true if (x,y) is on the map AND has walkable
// terrain. Does not consider dynamic occupancy.
func (w *World) IsWalkable(t Tile) bool {
	if t[0] < 0 || t[0] >= w.WidthTiles || t[1] < 0 || t[1] >= w.HeightTiles {
		return false
	}
	return w.walkable[t[1]][t[0]]
}

// CanEnter returns true if entity e can enter tile t right now:
// walkable terrain AND not occupied by a different entity.
func (w *World) CanEnter(e *Entity, t Tile) bool {
	if !w.IsWalkable(t) {
		return false
	}
	occ, ok := w.occupants[t]
	if !ok {
		return true
	}
	return occ == e.EntityID
}

// pickWanderTarget — find a random walkable tile within radius of an
// entity that we can begin a path to. Used by the demo AI.
func (w *World) pickWanderTarget(e *Entity, radius int) (Tile, bool) {
	for tries := 0; tries < 20; tries++ {
		dx := w.rng.IntN(2*radius+1) - radius
		dy := w.rng.IntN(2*radius+1) - radius
		t := Tile{e.LogicalTile[0] + dx, e.LogicalTile[1] + dy}
		if w.IsWalkable(t) && t != e.LogicalTile {
			return t, true
		}
	}
	return Tile{}, false
}

// startMove kicks off an A*-pathed move toward target. Returns true if
// a path was found and the first step has been committed.
func (w *World) startMove(e *Entity, target Tile) bool {
	if !w.IsWalkable(target) {
		return false
	}
	if target == e.LogicalTile {
		return true
	}
	path := w.findPath(e.LogicalTile, target, e)
	if len(path) < 2 {
		return false
	}
	// path[0] is current. Step to path[1].
	next := path[1]
	if !w.CanEnter(e, next) {
		return false
	}
	// Claim the next tile.
	w.occupants[next] = e.EntityID
	e.WalkFromTile = e.LogicalTile
	e.LogicalTile = next
	e.WalkProgress = 0
	e.walkPath = path[2:]
	e.targetTile = target
	e.CurrentAction = "move"
	e.Facing = stepFacing(e.WalkFromTile, next)
	return true
}

func stepFacing(from, to Tile) Facing {
	if to[0] > from[0] {
		return FacingE
	}
	if to[0] < from[0] {
		return FacingW
	}
	if to[1] > from[1] {
		return FacingS
	}
	return FacingN
}

// findPath: BFS bounded by max manhattan distance. If the goal is too
// far from start we reject early — agents requesting moves across a
// 1000×1000 world would otherwise BFS through ~1M tiles per attempt and
// block the tick.
//
// The 64-tile cap (~5× vision radius) is well above any single-step
// move the engine itself initiates; long-haul paths must be requested as
// a sequence of intermediate targets by the agent.
const maxPathDistance = 64

func (w *World) findPath(start, goal Tile, e *Entity) []Tile {
	if start == goal {
		return []Tile{start}
	}
	// Cheap manhattan reject before allocating BFS state.
	if absInt(goal[0]-start[0])+absInt(goal[1]-start[1]) > maxPathDistance {
		return nil
	}
	type node struct {
		t      Tile
		parent int // index into visited
	}
	visited := []node{{t: start, parent: -1}}
	seen := map[Tile]int{start: 0}
	queue := []int{0}
	dirs := []Tile{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	// Visit cap as a second safety net — diagonals + obstacles can blow
	// past manhattan in pathological maps. 4096 tiles ≈ 64² area covered.
	const maxNodes = 4096
	for len(queue) > 0 && len(visited) < maxNodes {
		idx := queue[0]
		queue = queue[1:]
		cur := visited[idx].t
		if cur == goal {
			// reconstruct
			path := []Tile{}
			for i := idx; i != -1; i = visited[i].parent {
				path = append([]Tile{visited[i].t}, path...)
			}
			return path
		}
		for _, d := range dirs {
			n := Tile{cur[0] + d[0], cur[1] + d[1]}
			if _, ok := seen[n]; ok {
				continue
			}
			if !w.IsWalkable(n) {
				continue
			}
			// Allow path through tiles occupied by OTHERS at planning
			// time — they may move by the time we arrive. But block our
			// path through trees / walls.
			if occ, occupied := w.occupants[n]; occupied && occ != e.EntityID && n == goal {
				// Dest is currently occupied; reject so the agent gets
				// target_occupied at submission. (BFS allows transit but
				// not stopping ON an occupant.)
				continue
			}
			seen[n] = len(visited)
			visited = append(visited, node{t: n, parent: idx})
			queue = append(queue, len(visited)-1)
		}
	}
	return nil
}

func (w *World) Tick() {
	w.mu.Lock()
	defer func() {
		// Publish a fresh snapshot BEFORE releasing the lock so observation
		// readers always see a consistent post-tick view.
		w.publishSnapshot()
		w.mu.Unlock()
	}()

	w.tick++

	// Drain queued actions FIRST. This serializes external agent intent
	// with the tick clock — actions enqueued before tick N execute in
	// tick N, in FIFO order. Cap at 2048 to keep tick latency bounded.
	w.drainActionQueue(2048)

	if w.onTick != nil {
		w.onTick(w, w.tick)
	}

	for _, e := range w.entities {
		// World-object archetypes (trees, rocks, items, blueprints,
		// buildings, decorations) DO NOT have a brain. The autonomous
		// wander / demo-action / auto-enter behavior below would
		// otherwise move them around the map every few seconds.
		// (This was the cause of trees walking 25 tiles from their
		// spawn point in 2 minutes.) Skip the per-tick behavior for
		// them — composable-system OnTick callbacks already ran above
		// and handle world-object state.
		if !isAgentArchetype(e.Archetype) {
			continue
		}
		// Skip bot-controlled entities — they receive commands from
		// external WS agents and must NOT be auto-moved by the engine.
		// Without this, an LLM's "Move east" gets overridden by the
		// wander loop on the next tick and the bot can't reach goals.
		if e.PlayerControlled {
			// Still advance ongoing walks so move actions can complete.
			if e.CurrentAction == "move" && e.WalkProgress < 1 {
				e.WalkProgress += 1.0 / float64(TicksPerStep)
				if e.WalkProgress >= 1 {
					e.WalkProgress = 1
					if len(e.walkPath) > 0 {
						next := e.walkPath[0]
						e.walkPath = e.walkPath[1:]
						w.startMove(e, next)
					} else {
						e.CurrentAction = ""
					}
				}
				e.recomputeRenderPos()
			}
			continue
		}
		// --- Inside a building: tick down, then exit. ---
		if e.InsideBuilding != "" {
			if e.insideTicks > 0 {
				e.insideTicks--
				continue
			}
			// Time to exit. Re-emerge at the door tile (the entity's
			// LogicalTile was stored at entry).
			e.InsideBuilding = ""
			e.Facing = FacingS
			if w.occupants[e.LogicalTile] == "" {
				w.occupants[e.LogicalTile] = e.EntityID
			}
			continue
		}

		// Decrement action timer; clear action when done.
		if e.actionTicks > 0 {
			e.actionTicks--
			if e.actionTicks == 0 {
				e.CurrentAction = ""
			}
			continue
		}

		// --- Auto-enter when LogicalTile is a door (Pokemon HG style).
		// Walking onto a registered door tile from any direction warps
		// the entity inside the building. We rate-limit by requiring
		// the entity be facing INTO the building (i.e. facing N — the
		// door tile is directly south of the footprint), so agents
		// just passing through don't accidentally enter when they didn't
		// intend to. Also fires only when the entity finished its walk
		// (WalkProgress >= 1) so we don't enter mid-step.
		if e.WalkProgress >= 1 {
			if ref, ok := w.buildingDoors[e.LogicalTile]; ok && e.Facing == FacingN {
				e.InsideBuilding = ref.Sprite
				e.insideTicks = 240 + w.rng.IntN(360) // 4-10 seconds
				if w.occupants[e.LogicalTile] == e.EntityID {
					delete(w.occupants, e.LogicalTile)
				}
				e.walkPath = nil
				e.CurrentAction = ""
				continue
			}
		}

		// Idle: pick a wander target.
		if e.CurrentAction == "" && len(e.walkPath) == 0 && e.WalkProgress >= 1 {
			// Once every ~5 s, demo a random action.
			if w.rng.IntN(300) == 0 {
				actions := []string{"attack", "interact", "hit"}
				e.CurrentAction = actions[w.rng.IntN(len(actions))]
				e.actionTicks = 36
				e.Facing = FacingS
				continue
			}
			// Otherwise wander.
			if w.rng.IntN(120) == 0 {
				if t, ok := w.pickWanderTarget(e, 6); ok {
					w.startMove(e, t)
				}
			}
		}

		// Advance walk.
		if e.CurrentAction == "move" && e.WalkProgress < 1 {
			e.WalkProgress += 1.0 / float64(TicksPerStep)
			if e.WalkProgress >= 1 {
				e.WalkProgress = 1
				// Release the from-tile.
				if w.occupants[e.WalkFromTile] == e.EntityID {
					delete(w.occupants, e.WalkFromTile)
				}
				e.WalkFromTile = e.LogicalTile
				// Take the next step if path has more tiles.
				if len(e.walkPath) > 0 {
					next := e.walkPath[0]
					if !w.CanEnter(e, next) {
						// Blocked mid-path. Stop and notify (event
						// emission lands with the agent wire path).
						e.walkPath = nil
						e.CurrentAction = ""
					} else {
						w.occupants[next] = e.EntityID
						e.WalkFromTile = e.LogicalTile
						e.LogicalTile = next
						e.WalkProgress = 0
						e.walkPath = e.walkPath[1:]
						e.Facing = stepFacing(e.WalkFromTile, next)
					}
				} else {
					e.CurrentAction = ""
				}
			}
		}

		e.recomputeRenderPos()
	}
}

func (w *World) Snapshot() WorldSnapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	ents := make([]*Entity, 0, len(w.entities))
	for _, e := range w.entities {
		cp := *e
		ents = append(ents, &cp)
	}
	return WorldSnapshot{
		Tick:        w.tick,
		MapID:       w.MapID,
		WidthTiles:  w.WidthTiles,
		HeightTiles: w.HeightTiles,
		Entities:    ents,
	}
}

type WorldSnapshot struct {
	Tick        uint64    `json:"tick"`
	MapID       string    `json:"map_id"`
	WidthTiles  int       `json:"width_tiles,omitempty"`
	HeightTiles int       `json:"height_tiles,omitempty"`
	Entities    []*Entity `json:"entities"`
}

// ViewImage — per-agent rasterized crop. Filled in by the Go-side
// rasterizer when the agent's vision_mode includes images.
type ViewImage struct {
	Format        string `json:"format"`
	Width         uint16 `json:"width"`
	Height        uint16 `json:"height"`
	Data          []byte `json:"data"`
	CenteredOnPos Tile   `json:"centered_on_pos"`
	Facing        Facing `json:"facing"`
}
