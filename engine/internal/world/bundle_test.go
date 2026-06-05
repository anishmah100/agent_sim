package world

import (
	"path/filepath"
	"testing"
)

func TestReadBundle_Eldoria(t *testing.T) {
	// Bundle lives at repo-root/worlds/eldoria/.
	// Tests run from the package dir, so resolve via relative path.
	dir := filepath.Join("..", "..", "..", "worlds", "eldoria")
	b, err := ReadBundle(dir)
	if err != nil {
		t.Fatalf("ReadBundle(%s): %v", dir, err)
	}
	if b.Name != "eldoria" {
		t.Fatalf("bundle.name: want eldoria, got %q", b.Name)
	}
	if b.DisplayName == "" {
		t.Fatal("bundle.display_name should not be empty")
	}
	if b.WorldFile != "world.json" {
		t.Fatalf("world.file: want world.json, got %q", b.WorldFile)
	}
	if b.ScenarioPkg != "fantasy_town" {
		t.Fatalf("scenario.pkg: want fantasy_town, got %q", b.ScenarioPkg)
	}
	if b.NPCsConfigPath() != filepath.Join(dir, "npcs.json") {
		t.Fatalf("NPCsConfigPath(): got %q", b.NPCsConfigPath())
	}
}

func TestReadBundle_Missing(t *testing.T) {
	_, err := ReadBundle("/nonexistent/bundle/path")
	if err == nil {
		t.Fatal("ReadBundle should error on missing dir")
	}
}

func TestLoadBundle_DevTest(t *testing.T) {
	// dev_test is the smallest world; loads fast.
	dir := filepath.Join("..", "..", "..", "worlds", "dev_test")
	w, b, err := LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle(%s): %v", dir, err)
	}
	if w == nil {
		t.Fatal("nil World")
	}
	if b == nil {
		t.Fatal("nil Bundle")
	}
	if w.MapID == "" {
		t.Fatal("world has empty MapID")
	}
	// dev_test is 60×40 per the bundle.toml description.
	if w.WidthTiles != 60 || w.HeightTiles != 40 {
		t.Fatalf("dev_test dims: want 60×40, got %d×%d", w.WidthTiles, w.HeightTiles)
	}
}

func TestLoadBundle_EldoriaCarriesRules(t *testing.T) {
	// Eldoria's bundle declares [rules.file]; LoadBundle should parse it
	// and attach a RuleSet to the World.
	dir := filepath.Join("..", "..", "..", "worlds", "eldoria")
	w, _, err := LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	if w.Rules == nil {
		t.Fatal("eldoria bundle declared rules.file but World.Rules is nil")
	}
	// Spot-check a tuning that eldoria's rules.star declares.
	if got := w.Rules.GetInt("attack_damage", -1); got != 10 {
		t.Fatalf("eldoria attack_damage: want 10, got %d", got)
	}
	if got := w.Rules.GetFloat("hunger_per_tick", -1); got != 0.0008 {
		t.Fatalf("eldoria hunger_per_tick: want 0.0008, got %v", got)
	}
	// And an item.
	if _, ok := w.Rules.Item("apple"); !ok {
		t.Fatal("eldoria should declare item 'apple'")
	}
}

func TestLoadBundle_DevTestHasNoRules(t *testing.T) {
	// dev_test bundle does NOT declare [rules.file] — World.Rules
	// should be nil and defaults should apply.
	dir := filepath.Join("..", "..", "..", "worlds", "dev_test")
	w, _, err := LoadBundle(dir)
	if err != nil {
		t.Fatalf("LoadBundle: %v", err)
	}
	if w.Rules != nil {
		t.Fatalf("dev_test bundle has no [rules]; expected nil RuleSet, got %v", w.Rules)
	}
	// nil-safe getter still returns the supplied default.
	if got := w.Rules.GetInt("attack_damage", 42); got != 42 {
		t.Fatalf("nil-safe default: want 42, got %d", got)
	}
}

func TestReadBundle_SchemaCheck(t *testing.T) {
	// Read all four bundles — none should fail the schema check.
	for _, name := range []string{"eldoria", "dev_test", "dev_wilderness", "soak_1000x1000"} {
		dir := filepath.Join("..", "..", "..", "worlds", name)
		b, err := ReadBundle(dir)
		if err != nil {
			t.Errorf("ReadBundle(%s): %v", name, err)
			continue
		}
		if b.Schema != "agent_sim/bundle/v1" {
			t.Errorf("%s: schema %q is not v1", name, b.Schema)
		}
		if b.Name != name {
			t.Errorf("%s: bundle.name mismatch — want %q, got %q", name, name, b.Name)
		}
	}
}
