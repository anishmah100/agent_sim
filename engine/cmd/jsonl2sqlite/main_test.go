package main

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

const sampleJSONL = `{"tick":1,"seq":0,"kind":"DamageDealt","category":"combat","payload":{"target":"goblin","amount":10}}
{"tick":2,"seq":1,"kind":"GoldTransferred","category":"economy","payload":{"from":"hero","to":"mari","amount":5}}
{"tick":3,"seq":2,"kind":"ReasoningTrace","category":"agent_reasoning","payload":{"entity_id":"hero","action_id":"a1","verb":"move","reasoning":"heading to blacksmith"}}
{"tick":4,"seq":3,"kind":"EntityMoved","category":"movement","payload":{"id":"hero"}}
`

func runTool(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s", err, string(out))
	}
}

func TestJSONLToSQLite_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "events.jsonl")
	out := filepath.Join(dir, "events.sqlite")
	if err := os.WriteFile(in, []byte(sampleJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	runTool(t, "-in", in, "-out", out)

	db, err := sql.Open("sqlite", out)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM events").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("events count: want 4, got %d", n)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM reasoning_traces").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("traces count: want 1, got %d", n)
	}
	// Spot-check the denormalized reasoning trace.
	var entity, verb, reasoning string
	if err := db.QueryRow(
		"SELECT entity_id, verb, reasoning FROM reasoning_traces WHERE action_id=?", "a1",
	).Scan(&entity, &verb, &reasoning); err != nil {
		t.Fatal(err)
	}
	if entity != "hero" || verb != "move" || reasoning != "heading to blacksmith" {
		t.Fatalf("trace mismatch: %s %s %s", entity, verb, reasoning)
	}
}

func TestJSONLToSQLite_IdempotentRerun(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "events.jsonl")
	out := filepath.Join(dir, "events.sqlite")
	if err := os.WriteFile(in, []byte(sampleJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	runTool(t, "-in", in, "-out", out)
	runTool(t, "-in", in, "-out", out)
	db, _ := sql.Open("sqlite", out)
	defer db.Close()
	var n int
	_ = db.QueryRow("SELECT COUNT(*) FROM events").Scan(&n)
	if n != 4 {
		t.Fatalf("rerun should not double-insert; got %d events", n)
	}
}

func TestJSONLToSQLite_IndexesExist(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "events.jsonl")
	out := filepath.Join(dir, "events.sqlite")
	if err := os.WriteFile(in, []byte(sampleJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	runTool(t, "-in", in, "-out", out)
	db, _ := sql.Open("sqlite", out)
	defer db.Close()
	want := []string{
		"idx_events_tick",
		"idx_events_category",
		"idx_events_kind",
		"idx_traces_entity",
		"idx_traces_tick",
	}
	for _, name := range want {
		var count int
		_ = db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", name,
		).Scan(&count)
		if count != 1 {
			t.Errorf("index %s missing", name)
		}
	}
}
