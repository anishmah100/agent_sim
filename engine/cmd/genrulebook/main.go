// genrulebook — emit worlds/<name>/RULEBOOK.md + rulebook.json for a
// world bundle. Used by CI / pre-commit to keep the committed rulebook
// fresh after every rules.star or system-manifest change.
//
// Usage:
//
//	go run ./cmd/genrulebook -bundle worlds/eldoria
//
// Writes worlds/eldoria/RULEBOOK.md and worlds/eldoria/rulebook.json
// (relative to the engine's working dir; the tool runs from the
// project root in CI).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/anishmah100/agent_sim/engine/internal/scenario/fantasy_town"
	"github.com/anishmah100/agent_sim/engine/internal/world"
	"github.com/anishmah100/agent_sim/engine/internal/world/rulebook"
)

const tickRateHz = 60

func main() {
	flagBundle := flag.String("bundle", "../worlds/eldoria", "world bundle directory")
	flagOutDir := flag.String("out-dir", "", "output dir (default = bundle dir)")
	flag.Parse()

	w, b, err := world.LoadBundle(*flagBundle)
	if err != nil {
		log.Fatalf("load bundle %s: %v", *flagBundle, err)
	}

	// Install the scenario to populate the manifest. We don't tick the
	// world — just need the system declarations.
	var host *world.SystemHost
	switch b.ScenarioPkg {
	case "fantasy_town":
		host = fantasy_town.Install(w)
	default:
		log.Fatalf("unknown scenario package: %q", b.ScenarioPkg)
	}
	man := host.Aggregator.Build()

	rb := rulebook.Build(rulebook.BuildInput{
		WorldName:   b.Name,
		DisplayName: b.DisplayName,
		Description: b.Description,
		ScenarioPkg: b.ScenarioPkg,
		MapID:       w.MapID,
		WidthTiles:  w.WidthTiles,
		HeightTiles: w.HeightTiles,
		TickRateHz:  tickRateHz,
		RuleSet:     w.Rules,
		Manifest:    man,
	})

	outDir := *flagOutDir
	if outDir == "" {
		outDir = *flagBundle
	}
	mdPath := filepath.Join(outDir, "RULEBOOK.md")
	jsonPath := filepath.Join(outDir, "rulebook.json")

	if err := os.WriteFile(mdPath, []byte(rulebook.RenderMarkdown(rb)), 0o644); err != nil {
		log.Fatalf("write %s: %v", mdPath, err)
	}
	enc, _ := json.MarshalIndent(rb, "", "  ")
	if err := os.WriteFile(jsonPath, enc, 0o644); err != nil {
		log.Fatalf("write %s: %v", jsonPath, err)
	}
	fmt.Printf("wrote %s (%d verbs, %d items, %d stats, %d tunings)\n",
		mdPath, len(rb.Verbs), len(rb.Items), len(rb.Stats), len(rb.Tunings))
}
