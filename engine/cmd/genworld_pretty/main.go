// genworld_pretty — generates a beautiful, cohesive 1500×1500 fantasy
// continent ("Eldoria") with multiple distinct regions connected by
// rivers, roads, and shorelines.
//
// Layout, north → south:
//
//   0–250    Northern Mountains (stone, sparse evergreens)
//   250–500  Pinewood Forest (dense green, river)
//   500–900  Central Grasslands + Farms (grass + tuft)
//   900–1200 Lake Mirin + wetlands (water + lily + reeds)
//   1200–1500 Sundered Dunes (sand desert)
//
// Eastern strip (x > 1200, all y): ocean
//
// Towns (drop a cluster of paths + named building stalls):
//
//   Frostvale   (180,  120) — mountain mining hamlet
//   Pinewood    (320,  380) — forest village
//   Greenfield  (600,  720) — farming village
//   Crossroads  (800,  900) — central capital
//   Saltport    (1280, 700) — coastal port
//   Dunehallow  (700, 1300) — desert oasis
//
// A central river flows from the northern mountains south through
// Pinewood, across the plains, into Lake Mirin, and out to the eastern
// sea — connecting all regions.
//
// Roads radiate from Crossroads to every other town.
//
// Decoration density: 1 every ~150 tiles for trees, 1 every 600 for
// rocks, 1 every 250 for ground variants. Yields ~15k decorations on
// a 1500×1500 map.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"strings"
)

type fileWorld struct {
	Schema      string            `json:"$schema,omitempty"`
	MapID       string            `json:"map_id"`
	DisplayName string            `json:"display_name"`
	TileSizePx  int               `json:"tile_size_px"`
	WidthTiles  int               `json:"width_tiles"`
	HeightTiles int               `json:"height_tiles"`
	TilesLegend map[string]string `json:"tiles_legend"`
	Tiles       []string          `json:"tiles"`
	Entities    []entityRec       `json:"entities"`
	Decorations []fileDeco        `json:"decorations,omitempty"`
}

type entityRec struct {
	EntityID    string `json:"entity_id"`
	Archetype   string `json:"archetype"`
	Pos         [2]int `json:"pos"`
	Facing      string `json:"facing"`
	DisplayName string `json:"display_name,omitempty"`
}

type fileDeco struct {
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Sprite     string  `json:"sprite"`
	HeightTile float64 `json:"height_tiles,omitempty"`
	FootprintW int     `json:"footprint_w,omitempty"`
	FootprintH int     `json:"footprint_h,omitempty"`
	Walkable   *bool   `json:"walkable,omitempty"`
}

const (
	GRASS = '.'
	DIRT  = ','
	PATH  = '_'
	STONE = '#'
	SAND  = '~'
	WATER = 'W'
)

type town struct {
	name, ref string
	cx, cy    int
	tier      int // 0 = hamlet, 1 = village, 2 = capital
}

// Eldoria settlements are organised as KINGDOMS — a big central city
// (capital or regional kingdom) with satellite villages clustered around
// it within 150 tiles, plus a wide scatter of LONE cottages between
// kingdoms. This produces "real kingdoms" with travel between them
// rather than 27 evenly-spaced towns.
//
// Lake Mirin centred at (1000, 1000) r~165 — all positions are at
// least 200 tiles clear.
var towns = []town{
	// Tier 2: ROYAL CAPITAL — central plain, biggest city in the world.
	{"Crossroads", "crossroads", 780, 880, 2},
	// Tier 1: FOUR REGIONAL KINGDOMS — each anchors its biome.
	{"Frostvale", "frostvale", 230, 230, 1},     // mountain kingdom
	{"Pinewood", "pinewood", 320, 460, 1},        // forest kingdom
	{"Saltport", "saltport", 1180, 700, 1},       // coastal kingdom
	{"Dunehallow", "dunehallow", 700, 1330, 1},   // desert kingdom
	{"Lakeshore", "lakeshore", 790, 1180, 1},     // lake kingdom
	// Tier 0: SATELLITE VILLAGES near each kingdom (within ~150 tiles).
	// Frostvale satellites:
	{"Stonemoor", "stonemoor", 360, 200, 0},
	{"Coldbrook", "coldbrook", 130, 320, 0},
	{"Ironvein", "ironvein", 320, 320, 0},
	// Pinewood satellites:
	{"Aspendell", "aspendell", 220, 540, 0},
	{"Mossglen", "mossglen", 420, 520, 0},
	{"Birchwood", "birchwood", 240, 380, 0},
	{"Riverbend", "riverbend", 470, 600, 0},
	// Crossroads satellites:
	{"Greenfield", "greenfield", 620, 780, 0},
	{"Westgate", "westgate", 600, 950, 0},
	{"Oakshade", "oakshade", 880, 800, 0},
	{"Eastfall", "eastfall", 1140, 850, 0},
	{"Greenrun", "greenrun", 700, 1020, 0},
	// Saltport satellites:
	{"Cliffhaven", "cliffhaven", 1140, 530, 0},
	{"Hawkspire", "hawkspire", 1110, 800, 0},
	{"Saltwatch", "saltwatch", 1170, 880, 0},
	// Dunehallow + Lakeshore satellites:
	{"Sandholme", "sandholme", 920, 1380, 0},
	{"Driftwell", "driftwell", 460, 1320, 0},
	{"Marshton", "marshton", 950, 1230, 0},
	{"Reedwater", "reedwater", 580, 1180, 0},
}

