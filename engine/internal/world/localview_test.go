package world

import "testing"

// glyphAt returns the local-view glyph for a world tile, or 0 if the tile
// is outside the rendered window.
func glyphAt(lv *LocalView, worldX, worldY int) byte {
	rx := worldX - lv.Origin[0]
	ry := worldY - lv.Origin[1]
	if ry < 0 || ry >= len(lv.Rows) {
		return 0
	}
	row := lv.Rows[ry]
	if rx < 0 || rx >= len(row) {
		return 0
	}
	return row[rx]
}

func TestLocalView_TerrainAndOverlays(t *testing.T) {
	w := loadTestWorld(t) // 10x6 grass, wall row y=2 cols 2..5, a@(1,1) b@(8,1)
	a := w.entities["a"]

	obs := w.BuildObservation(a, 1, nil)
	lv := obs.LocalView
	if lv == nil {
		t.Fatal("observation has no local_view")
	}

	// Geometry: radius 20 → 41x41 window, origin = self - radius.
	if lv.Radius != LocalViewRadius {
		t.Fatalf("radius=%d want %d", lv.Radius, LocalViewRadius)
	}
	side := 2*LocalViewRadius + 1
	if len(lv.Rows) != side {
		t.Fatalf("rows=%d want %d", len(lv.Rows), side)
	}
	wantOrigin := Tile{a.LogicalTile[0] - LocalViewRadius, a.LogicalTile[1] - LocalViewRadius}
	if lv.Origin != wantOrigin {
		t.Fatalf("origin=%v want %v", lv.Origin, wantOrigin)
	}

	// Self is '@' at the center of the window.
	if g := glyphAt(lv, a.LogicalTile[0], a.LogicalTile[1]); g != '@' {
		t.Fatalf("self glyph=%q want '@'", g)
	}

	// Walkable grass at (0,0) → '.'.
	if g := glyphAt(lv, 0, 0); g != '.' {
		t.Fatalf("grass(0,0) glyph=%q want '.'", g)
	}

	// Wall at (2,2) → '#'.
	if g := glyphAt(lv, 2, 2); g != '#' {
		t.Fatalf("wall(2,2) glyph=%q want '#'", g)
	}

	// Off-map tile (-1,-1) is inside the window but renders as ' '.
	if g := glyphAt(lv, -1, -1); g != ' ' {
		t.Fatalf("off-map(-1,-1) glyph=%q want ' '", g)
	}

	// Entity b at (8,1) is within vision+LOS of a, so it overlays as 'P'.
	if g := glyphAt(lv, 8, 1); g != 'P' {
		t.Fatalf("entity b(8,1) glyph=%q want 'P'", g)
	}

	// Legend is shipped and self-describing.
	if lv.Legend["@"] == "" || lv.Legend["#"] == "" {
		t.Fatalf("legend missing entries: %v", lv.Legend)
	}
}
