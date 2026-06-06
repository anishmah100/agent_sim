// genart_manifest — single-source-of-truth catalog generator.
//
// Walks `art/processed/` plus the existing per-category manifests and
// emits `art/manifests/sprites.json` — ONE file that every sprite-URL
// resolver in the frontend reads. Adding a new sprite then becomes:
// (1) drop the PNG, (2) add (or regenerate) the catalog entry, (3) it
// renders.
//
// Hand-tuned metadata (enterable flags, interior templates,
// footprint_tiles overrides, label) lives in
// `art/manifests/sprites.overrides.json`. The generator merges overrides
// over the auto-detected base, so a regen never loses your tweaks.
//
// Schema is described in `docs/CLEANUP_PLAN.md`. Categories used:
//
//   bld    Building (cottage, smithy, town hall, watchtower, granary)
//   stall  Market stall (red bread / blue fish / etc.)
//   veg    Vegetation + world resources (trees, bushes, boulders, ore)
//   int    Interior tile (floors, walls, rugs, doors)
//   prop   Interior prop (furniture, decor)
//   item   Inventory item
//   fx     Visual effect particle
//   ui     UI icon
//   char   Character / NPC (one entry per archetype, frames referenced)
//   stage  Construction stage frame (cottage_stage_0..5, wreckage, scaffolding)
//
// Sprite IDs are `<category>:<name>` lowercase.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Category struct {
	Label               string  `json:"label"`
	DefaultHeightTiles  float64 `json:"default_height_tiles"`
}

type Sprite struct {
	Path              string    `json:"path"`
	Label             string    `json:"label,omitempty"`
	Kind              string    `json:"kind,omitempty"`
	NativeSizePx      [2]int    `json:"native_size_px"`
	// FootprintTiles + RenderHeightTiles are pointers so omitempty can
	// distinguish "unset" from "explicitly zero" — only houses + named
	// buildings have a meaningful footprint; vegetation should rely on
	// the renderer's native-aspect fallback.
	FootprintTiles    *[2]int   `json:"footprint_tiles,omitempty"`
	RenderHeightTiles float64   `json:"render_height_tiles,omitempty"`
	Enterable         bool      `json:"enterable,omitempty"`
	InteriorTemplate  string    `json:"interior_template,omitempty"`
	Frames            *FrameSet `json:"frames,omitempty"`
}

func fp(w, h int) *[2]int { return &[2]int{w, h} }

type FrameSet struct {
	Dir       string              `json:"dir"`
	ByAction  map[string][]string `json:"by_action"`
	RefHeight int                 `json:"ref_height_px,omitempty"`
}

type Catalog struct {
	Schema     string              `json:"$schema"`
	BasePath   string              `json:"base_path,omitempty"`
	Extends    string              `json:"extends,omitempty"`
	Categories map[string]Category `json:"categories"`
	Sprites    map[string]Sprite   `json:"sprites"`
}

var defaultCategories = map[string]Category{
	"bld":   {Label: "Building", DefaultHeightTiles: 4},
	"stall": {Label: "Market stall", DefaultHeightTiles: 1.5},
	"veg":   {Label: "Vegetation", DefaultHeightTiles: 2},
	"int":   {Label: "Interior tile", DefaultHeightTiles: 1},
	"prop":  {Label: "Interior prop", DefaultHeightTiles: 1.5},
	"item":  {Label: "Inventory item", DefaultHeightTiles: 1},
	"fx":    {Label: "Visual effect", DefaultHeightTiles: 1},
	"ui":    {Label: "UI icon", DefaultHeightTiles: 1},
	"char":  {Label: "Character", DefaultHeightTiles: 1.5},
	"stage": {Label: "Construction stage", DefaultHeightTiles: 4},
}

func main() {
	artRoot := flag.String("art", "worlds/eldoria/art", "art root directory")
	out := flag.String("out", "worlds/eldoria/art/manifests/sprites.json", "output catalog path")
	overrides := flag.String("overrides", "worlds/eldoria/art/manifests/sprites.overrides.json", "hand-tuned overrides file (merged over auto-detected)")
	flag.Parse()

	cat := &Catalog{
		Schema:     "agent_sim/art/sprites/v1",
		Categories: defaultCategories,
		Sprites:    map[string]Sprite{},
	}

	processed := filepath.Join(*artRoot, "processed")
	if err := walkProcessed(processed, cat); err != nil {
		die("walk %s: %v", processed, err)
	}

	// Merge per-character frames from `art/manifests/character_frames/`.
	if err := loadCharacterFrames(*artRoot, cat); err != nil {
		fmt.Fprintln(os.Stderr, "warn: character frames:", err)
	}

	// Merge overrides last.
	if err := mergeOverrides(*overrides, cat); err != nil {
		fmt.Fprintln(os.Stderr, "warn: overrides:", err)
	}

	// Sort keys for stable output (catalog is a map; we encode it via a
	// sorted intermediate to keep diffs minimal).
	enc := json.NewEncoder(orderedFile(*out))
	enc.SetIndent("", "  ")
	if err := enc.Encode(cat); err != nil {
		die("encode: %v", err)
	}
	fmt.Printf("wrote %s: %d sprites across %d categories\n",
		*out, len(cat.Sprites), len(cat.Categories))
}

