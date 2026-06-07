package world

import "testing"

// Direct test that the snapshot observation path splits item
// entities into VisibleItems, not VisibleEntities. The user's wanderer
// bot was reporting `vision: 0 items` even at positions with multiple
// coin entities within radius 12 — this test makes the contract
// explicit so a regression in the split is caught at build time.
func TestSnapshot_VisibleItemsSplit(t *testing.T) {
	w := loadTestWorld(t)
	// Place an apple right next to entity "a" (which is at (1,1)).
	w.entities["coin_test_1"] = &Entity{
		EntityID:    "coin_test_1",
		Archetype:   "item",
		LogicalTile: Tile{2, 1},
		Extras:      map[string]any{"sprite": "item:coin_pouch"},
	}
	// Force a snapshot publish.
	w.publishSnapshot()
	snap := w.snapshot.Load()
	if snap == nil {
		t.Fatal("snapshot not published")
	}
	// Build the lock-free obs for entity "a".
	e := snap.Entities["a"]
	if e == nil {
		t.Fatal("snapshot missing entity a")
	}
	obs := snap.buildObservationSnap(e, 1, nil)
	if obs == nil {
		t.Fatal("nil observation")
	}
	if len(obs.VisibleItems) != 1 {
		t.Errorf("want 1 visible item, got %d (entities had %d)",
			len(obs.VisibleItems), len(obs.VisibleEntities))
		for _, v := range obs.VisibleEntities {
			t.Logf("  v_ent: %s archetype=%s pos=%v", v.EntityID, v.Archetype, v.Pos)
		}
		for _, v := range obs.VisibleItems {
			t.Logf("  v_item: %s sprite=%s pos=%v", v.EntityID, v.Sprite, v.Pos)
		}
	}
	// Ensure the item did NOT end up in VisibleEntities.
	for _, v := range obs.VisibleEntities {
		if v.Archetype == "item" {
			t.Errorf("item %s leaked into VisibleEntities", v.EntityID)
		}
	}
}
