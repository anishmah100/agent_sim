package world

import (
	"os"
	"path/filepath"
	"testing"
)

const interiorJSON = `{
  "map_id": "cottage_interior",
  "width_tiles": 8,
  "height_tiles": 6,
  "tiles_legend": {".":"floor_wood","#":"wall","D":"floor_wood"},
  "tiles": [
    "########",
    "#......#",
    "#......#",
    "#......#",
    "###DD###",
    "########"
  ],
  "entities": []
}`

func TestMultiMapWarp(t *testing.T) {
	dir := t.TempDir()
	wp := filepath.Join(dir, "overworld.json")
	if err := os.WriteFile(wp, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	ip := filepath.Join(dir, "cottage.json")
	if err := os.WriteFile(ip, []byte(interiorJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	w, _ := Load(wp)
	h := NewMultiMapHub(w)
	interior, err := h.LoadInterior(ip)
	if err != nil {
		t.Fatal(err)
	}
	if h.Get(interior.MapID) == nil {
		t.Fatal("interior not registered")
	}
	if !h.Warp("a", w.MapID, interior.MapID, Tile{3, 3}) {
		t.Fatal("warp failed")
	}
	if w.entities["a"] != nil {
		t.Fatal("a should be removed from overworld")
	}
	if interior.entities["a"] == nil {
		t.Fatal("a should be on interior")
	}
}
