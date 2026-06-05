package wire

import (
	"encoding/json"
	"net/http"

	"github.com/anishmah100/agent_sim/engine/internal/world"
	"github.com/anishmah100/agent_sim/engine/internal/world/rulebook"
)

// RulebookHandler serves the per-world Rulebook JSON at
// /api/v1/world/rulebook.json. The rulebook is the public zero-shot
// contract a bot reads on registration; the same data renders into
// worlds/<name>/RULEBOOK.md via cmd/genrulebook.
//
// Built at request time so it always reflects the live bundle +
// ruleset + manifest. The bundle is small (~20 KB JSON) so caching
// is fine in nginx / fastly above; engine returns Cache-Control max-age.
func RulebookHandler(w *world.World, b *world.Bundle, host *world.SystemHost, tickRateHz int) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		rw.Header().Set("Cache-Control", "public, max-age=60")
		if w == nil || host == nil || host.Aggregator == nil {
			http.Error(rw, `{"error":"no scenario installed"}`, http.StatusServiceUnavailable)
			return
		}
		in := rulebook.BuildInput{
			MapID:       w.MapID,
			WidthTiles:  w.WidthTiles,
			HeightTiles: w.HeightTiles,
			TickRateHz:  tickRateHz,
			RuleSet:     w.Rules,
			Manifest:    host.Aggregator.Build(),
		}
		if b != nil {
			in.WorldName = b.Name
			in.DisplayName = b.DisplayName
			in.Description = b.Description
			in.ScenarioPkg = b.ScenarioPkg
		}
		rb := rulebook.Build(in)
		enc := json.NewEncoder(rw)
		enc.SetIndent("", "  ")
		_ = enc.Encode(rb)
	}
}
