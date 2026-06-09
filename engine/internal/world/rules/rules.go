// Package rules loads a world's declarative ruleset from a Starlark
// file (typically worlds/<name>/rules.star).
//
// A ruleset declares:
//   - Tunings   — scalar world parameters (hunger_per_tick, attack_damage…)
//   - Items     — declarative item definitions (id, kind, props)
//   - Verbs     — predicate + effect for novel verbs (Phase 5 wires
//     these into the engine's verb registry; for now the
//     loader just stores them as opaque Starlark callables).
//
// Starlark was picked over Lua / custom YAML because it's:
//   - hermetic by design (no I/O, no random, no clocks → deterministic
//     experiments),
//   - Python-syntax (researchers' instinct),
//   - Go-native via go.starlark.net (no cgo),
//   - sandboxed by default.
//
// Phase WORLD-2 ships the loader + a tiny query API; existing systems
// keep using their hard-coded constants. Phase SUB-5/6 refactor those
// to read from RuleSet.GetFloat(...) etc.
package rules

import (
	"fmt"

	"go.starlark.net/starlark"
)

// RuleSet is the parsed, queryable view of a worlds/<name>/rules.star.
// All field maps are nil if not declared in the .star file — callers
// must use the helper methods, which return defaults for missing keys.
type RuleSet struct {
	// Path the ruleset was loaded from (for error messages).
	SourcePath string

	tunings map[string]starlark.Value
	items   map[string]*ItemDef
	verbs   map[string]*VerbDef
	stats   map[string]*StatDef
}

// ItemDef is the declarative shape of an item declared in rules.star.
// Phase 5 will hand these to the inventory system; for now the loader
// just stores them.
type ItemDef struct {
	ID    string
	Kind  string                 // "weapon" / "food" / "key" / …
	Props map[string]interface{} // free-form per-kind props
}

// VerbDef is a novel verb's predicate + effect, kept opaque until
// Phase 5 wires it into the verb registry.
type VerbDef struct {
	Name      string
	Predicate starlark.Callable // (state, actor, args) -> bool
	Effect    starlark.Callable // (state, actor, args) -> None
}

// StatDef declares a per-entity stat tracked by the world. Stats are
// stored in Entity.Extras under StatDef.Key by the engine systems that
// own them; this declaration drives the rulebook (so an agent knows
// "this world tracks hunger" without scraping code).
type StatDef struct {
	Key         string  // extras key — e.g. "hp", "gold", "hunger"
	Kind        string  // "int" | "float" | "bool"
	Min         float64 // soft lower bound (display + clamp)
	Max         float64 // soft upper bound
	Default     float64 // value at spawn
	Description string  // human-readable; appears in rulebook
}

// LoadStarlark reads + evaluates a rules.star file. The top-level
// script can call register_tuning, register_item, register_verb to
// populate the ruleset.
//
// Example rules.star:
//
//	register_tuning("hunger_per_tick", 0.001)
//	register_tuning("attack_damage",   10)
//	register_tuning("starting_gold",   25)
//
//	register_item({
//	    "id": "apple",
//	    "kind": "food",
//	    "props": {"satiety": 0.2},
//	})
func LoadStarlark(path string) (*RuleSet, error) {
	rs := &RuleSet{
		SourcePath: path,
		tunings:    map[string]starlark.Value{},
		items:      map[string]*ItemDef{},
		verbs:      map[string]*VerbDef{},
		stats:      map[string]*StatDef{},
	}

	thread := &starlark.Thread{
		Name: "rules-load",
		Print: func(_ *starlark.Thread, msg string) {
			// Starlark print() goes to stderr-ish. We forward as a noop
			// for now; experiments will route this to the trace log.
			_ = msg
		},
	}

	globals := starlark.StringDict{
		"register_tuning": starlark.NewBuiltin("register_tuning", rs.registerTuning),
		"register_item":   starlark.NewBuiltin("register_item", rs.registerItem),
		"register_verb":   starlark.NewBuiltin("register_verb", rs.registerVerb),
		"register_stat":   starlark.NewBuiltin("register_stat", rs.registerStat),
	}

	if _, err := starlark.ExecFile(thread, path, nil, globals); err != nil {
		return nil, fmt.Errorf("rules.star at %s: %w", path, err)
	}
	return rs, nil
}