// walkProcessed scans every PNG under art/processed/ and registers it
// in the catalog. Categorisation is path-driven (see resolveCategory).
func walkProcessed(root string, cat *Catalog) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".png") {
			return nil
		}
		// Skip derivatives — they're regenerated; never reference at runtime.
		base := filepath.Base(path)
		if strings.Contains(base, ".thumb.") || strings.Contains(base, ".preview_") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		category, name, ok := resolveCategory(rel)
		if !ok {
			return nil // not a catalog-relevant sprite
		}
		w, h := pngSize(path)
		if w == 0 || h == 0 {
			return nil
		}
		id := category + ":" + name
		// Don't clobber already-set entries (priority is first-seen).
		if _, exists := cat.Sprites[id]; exists {
			return nil
		}
		sp := Sprite{
			Path:         rel,
			NativeSizePx: [2]int{w, h},
		}
		// Inferred footprints / render heights for known building IDs.
		switch id {
		case "bld:000", "bld:001":
			sp.FootprintTiles = fp(5, 2)
			sp.RenderHeightTiles = 4
			sp.Enterable = true
			sp.InteriorTemplate = "cottage"
			sp.Kind = "house"
		case "bld:blacksmith":
			sp.FootprintTiles = fp(3, 2)
			sp.RenderHeightTiles = 3.5
			sp.Enterable = true
			sp.InteriorTemplate = "blacksmith"
			sp.Kind = "smithy"
		case "bld:town_hall":
			sp.FootprintTiles = fp(6, 2)
			sp.RenderHeightTiles = 4
			sp.Enterable = true
			sp.InteriorTemplate = "town_hall"
			sp.Kind = "civic"
		case "bld:granary":
			sp.FootprintTiles = fp(2, 2)
			sp.RenderHeightTiles = 4
			sp.Enterable = true
			sp.InteriorTemplate = "cottage"
			sp.Kind = "storage"
		case "bld:watchtower":
			sp.FootprintTiles = fp(2, 2)
			sp.RenderHeightTiles = 5
			sp.Enterable = true
			sp.InteriorTemplate = "cottage"
			sp.Kind = "military"
		case "bld:well":
			sp.RenderHeightTiles = 1.5
			sp.Kind = "civic"
		}
		cat.Sprites[id] = sp
		return nil
	})
}

// resolveCategory maps a path under processed/ to a (category, name).
// Returns ok=false for paths that aren't catalog-relevant (master sheets
// at the root, leftover v1 subdirs we don't index, etc.).
func resolveCategory(rel string) (category, name string, ok bool) {
	dir := filepath.ToSlash(filepath.Dir(rel))
	base := strings.TrimSuffix(filepath.Base(rel), ".png")
	switch {
	case strings.HasPrefix(dir, "objects/buildings"):
		// obj_000.png → bld:000
		if strings.HasPrefix(base, "obj_") {
			return "bld", strings.TrimPrefix(base, "obj_"), true
		}
		return "", "", false
	case strings.HasPrefix(dir, "objects/vegetation"):
		if strings.HasPrefix(base, "obj_") {
			return "veg", strings.TrimPrefix(base, "obj_"), true
		}
		return "", "", false
	case strings.HasPrefix(dir, "objects/interior"):
		if strings.HasPrefix(base, "obj_") {
			return "prop", strings.TrimPrefix(base, "obj_"), true
		}
		return "", "", false
	case strings.HasPrefix(dir, "objects/items"):
		if strings.HasPrefix(base, "obj_") {
			return "item", strings.TrimPrefix(base, "obj_"), true
		}
		return "", "", false
	case dir == "v2_resources_world_master":
		return "veg", base, true
	case dir == "v2_market_stall":
		// stall_red_bread_open → stall:red_bread_open
		return "stall", strings.TrimPrefix(base, "stall_"), true
	case dir == "v2_construction_stages":
		return "stage", base, true
	case dir == "v2_interior_tiles_master":
		return "int", base, true
	case dir == "v2_interior_props_master":
		return "prop", base, true
	case dir == "v2_items_master_v2":
		return "item", base, true
	case dir == "v2_fx_particles":
		return "fx", base, true
	case dir == "v2_ui_icons":
		return "ui", base, true
	case dir == "tiles/interior":
		// Legacy sheet-derived props live here too (anvil_sheet,
		// fireplace_stone_sheet, etc.). Route them as props so the
		// catalog has one entry per logical sprite regardless of which
		// folder it lives in.
		return "prop", base, true
	case dir == "tiles/interior/props_2w":
		return "prop", base, true
	case dir == "frames":
		// /processed/frames/<char>/... handled by loadCharacterFrames.
		return "", "", false
	case dir == ".":
		// Loose PNGs at processed/ root. Pick up the v2_<name>.png
		// named buildings; everything else (master sheets, character
		// portraits, downsampled previews) is intentionally skipped.
		if strings.HasPrefix(base, "v2_") {
			named := strings.TrimPrefix(base, "v2_")
			switch named {
			case "blacksmith", "town_hall", "granary", "watchtower", "well":
				return "bld", named, true
			}
		}
		return "", "", false
	}
	return "", "", false
}

