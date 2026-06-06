package world

// VitalsSnapshot — live entity vitals + inventory + equipped, as
// surfaced by /api/v1/agent/<id>/mental_state.vitals. Used by the
// inspector to show what an agent currently has on them. Designed
// to be flexible: inventory is a free-form list of item ids (with
// per-kind aggregation done client-side) so adding a new item kind
// requires no schema change here.

type VitalsItem struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type VitalsSnapshot struct {
	HP        int          `json:"hp"`
	MaxHP     int          `json:"max_hp"`
	Hunger    float64      `json:"hunger"`
	Gold      int          `json:"gold"`
	Inventory []VitalsItem `json:"inventory"`
	Equipped  map[string]string `json:"equipped"`
	// Resolves to "" when the entity isn't currently inside.
	InsideBuilding string `json:"inside_building,omitempty"`
}

// VitalsOf — returns a fresh snapshot of the named entity's vitals.
// Returns zero VitalsSnapshot when the entity is unknown. Safe for
// concurrent callers (takes the read lock).
func (w *World) VitalsOf(entityID string) VitalsSnapshot {
	w.mu.RLock()
	e := w.entities[entityID]
	if e == nil {
		w.mu.RUnlock()
		return VitalsSnapshot{}
	}
	extras := map[string]any{}
	for k, v := range e.Extras {
		extras[k] = v
	}
	insideBuilding := e.InsideBuilding
	w.mu.RUnlock()

	out := VitalsSnapshot{InsideBuilding: insideBuilding, Equipped: map[string]string{}}
	if v, ok := extras["hp"]; ok {
		out.HP = toInt(v)
	}
	if v, ok := extras["max_hp"]; ok {
		out.MaxHP = toInt(v)
	}
	if v, ok := extras["hunger"]; ok {
		out.Hunger = toFloat(v)
	}
	if v, ok := extras["gold"]; ok {
		out.Gold = toInt(v)
	}
	if eq, ok := extras["equipped"].(map[string]any); ok {
		for slot, raw := range eq {
			if s, ok := raw.(string); ok && s != "" {
				out.Equipped[slot] = s
			}
		}
	}
	// Inventory comes in as []string (code-set) or []any (JSON-decoded).
	// Aggregate by kind so the inspector renders "apple ×3" not three
	// rows of "item:apple#7", "item:apple#12", "item:apple#41".
	var inv []string
	switch x := extras["inventory"].(type) {
	case []string:
		inv = x
	case []any:
		for _, v := range x {
			if s, ok := v.(string); ok {
				inv = append(inv, s)
			}
		}
	}
	counts := map[string]int{}
	order := []string{}
	for _, id := range inv {
		k := itemKindString(id)
		if _, seen := counts[k]; !seen {
			order = append(order, k)
		}
		counts[k]++
	}
	for _, k := range order {
		out.Inventory = append(out.Inventory, VitalsItem{
			ID: k, Kind: k, Count: counts[k],
		})
	}
	return out
}

// itemKindString — strip "item:" prefix + "#suffix" from an id.
// Local copy to avoid an internal/systems/inventory import cycle.
func itemKindString(id string) string {
	if len(id) > 5 && id[:5] == "item:" {
		id = id[5:]
	}
	for i := 0; i < len(id); i++ {
		if id[i] == '#' {
			return id[:i]
		}
	}
	return id
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case float32:
		return int(x)
	}
	return 0
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	}
	return 0
}
