// Package main is the agent_sim engine binary.
//
// One process = one world. Configuration via flags. The engine ticks at
// 60Hz, serves WebSocket endpoints for viewers and agents, and exposes a
// minimal HTTP surface for registration + static info.
//
// This is the skeleton. Real subsystems land in internal/world (state +
// tick), internal/wire (FlatBuffers + WS message types), internal/scenario
// (verb registration, scenario loading). See docs/ARCHITECTURE.md.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

// Wall-clock tick rate. The engine simulates the world in fixed-rate
// steps; rendering interpolation lives on the client.
const tickRate = 60
const tickDuration = time.Second / tickRate

var (
	flagAddr     = flag.String("addr", "127.0.0.1:8080", "HTTP+WS listen address")
	flagWorld    = flag.String("world", "worlds/test.ldtk", "LDtk world file")
	flagScenario = flag.String("scenario", "fantasy_town", "scenario pack id")
)

// engineState holds the minimal handshake state for the skeleton. The
// real world state will live in internal/world.World once that exists.
type engineState struct {
	startedAt time.Time
	tick      atomic.Uint64
	world     string
	scenario  string
}

func (e *engineState) info() map[string]any {
	return map[string]any{
		"name":      "agent_sim engine",
		"version":   "0.0.1",
		"scenario":  e.scenario,
		"world":     e.world,
		"tick_rate": tickRate,
		"tick":      e.tick.Load(),
		"uptime_s":  time.Since(e.startedAt).Seconds(),
	}
}

func main() {
	flag.Parse()

	st := &engineState{
		startedAt: time.Now(),
		world:     *flagWorld,
		scenario:  *flagScenario,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/world/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(st.info())
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              *flagAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer cancel()

	go func() {
		log.Printf("engine listening on %s (scenario=%s world=%s)",
			*flagAddr, *flagScenario, *flagWorld)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// World tick loop. Skeleton: just increments a counter. Real
	// simulation work lands in world.Tick() once internal/world exists.
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
			st.tick.Add(1)
		}
	}
}
