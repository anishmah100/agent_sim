package world

import (
	"fmt"
	"strconv"
	"strings"
)

// Building interiors (HeartGold multi-map model).
//
// A decoration building (sprite "bld:NNN") has no entity and no authored
// interior. When an agent enters, we lazily generate a small walled room as
// its own World and warp the agent in; on exit we warp it back to the
// overworld door tile. See docs/INTERIORS_MULTIMAP_PLAN.md.
//
// Interiors are generated per building INSTANCE (keyed by the door tile) so
// two agents in two different houses are not in the same room. They are GC'd
// by the hub once empty (ProcessWarps).

// InteriorMapID returns the deterministic map id for a building instance's
// interior, keyed by sprite + the overworld door tile so each building has its
// own room.
func InteriorMapID(sprite string, door Tile) string {
	return fmt.Sprintf("interior:%s@%d,%d", sprite, door[0], door[1])
}

// ParseInteriorMapID is the inverse of InteriorMapID: it extracts the building
// sprite and overworld door tile from an "interior:<sprite>@x,y" map id.
// Returns ("", {0,0}) if the id isn't an interior id.
func ParseInteriorMapID(id string) (sprite string, door [2]int) {
	rest, ok := strings.CutPrefix(id, "interior:")
	if !ok {
		return "", [2]int{}
	}
	at := strings.LastIndexByte(rest, '@')
	if at < 0 {
		return rest, [2]int{}
	}
	sprite = rest[:at]
	coords := strings.Split(rest[at+1:], ",")
	if len(coords) == 2 {
		x, _ := strconv.Atoi(coords[0])
		y, _ := strconv.Atoi(coords[1])
		door = [2]int{x, y}
	}
	return sprite, door
}

// interiorDims picks a room size from the building footprint, clamped to a
// sane range so even a 1-tile stall gets a walkable room and a town hall
// doesn't get an aircraft hangar.
func interiorDims(fpW, fpH int) (w, h int) {
	w = fpW + 2
	if w < 7 {
		w = 7
	}
	if w > 14 {
		w = 14
	}
	h = fpH + 5
	if h < 7 {
		h = 7
	}
	if h > 11 {
		h = 11
	}
	return w, h
}

// GenerateInterior builds a fully-initialized interior World for a building:
// a wall border, a floor, an entrance/exit tile at the south-centre interior
// row, and a little flavour loot so the room isn't empty. The returned World
// is ready to register with the hub (Add) and tick.
func GenerateInterior(sprite string, door Tile, fpW, fpH int) (*World, error) {
	ww, hh := interiorDims(fpW, fpH)
	// Build the tile rows: '#' border wall, '.' floor, 'E' exit/entrance.
	exitX, exitY := ww/2, hh-2 // interior tile just above the bottom wall
	rows := make([]string, hh)
	for y := 0; y < hh; y++ {
		line := make([]byte, ww)
		for x := 0; x < ww; x++ {
			switch {
			case x == 0 || x == ww-1 || y == 0 || y == hh-1:
				line[x] = '#'
			case x == exitX && y == exitY:
				line[x] = 'E'
			default:
				line[x] = '.'
			}
		}
		rows[y] = string(line)
	}
	fw := fileWorld{
		MapID:       InteriorMapID(sprite, door),
		WidthTiles:  ww,
		HeightTiles: hh,
		TilesLegend: map[string]string{
			"#": "wall",
			".": "floor_wood",
			"E": "floor_wood", // exit mat — walkable floor; exit handled by verb/tile
		},
		Tiles: rows,
	}
	w, err := buildWorld(fw, "")
	if err != nil {
		return nil, err
	}
	w.interiorExitTile = Tile{exitX, exitY}
	return w, nil
}

// interiorEntrance returns the tile an entering agent should appear on (just
// inside the door — the exit tile).
func (w *World) interiorEntrance() Tile { return w.interiorExitTile }
