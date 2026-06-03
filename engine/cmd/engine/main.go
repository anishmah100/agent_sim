// Package main is the agent_sim engine binary.
//
// One process = one world. Configuration via flags. The engine ticks at
// 60Hz, serves WebSocket endpoints for viewers and agents, and exposes a
// minimal HTTP surface for registration + static info.
//
// See docs/ARCHITECTURE.md for the layer contracts.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/anishmah100/agent_sim/engine/internal/historian"
	"github.com/anishmah100/agent_sim/engine/internal/metrics"
	"github.com/anishmah100/agent_sim/engine/internal/npc"
	"github.com/anishmah100/agent_sim/engine/internal/scenario/fantasy_town"
	"github.com/anishmah100/agent_sim/engine/internal/wire"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const tickRate = 60
const tickDuration = time.Second / tickRate

var (
	flagAddr      = flag.String("addr", "127.0.0.1:8080", "HTTP+WS listen address")
	flagWorld     = flag.String("world", "worlds/dev_test.json", "world JSON file")
	flagScenario  = flag.String("scenario", "fantasy_town", "scenario pack id")
	flagEventLog  = flag.String("event-log", "", "if set, append every world event to this JSONL path (autoresearch substrate)")
	flagRingSize  = flag.Int("event-ring", 4096, "in-memory event ring size served by /api/v1/world/history")
	flagNPCConfig = flag.String("npc-config", "", "JSON config for NPC subprocesses to spawn (see internal/npc)")
)

func main() {
	flag.Parse()

	w, err := world.Load(*flagWorld)
	if err != nil {
		log.Fatalf("load world: %v", err)
	}
	log.Printf("loaded world %s (%dx%d, %d entities) from %s",
		w.MapID, w.WidthTiles, w.HeightTiles,
		len(w.Snapshot().Entities), *flagWorld)

	// Install the chosen scenario. Each scenario package declares which
	// composable systems to register; the SystemHost owns the verb
	// registry, event bus, spatial index, and service table.
	var host *world.SystemHost
	if *flagScenario == "fantasy_town" {
		host = fantasy_town.Install(w)
		log.Printf("installed scenario: %s (%d verbs)", fantasy_town.Name, host.Registry.VerbCount())
	}

	// Historian listens to every event on the bus. Wired after the
	// scenario installs systems so it sees subscriber-issued events too.
	hist, err := historian.New(*flagRingSize, *flagEventLog)
	if err != nil {
		log.Fatalf("historian init: %v", err)
	}
	defer hist.Close()
	if host != nil {
		hist.Attach(host.Bus)
		if *flagEventLog != "" {
			log.Printf("historian appending events to %s", *flagEventLog)
		}
	}

	startedAt := time.Now()
	var tick atomic.Uint64

	ctx, cancel := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer cancel()

	// NPC supervisor — start BEFORE the HTTP server so processes that
	// register on boot find the engine listening.
	var npcSup *npc.Supervisor
	if *flagNPCConfig != "" {
		cfg, err := npc.LoadConfig(*flagNPCConfig)
		if err != nil {
			log.Fatalf("npc config: %v", err)
		}
		npcSup = npc.New(cfg, log.Default())
		npcSup.Start(ctx)
		log.Printf("supervising %d NPC spec(s) from %s", len(cfg.NPCs), *flagNPCConfig)
	}

	hub := wire.NewViewerHub(ctx, w)
	agents := wire.NewAgentHub(ctx, w)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/world/info", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(rw).Encode(map[string]any{
			"name":      "agent_sim engine",
			"version":   "0.0.2",
			"scenario":  *flagScenario,
			"world":     w.MapID,
			"world_dims": []int{w.WidthTiles, w.HeightTiles},
			"tick_rate": tickRate,
			"tick":      tick.Load(),
			"uptime_s":  time.Since(startedAt).Seconds(),
		})
	})
	mux.HandleFunc("/healthz", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("ok"))
	})
	mux.HandleFunc("/ws/viewer", hub.Handle)
	mux.HandleFunc("/ws/agent", agents.HandleWS)
	mux.HandleFunc("/api/v1/agent/register", agents.HandleRegister)
	mux.HandleFunc("/api/v1/leaderboards", wire.LeaderboardsHandler(w))
	mux.HandleFunc("/api/v1/world/affordances", wire.AffordanceManifestHandler(host))
	mux.HandleFunc("/api/v1/world/history", historian.Handler(hist))

	// Prometheus-format /metrics. Stats sourced from the existing
	// counters (no client_golang dep).
	npcStats := func() int {
		if npcSup == nil {
			return 0
		}
		total := 0
		for _, s := range npcSup.Stats() {
			total += s.Restarts
		}
		return total
	}
	mux.HandleFunc("/metrics", metrics.Handler(metricsSource{
		startedAt:   startedAt,
		tickPtr:     &tick,
		world:       w,
		hub:         hub,
		agentHub:    agents,
		hist:        hist,
		npcRestarts: npcStats,
	}))

	// Static world JSON + art atlases. The engine serves these because:
	// 1. Same-origin = no CORS pain.
	// 2. The agent rasterizer (Milestone 4) needs to read the same
	//    files as the frontend so server-rendered crops match.
	// CORS open in v0.
	worldsDir, _ := filepath.Abs(filepath.Dir(*flagWorld))
	mux.Handle("/worlds/", http.StripPrefix("/worlds/",
		corsHandler(http.FileServer(http.Dir(worldsDir)))))

	// art/ is sibling-of-worlds in the repo. Convention: repo_root/art
	// and repo_root/worlds. Compute repo root from world flag.
	repoRoot := filepath.Dir(worldsDir)
	artDir := filepath.Join(repoRoot, "art")
	mux.Handle("/art/", http.StripPrefix("/art/",
		corsHandler(http.FileServer(http.Dir(artDir)))))

	srv := &http.Server{
		Addr:              *flagAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("engine listening on %s (scenario=%s)",
			*flagAddr, *flagScenario)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	tickTimer := time.NewTicker(tickDuration)
	defer tickTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("shutdown signal; flushing")
			shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
			defer c()
			_ = srv.Shutdown(shutdownCtx)
			if npcSup != nil {
				npcSup.Stop()
			}
			log.Println("clean exit")
			return

		case <-tickTimer.C:
			w.Tick()
			tick.Add(1)
		}
	}
}

// corsHandler adds Access-Control-Allow-Origin: * to the response.
// v0-only convenience; real CORS lockdown lands with the deploy story.
func corsHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Access-Control-Allow-Origin", "*")
		h.ServeHTTP(rw, r)
	})
}

// metricsSource implements metrics.Source by reading from the live
// world / hubs / historian / NPC supervisor.
type metricsSource struct {
	startedAt   time.Time
	tickPtr     *atomic.Uint64
	world       *world.World
	hub         *wire.ViewerHub
	agentHub    *wire.AgentHub
	hist        *historian.Historian
	npcRestarts func() int
}

func (s metricsSource) Tick() uint64           { return s.tickPtr.Load() }
func (s metricsSource) UptimeSeconds() float64 { return time.Since(s.startedAt).Seconds() }
func (s metricsSource) EntityCount() int       { return len(s.world.EntityIDs()) }
func (s metricsSource) ViewerCount() int       { return s.hub.Count() }
func (s metricsSource) AgentCount() int        { return s.agentHub.Count() }
func (s metricsSource) EventsEmitted() uint64 {
	if s.hist == nil {
		return 0
	}
	return s.hist.Stats().TotalEmitted
}
func (s metricsSource) NPCRestarts() int { return s.npcRestarts() }