// hash-based 2D value noise. Returns ~[-1, 1].
func vnoise(x, y, seed int64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xBF58476D1CE4E5B9 ^ uint64(seed)*0x94D049BB133111EB
	h = (h ^ (h >> 30)) * 0xBF58476D1CE4E5B9
	h = (h ^ (h >> 27)) * 0x94D049BB133111EB
	h = h ^ (h >> 31)
	return float64(h&0xFFFFFFFF)/float64(0xFFFFFFFF)*2 - 1
}

func smoothNoise(x, y float64, seed int64) float64 {
	xi := int64(math.Floor(x))
	yi := int64(math.Floor(y))
	fx := x - float64(xi)
	fy := y - float64(yi)
	n00 := vnoise(xi, yi, seed)
	n10 := vnoise(xi+1, yi, seed)
	n01 := vnoise(xi, yi+1, seed)
	n11 := vnoise(xi+1, yi+1, seed)
	sx := fx * fx * (3 - 2*fx)
	sy := fy * fy * (3 - 2*fy)
	a := n00*(1-sx) + n10*sx
	b := n01*(1-sx) + n11*sx
	return a*(1-sy) + b*sy
}

func fractalNoise(x, y float64, seed int64, octaves int) float64 {
	v := 0.0
	amp := 1.0
	freq := 1.0
	max := 0.0
	for i := 0; i < octaves; i++ {
		v += amp * smoothNoise(x*freq, y*freq, seed+int64(i))
		max += amp
		amp *= 0.5
		freq *= 2
	}
	return v / max
}

// biome assignment by y-band with noise-jittered boundaries.
//
// Design rule: each biome should look COHERENT — a viewer dropped into
// any chunk should immediately know what region they're in. So we keep
// within-biome variation LOW (only large noise scales, high thresholds)
// and reserve the visual transitions for the inter-biome boundaries,
// which use a smoother shore jitter at large scale (low-frequency,
// no high-frequency checker).
func biomeAt(x, y, w, h int, seed int64) byte {
	// Ocean: eastern coast with smooth, low-frequency shore jitter.
	shoreJitter := fractalNoise(float64(y)/120, 0, seed+101, 3) * 50
	if x > int(float64(w-1)-260+shoreJitter) {
		return WATER
	}
	// Offshore islands — very sparse so the coastline reads as ocean,
	// not as a sea-of-tiny-isles checker.
	if x > int(float64(w-1)-180+shoreJitter) {
		isle := fractalNoise(float64(x)/30, float64(y)/30, seed+103, 3)
		if isle > 0.7 {
			return SAND
		}
		return WATER
	}

	// Lake Mirin — irregular lake centered at (1000, 1000), big.
	dx := float64(x - 1000)
	dy := float64(y - 1000)
	lakeJ := fractalNoise(float64(x)/60, float64(y)/60, seed+302, 3) * 30
	if math.Sqrt(dx*dx+dy*dy)+lakeJ < 165 {
		return WATER
	}
	// Small inland pond near Greenfield.
	pdx, pdy := float64(x-540), float64(y-650)
	if math.Sqrt(pdx*pdx+pdy*pdy)+fractalNoise(float64(x)/15, float64(y)/15, seed+304, 3)*6 < 28 {
		return WATER
	}

	// Latitude jitter on biome boundaries — LOW frequency, MEDIUM
	// amplitude. This gives organic sweeping boundaries between regions
	// without producing high-frequency speckle inside any one region.
	bandJ := fractalNoise(float64(x)/140, float64(y)/140, seed+201, 3) * 45

	fy := float64(y) + bandJ
	switch {
	case fy < 240:
		// Northern mountains — mostly STONE; rare large dirt scree slopes.
		v := fractalNoise(float64(x)/45, float64(y)/45, seed+11, 3)
		if v > 0.45 {
			return DIRT
		}
		return STONE
	case fy < 360:
		// Foothill grass with rare stone outcrops.
		v := fractalNoise(float64(x)/50, float64(y)/50, seed+12, 3)
		if v > 0.55 {
			return STONE
		}
		return GRASS
	case fy < 580:
		// Pinewood Forest — pure GRASS (forest density comes from trees,
		// not from terrain checker).
		return GRASS
	case fy < 980:
		// Central Grasslands — pure GRASS with rare large dirt fields.
		v := fractalNoise(float64(x)/80, float64(y)/80, seed+14, 3)
		if v > 0.6 {
			return DIRT
		}
		return GRASS
	case fy < 1200:
		// Wetlands — GRASS with occasional ponds (not speckle dirt).
		v := fractalNoise(float64(x)/35, float64(y)/35, seed+15, 3)
		if v > 0.55 {
			return WATER
		}
		return GRASS
	default:
		// Sundered Dunes — pure SAND with rare stone outcrops + oases.
		v := fractalNoise(float64(x)/55, float64(y)/55, seed+16, 3)
		if v > 0.6 {
			return STONE
		}
		oasis := fractalNoise(float64(x)/25, float64(y)/25, seed+17, 3)
		if oasis > 0.7 {
			return WATER
		}
		return SAND
	}
}

