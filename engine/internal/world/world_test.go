package world

import (
	"os"
	"path/filepath"
	"testing"
)

// Tiny test world JSON. 10×6 grass with a single wall row and two
// entities. Used by several tests below.
const testWorldJSON = `{
  "map_id": "test_world",
  "width_tiles": 10,
  "height_tiles": 6,
  "tiles_legend": {".":"grass","#":"wall"},
  "tiles": [
    "..........",
    "..........",
    "..####....",
    "..........",
    "..........",
    ".........."
  ],
  "entities": [
    {"entity_id":"a","archetype":"trainer","pos":[1,1],"facing":"S","display_name":"A"},
    {"entity_id":"b","archetype":"trainer","pos":[8,1],"facing":"S","display_name":"B"}
  ]
}`

func writeTestWorld(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "test.json")
	if err := os.WriteFile(p, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func loadTestWorld(t *testing.T) *World {
	t.Helper()
	p := writeTestWorld(t)
	w, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return w
}

func TestLoad_Basic(t *testing.T) {
	w := loadTestWorld(t)
	if w.WidthTiles != 10 || w.HeightTiles != 6 {
		t.Fatalf("dims: got %dx%d", w.WidthTiles, w.HeightTiles)
	}
	if !w.IsWalkable(Tile{0, 0}) {
		t.Fatal("0,0 should be walkable (grass)")
	}
	if w.IsWalkable(Tile{2, 2}) {
		t.Fatal("2,2 should NOT be walkable (wall)")
	}
}

func TestLOS_WallBlocks(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	b := w.entities["b"]
	// A at (1,1), B at (8,1). Wall row at y=2. Same row, wall not on
	// the line of sight, so they should see each other (long range).
	if !w.lineOfSight(a.LogicalTile, b.LogicalTile) {
		t.Fatal("clear horizontal line should have LOS")
	}
	// Through-the-wall: 1,1 → 5,5. The straight line crosses y=2 in the
	// wall columns. Should be blocked.
	if w.lineOfSight(Tile{1, 1}, Tile{5, 5}) {
		t.Fatal("line through wall should be blocked")
	}
}

func TestSeesEntity_VisionRadius(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	b := w.entities["b"]
	// At default radius (12) and chebyshev distance 7 they see each other.
	if !w.SeesEntity(a, b, VisionRadius) {
		t.Fatal("a should see b within VisionRadius")
	}
	// At radius 3 they don't.
	if w.SeesEntity(a, b, 3) {
		t.Fatal("a should NOT see b at radius 3")
	}
}

func TestHearing_SpeakLocal(t *testing.T) {
	w := loadTestWorld(t)
	a := w.entities["a"]
	b := w.entities["b"]
	// Speak radius is 3. A and B are 7 apart so B does not hear.
	w.emitSpeech(a, "speech", "hi", 3)
	audB := w.VisibleAudible(b, 0)
	if len(audB) != 0 {
		t.Fatalf("B should not hear A's local speak; got %v", audB)
	}
	// Shout radius 15 — B hears.
	w.emitSpeech(a, "shout", "HEY", 15)
	audB = w.VisibleAudible(b, 0)
	found := false
	for _, e := range audB {
		if e.Text == "HEY" {
			found = true
		}
	}
	if !found {
		t.Fatal("B should hear A's shout")
	}
}

// Movement is the single-tile `step` verb now (the multi-tile engine-
// pathfinding `move` verb was removed in the movement redesign). Step
// behaviour is covered by step_test.go.

func TestObservationBuilder(t *testing.T) {
	w := loadTestWorld(t)
	obs := w.BuildObservationFor("a", 1, nil)
	if obs == nil {
		t.Fatal("nil observation")
	}
	if obs.Self.EntityID != "a" {
		t.Fatalf("self mismatch: %s", obs.Self.EntityID)
	}
	// B should be in visible_entities (clear LOS at distance 7).
	found := false
	for _, ve := range obs.VisibleEntities {
		if ve.EntityID == "b" {
			found = true
		}
	}
	if !found {
		t.Fatal("A's observation should include B")
	}
}

// TestObservation_ExtrasSummary_D9 — visible_entities[i].extras_summary
// must surface hp_bucket + equipped_slot + equipped_sprite from the
// observed entity, NOT inventory/gold/hunger (which stay private).
func TestObservation_ExtrasSummary_D9(t *testing.T) {
	w := loadTestWorld(t)
	// Equip B with a sword and put them at full HP.
	w.entities["b"].Extras = map[string]interface{}{
		"hp":      100,
		"max_hp":  100,
		"gold":    250,    // private — must NOT leak
		"hunger":  0.85,   // private — must NOT leak
		"inventory": []interface{}{"item:apple#1", "item:bread_loaf#2"},
		"equipped": map[string]interface{}{
			"weapon": "item:sword_short#7",
		},
	}
	obs := w.BuildObservationFor("a", 1, nil)
	var b *VisibleEntityState
	for i := range obs.VisibleEntities {
		if obs.VisibleEntities[i].EntityID == "b" {
			b = &obs.VisibleEntities[i]
		}
	}
	if b == nil {
		t.Fatal("B should be in A's visible_entities")
	}
	sum := b.ExtrasSummary
	if sum == nil {
		t.Fatal("B's extras_summary must be populated")
	}
	if sum["hp_bucket"] != "full" {
		t.Errorf("expected hp_bucket=full, got %v", sum["hp_bucket"])
	}
	if sum["equipped_slot"] != "weapon" {
		t.Errorf("expected equipped_slot=weapon, got %v", sum["equipped_slot"])
	}
	if sum["equipped_sprite"] != "item:sword_short" {
		t.Errorf("expected equipped_sprite=item:sword_short, got %v", sum["equipped_sprite"])
	}
	// Private state must NOT appear.
	for _, k := range []string{"gold", "hunger", "inventory"} {
		if _, present := sum[k]; present {
			t.Errorf("private key %s leaked into extras_summary: %v", k, sum)
		}
	}
	// Now wound B (hp=50, max_hp=100 → wounded bucket) and check.
	w.entities["b"].Extras["hp"] = 50
	obs = w.BuildObservationFor("a", 2, nil)
	for i := range obs.VisibleEntities {
		if obs.VisibleEntities[i].EntityID == "b" {
			b = &obs.VisibleEntities[i]
		}
	}
	if b.ExtrasSummary["hp_bucket"] != "wounded" {
		t.Errorf("expected hp_bucket=wounded at hp=50, got %v", b.ExtrasSummary["hp_bucket"])
	}
	// Dying (hp=20).
	w.entities["b"].Extras["hp"] = 20
	obs = w.BuildObservationFor("a", 3, nil)
	for i := range obs.VisibleEntities {
		if obs.VisibleEntities[i].EntityID == "b" {
			b = &obs.VisibleEntities[i]
		}
	}
	if b.ExtrasSummary["hp_bucket"] != "dying" {
		t.Errorf("expected hp_bucket=dying at hp=20, got %v", b.ExtrasSummary["hp_bucket"])
	}
}

// TestObservation_VisibleItems_D8 — item entities surface in
// visible_items, NOT visible_entities. Out-of-vision items and items
// inside buildings are excluded.
func TestObservation_VisibleItems_D8(t *testing.T) {
	w := loadTestWorld(t)
	// Place an item near A: apple at (2,1) — distance 1 from A at (1,1).
	w.entities["apple1"] = &Entity{
		EntityID:    "apple1",
		Archetype:   "item",
		DisplayName: "apple",
		LogicalTile: Tile{2, 1},
		Facing:      FacingS,
		Extras:      map[string]any{"sprite": "item:apple"},
	}
	// Place a coin pile farther: distance 6 from A but still in vision.
	w.entities["coins1"] = &Entity{
		EntityID:    "coins1",
		Archetype:   "item",
		LogicalTile: Tile{7, 1},
		Facing:      FacingS,
		Extras:      map[string]any{"sprite": "item:coins_small_pile", "quantity": 10},
	}
	// Place an item far away (out of vision radius 12 from A).
	w.entities["faraway"] = &Entity{
		EntityID:    "faraway",
		Archetype:   "item",
		LogicalTile: Tile{1, 5},
		Facing:      FacingS,
		Extras:      map[string]any{"sprite": "item:wood_log"},
	}
	// All test world is small (10x6) so even 'faraway' is in radius 12
	// — to actually test out-of-vision we need an item beyond vision.
	// Override the radius for this test to 3 so coins1 (distance 6)
	// and faraway are excluded.
	opts := defaultObsOpts()
	opts.Radius = 3
	obs := w.BuildObservation(w.entities["a"], 1, &opts)
	if obs == nil {
		t.Fatal("nil observation")
	}
	// apple1 must be in visible_items.
	var apple *VisibleItemState
	for i := range obs.VisibleItems {
		if obs.VisibleItems[i].EntityID == "apple1" {
			apple = &obs.VisibleItems[i]
		}
	}
	if apple == nil {
		t.Fatalf("expected apple1 in visible_items, got %v", obs.VisibleItems)
	}
	if apple.Sprite != "item:apple" {
		t.Errorf("expected sprite item:apple, got %q", apple.Sprite)
	}
	if apple.Label != "apple" {
		t.Errorf("expected label apple, got %q", apple.Label)
	}
	if apple.Quantity != 1 {
		t.Errorf("expected default quantity 1, got %d", apple.Quantity)
	}
	// coins1 and faraway must NOT appear (out of radius 3).
	for _, vi := range obs.VisibleItems {
		if vi.EntityID == "coins1" || vi.EntityID == "faraway" {
			t.Errorf("did not expect %s in visible_items at radius 3, got %v",
				vi.EntityID, obs.VisibleItems)
		}
	}
	// Items must NOT appear in visible_entities (D8 split is the
	// fundamental contract).
	for _, ve := range obs.VisibleEntities {
		if ve.EntityID == "apple1" {
			t.Error("apple1 leaked into visible_entities; should be visible_items only")
		}
	}
	// Now widen radius to 12 — coins1 must surface with quantity=10.
	opts.Radius = 12
	obs = w.BuildObservation(w.entities["a"], 2, &opts)
	var coins *VisibleItemState
	for i := range obs.VisibleItems {
		if obs.VisibleItems[i].EntityID == "coins1" {
			coins = &obs.VisibleItems[i]
		}
	}
	if coins == nil {
		t.Fatalf("expected coins1 in visible_items at radius 12, got %v", obs.VisibleItems)
	}
	if coins.Quantity != 10 {
		t.Errorf("expected quantity 10, got %d", coins.Quantity)
	}
}
