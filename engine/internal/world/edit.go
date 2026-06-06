package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// World tile editor — the engine side of the live editor panel in the
// frontend. The Editor.tsx surface was scaffolded long ago (Phase
// WORLD-3); the live paint + persist roundtrip (Phase WORLD-4) was
// deferred and never landed. This file is that piece.
//
// Wire contract:
//   POST /api/v1/world/edit
//   body: {"x":int, "y":int, "glyph":"."}
//   returns: {"ok":bool, "kind":"grass", "reason":""}
//
// On accept, the world's in-memory tile grid is updated (so observation
// + viewer snapshot pick it up next tick) and a sidecar overlay file
// `<bundle>/tile_edits.json` records the change for restart-time
// replay. We intentionally do NOT rewrite the big world.json on every
// paint — that file can be megabytes and the user paints fast.

// TileEdit is the on-disk + on-wire shape for one tile-paint action.
type TileEdit struct {
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Glyph string `json:"glyph"`
}

// SetTile updates the tile at (x,y) to use `glyph`. The glyph must
// exist in the world's TilesLegend. Recomputes walkable + vision +
// kind so the change takes effect immediately. Returns the resolved
// tile kind on success.
//
// Caller must hold the world write lock.
func (w *World) SetTile(x, y int, glyph string) (string, error) {
	if x < 0 || y < 0 || x >= w.WidthTiles || y >= w.HeightTiles {
		return "", fmt.Errorf("out_of_bounds:(%d,%d)", x, y)
	}
	if len(glyph) != 1 {
		return "", errors.New("glyph_must_be_one_char")
	}
	kind, ok := w.tilesLegend[glyph]
	if !ok {
		return "", fmt.Errorf("unknown_glyph:%q", glyph)
	}
	w.tileChars[y][x] = glyph[0]
	w.tileKindGrid[y][x] = kind
	w.walkable[y][x] = walkableKinds[kind]
	// Vision: walls block, everything else clears. Buildings/trees
	// are decoration-driven not tile-driven, so we only touch the
	// wall bit here. If the user paints a tree they'd use the
	// decoration path (Phase WORLD-5+).
	w.visionBlocks[y][x] = (kind == "wall")
	return kind, nil
}

