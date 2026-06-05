package rulebook

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/world/rules"
)

const sampleRulesStar = `
register_tuning("attack_damage", 10)
register_tuning("starting_gold", 25)

register_stat({
    "key":         "hunger",
    "kind":        "float",
    "min":         0.0,
    "max":         1.0,
    "default":     0.0,
    "description": "0=sated 1=starving",
})

register_item({
    "id":    "apple",
    "kind":  "food",
    "props": {"satiety": 0.25},
})

def noop(state, actor, args):
    return True

register_verb({
    "name":    "dominate",
    "precond": noop,
    "effect":  noop,
})
`

func TestBuild_FromMinimal(t *testing.T) {
	rs, err := rules.LoadStarlarkString("sample.star", sampleRulesStar)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	agg := manifest.NewAggregator("eldoria", "fantasy_town")
	agg.Add(manifest.SystemDeclaration{
		Name: "combat",
		Verbs: []manifest.VerbDeclaration{
			{Verb: "attack", Description: "Hit an adjacent target."},
		},
		Archetypes: []manifest.ArchetypeDecl{
			{Archetype: "goblin", Description: "Hostile mob."},
		},
	})
	man := agg.Build()

	rb := Build(BuildInput{
		WorldName:   "eldoria",
		DisplayName: "Eldoria",
		Description: "fantasy continent",
		ScenarioPkg: "fantasy_town",
		MapID:       "eldoria",
		WidthTiles:  1500,
		HeightTiles: 1500,
		TickRateHz:  60,
		RuleSet:     rs,
		Manifest:    man,
	})

	if rb.SchemaVersion != 1 {
		t.Fatalf("schema_version: got %d", rb.SchemaVersion)
	}
	if rb.World.Name != "eldoria" || rb.World.DisplayName != "Eldoria" {
		t.Fatalf("world info: %+v", rb.World)
	}
	if rb.Time.TickRateHz != 60 {
		t.Fatalf("tick rate: %d", rb.Time.TickRateHz)
	}
	if rb.Map.WidthTiles != 1500 {
		t.Fatalf("map width: %d", rb.Map.WidthTiles)
	}
	if len(rb.Stats) != 1 || rb.Stats[0].Key != "hunger" {
		t.Fatalf("stats: %+v", rb.Stats)
	}
	if len(rb.Tunings) != 2 {
		t.Fatalf("tunings: %+v", rb.Tunings)
	}
	if len(rb.Items) != 1 || rb.Items[0].ID != "apple" {
		t.Fatalf("items: %+v", rb.Items)
	}
	if len(rb.Verbs) != 2 {
		t.Fatalf("verbs: want 2 (attack common + dominate novel); got %d %+v", len(rb.Verbs), rb.Verbs)
	}
	// Verbs sorted by category: common before novel.
	if rb.Verbs[0].Name != "attack" || rb.Verbs[0].Category != "common" {
		t.Fatalf("first verb: want attack/common, got %+v", rb.Verbs[0])
	}
	if rb.Verbs[1].Name != "dominate" || rb.Verbs[1].Category != "novel" {
		t.Fatalf("second verb: want dominate/novel, got %+v", rb.Verbs[1])
	}
	if len(rb.NPCs) != 1 || rb.NPCs[0].Name != "goblin" {
		t.Fatalf("npcs: %+v", rb.NPCs)
	}
}

func TestBuild_DeterministicOrdering(t *testing.T) {
	// Build the same rulebook twice; the rendered MD should be byte-identical.
	rs, _ := rules.LoadStarlarkString("d.star", sampleRulesStar)
	agg := manifest.NewAggregator("eldoria", "fantasy_town")
	agg.Add(manifest.SystemDeclaration{
		Name: "combat",
		Verbs: []manifest.VerbDeclaration{
			{Verb: "attack"},
		},
	})
	man := agg.Build()

	in := BuildInput{
		WorldName: "eldoria", MapID: "eldoria",
		TickRateHz: 60, RuleSet: rs, Manifest: man,
	}
	md1 := RenderMarkdown(Build(in))
	md2 := RenderMarkdown(Build(in))
	if md1 != md2 {
		t.Fatal("repeated Build/Render produced different output — ordering not deterministic")
	}
}

func TestRenderMarkdown_SectionHeaders(t *testing.T) {
	rs, _ := rules.LoadStarlarkString("h.star", sampleRulesStar)
	rb := Build(BuildInput{
		WorldName:   "eldoria",
		DisplayName: "Eldoria",
		TickRateHz:  60,
		RuleSet:     rs,
	})
	md := RenderMarkdown(rb)
	for _, h := range []string{
		"# Eldoria — Rulebook",
		"## 1. Overview",
		"## 2. Time",
		"## 3. Map",
		"## 4. Stats",
		"## 5. Items",
		"## 6. Verbs",
		"## 7. NPC Archetypes",
		"## 8. Tunings",
		"## 9. Quirks",
	} {
		if !strings.Contains(md, h) {
			t.Errorf("RenderMarkdown missing section header %q", h)
		}
	}
}

func TestBuild_NilRuleSet(t *testing.T) {
	// Legacy bundles without rules.star — Build should still produce a
	// usable Rulebook with empty stats/items/tunings sections.
	rb := Build(BuildInput{
		WorldName: "dev_test", MapID: "dev_test",
		TickRateHz: 60,
	})
	if len(rb.Stats) != 0 || len(rb.Items) != 0 || len(rb.Tunings) != 0 {
		t.Fatalf("nil ruleset should yield empty sections")
	}
	// JSON serializes cleanly.
	if _, err := json.Marshal(rb); err != nil {
		t.Fatalf("marshal: %v", err)
	}
}
