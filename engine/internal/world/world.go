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
	// D8 — sprite + quantity exposed to the frontend renderer so item
	// entities are drawn with their actual sprite (not a hardcoded
	// wood_log fallback). Quantity is the stack size for coin piles
	// etc. Source is metadata for debugging (drop / world_seed / etc).
	"sprite":   true,
	"quantity": true,
	"source":   true,
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

	// Per-tile source glyph as it appears in world.json's `tiles` field.
	// Kept separately from tileKindGrid because the editor needs to
	// write back the exact glyph the world uses (e.g. ".", "#", "~"),
	// and TilesLegend maps glyph→kind one-way. Mutable: the editor
	// updates tileChars[y][x] then SetTile() recomputes walkable +
	// vision + kind so the world responds immediately. Same shape as
	// walkable.
	tileChars [][]byte

	// Per-glyph kind lookup so SetTile can recompute kind from a new
	// glyph without re-parsing world.json.
	tilesLegend map[string]string

	// Source path the world was loaded from. PersistTileEdits writes a
	// sidecar overlay file next to it so a restart sees the edits.
	sourcePath string

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
	// onSpawn fires whenever a NEW entity is added to the world at
	// runtime (SpawnEntity, SpawnEntityFromSpec, SpawnAgentEntity).
	// Without this, system seedSpawn hooks (combat.hp, money.gold,
	// vitals.hunger, …) only run on the entities present at world
	// load — runtime-spawned agents stayed at hp=0 forever, so the
	// killer's attacks against a freshly-registered bot couldn't
	// trigger EntityDied. Set by InstallScenario.
	onSpawn func(*Entity)

	// onActionAccepted fires after a verb returns Accepted=true.
	// Used by SystemHost to emit an ActionAccepted historian event so
	// native verbs (move, speak, …) that don't otherwise queue to the
	// bus still show up in the run log + the scorer.
	//
	// CRITICAL: the hook fires inside Tick under the world write lock.
	// It MUST NOT re-acquire any world lock (e.g. CurrentTick takes a
	// read lock; sync.RWMutex doesn't allow write→read re-entry, that
	// deadlocks the engine the first time any action lands). We pass
	// the tick value through so the hook can build a stamped event
	// without needing to read it back via a getter.
	onActionAccepted func(entityID, verb string, tick uint64, raw []byte)

	// onBuildingEntered / onBuildingExited fire when an entity uses the
	// native interact(affordance=enter|exit) path to step into or out
	// of a decoration-backed building. The property system already
	// emits these for entity-backed buildings; these hooks cover the
	// "bld:" decoration family (the kind Eldoria uses) so both paths
	// land structured events in the historian.
	onBuildingEntered func(entityID, building string, tick uint64)
	onBuildingExited  func(entityID, building string, tick uint64)

	// D19 — per-pair social interaction ledger. Lives on the world so
	// every verb handler (and the inspector endpoint) can reach it
	// without plumbing a separate service. Has its own lock; safe to
	// call under the world lock.
	social *socialLedger
}

// LockWrite / UnlockWrite expose the world's write lock to callers
// outside the package (the wire layer, primarily). Used by the tile
// editor handler so it can SetTile under the proper lock without
// having to route every mutation through an adapter. Tick takes the
// same lock — a paint operation is serialized with ticks.
func (w *World) LockWrite()   { w.mu.Lock() }
func (w *World) UnlockWrite() { w.mu.Unlock() }

// SocialPeersOf — D19. Per-pair interaction counts keyed by peer id.
// Empty map if no interactions yet. Safe for concurrent callers.
func (w *World) SocialPeersOf(entityID string) map[string]SocialCounts {
	if w.social == nil {
		return map[string]SocialCounts{}
	}
	return w.social.PeersOf(entityID)
}

// SocialCountsFor — D19. Per-pair counts between (a, b). Zero value
// if no interactions yet.
func (w *World) SocialCountsFor(a, b string) SocialCounts {
	if w.social == nil {
		return SocialCounts{}
	}
	return w.social.CountsFor(a, b)
}

// SocialEdges — D19/Society-Pulse. Every interaction pair once, for the
// frontend relationship overlay.
func (w *World) SocialEdges() []SocialEdge {
	if w.social == nil {
		return []SocialEdge{}
	}
	return w.social.AllEdges()
}

// SetOnActionAccepted installs the historian hook for accepted actions.
// Call once after scenario install, before Tick() begins.
func (w *World) SetOnActionAccepted(h func(entityID, verb string, tick uint64, raw []byte)) {
	w.mu.Lock()
	w.onActionAccepted = h
	w.mu.Unlock()
}