// carveRiver — Bresenham-ish carve from (sx,sy) → (ex,ey) with noise.
func carveRiver(grid [][]byte, sx, sy, ex, ey int, width int, seed int64) {
	w := len(grid[0])
	h := len(grid)
	steps := 6000
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		// Cubic ease so the river arcs through the middle naturally.
		// Plus per-step noise for serpentine wiggle.
		nx := smoothNoise(t*30, 0, seed+701) * 60
		ny := smoothNoise(0, t*30, seed+702) * 30
		x := int(math.Round(float64(sx)+(float64(ex-sx))*t + nx))
		y := int(math.Round(float64(sy)+(float64(ey-sy))*t + ny))
		// River banks of width w/2.
		for dy := -width; dy <= width; dy++ {
			for dx := -width; dx <= width; dx++ {
				xx, yy := x+dx, y+dy
				if xx < 0 || xx >= w || yy < 0 || yy >= h {
					continue
				}
				d := math.Sqrt(float64(dx*dx + dy*dy))
				if d <= float64(width) {
					grid[yy][xx] = WATER
				}
			}
		}
	}
}

// carveRoad — narrow path between two towns. Just the centerline tiles
// become PATH (cobble); we do NOT lay a dirt apron, since at game zoom
// the autotile transitions between dirt/grass/stone/sand all generate
// edge variants and the road ends up reading as a chunky brown stripe
// rather than a road. The tile autotiler already gives the path a clean
// edge against the biome.
func carveRoad(grid [][]byte, ax, ay, bx, by int, seed int64) {
	w := len(grid[0])
	h := len(grid)
	steps := 6000
	for s := 0; s <= steps; s++ {
		t := float64(s) / float64(steps)
		nx := smoothNoise(t*8, 0, seed+801) * 22
		ny := smoothNoise(0, t*8, seed+802) * 22
		x := int(math.Round(float64(ax) + (float64(bx-ax))*t + nx))
		y := int(math.Round(float64(ay) + (float64(by-ay))*t + ny))
		// 3-tile-wide road (centerline + one tile each side).
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				xx, yy := x+dx, y+dy
				if xx < 0 || xx >= w || yy < 0 || yy >= h {
					continue
				}
				// Bridge water (river crossings).
				if grid[yy][xx] == WATER {
					grid[yy][xx] = PATH
					continue
				}
				grid[yy][xx] = PATH
			}
		}
	}
}