// ApplyTileEditsOverlay applies any persisted tile_edits.json sidecar
// after the base world.json has loaded. Called automatically by Load
// when the sidecar exists. Idempotent: re-applies fine on every boot.
func (w *World) ApplyTileEditsOverlay() error {
	path := overlayPath(w.sourcePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var edits []TileEdit
	if err := json.Unmarshal(data, &edits); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, e := range edits {
		if _, err := w.SetTile(e.X, e.Y, e.Glyph); err != nil {
			// Skip bad edits but keep going. A stale overlay shouldn't
			// brick world boot.
			continue
		}
	}
	return nil
}

// AppendTileEditOverlay persists one edit to the sidecar. Cheap: the
// file is small (only edits, not the whole tile grid). We rewrite on
// every call instead of append-only so a corrupt half-write doesn't
// break boot — but the file stays tiny (a few hundred edits at most
// in practical use) so the cost is negligible.
func (w *World) AppendTileEditOverlay(e TileEdit) error {
	if w.sourcePath == "" {
		// In-memory only worlds (tests) — skip persistence.
		return nil
	}
	path := overlayPath(w.sourcePath)
	var edits []TileEdit
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &edits)
	}
	edits = append(edits, e)
	data, err := json.MarshalIndent(edits, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// overlayPath — sibling of the world.json, named "tile_edits.json".
// Keeps overlays per-bundle without touching the big base file.
func overlayPath(sourcePath string) string {
	if sourcePath == "" {
		return ""
	}
	dir := sourcePath[:lastSlash(sourcePath)+1]
	return dir + "tile_edits.json"
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

// TilesLegendMap exposes the world's glyph→kind legend so the wire
// layer can validate edits before locking the world.
func (w *World) TilesLegendMap() map[string]string {
	out := make(map[string]string, len(w.tilesLegend))
	for k, v := range w.tilesLegend {
		out[k] = v
	}
	return out
}

// DecorationEdit is the on-disk + on-wire shape for an editor
// placement OR removal. Persisted to `<bundle>/decoration_edits.json`
// alongside the tile_edits sidecar so editor changes survive engine
// restart.
//
// Op = "add" (default) inserts the decoration; Op = "remove" deletes
// the topmost decoration whose footprint contains (X, Y). Removal
// records use only Op + X + Y; the other fields are ignored.
type DecorationEdit struct {
	Op          string  `json:"op,omitempty"` // "" or "add" | "remove"
	X           int     `json:"x"`
	Y           int     `json:"y"`
	Sprite      string  `json:"sprite,omitempty"`
	HeightTiles float64 `json:"height_tiles,omitempty"`
	FootprintW  int     `json:"footprint_w,omitempty"`
	FootprintH  int     `json:"footprint_h,omitempty"`
	Walkable    bool    `json:"walkable,omitempty"`
}

// AddDecoration places a new decoration at (X, Y) with the given
// sprite + footprint. Updates walkable/vision/buildingDoors to match
// so the engine treats the new building / item just like a baked-in
// one. Rejects placements whose blocking footprint overlaps an
// existing non-walkable decoration's footprint (i.e. you can't drop
// a cottage on top of another cottage). Walkable items can stack
// anywhere — they don't claim tiles.
// Caller must hold the world write lock.
func (w *World) AddDecoration(e DecorationEdit) error {
	if e.X < 0 || e.Y < 0 || e.X >= w.WidthTiles || e.Y >= w.HeightTiles {
		return fmt.Errorf("out_of_bounds:(%d,%d)", e.X, e.Y)
	}
	if e.Sprite == "" {
		return errors.New("sprite_required")
	}
	fpW := e.FootprintW
	if fpW < 1 {
		fpW = 1
	}
	fpH := e.FootprintH
	if fpH < 1 {
		fpH = 1
	}
	// Overlap check for blocking placements. Build the set of tiles
	// this footprint claims, then look for any existing non-walkable
	// decoration whose footprint intersects.
	if !e.Walkable {
		for dy := 0; dy < fpH; dy++ {
			for dx := 0; dx < fpW; dx++ {
				nx := e.X + dx
				ny := e.Y - dy
				if nx < 0 || nx >= w.WidthTiles || ny < 0 || ny >= w.HeightTiles {
					continue
				}
				for _, d := range w.decorations {
					if d.Walkable {
						continue
					}
					dfpW := d.FootprintW
					if dfpW < 1 {
						dfpW = 1
					}
					dfpH := d.FootprintH
					if dfpH < 1 {
						dfpH = 1
					}
					if nx >= d.X && nx < d.X+dfpW && ny <= d.Y && ny > d.Y-dfpH {
						return fmt.Errorf("collision:(%d,%d) blocked by %s @ (%d,%d)",
							nx, ny, d.Sprite, d.X, d.Y)
					}
				}
			}
		}
	}
	w.decorations = append(w.decorations, DecorationRef{
		X: e.X, Y: e.Y, Sprite: e.Sprite,
		HeightTiles: e.HeightTiles,
		FootprintW:  fpW,
		FootprintH:  fpH,
		Walkable:    e.Walkable,
	})
	if e.Walkable {
		// Walkable decorations (items, ground props) don't claim tiles.
		return nil
	}
	// Block footprint slab.
	for dy := 0; dy < fpH; dy++ {
		for dx := 0; dx < fpW; dx++ {
			ny := e.Y - dy
			nx := e.X + dx
			if nx < 0 || nx >= w.WidthTiles || ny < 0 || ny >= w.HeightTiles {
				continue
			}
			w.walkable[ny][nx] = false
			if e.HeightTiles >= 1.5 {
				w.visionBlocks[ny][nx] = true
			}
		}
	}
	// Tall decorations: block the rows above the footprint that the
	// upper sprite paints into (cottage roof, watchtower spire).
	if e.HeightTiles >= 1.5 {
		extra := int(e.HeightTiles) - fpH
		if e.HeightTiles-float64(int(e.HeightTiles)) > 1e-9 {
			extra++
		}
		if extra < 1 {
			extra = 1
		}
		for k := 1; k <= extra; k++ {
			ny := e.Y - fpH - (k - 1)
			if ny < 0 {
				continue
			}
			for dx := 0; dx < fpW; dx++ {
				nx := e.X + dx
				if nx >= 0 && nx < w.WidthTiles {
					w.walkable[ny][nx] = false
				}
			}
		}
	}
	// Building → register a door tile south of the footprint centre.
	if fpW >= 2 && strings.HasPrefix(e.Sprite, "bld:") {
		doorX := e.X + fpW/2
		doorY := e.Y + 1
		if doorY < w.HeightTiles {
			w.buildingDoors[Tile{doorX, doorY}] = buildingRef{
				Sprite: e.Sprite, X: e.X, Y: e.Y,
			}
			w.walkable[doorY][doorX] = true
		}
	}
	return nil
}

// RemoveDecorationAt deletes the decoration covering (x, y) that the
// user most likely meant to click — the LARGEST footprint that
// contains the tile, breaking ties by latest-added. That maps cleanly
// to "click on a visible cottage, get the cottage" even when a tiny
// walkable item is dropped on top of it. Restores walkability +
// vision for the tiles that decoration claimed. Returns the removed
// decoration ref. Caller must hold the world write lock.
func (w *World) RemoveDecorationAt(x, y int) (DecorationRef, error) {
	if x < 0 || y < 0 || x >= w.WidthTiles || y >= w.HeightTiles {
		return DecorationRef{}, fmt.Errorf("out_of_bounds:(%d,%d)", x, y)
	}
	// Find the largest-footprint match; tie-break with latest-added.
	bestIdx := -1
	bestArea := -1
	for i := len(w.decorations) - 1; i >= 0; i-- {
		d := w.decorations[i]
		fpW := d.FootprintW
		if fpW < 1 {
			fpW = 1
		}
		fpH := d.FootprintH
		if fpH < 1 {
			fpH = 1
		}
		if !(x >= d.X && x < d.X+fpW && y <= d.Y && y > d.Y-fpH) {
			continue
		}
		area := fpW * fpH
		if area > bestArea {
			bestArea = area
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return DecorationRef{}, fmt.Errorf("no_decoration_at:(%d,%d)", x, y)
	}
	d := w.decorations[bestIdx]
	fpW := d.FootprintW
	if fpW < 1 {
		fpW = 1
	}
	fpH := d.FootprintH
	if fpH < 1 {
		fpH = 1
	}
	w.decorations = append(w.decorations[:bestIdx], w.decorations[bestIdx+1:]...)
	// Restore walkability / vision for this slab. Reset to the
	// underlying tile's base walkability — if another decoration
	// covers the same tile, a subsequent add will re-block it.
	if !d.Walkable {
		for dy := 0; dy < fpH; dy++ {
			for dx := 0; dx < fpW; dx++ {
				nx := d.X + dx
				ny := d.Y - dy
				if nx < 0 || nx >= w.WidthTiles || ny < 0 || ny >= w.HeightTiles {
					continue
				}
				kind := w.tileKindGrid[ny][nx]
				w.walkable[ny][nx] = walkableKinds[kind]
				w.visionBlocks[ny][nx] = (kind == "wall")
			}
		}
		// Also clear extra rows the visual envelope covered.
		if d.HeightTiles >= 1.5 {
			extra := int(d.HeightTiles) - fpH
			if d.HeightTiles-float64(int(d.HeightTiles)) > 1e-9 {
				extra++
			}
			if extra < 1 {
				extra = 1
			}
			for k := 1; k <= extra; k++ {
				ny := d.Y - fpH - (k - 1)
				if ny < 0 {
					continue
				}
				for dx := 0; dx < fpW; dx++ {
					nx := d.X + dx
					if nx >= 0 && nx < w.WidthTiles {
						kind := w.tileKindGrid[ny][nx]
						w.walkable[ny][nx] = walkableKinds[kind]
					}
				}
			}
		}
	}
	// If this was a building, drop its door registration.
	if fpW >= 2 && strings.HasPrefix(d.Sprite, "bld:") {
		doorX := d.X + fpW/2
		doorY := d.Y + 1
		delete(w.buildingDoors, Tile{doorX, doorY})
	}
	return d, nil
}

// ApplyDecorationEditsOverlay replays any persisted decoration_edits.json
// on top of the base bundle's decorations. Called after Load so editor
// placements survive engine restarts. Op="" or "add" inserts;
// Op="remove" deletes. Idempotent.
func (w *World) ApplyDecorationEditsOverlay() error {
	path := decoOverlayPath(w.sourcePath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var edits []DecorationEdit
	if err := json.Unmarshal(data, &edits); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, e := range edits {
		switch e.Op {
		case "remove":
			_, _ = w.RemoveDecorationAt(e.X, e.Y)
		default:
			_ = w.AddDecoration(e)
		}
	}
	return nil
}

// AppendDecorationEditOverlay persists one decoration add to the
// sidecar so it survives engine restart.
func (w *World) AppendDecorationEditOverlay(e DecorationEdit) error {
	if w.sourcePath == "" {
		return nil
	}
	path := decoOverlayPath(w.sourcePath)
	var edits []DecorationEdit
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &edits)
	}
	edits = append(edits, e)
	data, err := json.MarshalIndent(edits, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func decoOverlayPath(sourcePath string) string {
	if sourcePath == "" {
		return ""
	}
	dir := sourcePath[:lastSlash(sourcePath)+1]
	return dir + "decoration_edits.json"
}