// SetOnBuildingEntered installs the historian hook for decoration-
// backed building entry. Called from SystemHost so the bus sees
// EnteredBuilding regardless of whether the building was an entity
// (property system path) or a decoration (this path).
func (w *World) SetOnBuildingEntered(h func(entityID, building string, tick uint64)) {
	w.mu.Lock()
	w.onBuildingEntered = h
	w.mu.Unlock()
}

func (w *World) SetOnBuildingExited(h func(entityID, building string, tick uint64)) {
	w.mu.Lock()
	w.onBuildingExited = h
	w.mu.Unlock()
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
	w.onSpawn = onSpawn
	if onSpawn != nil {
		for _, e := range w.entities {
			onSpawn(e)
		}
	}
	w.mu.Unlock()
}

// fireSpawnHook fires the scenario's onSpawn for a runtime-spawned
// entity. Caller must hold the world write lock — seedSpawn callbacks
// mutate Extras directly, no extra locking required.
func (w *World) fireSpawnHook(e *Entity) {
	if w.onSpawn != nil {
		w.onSpawn(e)
	}
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
	EntityID    string         `json:"entity_id"`
	Archetype   string         `json:"archetype"`
	Pos         [2]int         `json:"pos"`
	Facing      Facing         `json:"facing"`
	DisplayName string         `json:"display_name"`
	// Extras lets world.json declare item entities with a sprite +
	// quantity (e.g., scattered wealth from D7). Optional. Any keys
	// land directly into entity.Extras.
	Extras      map[string]any `json:"extras,omitempty"`
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
		social:      newSocialLedger(),
	}
	w.visionBlocks = make([][]bool, fw.HeightTiles)
	w.tileKindGrid = make([][]string, fw.HeightTiles)
	w.tileChars = make([][]byte, fw.HeightTiles)
	w.tilesLegend = fw.TilesLegend
	w.sourcePath = path
	for y := 0; y < fw.HeightTiles; y++ {
		w.visionBlocks[y] = make([]bool, fw.WidthTiles)
		w.tileKindGrid[y] = make([]string, fw.WidthTiles)
		w.tileChars[y] = make([]byte, fw.WidthTiles)
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
			var ch byte = ' '
			if x < len(row) {
				ch = row[x]
				kind = fw.TilesLegend[string(ch)]
			}
			w.tileChars[y][x] = ch
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
	// Replay any user tile edits from the sidecar overlay BEFORE
	// applying decorations. Editor paints land on the base grid;
	// decorations (trees / buildings) are positioned by the world
	// generator and shouldn't be overwritten by tile painting.
	// Errors here are non-fatal: a corrupt overlay just gets ignored
	// so the world still boots.
	if err := w.ApplyTileEditsOverlay(); err != nil {
		// Log but don't fail. Caller can see the error via the
		// engine's stderr; the world is still usable.
		fmt.Fprintf(os.Stderr, "world: tile edits overlay: %v\n", err)
	}

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
			Extras:       fe.Extras,
		}
		e.recomputeRenderPos()
		w.entities[fe.EntityID] = e
		// Item entities don't claim tile occupancy (D7 + D8 — many items
		// can share a tile; agents walk over them and pick them up).
		// Agent-archetype entities still claim their tile.
		if fe.Archetype != "item" {
			w.occupants[tile] = fe.EntityID
		}
	}

	// Replay any editor-placed decorations on top of the base bundle
	// AFTER entity loading so additive overlays survive engine restart.
	// Non-fatal: a corrupt overlay falls through to a clean boot.
	if err := w.ApplyDecorationEditsOverlay(); err != nil {
		fmt.Fprintf(os.Stderr, "world: decoration edits overlay: %v\n", err)
	}

	// Post-load displacement pass: any entity whose declared tile got
	// stomped by a decoration footprint (procedural worldgen places
	// entities + decorations independently) ends up stuck — every
	// move action rejects because the source tile is non-walkable
	// and the dispatcher's tile-occupancy lookup goes haywire. The
	// user reported finding npc_aspendell_3's "child of Birchwood"
	// frozen ON TOP of a cottage; that's this case. We relocate
	// each such entity to the nearest walkable tile within a small
	// radius and warn so the worldgen can be tightened later.
	w.relocateStuckEntities()

	return w, nil
}

