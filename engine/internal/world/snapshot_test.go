package world

import (
	"testing"
)

func TestSnapshot_ImmutableAfterPublish(t *testing.T) {
	w := loadTestWorld(t)
	w.Tick()
	snap := w.LoadSnapshot()
	if snap == nil {
		t.Fatal("nil snapshot after tick")
	}
	// Take a snapshot of entity 'a's tile, then move it via the queue
	// and tick again. The OLD snapshot must keep the pre-move tile —
	// observation builders must see a stable view.
	preTile := snap.Entities["a"].LogicalTile

	// Mutate the live entity (simulating a move) and tick to publish a
	// new snapshot. We use the unlocked path because we hold no lock.
	w.mu.Lock()
	w.entities["a"].LogicalTile = Tile{5, 5}
	w.mu.Unlock()
	w.Tick()

	if snap.Entities["a"].LogicalTile != preTile {
		t.Fatalf("old snapshot mutated: was %v, now %v",
			preTile, snap.Entities["a"].LogicalTile)
	}
	if w.LoadSnapshot().Entities["a"].LogicalTile != (Tile{5, 5}) {
		t.Fatalf("new snapshot didn't pick up the move")
	}
}

func TestSnapshot_SpatialIndexPopulated(t *testing.T) {
	w := loadTestWorld(t)
	w.Tick()
	snap := w.LoadSnapshot()
	// Entity 'a' at (1,1), 'b' at (8,1) — spatial index should locate
	// them at their LogicalTile.
	idsA := snap.EntityAtTile[Tile{1, 1}]
	if len(idsA) != 1 || idsA[0] != "a" {
		t.Fatalf("EntityAtTile{1,1}: want [a], got %v", idsA)
	}
	idsB := snap.EntityAtTile[Tile{8, 1}]
	if len(idsB) != 1 || idsB[0] != "b" {
		t.Fatalf("EntityAtTile{8,1}: want [b], got %v", idsB)
	}
	// A random empty tile has no entities.
	if got := snap.EntityAtTile[Tile{3, 3}]; len(got) != 0 {
		t.Fatalf("empty tile should have no entities, got %v", got)
	}
}

func TestSnapshot_StaticGridsSharedByReference(t *testing.T) {
	w := loadTestWorld(t)
	w.Tick()
	snap := w.LoadSnapshot()
	// The vision-blocks + tile-kind grids are shared by reference
	// (they're immutable after world load). Cheap sanity check that
	// the snapshot didn't deep-copy them — pointers identical.
	if len(snap.VisionBlocks) != w.HeightTiles {
		t.Fatalf("VisionBlocks length: got %d want %d",
			len(snap.VisionBlocks), w.HeightTiles)
	}
	// Mutate the world's visionBlocks (DANGEROUS in real code; ok in
	// a test under controlled conditions). The snapshot sees the change
	// because it shares the slice.
	w.mu.Lock()
	w.visionBlocks[0][0] = true
	w.mu.Unlock()
	if !snap.VisionBlocks[0][0] {
		t.Fatal("vision blocks were copied, expected shared reference")
	}
}

func TestSnapshot_BuildObservationLockFree(t *testing.T) {
	w := loadTestWorld(t)
	w.Tick()
	// The lock-free path: snapshot.BuildObservation must work without
	// the caller holding any lock. We hold no lock here.
	snap := w.LoadSnapshot()
	e := snap.Entities["a"]
	if e == nil {
		t.Fatal("missing entity")
	}
	obs := snap.buildObservationSnap(e, 1, nil)
	if obs == nil {
		t.Fatal("nil observation")
	}
	if obs.Self.EntityID != "a" {
		t.Fatalf("obs.Self.EntityID: got %q", obs.Self.EntityID)
	}
	// 'b' is in the spatial index at (8,1); within default vision radius
	// of 12 from (1,1) (chebyshev 7), should appear.
	foundB := false
	for _, v := range obs.VisibleEntities {
		if v.EntityID == "b" {
			foundB = true
			break
		}
	}
	if !foundB {
		t.Fatal("entity b should be in visible entities (chebyshev 7 ≤ 12)")
	}
}

// (TestFindPath_BoundedByManhattan removed: the engine no longer pathfinds
// for agents — navigation moved into the agent harness (agents/common/nav.py)
// and the engine only executes single-tile `step` actions. See
// docs/AGENT_MOVEMENT_REDESIGN.md.)
