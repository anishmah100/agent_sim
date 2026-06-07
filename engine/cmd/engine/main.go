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
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/anishmah100/agent_sim/engine/internal/historian"
	"github.com/anishmah100/agent_sim/engine/internal/metrics"
	"github.com/anishmah100/agent_sim/engine/internal/npc"
	"github.com/anishmah100/agent_sim/engine/internal/persist"
	"github.com/anishmah100/agent_sim/engine/internal/scenario/fantasy_town"
	"github.com/anishmah100/agent_sim/engine/internal/security"
	"github.com/anishmah100/agent_sim/engine/internal/wire"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

const tickRate = 60
const tickDuration = time.Second / tickRate

// D11 — time multiplier (configurable via -time-mult flag). Engine
// always advances 1 in-game tick per scheduler tick; the SCHEDULER
// fires faster when mult>1, so a 4x run packs 4 in-game min into
// 1 real min. All in-game-tick-denominated durations (hunger_per_tick,
// respawn_interval, etc.) remain invariant to multiplier.

var (
	flagAddr     = flag.String("addr", "127.0.0.1:8080", "HTTP+WS listen address")
	flagBundle   = flag.String("bundle", "worlds/eldoria", "world bundle directory (contains bundle.toml + world.json). Preferred over -world/-scenario.")
	flagWorld    = flag.String("world", "", "[legacy] direct path to a world.json. If set, overrides -bundle's world.")
	flagScenario = flag.String("scenario", "", "[legacy] scenario package id. If set, overrides bundle's scenario.")
	flagEventLog          = flag.String("event-log", "", "if set, append every world event to this JSONL path (autoresearch substrate)")
	flagEventMute         = flag.String("event-mute", "", "comma-separated list of event categories to drop (system, movement, combat, economy, social, agent_reasoning, world)")
	flagCaptureReasoning  = flag.Bool("capture-reasoning", false, "engine-level enable for capturing per-action 'reasoning' traces. Per-agent share_reasoning must ALSO be true.")
	flagRingSize  = flag.Int("event-ring", 4096, "in-memory event ring size served by /api/v1/world/history")
	flagNPCConfig = flag.String("npc-config", "", "JSON config for NPC subprocesses to spawn. If empty, falls back to the bundle's npcs.config (if any).")
	flagSnapDir   = flag.String("snapshot-dir", "", "if set, save world snapshots to this dir and restore on boot")
	flagSnapEvery = flag.Duration("snapshot-every", 60*time.Second, "how often to write a snapshot (0 disables)")
	flagTimeMult  = flag.Float64("time-mult", 1.0, "in-game time multiplier (D11). 1.0 = real-time; 4.0 = 4x speed (dev iteration); higher values pack more in-game minutes into each real minute. The engine scheduler tick rate is multiplied by this value.")
	flagNarratorJSONL = flag.String("narrator-jsonl", ".runlog/narrator.jsonl", "path to the narrator's output JSONL, served by /api/v1/narrator/recent for the Story Feed UI. The narrator process (tools/narrator) writes it; the engine only reads it.")

	// Security
	flagCORS    = flag.String("cors-allow", "", "comma-separated CORS allowlist (origins). Empty disables CORS.")
	flagJWTSecret = flag.String("jwt-secret", "", "HMAC secret for verifying agent registration JWTs. Empty disables JWT (dev only).")
	flagRegRate = flag.Float64("register-rate", 1, "registrations per second per IP (token bucket refill)")
	flagRegBurst = flag.Int("register-burst", 5, "registrations burst per IP")
)

