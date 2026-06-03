package verbalquests_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anishmah100/agent_sim/engine/internal/core/eventbus"
	"github.com/anishmah100/agent_sim/engine/internal/core/manifest"
	"github.com/anishmah100/agent_sim/engine/internal/core/spatial"
	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/systems/verbalquests"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const testWorldJSON = `{
  "map_id": "vq_test",
  "width_tiles": 6,
  "height_tiles": 4,
  "tiles_legend": {".":"grass"},
  "tiles": ["......","......","......","......"],
  "entities": [
    {"entity_id":"hero","archetype":"trainer","pos":[1,1],"facing":"S","display_name":"Hero"},
    {"entity_id":"merchant","archetype":"merchant","pos":[2,1],"facing":"S","display_name":"Merchant"}
  ]
}`

func boot(t *testing.T) (*world.WorldAdapter, *syscore.ConcreteRegistry) {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "w.json")
	if err := os.WriteFile(p, []byte(testWorldJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	w, err := world.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	bus := eventbus.New()
	spat := spatial.New()
	for _, id := range w.EntityIDsUnlocked() {
		e := w.EntityByIDUnlocked(id)
		if e != nil {
			spat.Add(e.EntityID, e.LogicalTile)
		}
	}
	wa := world.NewWorldAdapter(w, bus, spat)
	agg := manifest.NewAggregator("vq_test", "test")
	reg := syscore.NewConcreteRegistry(bus, agg)
	reg.BeginSystem("verbalquests")
	verbalquests.New().RegisterWith(reg)
	reg.EndSystem()
	reg.InstallServicesInto(wa)
	for _, id := range wa.EntityIDs() {
		reg.RunOnEntitySpawn(wa, wa.EntityByID(id))
	}
	return wa, reg
}

func handle(t *testing.T, reg *syscore.ConcreteRegistry, wa *world.WorldAdapter, actor, verb, raw string) syscore.ActionResult {
	t.Helper()
	return reg.Handle(wa, wa.EntityByID(actor), &syscore.ActionEnvelope{
		ActionID: "1", Verb: verb, Raw: []byte(raw),
	})
}

func TestSeedContractsList(t *testing.T) {
	wa, _ := boot(t)
	hero := wa.EntityByID("hero")
	v, ok := hero.GetExtra("contracts")
	if !ok {
		t.Fatal("contracts should be seeded")
	}
	if items, _ := v.([]any); len(items) != 0 {
		t.Fatalf("expected empty contracts, got %v", v)
	}
}

func TestProposeAddsToBothParties(t *testing.T) {
	wa, reg := boot(t)
	res := handle(t, reg, wa, "hero", "propose_task",
		`{"target":"merchant","terms":"bring me 3 logs","reward":"5 gold"}`)
	if !res.Accepted {
		t.Fatalf("propose should accept: %s", res.Reason)
	}
	for _, id := range []string{"hero", "merchant"} {
		e := wa.EntityByID(id)
		v, _ := e.GetExtra("contracts")
		items, _ := v.([]any)
		if len(items) != 1 {
			t.Fatalf("%s should have 1 contract, got %d", id, len(items))
		}
		m, _ := items[0].(map[string]any)
		if m["status"] != "proposed" {
			t.Fatalf("%s contract status=%v", id, m["status"])
		}
	}
}

func TestAcceptOnlyByTarget(t *testing.T) {
	wa, reg := boot(t)
	_ = handle(t, reg, wa, "hero", "propose_task",
		`{"target":"merchant","terms":"bring me 3 logs","reward":"5 gold"}`)
	// Find the contract id by reading off merchant's ledger.
	merch := wa.EntityByID("merchant")
	v, _ := merch.GetExtra("contracts")
	items, _ := v.([]any)
	id, _ := items[0].(map[string]any)["id"].(string)

	// Hero (proposer) tries to accept their own proposal → rejected.
	r := handle(t, reg, wa, "hero", "accept_task", `{"id":"`+id+`"}`)
	if r.Accepted || r.Reason != "not_authorized" {
		t.Fatalf("proposer accept should reject (not_authorized), got accepted=%v reason=%s", r.Accepted, r.Reason)
	}

	// Merchant (target) accepts → status becomes accepted on both sides.
	r = handle(t, reg, wa, "merchant", "accept_task", `{"id":"`+id+`"}`)
	if !r.Accepted {
		t.Fatalf("target accept should succeed: %s", r.Reason)
	}
	for _, who := range []string{"hero", "merchant"} {
		e := wa.EntityByID(who)
		v, _ := e.GetExtra("contracts")
		items, _ := v.([]any)
		m, _ := items[0].(map[string]any)
		if m["status"] != "accepted" {
			t.Fatalf("%s status=%v", who, m["status"])
		}
	}
}

func TestCompleteOnlyByProposer(t *testing.T) {
	wa, reg := boot(t)
	_ = handle(t, reg, wa, "hero", "propose_task",
		`{"target":"merchant","terms":"bring me 3 logs","reward":"5 gold"}`)
	merch := wa.EntityByID("merchant")
	v, _ := merch.GetExtra("contracts")
	items := v.([]any)
	id := items[0].(map[string]any)["id"].(string)
	_ = handle(t, reg, wa, "merchant", "accept_task", `{"id":"`+id+`"}`)

	// Merchant tries to complete → not authorized.
	r := handle(t, reg, wa, "merchant", "complete_task", `{"id":"`+id+`"}`)
	if r.Accepted || r.Reason != "not_authorized" {
		t.Fatalf("non-proposer complete should reject: accepted=%v reason=%s", r.Accepted, r.Reason)
	}

	// Hero (proposer) completes → status flips.
	r = handle(t, reg, wa, "hero", "complete_task", `{"id":"`+id+`"}`)
	if !r.Accepted {
		t.Fatalf("proposer complete should succeed: %s", r.Reason)
	}
}

func TestEmptyTermsRejects(t *testing.T) {
	wa, reg := boot(t)
	r := handle(t, reg, wa, "hero", "propose_task",
		`{"target":"merchant","terms":"","reward":"5 gold"}`)
	if r.Accepted || r.Reason != "empty_terms" {
		t.Fatalf("expected empty_terms rejection, got %v", r)
	}
}

func TestSelfTargetRejects(t *testing.T) {
	wa, reg := boot(t)
	r := handle(t, reg, wa, "hero", "propose_task",
		`{"target":"hero","terms":"do nothing","reward":"0"}`)
	if r.Accepted || r.Reason != "self_target" {
		t.Fatalf("expected self_target rejection, got %v", r)
	}
}

func TestEventsEmitted(t *testing.T) {
	wa, reg := boot(t)
	prop, acc, comp := 0, 0, 0
	wa.Bus.Subscribe("TaskProposed", func(_ eventbus.WorldCtx, _ eventbus.Event) { prop++ })
	wa.Bus.Subscribe("TaskAccepted", func(_ eventbus.WorldCtx, _ eventbus.Event) { acc++ })
	wa.Bus.Subscribe("TaskCompleted", func(_ eventbus.WorldCtx, _ eventbus.Event) { comp++ })

	_ = handle(t, reg, wa, "hero", "propose_task",
		`{"target":"merchant","terms":"deliver","reward":"5g"}`)
	merch := wa.EntityByID("merchant")
	v, _ := merch.GetExtra("contracts")
	id := v.([]any)[0].(map[string]any)["id"].(string)
	_ = handle(t, reg, wa, "merchant", "accept_task", `{"id":"`+id+`"}`)
	_ = handle(t, reg, wa, "hero", "complete_task", `{"id":"`+id+`"}`)
	wa.Bus.Drain(wa)
	if prop != 1 || acc != 1 || comp != 1 {
		t.Fatalf("expected 1 of each, got prop=%d acc=%d comp=%d", prop, acc, comp)
	}
}
