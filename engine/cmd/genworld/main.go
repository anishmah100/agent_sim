// genworld — emits a procedural NxM world JSON for soak testing.
// All grass, sparse trees/rocks, no entities (agents auto-spawn on register).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	Entities    []any             `json:"entities"`
	Decorations []fileDeco        `json:"decorations,omitempty"`
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

func main() {
	w := flag.Int("w", 1000, "world width in tiles")
	h := flag.Int("h", 1000, "world height in tiles")
	out := flag.String("out", "../worlds/soak_1000x1000/world.json", "output JSON path")
	seed := flag.Uint64("seed", 1, "PRNG seed")
	decoEvery := flag.Int("deco-every", 200, "1-in-N tiles gets a tree/rock (0 disables)")
	flag.Parse()

	rng := rand.New(rand.NewPCG(*seed, *seed^0xdeadbeef))

	tiles := make([]string, *h)
	row := strings.Repeat(".", *w) // '.' = grass
	for y := 0; y < *h; y++ {
		tiles[y] = row
	}

	decos := []fileDeco{}
	if *decoEvery > 0 {
		blocking := true
		for y := 0; y < *h; y++ {
			for x := 0; x < *w; x++ {
				if rng.IntN(*decoEvery) != 0 {
					continue
				}
				sprite := "veg:tree_oak"
				if rng.IntN(3) == 0 {
					sprite = "veg:rock_big"
				}
				decos = append(decos, fileDeco{
					X: x, Y: y,
					Sprite:     sprite,
					HeightTile: 2.0,
					FootprintW: 1,
					FootprintH: 1,
					Walkable:   &[]bool{!blocking}[0],
				})
			}
		}
	}

	fw := fileWorld{
		Schema:      "agent_sim/world/v1",
		MapID:       fmt.Sprintf("soak_%dx%d", *w, *h),
		DisplayName: fmt.Sprintf("Soak Test %dx%d", *w, *h),
		TileSizePx:  16,
		WidthTiles:  *w,
		HeightTiles: *h,
		TilesLegend: map[string]string{".": "grass"},
		Tiles:       tiles,
		Entities:    []any{},
		Decorations: decos,
	}

	data, err := json.MarshalIndent(fw, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(*out, data, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%dx%d, %d decorations)\n", *out, *w, *h, len(decos))
}
