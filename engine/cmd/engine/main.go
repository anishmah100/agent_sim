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

	"github.com/anishmah100/agent_sim/engine/internal/scenario/fantasy_town"
	"github.com/anishmah100/agent_sim/engine/internal/wire"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const tickRate = 60
const tickDuration = time.Second / tickRate

var (
	flagAddr     = flag.String("addr", "127.0.0.1:8080", "HTTP+WS listen address")
	flagWorld    = flag.String("world", "worlds/dev_test.json", "world JSON file")
	flagScenario = flag.String("scenario", "fantasy_town", "scenario pack id")
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

	// Install the chosen scenario. Currently only fantasy_town exists;
	// future scenarios register here too.
	if *flagScenario == "fantasy_town" {
		s := fantasy_town.New()
		verbs := make(map[string]func(*world.World, *world.Entity, *world.ActionEnvelope) world.ActionResult)
		for _, v := range s.Verbs() {
			h := s.Handler(v)
			if h != nil {
				verbs[v] = h
			}
		}
		w.InstallScenario(verbs, func(w *world.World, t uint64) { s.OnTick(w, t) }, s.OnEntitySpawn)
		log.Printf("installed scenario: %s (%d verbs)", s.Name(), len(verbs))
	}

	startedAt := time.Now()
	var tick atomic.Uint64

	ctx, cancel := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer cancel()

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