// stampTownCenter — paint a cluster of paths around (cx,cy) of radius r
// and return decoration slots for buildings/stalls.
func stampTownCenter(grid [][]byte, t town) []fileDeco {
	w := len(grid[0])
	h := len(grid)
	// Town plaza: 30-radius square of grass + path grid.
	r := 28
	if t.tier == 2 {
		r = 44 // bigger plaza for capital
	} else if t.tier == 0 {
		r = 18 // tiny hamlet
	}
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			xx, yy := t.cx+dx, t.cy+dy
			if xx < 0 || xx >= w || yy < 0 || yy >= h {
				continue
			}
			// Keep water (rivers/lakes) intact — towns work around rivers.
			if grid[yy][xx] == WATER {
				continue
			}
			d2 := dx*dx + dy*dy
			// Cobble inner core of plaza.
			if d2 <= r*r/4 {
				grid[yy][xx] = PATH
				continue
			}
			// Mid-ring: dirt apron to match the biome cleanly. Skip the
			// outer rim entirely — let the biome's own tile carry through
			// so the town doesn't paste a foreign grass blob onto sand or
			// stone.
			if d2 <= r*r/2 {
				grid[yy][xx] = DIRT
			}
		}
	}

	// Building/stall layout.
	decos := []fileDeco{}
	hMax := h
	wMax := w
	addDeco := func(dx, dy int, sprite string, fw, fhe int, htile float64, walk bool) {
		x, y := t.cx+dx, t.cy+dy
		if x < 0 || x >= wMax || y < 0 || y >= hMax {
			return
		}
		// Don't place a building on water (river running through the
		// town center, ocean encroachment, etc.). Check the footprint
		// rect; if any cell is water, bail.
		for ddy := 0; ddy < fhe; ddy++ {
			for ddx := 0; ddx < fw; ddx++ {
				cx, cy := x+ddx, y-ddy
				if cx < 0 || cx >= wMax || cy < 0 || cy >= hMax {
					continue
				}
				if grid[cy][cx] == WATER {
					return
				}
			}
		}
		walkP := walk
		decos = append(decos, fileDeco{
			X: x, Y: y, Sprite: sprite,
			HeightTile: htile, FootprintW: fw, FootprintH: fhe,
			Walkable: &walkP,
		})
	}

	// AUDITED sprite catalog — only IDs verified by eye to be actual
	// houses are used as houses. The rest (bld:002 door, bld:003 window,
	// bld:008 well, bld:009 lamp, etc.) are NEVER passed as buildings.
	//
	// Houses (visually verified):
	//   bld:000 — red-roof cottage (large)
	//   bld:001 — brown-roof cottage (large)
	//
	// V2 named enterable buildings:
	//   bld:blacksmith, bld:town_hall, bld:granary, bld:watchtower
	//
	// V2 well — placed as a small decoration ONLY, not an enterable house.
	houseA := "bld:000"
	houseB := "bld:001"
	switch t.tier {
	case 2: // Royal capital — sprawling, all stalls, multi-district
		// North admin block
		addDeco(-3, -16, "bld:town_hall", 6, 2, 4, false)
		addDeco(-15, -14, houseA, 5, 2, 4, false)
		addDeco(10, -14, houseB, 5, 2, 4, false)
		// East residential row
		addDeco(14, -6, houseA, 5, 2, 4, false)
		addDeco(14, 0, houseB, 5, 2, 4, false)
		addDeco(14, 6, houseA, 5, 2, 4, false)
		// West residential row
		addDeco(-18, -6, houseB, 5, 2, 4, false)
		addDeco(-18, 0, houseA, 5, 2, 4, false)
		addDeco(-18, 6, houseB, 5, 2, 4, false)
		// South market — ALL 6 stall colours
		addDeco(-9, 14, "bld:stall_red_bread_open", 0, 0, 1.5, false)
		addDeco(-6, 14, "bld:stall_green_veg_open", 0, 0, 1.5, false)
		addDeco(-3, 14, "bld:stall_blue_fish_open", 0, 0, 1.5, false)
		addDeco(3, 14, "bld:stall_purple_cloth_open", 0, 0, 1.5, false)
		addDeco(6, 14, "bld:stall_gold_cheese_open", 0, 0, 1.5, false)
		addDeco(9, 14, "bld:stall_brown_smith_open", 0, 0, 1.5, false)
		// Industry
		addDeco(-14, 12, "bld:blacksmith", 3, 2, 3.5, false)
		addDeco(11, 12, "bld:granary", 2, 2, 4, false)
		// Civic — well is decorative (1×1, NOT clickable as a house)
		addDeco(0, 18, "bld:well", 0, 0, 1.5, false)
		addDeco(-22, 14, "bld:watchtower", 2, 2, 5, false)
		addDeco(20, 14, "bld:watchtower", 2, 2, 5, false)
	case 1: // City — multiple cottages, smithy, granary, stalls, tower
		addDeco(0, -10, houseA, 5, 2, 4, false)
		addDeco(-10, -2, houseB, 5, 2, 4, false)
		addDeco(10, -2, houseA, 5, 2, 4, false)
		addDeco(-10, 8, "bld:blacksmith", 3, 2, 3.5, false)
		addDeco(10, 8, "bld:granary", 2, 2, 4, false)
		addDeco(0, 12, "bld:well", 0, 0, 1.5, false)
		addDeco(-5, 14, "bld:stall_red_bread_open", 0, 0, 1.5, false)
		addDeco(-2, 14, "bld:stall_green_veg_open", 0, 0, 1.5, false)
		addDeco(2, 14, "bld:stall_blue_fish_open", 0, 0, 1.5, false)
		addDeco(5, 14, "bld:stall_purple_cloth_open", 0, 0, 1.5, false)
		addDeco(-15, 12, "bld:watchtower", 2, 2, 5, false)
	case 0:
		// Tier 0 layout varies by a hash of the town's ref so no two
		// villages look identical. Five layout templates chosen so the
		// player reads them as distinct places, not copies.
		hash := 0
		for _, c := range t.ref {
			hash = hash*31 + int(c)
		}
		switch hash % 5 {
		case 0: // Triangle: 3 cottages around a central well
			addDeco(0, -6, houseA, 5, 2, 4, false)
			addDeco(-8, 2, houseB, 5, 2, 4, false)
			addDeco(8, 2, houseA, 5, 2, 4, false)
			addDeco(0, 6, "bld:well", 0, 0, 1.5, false)
			addDeco(-3, 10, "bld:stall_red_bread_open", 0, 0, 1.5, false)
			addDeco(3, 10, "bld:stall_green_veg_open", 0, 0, 1.5, false)
		case 1: // Row: 4 cottages in a north–south line, watchtower N
			addDeco(0, -10, "bld:watchtower", 2, 2, 5, false)
			addDeco(-8, -4, houseA, 5, 2, 4, false)
			addDeco(8, -4, houseB, 5, 2, 4, false)
			addDeco(-8, 6, houseB, 5, 2, 4, false)
			addDeco(8, 6, houseA, 5, 2, 4, false)
			addDeco(0, 4, "bld:well", 0, 0, 1.5, false)
			addDeco(0, 10, "bld:stall_blue_fish_open", 0, 0, 1.5, false)
		case 2: // Square: 4 cottages around an open plaza + granary
			addDeco(-9, -7, houseA, 5, 2, 4, false)
			addDeco(7, -7, houseB, 5, 2, 4, false)
			addDeco(-9, 7, houseB, 5, 2, 4, false)
			addDeco(7, 7, houseA, 5, 2, 4, false)
			addDeco(0, -2, "bld:well", 0, 0, 1.5, false)
			addDeco(0, 11, "bld:granary", 2, 2, 4, false)
			addDeco(-4, 11, "bld:stall_purple_cloth_open", 0, 0, 1.5, false)
		case 3: // Crossroads: a road junction with 5 cottages + smithy
			addDeco(0, -6, houseA, 5, 2, 4, false)
			addDeco(-10, 0, houseB, 5, 2, 4, false)
			addDeco(10, 0, houseA, 5, 2, 4, false)
			addDeco(-6, 8, houseB, 5, 2, 4, false)
			addDeco(6, 8, houseA, 5, 2, 4, false)
			addDeco(0, 12, "bld:blacksmith", 3, 2, 3.5, false)
			addDeco(-3, 5, "bld:well", 0, 0, 1.5, false)
		case 4: // Tower hamlet: a watchtower flanked by two cottages
			addDeco(0, -3, "bld:watchtower", 2, 2, 5, false)
			addDeco(-9, 4, houseA, 5, 2, 4, false)
			addDeco(8, 4, houseB, 5, 2, 4, false)
			addDeco(0, 9, "bld:well", 0, 0, 1.5, false)
			addDeco(-3, 11, "bld:stall_gold_cheese_open", 0, 0, 1.5, false)
		}
	}
	return decos
}

