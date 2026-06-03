package world

// Area-of-interest helpers. The viewer protocol subscribes to one or
// more rectangular CHUNKS; only entities + decorations within those
// chunks are sent. Same model the agent observation builder uses for
// the vision radius.
//
// Chunks are addressed by (cx, cy) at ChunkSize tile granularity.
// For our current 60×40 world that's only ~10 chunks; the model
// scales naturally to the 1000×1000 target.

const ChunkSize = 16

// ChunkOf returns the chunk address containing the given tile.
func ChunkOf(t Tile) [2]int {
	return [2]int{t[0] / ChunkSize, t[1] / ChunkSize}
}

// VisibleChunksRect returns the chunk addresses overlapping a
// rectangular tile range [x0,x1) × [y0,y1).
func VisibleChunksRect(x0, y0, x1, y1 int) [][2]int {
	if x1 <= x0 || y1 <= y0 {
		return nil
	}
	cx0, cy0 := x0/ChunkSize, y0/ChunkSize
	cx1, cy1 := (x1-1)/ChunkSize, (y1-1)/ChunkSize
	out := make([][2]int, 0, (cx1-cx0+1)*(cy1-cy0+1))
	for cy := cy0; cy <= cy1; cy++ {
		for cx := cx0; cx <= cx1; cx++ {
			out = append(out, [2]int{cx, cy})
		}
	}
	return out
}

// SnapshotForChunks returns a viewer-facing snapshot filtered to the
// requested chunks. Entities outside the chunks are dropped, as are
// decorations whose footprint doesn't overlap.
func (w *World) SnapshotForChunks(chunks [][2]int) WorldSnapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if len(chunks) == 0 {
		// Nothing requested = nothing sent. Cheap empty snapshot.
		return WorldSnapshot{MapID: w.MapID, Tick: w.tick}
	}
	inChunk := func(t Tile) bool {
		c := ChunkOf(t)
		for _, k := range chunks {
			if c == k {
				return true
			}
		}
		return false
	}
	out := WorldSnapshot{
		MapID:       w.MapID,
		Tick:        w.tick,
		WidthTiles:  w.WidthTiles,
		HeightTiles: w.HeightTiles,
	}
	for _, e := range w.entities {
		if inChunk(e.LogicalTile) {
			cp := *e
			out.Entities = append(out.Entities, &cp)
		}
	}
	return out
}
