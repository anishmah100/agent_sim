package persist

import (
	"os"
	"path/filepath"
	"testing"
)

const testWorldJSON = `{
  "map_id": "persist_test",
  "width_tiles": 4,
  "height_tiles": 4,
  "tiles_legend": {".":"grass"},
  "tiles": ["....","....","....","...."],
  "entities": [
    {"entity_id":"hero","archetype":"trainer","pos":[1,1],"facing":"S","display_name":"Hero"}
  ]
}`

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	wp := filepath.Join(dir, "w.json")
	if err := os.WriteFile(wp, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatalf("write world: %v", err)
	}
	w, err := loadWorld(t, wp)
	if err != nil {
		t.Fatalf("load world: %v", err)
	}
	// Stash custom extras.
	w.ApplySnapshot("hero", map[string]any{"gold": 999, "hp": 50}, "")

	saveDir := PathFor(dir, "persist_test")
	if _, err := Write(w, saveDir); err != nil {
		t.Fatalf("write: %v", err)
	}
	latest := LatestPath(saveDir)
	if latest == "" {
		t.Fatal("LatestPath empty after write")
	}

	// Reload fresh world, then restore.
	w2, err := loadWorld(t, wp)
	if err != nil {
		t.Fatal(err)
	}
	if err := Restore(w2, latest); err != nil {
		t.Fatalf("restore: %v", err)
	}
	h := w2.EntityByID("hero")
	if h == nil {
		t.Fatal("hero missing after restore")
	}
	gold, _ := h.Extras["gold"].(float64) // JSON unmarshals numbers as float64
	if gold != 999 {
		t.Fatalf("gold restored as %v", h.Extras["gold"])
	}
}
