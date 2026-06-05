// Package rulebook builds the per-world Rulebook from the bundle, the
// loaded Starlark RuleSet, and the engine manifest. The Rulebook is
// the public contract a bot reads on registration (zero-shot transfer)
// and the human-readable doc that lives committed at
// worlds/<name>/RULEBOOK.md.
//
// One source-of-truth assembly: bundle.toml (name + description),
// rules.star (tunings + items + stats + novel verbs), engine manifest
// (system verbs + archetypes). The Rulebook struct is the single
// type that powers both rendering paths.
package rulebook

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/world/rules"
)

// Rulebook is the public per-world contract. Sections follow the
// fixed table-of-contents declared in docs/AGENT_ARCHITECTURE_PLAN.md.
type Rulebook struct {
	SchemaVersion int        `json:"schema_version"`
	World         WorldInfo  `json:"world"`
	Time          TimeInfo   `json:"time"`
	Map           MapInfo    `json:"map"`
	Stats         []Stat     `json:"stats"`
	Tunings       []Tuning   `json:"tunings"`
	Items         []Item     `json:"items"`
	Verbs         []Verb     `json:"verbs"`
	NPCs          []Archetype `json:"npc_archetypes"`
	Quirks        []string   `json:"quirks"`
}

type WorldInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Scenario    string `json:"scenario"`
}

type TimeInfo struct {
	TickRateHz int `json:"tick_rate_hz"`
}

type MapInfo struct {
	WidthTiles  int    `json:"width_tiles"`
	HeightTiles int    `json:"height_tiles"`
	MapID       string `json:"map_id"`
}