// LoadStarlarkString is the in-memory variant used by tests.
func LoadStarlarkString(name, src string) (*RuleSet, error) {
	rs := &RuleSet{
		SourcePath: name,
		tunings:    map[string]starlark.Value{},
		items:      map[string]*ItemDef{},
		verbs:      map[string]*VerbDef{},
		stats:      map[string]*StatDef{},
	}
	thread := &starlark.Thread{Name: "rules-load-test"}
	globals := starlark.StringDict{
		"register_tuning": starlark.NewBuiltin("register_tuning", rs.registerTuning),
		"register_item":   starlark.NewBuiltin("register_item", rs.registerItem),
		"register_verb":   starlark.NewBuiltin("register_verb", rs.registerVerb),
		"register_stat":   starlark.NewBuiltin("register_stat", rs.registerStat),
	}
	if _, err := starlark.ExecFile(thread, name, src, globals); err != nil {
		return nil, fmt.Errorf("rules.star (%s): %w", name, err)
	}
	return rs, nil
}

// --- Built-ins exposed to the Starlark script ---

func (rs *RuleSet) registerTuning(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var name string
	var value starlark.Value
	if err := starlark.UnpackPositionalArgs("register_tuning", args, kw, 2, &name, &value); err != nil {
		return nil, err
	}
	rs.tunings[name] = value
	return starlark.None, nil
}

func (rs *RuleSet) registerItem(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var d *starlark.Dict
	if err := starlark.UnpackPositionalArgs("register_item", args, kw, 1, &d); err != nil {
		return nil, err
	}
	id, ok := dictGetString(d, "id")
	if !ok || id == "" {
		return nil, fmt.Errorf("register_item: id is required (string)")
	}
	kind, _ := dictGetString(d, "kind")
	props := map[string]interface{}{}
	if v, found, _ := d.Get(starlark.String("props")); found {
		if pd, ok := v.(*starlark.Dict); ok {
			for _, item := range pd.Items() {
				k, ok := starlark.AsString(item[0])
				if !ok {
					continue
				}
				props[k] = unwrap(item[1])
			}
		}
	}
	rs.items[id] = &ItemDef{ID: id, Kind: kind, Props: props}
	return starlark.None, nil
}

func (rs *RuleSet) registerStat(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var d *starlark.Dict
	if err := starlark.UnpackPositionalArgs("register_stat", args, kw, 1, &d); err != nil {
		return nil, err
	}
	key, ok := dictGetString(d, "key")
	if !ok || key == "" {
		return nil, fmt.Errorf("register_stat: key is required (string)")
	}
	st := &StatDef{Key: key, Kind: "float"}
	if v, _ := dictGetString(d, "kind"); v != "" {
		st.Kind = v
	}
	if v, _ := dictGetString(d, "description"); v != "" {
		st.Description = v
	}
	if v, found, _ := d.Get(starlark.String("min")); found {
		st.Min = asFloat(v)
	}
	if v, found, _ := d.Get(starlark.String("max")); found {
		st.Max = asFloat(v)
	}
	if v, found, _ := d.Get(starlark.String("default")); found {
		st.Default = asFloat(v)
	}
	rs.stats[key] = st
	return starlark.None, nil
}

func (rs *RuleSet) registerVerb(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kw []starlark.Tuple) (starlark.Value, error) {
	var d *starlark.Dict
	if err := starlark.UnpackPositionalArgs("register_verb", args, kw, 1, &d); err != nil {
		return nil, err
	}
	name, ok := dictGetString(d, "name")
	if !ok || name == "" {
		return nil, fmt.Errorf("register_verb: name is required (string)")
	}
	v := &VerbDef{Name: name}
	if val, found, _ := d.Get(starlark.String("precond")); found {
		if c, ok := val.(starlark.Callable); ok {
			v.Predicate = c
		}
	}
	if val, found, _ := d.Get(starlark.String("effect")); found {
		if c, ok := val.(starlark.Callable); ok {
			v.Effect = c
		}
	}
	rs.verbs[name] = v
	return starlark.None, nil
}

// --- Query API for Go callers ---

// GetFloat returns the tuning's value as a float64. Returns the
// supplied default if the tuning is missing or not numeric.
func (rs *RuleSet) GetFloat(name string, defaultValue float64) float64 {
	if rs == nil {
		return defaultValue
	}
	v, ok := rs.tunings[name]
	if !ok {
		return defaultValue
	}
	switch n := v.(type) {
	case starlark.Float:
		return float64(n)
	case starlark.Int:
		if i, ok := n.Int64(); ok {
			return float64(i)
		}
	}
	return defaultValue
}

// GetInt is the integer variant.
func (rs *RuleSet) GetInt(name string, defaultValue int) int {
	if rs == nil {
		return defaultValue
	}
	v, ok := rs.tunings[name]
	if !ok {
		return defaultValue
	}
	switch n := v.(type) {
	case starlark.Int:
		if i, ok := n.Int64(); ok {
			return int(i)
		}
	case starlark.Float:
		return int(n)
	}
	return defaultValue
}

