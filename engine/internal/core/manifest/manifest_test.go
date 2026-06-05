package manifest

import (
	"encoding/json"
	"testing"
)

func TestAggregator_DefaultCategoryIsCommon(t *testing.T) {
	a := NewAggregator("eldoria", "fantasy_town")
	a.Add(SystemDeclaration{
		Name: "test_system",
		Verbs: []VerbDeclaration{
			{Verb: "attack" /* Category intentionally empty */},
			{Verb: "look_at", Category: VerbCategoryCore},
		},
	})
	m := a.Build()
	if len(m.Systems) != 1 || len(m.Systems[0].Verbs) != 2 {
		t.Fatalf("aggregator should preserve verbs; got %v", m.Systems)
	}
	if got := m.Systems[0].Verbs[0].Category; got != VerbCategoryCommon {
		t.Fatalf("unset verb category should default to %q, got %q", VerbCategoryCommon, got)
	}
	if got := m.Systems[0].Verbs[1].Category; got != VerbCategoryCore {
		t.Fatalf("explicit Core category should survive; got %q", got)
	}
}

func TestAggregator_CategorySerializes(t *testing.T) {
	a := NewAggregator("eldoria", "fantasy_town")
	a.Add(SystemDeclaration{
		Name: "test_system",
		Verbs: []VerbDeclaration{
			{Verb: "attack"},
		},
	})
	m := a.Build()
	blob, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Default value lands in JSON as "common".
	if want := `"category":"common"`; !contains(string(blob), want) {
		t.Fatalf("manifest JSON missing %q; got %s", want, string(blob))
	}
}

func TestVerbCategories_AreStable(t *testing.T) {
	// Frontend + SDK depend on these literal strings; freezing them
	// here prevents accidental rename.
	if VerbCategoryCore != "core" {
		t.Fatalf("VerbCategoryCore renamed; was 'core', now %q", VerbCategoryCore)
	}
	if VerbCategoryCommon != "common" {
		t.Fatalf("VerbCategoryCommon renamed; was 'common', now %q", VerbCategoryCommon)
	}
	if VerbCategoryNovel != "novel" {
		t.Fatalf("VerbCategoryNovel renamed; was 'novel', now %q", VerbCategoryNovel)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