// relocateStuckEntities scans every loaded entity, finds those on
// non-walkable tiles (because a decoration footprint stomped their
// spawn after the entity was placed), and moves them to the closest
// walkable tile within RELOCATE_RADIUS. Caller must hold no locks —
// runs inside Load before any concurrent access exists.
func (w *World) relocateStuckEntities() {
	const radius = 8
	moved := 0
	stuck := 0
	for id, e := range w.entities {
		if w.IsWalkable(e.LogicalTile) {
			continue
		}
		// BFS outward until we find a walkable tile or exhaust the
		// radius. Diamond distance: prefer cardinal neighbours, then
		// step out. We don't need a real BFS — a ring scan is fine.
		newTile, ok := w.nearestWalkable(e.LogicalTile, radius)
		if !ok {
			stuck++
			fmt.Fprintf(os.Stderr,
				"world: entity %s stuck at %v — no walkable tile within %d (worldgen bug)\n",
				id, e.LogicalTile, radius)
			continue
		}
		// Update occupancy maps to reflect the new tile.
		if w.occupants[e.LogicalTile] == id {
			delete(w.occupants, e.LogicalTile)
		}
		e.LogicalTile = newTile
		e.WalkFromTile = newTile
		e.recomputeRenderPos()
		w.occupants[newTile] = id
		moved++
	}
	if moved > 0 || stuck > 0 {
		fmt.Fprintf(os.Stderr,
			"world: relocated %d stuck entities (%d unsalvageable)\n",
			moved, stuck)
	}
}

func (w *World) nearestWalkable(from Tile, radius int) (Tile, bool) {
	// Try the 8-neighbours first (often enough), then expand.
	for r := 1; r <= radius; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				// Only ring cells.
				if absInt(dx) != r && absInt(dy) != r {
					continue
				}
				t := Tile{from[0] + dx, from[1] + dy}
				if !w.IsWalkable(t) {
					continue
				}
				if _, occupied := w.occupants[t]; occupied {
					continue
				}
				return t, true
			}
		}
	}
	return Tile{}, false
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
	// BLK-3: findPath now returns a best-effort path toward an unreachable
	// (walkable) target — gets the agent as close as it can rather than
	// returning nil, so agents stop wedging retrying the same blocked move.
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
	// BUG FIX (BLK-3): track the explored tile CLOSEST to the goal so
	// that when the exact goal is unreachable (blocked by a wall, sitting
	// on a non-walkable decoration tile, or occupied), we still return a
	// best-effort path that makes progress toward it — instead of nil,
	// which left agents wedged retrying the same blocked move ("no_path"
	// was the #1 logged failure). Distance is manhattan-to-goal.
	bestIdx := 0
	bestDist := absInt(goal[0]-start[0]) + absInt(goal[1]-start[1])
	reconstruct := func(idx int) []Tile {
		// B14: append then reverse (O(n)) instead of prepend-in-loop (O(n²)).
		path := []Tile{}
		for i := idx; i != -1; i = visited[i].parent {
			path = append(path, visited[i].t)
		}
		for l, r := 0, len(path)-1; l < r; l, r = l+1, r-1 {
			path[l], path[r] = path[r], path[l]
		}
		return path
	}
	// Visit cap as a second safety net — diagonals + obstacles can blow
	// past manhattan in pathological maps. 4096 tiles ≈ 64² area covered.
	const maxNodes = 4096
	for len(queue) > 0 && len(visited) < maxNodes {
		idx := queue[0]
		queue = queue[1:]
		cur := visited[idx].t
		if cur == goal {
			return reconstruct(idx)
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
			if d := absInt(n[0]-goal[0]) + absInt(n[1]-goal[1]); d < bestDist {
				bestDist = d
				bestIdx = len(visited) - 1
			}
		}
	}
	// Goal unreachable: return the best-effort path toward it (empty if
	// we couldn't get any closer than where we started — caller treats
	// len<2 as no_path).
	if bestIdx != 0 {
		return reconstruct(bestIdx)
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

	atomic.AddUint64(&w.tick, 1)

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

		// D3 — legacy demo-wander + random-action loop REMOVED.
		// Non-PlayerControlled entities used to wander randomly and
		// occasionally fire fake attack/interact/hit actions every ~5
		// sec, which contaminated emergence studies. After D3 the only
		// motion of non-PlayerControlled entities comes from
		// rule-based bot archetypes (D16) connecting via the SDK like
		// any other agent. If a non-PlayerControlled entity exists at
		// all, it idles in place.

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
		// Deep-copy Extras. Without this, `cp.Extras` is still the
		// live map pointer; the viewer hub serializes this snapshot
		// AFTER we drop the read lock here, so Tick can be writing
		// into the live entity's Extras while encoding/json iterates
		// it (Entity.MarshalJSON in world.go ranges over e.Extras).
		// That's the same class of bug we fixed in publishSnapshot()
		// for the per-agent observation path — but every snapshot
		// consumer (viewer, /world/info, persist) needed the same
		// guard. Engine crashed mid-render in the live UI without it.
		cp.Extras = copyExtras(e.Extras)
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
