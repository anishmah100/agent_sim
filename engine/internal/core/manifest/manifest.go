// Package manifest defines the affordance manifest structs.
//
// Locked by docs/AFFORDANCE_MANIFEST.md.
//
// Engine aggregates per-system contributions into one Manifest; serves
// it at GET /api/v1/world/affordances. Bots fetch at register; UI
// reads at boot to render the World Rulebook page. Single source of
// truth.
package manifest

import "encoding/json"

type Manifest struct {
	World         string                `json:"world"`
	Scenario      string                `json:"scenario"`
	SchemaVersion int                   `json:"schema_version"`
	Systems       []SystemDeclaration   `json:"systems"`
}

type SystemDeclaration struct {
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Verbs         []VerbDeclaration     `json:"verbs"`
	StateFields   []StateFieldDecl      `json:"state_fields"`
	SoundsEmitted []SoundDecl           `json:"sounds_emitted"`
	Archetypes    []ArchetypeDecl       `json:"archetypes"`
}

type VerbDeclaration struct {
	Verb             string          `json:"verb"`
	Description      string          `json:"description"`
	ParamsSchema     json.RawMessage `json:"params_schema"`
	Preconditions    []string        `json:"preconditions"`
	RejectionReasons []string        `json:"rejection_reasons"`
	EmitsEvents      []string        `json:"emits_events,omitempty"`
	Examples         []VerbExample   `json:"examples"`
}

type VerbExample struct {
	Params json.RawMessage `json:"params"`
	Result string          `json:"result"`
}

type StateFieldDecl struct {
	Key                   string `json:"key"`
	Type                  string `json:"type"`
	Owner                 string `json:"owner"`
	PublicAtAnyDistance   bool   `json:"public_at_any_distance"`
	PublicWithinDistance  int    `json:"public_within_distance,omitempty"`
	Meaning               string `json:"meaning"`
}

type SoundDecl struct {
	Kind        string `json:"kind"`
	Description string `json:"description"`
	EmittedBy   string `json:"emitted_by"`
}

type ArchetypeDecl struct {
	Archetype       string          `json:"archetype"`
	Description     string          `json:"description"`
	DefaultExtras   json.RawMessage `json:"default_extras,omitempty"`
	DefaultVerbsUsed []string       `json:"default_verbs_used,omitempty"`
}

// Aggregator builds a Manifest from per-system contributions.
type Aggregator struct {
	world    string
	scenario string
	systems  []SystemDeclaration
}

func NewAggregator(world, scenario string) *Aggregator {
	return &Aggregator{world: world, scenario: scenario}
}

func (a *Aggregator) Add(decl SystemDeclaration) {
	a.systems = append(a.systems, decl)
}

func (a *Aggregator) Build() Manifest {
	return Manifest{
		World:         a.world,
		Scenario:      a.scenario,
		SchemaVersion: 1,
		Systems:       a.systems,
	}
}
