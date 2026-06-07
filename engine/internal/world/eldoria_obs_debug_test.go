package world

import (
	"path/filepath"
	"runtime"
	"testing"
)

// TestEldoriaSnapshotItemsVision — load the actual Eldoria world.json
// and verify that an observation builder placed at (778, 892) returns
// the coin items that ARE in the snapshot near that tile.
//
// The wanderer at this position in production reports `visible_items: 0`
// even though .runlog/snapshots/latest.json contains coins at (785, 903),
// (779, 887), (784, 891) within radius 12. Reproducing here makes the
// bug debuggable with `go test -v -run TestEldoriaSnapshotItemsVision`.
func TestEldoriaSnapshotItemsVision(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = .../engine/internal/world/eldoria_obs_debug_test.go
	// repo root is 4 dirs up (engine, internal, world, file)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "agent_sim")
	worldPath := filepath.Join(repoRoot, "worlds", "eldoria", "world.json")
	w, err := Load(worldPath)
	if err != nil {
		t.Fatalf("Load eldoria: %v", err)
	}
	// Force a snapshot publish — Tick() does this normally but we
	// bypass Tick to avoid all the scenario hooks.
	w.mu.Lock()
	w.publishSnapshot()
	w.mu.Unlock()

	snap := w.snapshot.Load()
	if snap == nil {
		t.Fatal("snapshot nil after publishSnapshot")
	}

	// How many items in the snapshot map?
	itemCount := 0
	for _, e := range snap.Entities {
		if e.Archetype == "item" {
			itemCount++
		}
	}
	t.Logf("snapshot has %d total item entities", itemCount)
	if itemCount == 0 {
		t.Fatal("snapshot has zero item entities — load failed?")
	}

	// Spawn a synthetic probe agent at (778, 892) — same as the
	// real wanderer's position from production logs.
	probe := &Entity{
		EntityID:    "vision_probe",
		Archetype:   "trainer",
		LogicalTile: Tile{778, 892},
		Facing:      FacingS,
		Extras:      map[string]any{},
	}
	w.entities[probe.EntityID] = probe
	// Simulate what runtime does: a few ticks pass.
	for i := 0; i < 5; i++ {
		w.Tick()
	}
	snap = w.snapshot.Load()

	// Find probe in the new snapshot and ask for its observation.
	probeSnap := snap.Entities[probe.EntityID]
	if probeSnap == nil {
		t.Fatal("probe missing from snapshot")
	}
	obs := snap.buildObservationSnap(probeSnap, 1, nil)
	if obs == nil {
		t.Fatal("nil obs")
	}

	t.Logf("at (778, 892), probe sees: %d items, %d entities, %d objects",
		len(obs.VisibleItems), len(obs.VisibleEntities), len(obs.VisibleObjects))
	for _, vi := range obs.VisibleItems[:min(8, len(obs.VisibleItems))] {
		t.Logf("  item: %s at %v sprite=%s", vi.EntityID, vi.Pos, vi.Sprite)
	}
	if len(obs.VisibleEntities) > 0 {
		for _, ve := range obs.VisibleEntities[:min(5, len(obs.VisibleEntities))] {
			t.Logf("  ent:  %s archetype=%s at %v", ve.EntityID, ve.Archetype, ve.Pos)
		}
	}

	// Count items at tiles within Chebyshev 12 of (778, 892) in the
	// snapshot map. If that count is > 0 but VisibleItems is 0, the
	// bounding-box scan in buildObservationSnap is missing them.
	expected := 0
	for _, e := range snap.Entities {
		if e.Archetype != "item" {
			continue
		}
		dx := e.LogicalTile[0] - 778
		if dx < 0 {
			dx = -dx
		}
		dy := e.LogicalTile[1] - 892
		if dy < 0 {
			dy = -dy
		}
		if dx <= 12 && dy <= 12 {
			expected++
			t.Logf("  EXPECTED item: %s at %v (dx=%d dy=%d)",
				e.EntityID, e.LogicalTile, dx, dy)
		}
	}
	t.Logf("expected items within radius 12: %d", expected)
	if expected > 0 && len(obs.VisibleItems) == 0 {
		t.Errorf("BUG: snapshot has %d items in range but VisibleItems is empty",
			expected)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
