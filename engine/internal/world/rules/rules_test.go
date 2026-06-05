package rules

import (
	"testing"
)

const minimalRuleset = `
register_tuning("hunger_per_tick", 0.001)
register_tuning("attack_damage",   10)
register_tuning("starting_gold",   25)
register_tuning("can_pickpocket",  True)

register_item({
    "id":    "apple",
    "kind":  "food",
    "props": {"satiety": 0.2, "weight": 0.1},
})

register_item({
    "id":    "iron_sword",
    "kind":  "weapon",
    "props": {"damage": 15, "two_handed": False},
})

def dominate_precond(state, actor, target):
    return True

def dominate_effect(state, actor, target):
    pass

register_verb({
    "name":    "dominate",
    "precond": dominate_precond,
    "effect":  dominate_effect,
})
`

func TestLoadStarlark_Tunings(t *testing.T) {
	rs, err := LoadStarlarkString("minimal.star", minimalRuleset)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := rs.GetFloat("hunger_per_tick", 0); got != 0.001 {
		t.Fatalf("hunger_per_tick: want 0.001, got %v", got)
	}
	if got := rs.GetInt("attack_damage", 0); got != 10 {
		t.Fatalf("attack_damage: want 10, got %v", got)
	}
	if got := rs.GetInt("starting_gold", 0); got != 25 {
		t.Fatalf("starting_gold: want 25, got %v", got)
	}
	if got := rs.GetBool("can_pickpocket", false); !got {
		t.Fatal("can_pickpocket: want true")
	}
}

func TestLoadStarlark_Defaults(t *testing.T) {
	rs, _ := LoadStarlarkString("empty.star", "")
	// Missing keys fall back to the supplied default.
	if got := rs.GetFloat("not_a_tuning", 7.5); got != 7.5 {
		t.Fatalf("missing tuning should return default 7.5, got %v", got)
	}
	if got := rs.GetInt("not_a_tuning", 99); got != 99 {
		t.Fatalf("missing tuning should return default 99, got %v", got)
	}
	if rs.HasTuning("not_a_tuning") {
		t.Fatal("HasTuning should be false for missing key")
	}
	if rs.HasTuning("never_declared") {
		t.Fatal("never-declared tunings should not exist")
	}
}

func TestLoadStarlark_Items(t *testing.T) {
	rs, err := LoadStarlarkString("items.star", minimalRuleset)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	apple, ok := rs.Item("apple")
	if !ok {
		t.Fatal("apple not registered")
	}
	if apple.Kind != "food" {
		t.Fatalf("apple.Kind: want food, got %q", apple.Kind)
	}
	if got, _ := apple.Props["satiety"].(float64); got != 0.2 {
		t.Fatalf("apple.satiety: want 0.2, got %v", apple.Props["satiety"])
	}
	if got, _ := apple.Props["weight"].(float64); got != 0.1 {
		t.Fatalf("apple.weight: want 0.1, got %v", apple.Props["weight"])
	}

	sword, ok := rs.Item("iron_sword")
	if !ok {
		t.Fatal("iron_sword not registered")
	}
	if got, _ := sword.Props["damage"].(int64); got != 15 {
		t.Fatalf("iron_sword.damage: want 15, got %v", sword.Props["damage"])
	}
	if got, _ := sword.Props["two_handed"].(bool); got {
		t.Fatal("iron_sword.two_handed: want false")
	}
}

func TestLoadStarlark_Verbs(t *testing.T) {
	rs, err := LoadStarlarkString("verbs.star", minimalRuleset)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	v, ok := rs.Verb("dominate")
	if !ok {
		t.Fatal("dominate verb not registered")
	}
	if v.Name != "dominate" {
		t.Fatalf("verb name: want dominate, got %q", v.Name)
	}
	if v.Predicate == nil {
		t.Fatal("verb.Predicate is nil — Starlark callable missing")
	}
	if v.Effect == nil {
		t.Fatal("verb.Effect is nil — Starlark callable missing")
	}
}

func TestLoadStarlark_ItemIDsAndVerbNames(t *testing.T) {
	rs, _ := LoadStarlarkString("listing.star", minimalRuleset)
	if got := len(rs.ItemIDs()); got != 2 {
		t.Fatalf("ItemIDs: want 2, got %d", got)
	}
	if got := len(rs.VerbNames()); got != 1 {
		t.Fatalf("VerbNames: want 1, got %d", got)
	}
}

func TestLoadStarlark_HermeticDeterministic(t *testing.T) {
	// Starlark is supposed to be deterministic — random/time/io must not
	// be reachable from a ruleset.
	const naughty = `
		x = time.now()
	`
	_, err := LoadStarlarkString("naughty.star", naughty)
	if err == nil {
		t.Fatal("expected error: time module should not be importable from a ruleset")
	}
}

func TestLoadStarlark_Stats(t *testing.T) {
	const src = `
register_stat({
    "key":         "hunger",
    "kind":        "float",
    "min":         0.0,
    "max":         1.0,
    "default":     0.0,
    "description": "0=sated 1=starving",
})

register_stat({
    "key":         "hp",
    "kind":        "int",
    "min":         0,
    "max":         100,
    "default":     100,
    "description": "Hit points.",
})
`
	rs, err := LoadStarlarkString("stats.star", src)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	h, ok := rs.Stat("hunger")
	if !ok {
		t.Fatal("hunger not registered")
	}
	if h.Kind != "float" || h.Max != 1.0 {
		t.Fatalf("hunger fields: %+v", h)
	}
	hp, ok := rs.Stat("hp")
	if !ok {
		t.Fatal("hp not registered")
	}
	if hp.Kind != "int" || hp.Default != 100 {
		t.Fatalf("hp fields: %+v", hp)
	}
	if got := len(rs.StatKeys()); got != 2 {
		t.Fatalf("StatKeys: want 2, got %d", got)
	}
}

func TestRuleSet_NilSafe(t *testing.T) {
	// Engine code may legitimately have a nil RuleSet (no rules.star
	// declared) — every getter must tolerate that.
	var rs *RuleSet
	if got := rs.GetFloat("x", 1.5); got != 1.5 {
		t.Fatalf("nil GetFloat: want default, got %v", got)
	}
	if got := rs.GetInt("x", 3); got != 3 {
		t.Fatalf("nil GetInt: want default, got %v", got)
	}
	if rs.HasTuning("x") {
		t.Fatal("nil HasTuning: want false")
	}
	if rs.TuningNames() != nil {
		t.Fatal("nil TuningNames: want nil slice")
	}
	if _, ok := rs.Item("apple"); ok {
		t.Fatal("nil Item: want not ok")
	}
}