func main() {
	flag.Parse()

	// Resolve the world bundle + the effective world / scenario / npcs
	// config paths. Bundle is the source-of-truth; legacy flags override
	// individual fields for ad-hoc dev.
	var (
		w           *world.World
		bundle      *world.Bundle
		scenarioPkg = *flagScenario // legacy override (if set)
		npcConfig   = *flagNPCConfig
		worldSrc    string
	)
	switch {
	case *flagWorld != "":
		// Legacy: direct path to world.json, no bundle.
		var err error
		w, err = world.Load(*flagWorld)
		if err != nil {
			log.Fatalf("load world (legacy -world): %v", err)
		}
		worldSrc = *flagWorld
		if scenarioPkg == "" {
			scenarioPkg = "fantasy_town"
		}
	default:
		// Preferred path: bundle.
		var err error
		w, bundle, err = world.LoadBundle(*flagBundle)
		if err != nil {
			log.Fatalf("load bundle %s: %v", *flagBundle, err)
		}
		worldSrc = filepath.Join(bundle.Dir, bundle.WorldFile)
		if scenarioPkg == "" {
			scenarioPkg = bundle.ScenarioPkg
		}
		if npcConfig == "" {
			npcConfig = bundle.NPCsConfigPath()
		}
	}
	log.Printf("loaded world %s (%dx%d, %d entities) from %s",
		w.MapID, w.WidthTiles, w.HeightTiles,
		len(w.Snapshot().Entities), worldSrc)
	if bundle != nil {
		log.Printf("bundle: %s — scenario=%s art_pack=%q",
			bundle.Name, scenarioPkg, bundle.ArtPack)
	}

	// Restore from the latest snapshot in the snapshot dir, if one
	// exists. The world JSON gave us the static map; this overlays
	// dynamic state (HP, gold, inventory, contracts, inside_building).
	if *flagSnapDir != "" {
		if p := persist.LatestPath(*flagSnapDir); p != "" {
			if err := persist.Restore(w, p); err != nil {
				log.Printf("snapshot restore from %s failed: %v", p, err)
			} else {
				log.Printf("restored snapshot from %s", p)
			}
		}
	}

	// Install the chosen scenario. Each scenario package declares which
	// composable systems to register; the SystemHost owns the verb
	// registry, event bus, spatial index, and service table.
	var host *world.SystemHost
	switch scenarioPkg {
	case "fantasy_town":
		host = fantasy_town.Install(w)
		log.Printf("installed scenario: %s (%d verbs)", fantasy_town.Name, host.Registry.VerbCount())
	case "":
		log.Printf("no scenario installed (bundle did not specify one)")
	default:
		log.Fatalf("unknown scenario package: %q", scenarioPkg)
	}

	// Historian listens to every event on the bus. Wired after the
	// scenario installs systems so it sees subscriber-issued events too.
	mute := map[string]bool{}
	for _, cat := range strings.Split(*flagEventMute, ",") {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			mute[cat] = true
		}
	}
	hist, err := historian.NewWithFilter(*flagRingSize, *flagEventLog, historian.CategoryFilter{Disabled: mute})
	if err != nil {
		log.Fatalf("historian init: %v", err)
	}
	defer hist.Close()
	if host != nil {
		hist.Attach(host.Bus)
		if *flagEventLog != "" {
			log.Printf("historian appending events to %s", *flagEventLog)
		}
		if len(mute) > 0 {
			log.Printf("historian muting categories: %v", mute)
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
	if npcConfig != "" {
		cfg, err := npc.LoadConfig(npcConfig)
		if err != nil {
			log.Fatalf("npc config: %v", err)
		}
		// vars feed ${KEY} substitution in spec.Args — npcs.json refers
		// to ${ENGINE_ADDR} so it tracks the -addr flag instead of
		// hard-coding a port that drifts (8080 default vs. 8088 in the
		// A9 smoke). Add new vars here as needed; the supervisor leaves
		// unmatched ${KEY} tokens literal.
		npcSup = npc.New(cfg, log.Default(), map[string]string{
			"ENGINE_ADDR": *flagAddr,
		})
		npcSup.Start(ctx)
		log.Printf("supervising %d NPC spec(s) from %s", len(cfg.NPCs), npcConfig)
	}

	hub := wire.NewViewerHub(ctx, w)
	agents := wire.NewAgentHub(ctx, w)

	// Layered reasoning capture. -capture-reasoning AND the per-agent
	// share_reasoning flag must both be true for a trace to land in
	// the historian. See docs/EXPERIMENT_SYSTEM_PLAN.md §8.
	agents.SetCaptureReasoning(*flagCaptureReasoning)
	agents.OnReasoning = func(entityID, actionID, verb, reasoning string) {
		hist.LogReasoning(w.CurrentTick(), historian.ReasoningTrace{
			EntityID:  entityID,
			ActionID:  actionID,
			Verb:      verb,
			Reasoning: reasoning,
		})
	}
	agents.OnReflection = func(entityID, note string) {
		hist.LogReflection(w.CurrentTick(), historian.ReflectiveNote{
			EntityID: entityID,
			Note:     note,
		})
	}
	agents.OnMentalNote = func(entityID, text, tag string, slots map[string]string) {
		hist.LogMentalNote(w.CurrentTick(), historian.MentalNote{
			EntityID: entityID,
			Text:     text,
			Tag:      tag,
			Slots:    slots,
		})
	}

	// Security middleware — CORS allowlist + per-IP rate limit on
	// /register + JWT verification on /register. Each is opt-in via flag.
	var corsAllowlist []string
	if *flagCORS != "" {
		for _, o := range splitComma(*flagCORS) {
			corsAllowlist = append(corsAllowlist, o)
		}
	}
	corsMid := security.CORS(corsAllowlist)
	regRateMid := security.RateLimit(*flagRegRate, *flagRegBurst)
	var regJWTMid func(http.Handler) http.Handler
	if *flagJWTSecret != "" {
		regJWTMid = security.RequireJWT([]byte(*flagJWTSecret))
	} else {
		regJWTMid = func(h http.Handler) http.Handler { return h }
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/world/info", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
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
	// /register: rate-limit per IP → JWT verify → handler
	mux.Handle("/api/v1/agent/register",
		regRateMid(regJWTMid(http.HandlerFunc(agents.HandleRegister))))
	mux.HandleFunc("/api/v1/leaderboards", wire.LeaderboardsHandler(w))
	mux.HandleFunc("/api/v1/world/affordances", wire.AffordanceManifestHandler(host))
	mux.HandleFunc("/api/v1/world/rulebook.json", wire.RulebookHandler(w, bundle, host, tickRate))
	mux.HandleFunc("/api/v1/world/history", historian.Handler(hist))
	mux.HandleFunc("/api/v1/world/edit", wire.TileEditHandler(w))
	mux.HandleFunc("/api/v1/world/edit_deco", wire.DecorationEditHandler(w))
	mux.HandleFunc("/api/v1/agents", wire.AgentsListHandler(agents, w))
	// AGENT-A7 inspector → mental_state endpoint. Path includes the
	// entity id; the handler parses it out.
	mux.HandleFunc("/api/v1/agent/", wire.MentalStateHandler(hist, *flagCaptureReasoning, w))
	// Debug: synthetic-vision query so we can diagnose D8 routing live.
	mux.HandleFunc("/api/v1/debug/vision", wire.DebugVisionHandler(w))
	// D15/D17 — Story Feed reads the narrator's output file via this.
	mux.HandleFunc("/api/v1/narrator/recent", wire.NarratorHandler(*flagNarratorJSONL))

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
	//
	// worldsDir resolution. When the engine boots via -bundle (the new
	// path), flagWorld is empty and filepath.Dir("") returns "." —
	// the static serve then pointed at CWD, and the frontend's
	// GET /worlds/eldoria.json 404'd. Frontend shows entities as
	// rectangles on a black canvas with "world load failed:
	// Unexpected token '<', '<!doctype ...". Anchor worldsDir to
	// the actual source file we loaded from instead.
	worldsDir, _ := filepath.Abs(filepath.Dir(worldSrc))
	if filepath.Base(worldsDir) == filepath.Base(*flagBundle) {
		// Bundle layout is worlds/<bundle>/world.json. The frontend
		// requests /worlds/<name>.json — strip one more level so
		// /worlds/eldoria.json resolves to worlds/eldoria/world.json
		// via the alias below, and unrelated bundles under worlds/
		// still resolve correctly.
		worldsDir = filepath.Dir(worldsDir)
	}
	mux.Handle("/worlds/", http.StripPrefix("/worlds/",
		corsHandler(http.FileServer(http.Dir(worldsDir)))))

	// Bundle-mode alias: the frontend asks for /worlds/<name>.json
	// (legacy single-file naming). With -bundle the file is actually
	// at worlds/<name>/world.json. Map the alias so the frontend
	// doesn't need to know about bundles.
	if *flagBundle != "" {
		bundleName := filepath.Base(*flagBundle)
		alias := "/worlds/" + bundleName + ".json"
		mux.HandleFunc(alias, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, worldSrc)
		})
	}

	// Art lives inside the bundle: worlds/<bundle>/art/. Each world
	// owns its visual assets (manifests + processed sprites + style
	// anchor), so swapping bundles swaps the look-and-feel cleanly
	// without a shared top-level art/ directory full of unrelated
	// assets bleeding across worlds.
	//
	// bundleArtDir resolution:
	//   - bundle mode: worlds/<bundle>/art/
	//   - legacy single-file mode: same dir as world.json + /art
	bundleArtDir := filepath.Join(filepath.Dir(worldSrc), "art")
	mux.Handle("/art/", http.StripPrefix("/art/",
		corsHandler(http.FileServer(http.Dir(bundleArtDir)))))

	srv := &http.Server{
		Addr:              *flagAddr,
		Handler:           corsMid(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("engine listening on %s (scenario=%s)",
			*flagAddr, *flagScenario)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	// D11 — scale the tick interval by the time multiplier so the
	// engine ticks faster (more in-game time per real second) without
	// changing in-game-tick-denominated durations elsewhere.
	mult := *flagTimeMult
	if mult <= 0 {
		mult = 1.0
	}
	scaledTick := time.Duration(float64(tickDuration) / mult)
	if scaledTick < time.Millisecond {
		scaledTick = time.Millisecond
	}
	if mult != 1.0 {
		log.Printf("D11 time multiplier: %.2fx (scheduler tick = %v vs nominal %v)",
			mult, scaledTick, tickDuration)
	}
	tickTimer := time.NewTicker(scaledTick)
	defer tickTimer.Stop()

	// Snapshot timer — writes world state periodically when -snapshot-dir
	// is set. A final write fires at shutdown so the next boot picks up
	// the latest live state.
	var snapTimer *time.Ticker
	if *flagSnapDir != "" && *flagSnapEvery > 0 {
		snapTimer = time.NewTicker(*flagSnapEvery)
		defer snapTimer.Stop()
		log.Printf("snapshot every %v → %s", *flagSnapEvery, *flagSnapDir)
	}

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
			// Final snapshot before exit so the next boot resumes
			// from the live tick.
			if *flagSnapDir != "" {
				if p, err := persist.Write(w, *flagSnapDir); err != nil {
					log.Printf("final snapshot failed: %v", err)
				} else {
					log.Printf("final snapshot → %s", p)
				}
			}
			log.Println("clean exit")
			return

		case <-tickTimer.C:
			w.Tick()
			tick.Add(1)

		case <-tickerOrNop(snapTimer):
			if p, err := persist.Write(w, *flagSnapDir); err != nil {
				log.Printf("snapshot failed: %v", err)
			} else {
				log.Printf("snapshot → %s", p)
			}
		}
	}
}

// tickerOrNop returns the ticker's channel, or a nil channel if the
// ticker is nil. A receive on a nil channel blocks forever, which is
// exactly what we want for the optional snapshot timer in the main
// select loop.
func tickerOrNop(t *time.Ticker) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

// corsHandler — passthrough now that the real allowlist-based CORS
// middleware wraps the mux at the top level.
func corsHandler(h http.Handler) http.Handler { return h }

func splitComma(s string) []string {
	parts := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
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