// loadCharacterFrames walks art/manifests/character_frames/<char>.json
// and produces one `char:<id>` entry per character with a FrameSet
// describing the per-action frames.
func loadCharacterFrames(artRoot string, cat *Catalog) error {
	dir := filepath.Join(artRoot, "manifests", "character_frames")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := "char:" + strings.TrimSuffix(e.Name(), ".json")
		framesDir := "frames/" + strings.TrimSuffix(e.Name(), ".json")
		// Per-character frames live under processed/frames/<char>/
		full := filepath.Join(artRoot, "processed", framesDir)
		stat, err := os.Stat(full)
		if err != nil || !stat.IsDir() {
			continue
		}
		cat.Sprites[id] = Sprite{
			Path: framesDir,
			Kind: "character",
			Frames: &FrameSet{
				Dir:      framesDir,
				ByAction: map[string][]string{},
			},
		}
	}
	return nil
}

// mergeOverrides reads an optional overrides JSON keyed by sprite id
// (e.g. {"bld:000": {"enterable": true, "label": "Cottage"}}) and
// applies its fields on top of the auto-detected entry. Unknown keys
// are added as new entries (useful when a sprite needs metadata but
// doesn't yet have a PNG).
func mergeOverrides(path string, cat *Catalog) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// Two-step parse so the file can carry documentation / example
	// blobs alongside real overrides without failing the whole load.
	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return err
	}
	for id, msg := range rawMap {
		if strings.HasPrefix(id, "_") {
			continue
		}
		var override Sprite
		if err := json.Unmarshal(msg, &override); err != nil {
			fmt.Fprintf(os.Stderr, "warn: override %q: %v\n", id, err)
			continue
		}
		base := cat.Sprites[id] // zero-value if absent
		if override.Path != "" {
			base.Path = override.Path
		}
		if override.Label != "" {
			base.Label = override.Label
		}
		if override.Kind != "" {
			base.Kind = override.Kind
		}
		if override.NativeSizePx != [2]int{} {
			base.NativeSizePx = override.NativeSizePx
		}
		if override.FootprintTiles != nil {
			base.FootprintTiles = override.FootprintTiles
		}
		if override.RenderHeightTiles != 0 {
			base.RenderHeightTiles = override.RenderHeightTiles
		}
		if override.Enterable {
			base.Enterable = true
		}
		if override.InteriorTemplate != "" {
			base.InteriorTemplate = override.InteriorTemplate
		}
		if override.Frames != nil {
			base.Frames = override.Frames
		}
		cat.Sprites[id] = base
	}
	return nil
}

func pngSize(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// orderedFile opens `path` for writing and returns it. The json encoder
// writes map keys in iteration order; Go 1.12+ orders json.Encode of
// Go maps deterministically by key.
func orderedFile(path string) *os.File {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		die("mkdir %s: %v", path, err)
	}
	f, err := os.Create(path)
	if err != nil {
		die("create %s: %v", path, err)
	}
	return f
}

// (sort.Strings + strconv intentionally referenced so future stable-output
// helpers can leverage them without re-importing.)
var _ = sort.Strings
var _ = strconv.Itoa
