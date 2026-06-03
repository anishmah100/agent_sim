// Package raster renders per-agent N×N tile crops as PNG bytes for
// multimodal agents. Built once at engine startup from the same art
// atlas the frontend serves; cached in memory.
package raster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	stddraw "image/draw"
	"image/png"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const tileSizePx = 16

// Rasterizer holds preloaded tile + decoration textures and the
// current world. Render() produces a PNG crop centered on an entity.
type Rasterizer struct {
	tiles map[string]image.Image       // tile kind name → texture
	decs  map[string]image.Image       // veg:NNN / bld:NNN → texture
	world *world.World
}

// New builds a rasterizer. `artDir` is the absolute path to the
// repo's `art/processed/` directory. Errors loading textures are
// logged but non-fatal — missing tiles render as solid color.
func New(w *world.World, artDir string) (*Rasterizer, error) {
	r := &Rasterizer{
		tiles: map[string]image.Image{},
		decs:  map[string]image.Image{},
		world: w,
	}
	r.loadTiles(filepath.Join(artDir, "tiles", "overworld"))
	r.loadDecorations(filepath.Join(artDir, "objects"))
	return r, nil
}

func (r *Rasterizer) loadTiles(dir string) {
	manifest := filepath.Join(filepath.Dir(filepath.Dir(dir)), "..", "manifests", "overworld_tileset.json")
	data, err := os.ReadFile(manifest)
	if err != nil {
		return
	}
	var m struct {
		Tiles        []struct{ Name string `json:"name"` } `json:"tiles"`
		KindDefaults map[string]string                      `json:"kind_defaults"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	for _, t := range m.Tiles {
		if img := loadPNG(filepath.Join(dir, t.Name+".png")); img != nil {
			r.tiles[t.Name] = img
		}
	}
	// Map kind aliases to their default tile.
	for kind, tileName := range m.KindDefaults {
		if img := r.tiles[tileName]; img != nil {
			r.tiles[kind] = img
		}
	}
}

func (r *Rasterizer) loadDecorations(objectsDir string) {
	for _, cat := range []string{"vegetation", "buildings"} {
		categoryDir := filepath.Join(objectsDir, cat)
		entries, err := os.ReadDir(categoryDir)
		if err != nil {
			continue
		}
		var prefix string
		if cat == "vegetation" {
			prefix = "veg:"
		} else {
			prefix = "bld:"
		}
		for _, e := range entries {
			name := e.Name()
			if len(name) < 9 || name[len(name)-4:] != ".png" || name[:4] != "obj_" {
				continue
			}
			id := prefix + name[4:7]
			if img := loadPNG(filepath.Join(categoryDir, name)); img != nil {
				r.decs[id] = img
			}
		}
	}
}

// Render produces a PNG crop of size (cropTiles*tileSizePx)² centered
// on the entity's tile.
func (r *Rasterizer) Render(entityID string, cropTiles int) ([]byte, error) {
	e := r.world.EntityByID(entityID)
	if e == nil {
		return nil, fmt.Errorf("unknown entity %s", entityID)
	}
	half := cropTiles / 2
	cx, cy := e.LogicalTile[0], e.LogicalTile[1]
	x0, y0 := cx-half, cy-half
	x1, y1 := x0+cropTiles, y0+cropTiles

	outW, outH := cropTiles*tileSizePx, cropTiles*tileSizePx
	rgba := image.NewRGBA(image.Rect(0, 0, outW, outH))
	// Background: void color so off-map areas stay legible.
	fillRGBA(rgba, image.NewUniform(uniformColor(0x18, 0x14, 0x25, 0xff)))

	// Terrain
	for ty := y0; ty < y1; ty++ {
		for tx := x0; tx < x1; tx++ {
			kind := r.world.TileKindAt(tx, ty)
			if kind == "" {
				continue
			}
			tex := r.tiles[kind]
			if tex == nil {
				continue
			}
			dst := image.Rect(
				(tx-x0)*tileSizePx, (ty-y0)*tileSizePx,
				(tx-x0+1)*tileSizePx, (ty-y0+1)*tileSizePx,
			)
			xdraw.NearestNeighbor.Scale(rgba, dst, tex, tex.Bounds(), xdraw.Over, nil)
		}
	}
	// Decorations — read via accessor.
	for _, d := range r.world.DecorationsInRect(x0, y0, x1, y1) {
		tex := r.decs[d.Sprite]
		if tex == nil {
			continue
		}
		// Anchor bottom-center at footprint.
		hTiles := d.HeightTiles
		if hTiles <= 0 {
			hTiles = 2
		}
		fpW := d.FootprintW
		if fpW < 1 {
			fpW = 1
		}
		renderW := fpW * tileSizePx
		renderH := int(hTiles * float64(tileSizePx))
		// SW corner in crop coords
		swX := (d.X - x0) * tileSizePx
		swY := (d.Y - y0+1) * tileSizePx
		dst := image.Rect(swX, swY-renderH, swX+renderW, swY)
		xdraw.NearestNeighbor.Scale(rgba, dst, tex, tex.Bounds(), xdraw.Over, nil)
	}
	// Entities — current map snapshot.
	for _, id := range r.world.EntityIDs() {
		ent := r.world.EntityByID(id)
		if ent == nil || ent.InsideBuilding != "" {
			continue
		}
		ex, ey := ent.LogicalTile[0], ent.LogicalTile[1]
		if ex < x0 || ex >= x1 || ey < y0 || ey >= y1 {
			continue
		}
		// 4×4 swatch for placeholder; real character art lands later.
		cx := (ex - x0) * tileSizePx
		cy := (ey - y0) * tileSizePx
		col := uniformColor(0xff, 0x88, 0x33, 0xff)
		if ent.EntityID == entityID {
			col = uniformColor(0x33, 0xff, 0x88, 0xff)
		}
		fillRect(rgba, image.Rect(cx+5, cy+4, cx+11, cy+12), col)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// === helpers ===

func loadPNG(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

func fillRGBA(dst *image.RGBA, src image.Image) {
	stddraw.Draw(dst, dst.Bounds(), src, image.Point{}, stddraw.Src)
}

func fillRect(dst *image.RGBA, r image.Rectangle, col image.Image) {
	stddraw.Draw(dst, r, col, image.Point{}, stddraw.Src)
}

func uniformColor(r, g, b, a uint8) *image.Uniform {
	return image.NewUniform(rgba{r, g, b, a})
}

type rgba struct{ R, G, B, A uint8 }

func (c rgba) RGBA() (uint32, uint32, uint32, uint32) {
	rr := uint32(c.R)
	gg := uint32(c.G)
	bb := uint32(c.B)
	aa := uint32(c.A)
	rr |= rr << 8
	gg |= gg << 8
	bb |= bb << 8
	aa |= aa << 8
	return rr, gg, bb, aa
}