// GetBool is the bool variant.
func (rs *RuleSet) GetBool(name string, defaultValue bool) bool {
	if rs == nil {
		return defaultValue
	}
	v, ok := rs.tunings[name]
	if !ok {
		return defaultValue
	}
	if b, ok := v.(starlark.Bool); ok {
		return bool(b)
	}
	return defaultValue
}

// SetTuningFloat overrides (or creates) a tuning value at runtime. Used
// by the engine's -tuning boot flag to tweak declarative tunings without
// editing the bundle's rules.star (e.g. flooding item respawn for a demo).
// Stored as a starlark.Float; GetInt narrows it to int on read.
func (rs *RuleSet) SetTuningFloat(name string, value float64) {
	if rs == nil {
		return
	}
	if rs.tunings == nil {
		rs.tunings = map[string]starlark.Value{}
	}
	rs.tunings[name] = starlark.Float(value)
}

// HasTuning reports whether the ruleset declared a tuning for this key.
func (rs *RuleSet) HasTuning(name string) bool {
	if rs == nil {
		return false
	}
	_, ok := rs.tunings[name]
	return ok
}

// TuningNames returns the list of declared tuning keys (unsorted).
// Useful for the rulebook renderer.
func (rs *RuleSet) TuningNames() []string {
	if rs == nil {
		return nil
	}
	out := make([]string, 0, len(rs.tunings))
	for k := range rs.tunings {
		out = append(out, k)
	}
	return out
}

// Item returns the declarative definition for an item by ID.
func (rs *RuleSet) Item(id string) (*ItemDef, bool) {
	if rs == nil {
		return nil, false
	}
	d, ok := rs.items[id]
	return d, ok
}

// ItemIDs lists every item ID declared in the ruleset.
func (rs *RuleSet) ItemIDs() []string {
	if rs == nil {
		return nil
	}
	out := make([]string, 0, len(rs.items))
	for id := range rs.items {
		out = append(out, id)
	}
	return out
}

// Verb returns the declarative definition for a novel verb by name.
// Phase 5 will iterate VerbNames() to register these into the engine.
func (rs *RuleSet) Verb(name string) (*VerbDef, bool) {
	if rs == nil {
		return nil, false
	}
	v, ok := rs.verbs[name]
	return v, ok
}

// VerbNames lists every novel verb name declared in the ruleset.
func (rs *RuleSet) VerbNames() []string {
	if rs == nil {
		return nil
	}
	out := make([]string, 0, len(rs.verbs))
	for n := range rs.verbs {
		out = append(out, n)
	}
	return out
}

// Stat returns the declarative definition for a stat by its extras key.
func (rs *RuleSet) Stat(key string) (*StatDef, bool) {
	if rs == nil {
		return nil, false
	}
	s, ok := rs.stats[key]
	return s, ok
}

// StatKeys lists every stat key declared in the ruleset.
func (rs *RuleSet) StatKeys() []string {
	if rs == nil {
		return nil
	}
	out := make([]string, 0, len(rs.stats))
	for k := range rs.stats {
		out = append(out, k)
	}
	return out
}

// --- helpers ---

func asFloat(v starlark.Value) float64 {
	switch n := v.(type) {
	case starlark.Float:
		return float64(n)
	case starlark.Int:
		if i, ok := n.Int64(); ok {
			return float64(i)
		}
	}
	return 0
}

func dictGetString(d *starlark.Dict, key string) (string, bool) {
	v, found, _ := d.Get(starlark.String(key))
	if !found {
		return "", false
	}
	s, ok := starlark.AsString(v)
	return s, ok
}

// unwrap converts a Starlark value into a plain Go interface{} suitable
// for storing in ItemDef.Props. Recursive for dicts; everything else
// becomes its Go primitive.
func unwrap(v starlark.Value) interface{} {
	switch n := v.(type) {
	case starlark.String:
		return string(n)
	case starlark.Bool:
		return bool(n)
	case starlark.Int:
		if i, ok := n.Int64(); ok {
			return i
		}
	case starlark.Float:
		return float64(n)
	case *starlark.Dict:
		out := map[string]interface{}{}
		for _, item := range n.Items() {
			k, ok := starlark.AsString(item[0])
			if !ok {
				continue
			}
			out[k] = unwrap(item[1])
		}
		return out
	case *starlark.List:
		out := make([]interface{}, 0, n.Len())
		for i := 0; i < n.Len(); i++ {
			out = append(out, unwrap(n.Index(i)))
		}
		return out
	}
	return nil
}