// scatterFarms places lone cottages, hay-bale farmsteads, watchtowers,
// AND small forest/grassland hamlet clusters so the world reads as
// densely-populated across every biome, not just kingdoms with empty
// land between. Also stamps KINGDOM SPRAWL — extra cottages within 100
// tiles of every kingdom centre so the city core spills outward into
// the surrounding countryside.
func scatterFarms(grid [][]byte, rng *rand.Rand, seed int64) []fileDeco {
	w := len(grid[0])
	h := len(grid)
	out := []fileDeco{}

	// Kingdom sprawl: each kingdom (tier 1 or 2) gets 12-20 lone
	// cottages + props within a ring 30-90 tiles from the centre.
	for _, t := range towns {
		if t.tier == 0 {
			continue
		}
		sprawlCount := 12
		if t.tier == 2 {
			sprawlCount = 30
		}
		for i := 0; i < sprawlCount; i++ {
			ang := rng.Float64() * 2 * math.Pi
			r := 35.0 + rng.Float64()*55
			x := t.cx + int(math.Cos(ang)*r)
			y := t.cy + int(math.Sin(ang)*r)
			if x < 5 || x >= w-5 || y < 5 || y >= h-5 {
				continue
			}
			k := grid[y][x]
			if k != GRASS && k != DIRT && k != SAND {
				continue
			}
			sprite := "bld:000"
			if (x+y+i)%2 == 0 {
				sprite = "bld:001"
			}
			walkP := false
			out = append(out, fileDeco{
				X: x, Y: y, Sprite: sprite,
				HeightTile: 4, FootprintW: 5, FootprintH: 2, Walkable: &walkP,
			})
			// Sprinkle 2-3 garden / prop tiles around the cottage.
			walkT := true
			gardens := []string{"veg:flowers_red", "veg:flowers_yellow", "veg:flowers_blue", "veg:bush_berry_red", "veg:hay_bales"}
			for j := 0; j < 2; j++ {
				dx := rng.IntN(6) - 3
				dy := rng.IntN(6) - 3
				if x+dx < 0 || x+dx >= w || y+dy < 0 || y+dy >= h {
					continue
				}
				out = append(out, fileDeco{
					X: x + dx, Y: y + dy,
					Sprite: gardens[rng.IntN(len(gardens))],
					HeightTile: 0.6, Walkable: &walkT,
				})
			}
		}
	}

	// Wild outposts everywhere — 1 seed per ~3500 tiles = ~640 on a
	// 1500×1500 map. Mix of clusters + lone homesteads.
	target := w * h / 3500
	placed := 0
	for tries := 0; tries < target*30 && placed < target; tries++ {
		x := rng.IntN(w-10) + 5
		y := rng.IntN(h-10) + 5
		k := grid[y][x]
		// Skip water, mountains, town plazas (path), the desert (too
		// barren for farms) and tiles near rivers.
		if k != GRASS && k != DIRT {
			continue
		}
		if y < 240 || y > 1180 {
			continue
		}
		// Don't place too close to a town center, OR too close to a
		// structure we already placed (cluster spacing).
		tooClose := false
		for _, t := range towns {
			dx, dy := x-t.cx, y-t.cy
			if dx*dx+dy*dy < 60*60 {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		// Min spacing to previous outposts so the world doesn't degenerate
		// into 1 mega-blob.
		for _, prev := range out {
			dx, dy := x-prev.X, y-prev.Y
			if dx*dx+dy*dy < 25*25 {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		// Only ACTUAL houses + verified named buildings get used here.
		// Audited sprites in /art/processed/objects/buildings/:
		//   000 = red-roof cottage, 001 = brown-roof cottage.
		//   002–030 = doors, windows, lamps, signs, benches, fences,
		//   chimneys, roof tiles — NOT houses, NEVER passed here.
		walkP := false
		walkT := true
		altHouse := func(seed int) string {
			if seed%2 == 0 {
				return "bld:001"
			}
			return "bld:000"
		}
		roll := rng.IntN(12)
		switch {
		case roll < 3:
			// HAMLET CLUSTER — 3 cottages around a tiny clearing + well
			// + hay bales. Makes the wilderness read as inhabited.
			for i, off := range [][2]int{{-4, -2}, {4, -2}, {0, 4}} {
				dx, dy := off[0], off[1]
				if x+dx < 0 || x+dx >= w || y+dy < 0 || y+dy >= h {
					continue
				}
				out = append(out, fileDeco{
					X: x + dx, Y: y + dy, Sprite: altHouse(i + x + y),
					HeightTile: 4, FootprintW: 5, FootprintH: 2, Walkable: &walkP,
				})
			}
			out = append(out, fileDeco{
				X: x, Y: y - 1, Sprite: "bld:well",
				HeightTile: 1.5, FootprintW: 1, FootprintH: 1, Walkable: &walkP,
			})
			// Sprinkle props around the hamlet so it feels lived-in.
			extras := []string{"veg:hay_bales", "veg:logs_pile", "veg:wheelbarrow_hay", "veg:flowers_yellow", "veg:flowers_red", "veg:bush_round_green"}
			for i := 0; i < 6; i++ {
				dx := rng.IntN(12) - 6
				dy := rng.IntN(12) - 6
				if x+dx < 0 || x+dx >= w || y+dy < 0 || y+dy >= h {
					continue
				}
				out = append(out, fileDeco{
					X: x + dx, Y: y + dy,
					Sprite: extras[rng.IntN(len(extras))],
					HeightTile: 0.9, Walkable: &walkT,
				})
			}
			placed++
		case roll < 6:
			// Lone cottage with a small garden.
			out = append(out, fileDeco{
				X: x, Y: y, Sprite: altHouse(x + y),
				HeightTile: 4, FootprintW: 5, FootprintH: 2, Walkable: &walkP,
			})
			gardens := []string{"veg:flowers_red", "veg:flowers_blue", "veg:flowers_yellow", "veg:bush_berry_red", "veg:bush_berry_blue"}
			for i := 0; i < 4; i++ {
				dx := rng.IntN(8) - 4
				dy := rng.IntN(8) - 4
				if x+dx < 0 || x+dx >= w || y+dy < 0 || y+dy >= h {
					continue
				}
				out = append(out, fileDeco{
					X: x + dx, Y: y + dy,
					Sprite: gardens[rng.IntN(len(gardens))],
					HeightTile: 0.6, Walkable: &walkT,
				})
			}
			placed++
		case roll < 8:
			// Watchtower with a campfire below (logs_pile stands in).
			out = append(out, fileDeco{
				X: x, Y: y, Sprite: "bld:watchtower",
				HeightTile: 5, FootprintW: 2, FootprintH: 2, Walkable: &walkP,
			})
			out = append(out, fileDeco{
				X: x, Y: y + 4, Sprite: "veg:logs_pile",
				HeightTile: 0.7, Walkable: &walkT,
			})
			placed++
		case roll < 10:
			// Working farmstead — granary + lots of hay + a cottage nearby.
			out = append(out, fileDeco{
				X: x, Y: y, Sprite: "bld:granary",
				HeightTile: 4, FootprintW: 2, FootprintH: 2, Walkable: &walkP,
			})
			out = append(out, fileDeco{
				X: x + 6, Y: y, Sprite: altHouse(x),
				HeightTile: 4, FootprintW: 5, FootprintH: 2, Walkable: &walkP,
			})
			for i := 0; i < 6; i++ {
				dx := rng.IntN(10) - 5
				dy := rng.IntN(10) - 5
				if x+dx < 0 || x+dx >= w || y+dy < 0 || y+dy >= h {
					continue
				}
				kind := "veg:hay_bales"
				if i%3 == 0 {
					kind = "veg:wheelbarrow_hay"
				}
				out = append(out, fileDeco{
					X: x + dx, Y: y + dy, Sprite: kind,
					HeightTile: 1.0, Walkable: &walkT,
				})
			}
			placed++
		default:
			// Blacksmith outpost + a couple of barrels/logs (decorative).
			out = append(out, fileDeco{
				X: x, Y: y, Sprite: "bld:blacksmith",
				HeightTile: 3.5, FootprintW: 3, FootprintH: 2, Walkable: &walkP,
			})
			out = append(out, fileDeco{
				X: x + 4, Y: y + 1, Sprite: "veg:logs_pile",
				HeightTile: 0.8, Walkable: &walkT,
			})
			placed++
		}
	}
	return out
}

func main() {
	w := flag.Int("w", 1500, "world width in tiles")
	h := flag.Int("h", 1500, "world height in tiles")
	seed := flag.Int64("seed", 42, "PRNG seed")
	out := flag.String("out", "../worlds/eldoria/world.json", "output JSON path")
	flag.Parse()

	rng := rand.New(rand.NewPCG(uint64(*seed), uint64(*seed)^0xdeadbeef))

	// Build the tile grid.
	grid := make([][]byte, *h)
	for y := 0; y < *h; y++ {
		grid[y] = make([]byte, *w)
		for x := 0; x < *w; x++ {
			grid[y][x] = biomeAt(x, y, *w, *h, *seed)
		}
	}

	// River: flows from northern mountains south through forest into Lake.
	carveRiver(grid, 250, 0, 1000, 1000, 5, *seed)
	// Tributary from the eastern foothills joining the main river.
	carveRiver(grid, 700, 200, 700, 880, 3, *seed+2)
	// River exits east from lake to the eastern ocean.
	carveRiver(grid, 1050, 1050, 1450, 700, 5, *seed+1)

	// Stamp town plazas + collect their building decos.
	var decos []fileDeco
	for _, t := range towns {
		decos = append(decos, stampTownCenter(grid, t)...)
	}

	// Road network — capital is towns[0]. Connect every other town to
	// it (radial trunk) AND link adjacent neighbours so the network
	// reads as a continent of trade routes rather than a hub-spoke star.
	capital := towns[0]
	for _, t := range towns[1:] {
		carveRoad(grid, t.cx, t.cy, capital.cx, capital.cy, *seed+int64(t.cx))
	}
	// Adjacent-pair links — pick nearest-neighbour for each non-capital
	// town and lay a short road. This produces a sparse mesh.
	for i := 1; i < len(towns); i++ {
		a := towns[i]
		var nearest *town
		nd := 0
		for j := 1; j < len(towns); j++ {
			if i == j {
				continue
			}
			b := towns[j]
			d := (a.cx-b.cx)*(a.cx-b.cx) + (a.cy-b.cy)*(a.cy-b.cy)
			if nearest == nil || d < nd {
				nb := b
				nearest = &nb
				nd = d
			}
		}
		if nearest != nil && nd < 280*280 {
			carveRoad(grid, a.cx, a.cy, nearest.cx, nearest.cy, *seed+int64(i*131))
		}
	}

	// Sprinkle isolated farms, watchtowers, lone cottages — produced AFTER
	// roads so they can land near a road but BEFORE the second plaza
	// stamp so plazas still override their tiles cleanly.
	decos = append(decos, scatterFarms(grid, rng, *seed)...)

	// Restamp town plazas AFTER road carving so the plaza overwrites any
	// road that got drawn through the town center.
	for _, t := range towns {
		stampTownCenter(grid, t)
	}

	// Place vegetation by biome.
	for y := 0; y < *h; y++ {
		for x := 0; x < *w; x++ {
			k := grid[y][x]
			if k != GRASS && k != DIRT && k != STONE && k != SAND {
				continue
			}
			// Per-biome density: denser in forest, sparser in plains/desert.
			density := 150
			switch {
			case y < 240:
				density = 300 // sparse mountains
			case y < 560:
				density = 90  // Pinewood forest — thinned to make room for towns
			case y < 980:
				density = 180 // patchy grasslands
			case y < 1200:
				density = 140 // wetland reeds
			default:
				density = 260 // sparse desert
			}
			if rng.IntN(density) != 0 {
				continue
			}
			sprite := ""
			fp := false
			height := 2.0
			fw, fhe := 1, 1
			// Sprite IDs below MUST exist in art/processed/v2_resources_world_master/
			// — see Decoration.ts spriteUrl().
			switch {
			case y < 240:
				// Mountain conifers + boulders.
				if k == STONE {
					switch rng.IntN(6) {
					case 0, 1:
						sprite = "veg:tree_pine"
					case 2:
						sprite = "veg:boulder_iron_ore"
					case 3:
						sprite = "veg:boulder_mossy"
					case 4:
						sprite = "veg:boulder_large_cracked"
					default:
						sprite = "veg:rock_small"
					}
				} else if k == DIRT {
					if rng.IntN(2) == 0 {
						sprite = "veg:tree_pine"
					} else {
						sprite = "veg:boulder_medium"
					}
				}
			case y < 560:
				// Pinewood Forest — DENSE pines + birches + bushes.
				if k == GRASS {
					r := rng.IntN(12)
					switch {
					case r < 6:
						sprite = "veg:tree_pine"
					case r < 9:
						sprite = "veg:tree_birch"
					case r < 10:
						sprite = "veg:tree_oak"
					case r < 11:
						sprite = "veg:bush_round_dark"
						fp = true
						height = 1.0
					default:
						sprite = "veg:mushrooms_red"
						fp = true
						height = 0.6
					}
				} else if k == DIRT {
					sprite = "veg:rock_small"
				}
			case y < 980:
				// Grasslands — oaks, apples, flowers, sunflowers.
				if k == GRASS {
					switch rng.IntN(10) {
					case 0, 1:
						sprite = "veg:tree_oak"
					case 2:
						sprite = "veg:tree_apple"
					case 3, 4:
						sprite = "veg:bush_round_green"
						fp = true
						height = 1.0
					case 5:
						sprite = "veg:flowers_red"
						fp = true
						height = 0.5
					case 6:
						sprite = "veg:flowers_yellow"
						fp = true
						height = 0.5
					case 7:
						sprite = "veg:flowers_purple"
						fp = true
						height = 0.5
					case 8:
						sprite = "veg:sunflower"
						fp = true
						height = 1.2
					default:
						sprite = "veg:grass_tall"
						fp = true
						height = 0.7
					}
				}
			case y < 1200:
				// Wetlands — reeds, willows, lily pads.
				if k == WATER {
					if rng.IntN(2) == 0 {
						sprite = "veg:lily_pads"
						fp = true
						height = 0.4
					}
				} else if k == GRASS || k == DIRT {
					switch rng.IntN(5) {
					case 0, 1:
						sprite = "veg:reeds_cattail"
						fp = true
						height = 1.0
					case 2:
						sprite = "veg:tree_willow"
					case 3:
						sprite = "veg:bush_round_green"
						fp = true
						height = 1.0
					default:
						sprite = "veg:flowers_blue"
						fp = true
						height = 0.5
					}
				}
			default:
				// Sundered Dunes — dry rocks, stalagmites, dead trees, crystals.
				if k == SAND {
					switch rng.IntN(8) {
					case 0, 1:
						sprite = "veg:stalagmite"
					case 2:
						sprite = "veg:tree_dead"
					case 3:
						sprite = "veg:stones_pile"
						fp = true
						height = 0.8
					case 4:
						sprite = "veg:crystal_purple"
						fp = true
						height = 1.0
					default:
						sprite = "veg:rock_small"
					}
				} else if k == STONE {
					sprite = "veg:boulder_large_cracked"
				}
			}
			if sprite == "" {
				continue
			}
			walk := fp
			// FootprintW/H are deliberately omitted (0) so the renderer
			// falls back to the sprite's NATIVE aspect ratio when sizing
			// to height_tiles. Forcing FootprintW=1 forces width to 16px,
			// which squishes wide sprites (boulders, lily pads, flowers)
			// and stretches tall ones — exactly the visible bug.
			_ = fw
			_ = fhe
			decos = append(decos, fileDeco{
				X: x, Y: y, Sprite: sprite,
				HeightTile: height,
				Walkable:   &walk,
			})
		}
	}

	// AUDITED character sprites — every one of these has a PNG under
	// art/processed/<name>.png and animation frames under
	// art/processed/frames/<name>/. We guarantee each appears at least
	// a handful of times across the world.
	allCharacters := []string{
		"baker", "blacksmith_npc", "child", "cloaked_wanderer",
		"drifter", "goblin", "iron_guard", "mason", "mayor",
		"trainer_lyra_blue", "trainer_red", "wizard", "woodcutter",
	}

	// Per-settlement archetype distributions — chosen so the personality
	// of each settlement reads (a forest village has woodcutters, a
	// fortress has iron guards, etc.).
	capitalPool := []string{
		"mayor", "mayor", "iron_guard", "iron_guard", "iron_guard",
		"trainer_red", "trainer_lyra_blue", "wizard",
		"baker", "blacksmith_npc", "mason", "child",
	}
	cityPool := []string{
		"baker", "blacksmith_npc", "mason", "iron_guard",
		"trainer_red", "trainer_lyra_blue",
		"cloaked_wanderer", "child", "mayor",
	}
	villagePool := []string{
		"baker", "woodcutter", "child", "child",
		"drifter", "mason",
	}

	var ents []entityRec
	npcID := 0
	for _, t := range towns {
		var pool []string
		count := 4
		if t.tier == 2 {
			pool = capitalPool
			count = 14
		} else if t.tier == 1 {
			pool = cityPool
			count = 9
		} else {
			pool = villagePool
		}
		for i := 0; i < count; i++ {
			npcID++
			dx := rng.IntN(40) - 20
			dy := rng.IntN(40) - 20
			arch := pool[rng.IntN(len(pool))]
			ents = append(ents, entityRec{
				EntityID:    fmt.Sprintf("npc_%s_%d", t.ref, i),
				Archetype:   arch,
				Pos:         [2]int{t.cx + dx, t.cy + dy},
				Facing:      "S",
				DisplayName: fmt.Sprintf("%s of %s", arch, t.name),
			})
		}
	}

	// Wilderness NPCs — biome-flavoured pools, each containing several
	// distinct archetypes so the wilderness reads as populated, not as
	// "the wandering drifter biome".
	wildCount := (*w * *h) / 25000 // ~90 on a 1500×1500 map
	for i := 0; i < wildCount*4 && len(ents) < 250; i++ {
		x := rng.IntN(*w-20) + 10
		y := rng.IntN(*h-20) + 10
		k := grid[y][x]
		if k == WATER || k == STONE {
			continue
		}
		// Avoid town centers.
		tooClose := false
		for _, t := range towns {
			if (x-t.cx)*(x-t.cx)+(y-t.cy)*(y-t.cy) < 60*60 {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		var arch string
		switch {
		case y < 350:
			// Mountain wilderness — miners + goblins + drifters
			arch = []string{"blacksmith_npc", "mason", "goblin", "drifter", "iron_guard"}[rng.IntN(5)]
		case y < 580:
			// Pine forest — woodcutters dominate
			arch = []string{"woodcutter", "woodcutter", "woodcutter", "cloaked_wanderer", "drifter", "child"}[rng.IntN(6)]
		case y < 980:
			// Grasslands — bakers walking between towns, guards, travellers
			arch = []string{"drifter", "baker", "iron_guard", "trainer_red", "child", "mason"}[rng.IntN(6)]
		case y < 1200:
			// Wetlands — drifters + cloaked wanderers + a goblin
			arch = []string{"drifter", "cloaked_wanderer", "goblin", "trainer_lyra_blue"}[rng.IntN(4)]
		default:
			// Desert — wizards + cloaked figures + goblins + lone drifters
			arch = []string{"wizard", "wizard", "cloaked_wanderer", "drifter", "goblin"}[rng.IntN(5)]
		}
		npcID++
		ents = append(ents, entityRec{
			EntityID:    fmt.Sprintf("wild_%d", npcID),
			Archetype:   arch,
			Pos:         [2]int{x, y},
			Facing:      "S",
			DisplayName: arch,
		})
	}

	// Coverage check — if any character archetype didn't get used, spawn
	// a few in a plausible location. Guarantees every audited sprite is
	// represented in the world.
	usedCounts := make(map[string]int)
	for _, e := range ents {
		usedCounts[e.Archetype]++
	}
	for _, arch := range allCharacters {
		need := 4 - usedCounts[arch]
		for n := 0; n < need; n++ {
			for tries := 0; tries < 40; tries++ {
				x := rng.IntN(*w-20) + 10
				y := rng.IntN(*h-20) + 10
				k := grid[y][x]
				if k == WATER || k == STONE {
					continue
				}
				npcID++
				ents = append(ents, entityRec{
					EntityID:    fmt.Sprintf("guarantee_%s_%d", arch, n),
					Archetype:   arch,
					Pos:         [2]int{x, y},
					Facing:      "S",
					DisplayName: arch,
				})
				break
			}
		}
	}

	// Serialize tile grid into row strings.
	tiles := make([]string, *h)
	for y := 0; y < *h; y++ {
		var b strings.Builder
		b.Grow(*w)
		for x := 0; x < *w; x++ {
			b.WriteByte(grid[y][x])
		}
		tiles[y] = b.String()
	}

	fw := fileWorld{
		Schema:      "agent_sim/world/v1",
		MapID:       "eldoria",
		DisplayName: "Eldoria",
		TileSizePx:  16,
		WidthTiles:  *w,
		HeightTiles: *h,
		TilesLegend: map[string]string{
			".": "grass",
			",": "dirt",
			"_": "path",
			"#": "stone",
			"~": "sand",
			"W": "water",
		},
		Tiles:       tiles,
		Entities:    ents,
		Decorations: decos,
	}

	data, err := json.Marshal(fw)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%dx%d, %d entities, %d decorations, %d MB)\n",
		*out, *w, *h, len(ents), len(decos), len(data)/1024/1024)
}
