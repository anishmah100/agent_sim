package spatial

import (
	"sort"
	"testing"
)

func TestIndex_AddRemoveAt(t *testing.T) {
	i := New()
	i.Add("a", Tile{1, 1})
	i.Add("b", Tile{1, 1})
	i.Add("c", Tile{2, 5})
	if i.Size() != 3 {
		t.Fatalf("size: %d", i.Size())
	}
	atOne := i.EntityAt(Tile{1, 1})
	sort.Strings(atOne)
	if len(atOne) != 2 || atOne[0] != "a" || atOne[1] != "b" {
		t.Fatalf("at (1,1): %v", atOne)
	}
	i.Remove("a")
	atOne = i.EntityAt(Tile{1, 1})
	if len(atOne) != 1 || atOne[0] != "b" {
		t.Fatalf("after remove a: %v", atOne)
	}
}

func TestIndex_Move(t *testing.T) {
	i := New()
	i.Add("a", Tile{1, 1})
	i.Move("a", Tile{5, 5})
	if loc, _ := i.LocationOf("a"); loc != [2]int{5, 5} {
		t.Fatalf("LocationOf: %v", loc)
	}
	if len(i.EntityAt(Tile{1, 1})) != 0 {
		t.Fatal("(1,1) should be empty after move")
	}
	if len(i.EntityAt(Tile{5, 5})) != 1 {
		t.Fatal("(5,5) should contain a")
	}
}

func TestIndex_EntitiesInRadius(t *testing.T) {
	i := New()
	i.Add("center", Tile{10, 10})
	i.Add("close", Tile{11, 10})
	i.Add("edge", Tile{13, 10})
	i.Add("far", Tile{20, 20})
	r := i.EntitiesInRadius(Tile{10, 10}, 3)
	sort.Strings(r)
	if len(r) != 3 || r[0] != "center" || r[1] != "close" || r[2] != "edge" {
		t.Fatalf("radius 3: %v", r)
	}
}

func TestIndex_EntitiesInRect(t *testing.T) {
	i := New()
	i.Add("inside", Tile{5, 5})
	i.Add("outside", Tile{0, 0})
	r := i.EntitiesInRect(2, 2, 10, 10)
	if len(r) != 1 || r[0] != "inside" {
		t.Fatalf("rect: %v", r)
	}
}
