package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetTile_PaintGrassToWall(t *testing.T) {
	w := loadTestWorld(t)
	// In testWorldJSON the row at y=1 reads "..........". (0,1) is grass.
	w.mu.Lock()
	if w.tileKindGrid[1][0] != "grass" {
		t.Fatalf("setup: expected grass at (0,1), got %q", w.tileKindGrid[1][0])
	}
	if !w.walkable[1][0] {
		t.Fatalf("setup: expected walkable at (0,1)")
	}
	kind, err := w.SetTile(0, 1, "#")
	w.mu.Unlock()
	if err != nil {
		t.Fatalf("SetTile: %v", err)
	}
	if kind != "wall" {
		t.Fatalf("expected kind=wall, got %q", kind)
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.tileKindGrid[1][0] != "wall" {
		t.Fatalf("tileKindGrid not updated: %q", w.tileKindGrid[1][0])
	}
	if w.walkable[1][0] {
		t.Fatalf("walkable not cleared")
	}
	if !w.visionBlocks[1][0] {
		t.Fatalf("visionBlocks not set for wall")
	}
	if w.tileChars[1][0] != '#' {
		t.Fatalf("tileChars not updated: got %q", string(w.tileChars[1][0]))
	}
}

func TestSetTile_OutOfBoundsRejected(t *testing.T) {
	w := loadTestWorld(t)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.SetTile(-1, 0, "."); err == nil {
		t.Fatal("SetTile(-1,0) should reject")
	}
	if _, err := w.SetTile(0, w.HeightTiles, "."); err == nil {
		t.Fatal("SetTile(y=height) should reject")
	}
}

func TestSetTile_UnknownGlyphRejected(t *testing.T) {
	w := loadTestWorld(t)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, err := w.SetTile(0, 0, "?"); err == nil {
		t.Fatal("unknown glyph should reject")
	}
}

func TestTileEditsOverlay_RoundTrip(t *testing.T) {
	// Write the test world to a temp file (so we have a sourcePath
	// to write a sibling overlay against), Load(), edit + persist,
	// re-load, confirm the edit replays.
	dir := t.TempDir()
	p := filepath.Join(dir, "test_world.json")
	if err := os.WriteFile(p, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	w, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	w.mu.Lock()
	kind, err := w.SetTile(3, 3, "#")
	w.mu.Unlock()
	if err != nil {
		t.Fatalf("SetTile: %v", err)
	}
	if kind != "wall" {
		t.Fatalf("expected wall, got %q", kind)
	}
	if err := w.AppendTileEditOverlay(TileEdit{X: 3, Y: 3, Glyph: "#"}); err != nil {
		t.Fatalf("append overlay: %v", err)
	}
	// Verify the overlay file was written next to world.json.
	overlay := filepath.Join(dir, "tile_edits.json")
	data, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatalf("overlay missing: %v", err)
	}
	var edits []TileEdit
	if err := json.Unmarshal(data, &edits); err != nil {
		t.Fatalf("parse overlay: %v", err)
	}
	if len(edits) != 1 || edits[0].X != 3 || edits[0].Y != 3 || edits[0].Glyph != "#" {
		t.Fatalf("overlay content wrong: %+v", edits)
	}
	// Now re-load the world from the same path. The overlay should
	// auto-apply via ApplyTileEditsOverlay() called from Load.
	w2, err := Load(p)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if w2.tileKindGrid[3][3] != "wall" {
		t.Fatalf("overlay didn't replay: %q", w2.tileKindGrid[3][3])
	}
}

func TestTileEditsOverlay_CorruptIsNonFatal(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test_world.json")
	if err := os.WriteFile(p, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Write a garbage overlay sidecar.
	if err := os.WriteFile(filepath.Join(dir, "tile_edits.json"),
		[]byte("not json"), 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	_, err := Load(p)
	if err != nil {
		t.Fatalf("Load should ignore corrupt overlay, got: %v", err)
	}
}

func TestTilesLegendMap_IsCopy(t *testing.T) {
	w := loadTestWorld(t)
	m := w.TilesLegendMap()
	if _, ok := m["."]; !ok {
		t.Fatal("legend should include grass glyph '.'")
	}
	// Mutating the returned map must not affect the world.
	m["X"] = "evil"
	if _, ok := w.tilesLegend["X"]; ok {
		t.Fatal("TilesLegendMap returned a shared map — should be a copy")
	}
	_ = strings.Builder{}
}
