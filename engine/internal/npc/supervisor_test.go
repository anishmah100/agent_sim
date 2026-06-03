package npc

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "cfg.json")
	body := `{
      "npcs": [
        {"name":"a","command":"echo","args":["hi"],"count":2,"auto_restart":false},
        {"name":"b","command":"true","auto_restart":true}
      ]
    }`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.NPCs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(cfg.NPCs))
	}
	if cfg.NPCs[0].Count != 2 {
		t.Fatalf("count=%d", cfg.NPCs[0].Count)
	}
}

func TestSupervisorRunsAndExits(t *testing.T) {
	// 'true' exits immediately; with auto_restart=false, the runLoop
	// should fall through after one iteration.
	cfg := &Config{NPCs: []Spec{
		{Name: "once", Command: "true", AutoRestart: false},
	}}
	logger := log.New(io.Discard, "", 0)
	sup := New(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	sup.Start(ctx)
	// Give it a moment to spawn + exit.
	time.Sleep(150 * time.Millisecond)
	cancel()
	done := make(chan struct{})
	go func() { sup.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor did not stop in time")
	}
	stats := sup.Stats()
	if len(stats) != 1 || stats[0].Restarts == 0 {
		t.Fatalf("expected 1 stat with >=1 restart count, got %v", stats)
	}
	if stats[0].Running {
		t.Fatal("process should not be running after Stop")
	}
}

func TestSupervisorAutoRestarts(t *testing.T) {
	cfg := &Config{NPCs: []Spec{
		{Name: "loop", Command: "true", AutoRestart: true},
	}}
	logger := log.New(io.Discard, "", 0)
	sup := New(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	sup.Start(ctx)
	// 'true' exits instantly; with auto_restart on, we should see
	// multiple restarts in ~2 seconds (backoff starts at 500ms).
	time.Sleep(1500 * time.Millisecond)
	cancel()
	done := make(chan struct{})
	go func() { sup.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor did not stop in time")
	}
	stats := sup.Stats()
	if stats[0].Restarts < 2 {
		t.Fatalf("expected at least 2 restarts, got %d", stats[0].Restarts)
	}
}

func TestSupervisorCountExpands(t *testing.T) {
	cfg := &Config{NPCs: []Spec{
		{Name: "tri", Command: "true", Count: 3, AutoRestart: false},
	}}
	logger := log.New(io.Discard, "", 0)
	sup := New(cfg, logger)
	ctx, cancel := context.WithCancel(context.Background())
	sup.Start(ctx)
	time.Sleep(150 * time.Millisecond)
	cancel()
	sup.Stop()
	stats := sup.Stats()
	if len(stats) != 3 {
		t.Fatalf("expected 3 supervised processes, got %d", len(stats))
	}
}
