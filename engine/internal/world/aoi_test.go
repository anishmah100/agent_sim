package world

import "testing"

func TestChunkOf(t *testing.T) {
	if ChunkOf(Tile{0, 0}) != [2]int{0, 0} {
		t.Fatal("0,0 should be chunk 0,0")
	}
	if ChunkOf(Tile{ChunkSize, 0}) != [2]int{1, 0} {
		t.Fatal("ChunkSize,0 should be chunk 1,0")
	}
}

func TestVisibleChunksRect(t *testing.T) {
	got := VisibleChunksRect(0, 0, ChunkSize*2, ChunkSize)
	want := [][2]int{{0, 0}, {1, 0}}
	if len(got) != len(want) {
		t.Fatalf("len got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("[%d] got %v want %v", i, got[i], want[i])
		}
	}
}