type Stat struct {
	Key         string  `json:"key"`
	Kind        string  `json:"kind"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	Default     float64 `json:"default"`
	Description string  `json:"description"`
}

type Tuning struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type Item struct {
	ID    string                 `json:"id"`
	Kind  string                 `json:"kind"`
	Props map[string]interface{} `json:"props,omitempty"`
}

type Verb struct {
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	System           string   `json:"system,omitempty"`
	Description      string   `json:"description,omitempty"`
	Preconditions    []string `json:"preconditions,omitempty"`
	RejectionReasons []string `json:"rejection_reasons,omitempty"`
	EmitsEvents      []string `json:"emits_events,omitempty"`
}

type Archetype struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// BuildInput is the source-of-truth bundle passed into Build.
type BuildInput struct {
	WorldName        string
	DisplayName      string
	Description      string
	ScenarioPkg      string
	MapID            string
	WidthTiles       int
	HeightTiles      int
	TickRateHz       int
	RuleSet          *rules.RuleSet
	Manifest         manifest.Manifest
}

// Build assembles the Rulebook from the parsed bundle + ruleset +
// engine manifest. Deterministic ordering: alphabetical within each
// section so diffs across runs are stable.
func Build(in BuildInput) Rulebook {
	rb := Rulebook{
		SchemaVersion: 1,
		World: WorldInfo{
			Name:        in.WorldName,
			DisplayName: in.DisplayName,
			Description: in.Description,
			Scenario:    in.ScenarioPkg,
		},
		Time: TimeInfo{TickRateHz: in.TickRateHz},
		Map: MapInfo{
			WidthTiles:  in.WidthTiles,
			HeightTiles: in.HeightTiles,
			MapID:       in.MapID,
		},
	}

	// Stats (from RuleSet).
	if in.RuleSet != nil {
		for _, k := range sortedStrings(in.RuleSet.StatKeys()) {
			s, ok := in.RuleSet.Stat(k)
			if !ok {
				continue
			}
			rb.Stats = append(rb.Stats, Stat{
				Key:         s.Key,
				Kind:        s.Kind,
				Min:         s.Min,
				Max:         s.Max,
				Default:     s.Default,
				Description: s.Description,
			})
		}
		// Tunings.
		for _, name := range sortedStrings(in.RuleSet.TuningNames()) {
			rb.Tunings = append(rb.Tunings, Tuning{
				Name:  name,
				Value: in.RuleSet.GetFloat(name, 0),
			})
		}
		// Items.
		for _, id := range sortedStrings(in.RuleSet.ItemIDs()) {
			it, ok := in.RuleSet.Item(id)
			if !ok {
				continue
			}
			rb.Items = append(rb.Items, Item{
				ID:    it.ID,
				Kind:  it.Kind,
				Props: it.Props,
			})
		}
		// Quirks — currently just the list of novel verbs declared in
		// rules.star. As novel verbs gain richer metadata in future
		// phases this section grows.
		for _, name := range sortedStrings(in.RuleSet.VerbNames()) {
			rb.Quirks = append(rb.Quirks, "novel verb: "+name)
		}
	}

	// Verbs (from engine manifest). Engine + common-library verbs.
	type seen struct{ system string }
	verbSet := map[string]seen{}
	for _, sys := range in.Manifest.Systems {
		for _, v := range sys.Verbs {
			verbSet[v.Verb] = seen{system: sys.Name}
			rb.Verbs = append(rb.Verbs, Verb{
				Name:             v.Verb,
				Category:         orDefault(v.Category, manifest.VerbCategoryCommon),
				System:           sys.Name,
				Description:      v.Description,
				Preconditions:    v.Preconditions,
				RejectionReasons: v.RejectionReasons,
				EmitsEvents:      v.EmitsEvents,
			})
		}
		for _, a := range sys.Archetypes {
			rb.NPCs = append(rb.NPCs, Archetype{
				Name:        a.Archetype,
				Description: a.Description,
			})
		}
	}
	// Append novel verbs (from rules.star) that weren't already in the
	// engine manifest. They land with Category = "novel".
	if in.RuleSet != nil {
		for _, name := range sortedStrings(in.RuleSet.VerbNames()) {
			if _, ok := verbSet[name]; ok {
				continue
			}
			rb.Verbs = append(rb.Verbs, Verb{
				Name:     name,
				Category: manifest.VerbCategoryNovel,
				System:   "rules.star",
			})
		}
	}
	// Stable sort on verbs by (category, name) so diff is clean.
	sort.SliceStable(rb.Verbs, func(i, j int) bool {
		if rb.Verbs[i].Category != rb.Verbs[j].Category {
			return categoryOrder(rb.Verbs[i].Category) < categoryOrder(rb.Verbs[j].Category)
		}
		return rb.Verbs[i].Name < rb.Verbs[j].Name
	})
	sort.SliceStable(rb.NPCs, func(i, j int) bool {
		return rb.NPCs[i].Name < rb.NPCs[j].Name
	})
	return rb
}

// RenderMarkdown turns a Rulebook into a stable human-readable RULEBOOK.md.
// Sections follow the fixed table-of-contents.
func RenderMarkdown(rb Rulebook) string {
	var b strings.Builder
	w := &b

	fmt.Fprintf(w, "# %s — Rulebook\n\n", coalesce(rb.World.DisplayName, rb.World.Name))
	if rb.World.Description != "" {
		fmt.Fprintf(w, "%s\n\n", rb.World.Description)
	}
	fmt.Fprintf(w, "*Auto-generated from `worlds/%s/{bundle.toml, rules.star}` + the engine manifest. Do not edit by hand.*\n\n", rb.World.Name)
	fmt.Fprintln(w, "---")

	fmt.Fprintln(w, "\n## 1. Overview")
	fmt.Fprintf(w, "- Scenario: `%s`\n", rb.World.Scenario)
	fmt.Fprintf(w, "- Schema version: %d\n", rb.SchemaVersion)

	fmt.Fprintln(w, "\n## 2. Time")
	fmt.Fprintf(w, "- Tick rate: %d Hz\n", rb.Time.TickRateHz)

	fmt.Fprintln(w, "\n## 3. Map")
	fmt.Fprintf(w, "- Dimensions: %d × %d tiles\n", rb.Map.WidthTiles, rb.Map.HeightTiles)
	fmt.Fprintf(w, "- Map id: `%s`\n", rb.Map.MapID)

	fmt.Fprintln(w, "\n## 4. Stats")
	if len(rb.Stats) == 0 {
		fmt.Fprintln(w, "_None declared._")
	} else {
		fmt.Fprintln(w, "| Key | Kind | Range | Default | Meaning |")
		fmt.Fprintln(w, "|---|---|---|---|---|")
		for _, s := range rb.Stats {
			fmt.Fprintf(w, "| `%s` | %s | %g–%g | %g | %s |\n",
				s.Key, s.Kind, s.Min, s.Max, s.Default, s.Description)
		}
	}

	fmt.Fprintln(w, "\n## 5. Items")
	if len(rb.Items) == 0 {
		fmt.Fprintln(w, "_None declared._")
	} else {
		fmt.Fprintln(w, "| ID | Kind | Props |")
		fmt.Fprintln(w, "|---|---|---|")
		for _, it := range rb.Items {
			fmt.Fprintf(w, "| `%s` | %s | %s |\n",
				it.ID, it.Kind, propsString(it.Props))
		}
	}

	fmt.Fprintln(w, "\n## 6. Verbs")
	if len(rb.Verbs) == 0 {
		fmt.Fprintln(w, "_None declared._")
	} else {
		fmt.Fprintln(w, "| Name | Category | System | Description |")
		fmt.Fprintln(w, "|---|---|---|---|")
		for _, v := range rb.Verbs {
			fmt.Fprintf(w, "| `%s` | %s | `%s` | %s |\n",
				v.Name, v.Category, v.System, v.Description)
		}
	}

	fmt.Fprintln(w, "\n## 7. NPC Archetypes")
	if len(rb.NPCs) == 0 {
		fmt.Fprintln(w, "_None declared._")
	} else {
		for _, a := range rb.NPCs {
			fmt.Fprintf(w, "- `%s` — %s\n", a.Name, a.Description)
		}
	}

	fmt.Fprintln(w, "\n## 8. Tunings")
	if len(rb.Tunings) == 0 {
		fmt.Fprintln(w, "_None declared._")
	} else {
		fmt.Fprintln(w, "| Name | Value |")
		fmt.Fprintln(w, "|---|---|")
		for _, t := range rb.Tunings {
			fmt.Fprintf(w, "| `%s` | %g |\n", t.Name, t.Value)
		}
	}

	fmt.Fprintln(w, "\n## 9. Quirks")
	if len(rb.Quirks) == 0 {
		fmt.Fprintln(w, "_None declared._")
	} else {
		for _, q := range rb.Quirks {
			fmt.Fprintf(w, "- %s\n", q)
		}
	}
	return b.String()
}

// --- helpers ---

func sortedStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}

func categoryOrder(cat string) int {
	switch cat {
	case manifest.VerbCategoryCore:
		return 0
	case manifest.VerbCategoryCommon:
		return 1
	case manifest.VerbCategoryNovel:
		return 2
	}
	return 3
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func propsString(p map[string]interface{}) string {
	if len(p) == 0 {
		return ""
	}
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, p[k]))
	}
	return strings.Join(parts, ", ")
}
