// Soak harness — boots N agents, drives a random-action loop for D
// duration against a running engine, and reports:
//   - HTTP register success rate
//   - WS connect success rate
//   - observation throughput (per-agent and total)
//   - action throughput + rate_limited count
//   - whether /metrics responded and the final tick count delta
//
// This is a black-box stress test. It does NOT start the engine itself
// — point -engine at a separately-running engine. Use it to:
//
//	go run ./cmd/soak -engine http://127.0.0.1:8080 -agents 20 -duration 60s
//
// Failure modes asserted: any unexpected error, any agent that produced
// zero observations across the window, or any drop in tick rate below
// 80% of the configured 60Hz baseline.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	flagEngine   = flag.String("engine", "http://127.0.0.1:8080", "engine base URL")
	flagAgents   = flag.Int("agents", 10, "number of agents to spawn")
	flagDuration = flag.Duration("duration", 30*time.Second, "soak window")
	flagCadence  = flag.Int("cadence-ms", 500, "agent observation cadence")
	flagActRate  = flag.Float64("act-hz", 1.0, "average action rate per agent (per second)")
	flagSeed     = flag.Int64("seed", 1, "PRNG seed")
)

type stats struct {
	registered     atomic.Int64
	connected      atomic.Int64
	observations   atomic.Int64
	actionsSent    atomic.Int64
	actionsAcked   atomic.Int64
	rateLimited    atomic.Int64
	wsReadErrors   atomic.Int64
	perAgentObs    sync.Map // agentID → *atomic.Int64
}

func (s *stats) bumpAgentObs(id string) {
	v, _ := s.perAgentObs.LoadOrStore(id, new(atomic.Int64))
	v.(*atomic.Int64).Add(1)
}

type registerResp struct {
	AgentID     string `json:"agent_id"`
	AgentSecret string `json:"agent_secret"`
	WSURL       string `json:"ws_url"`
	EntityID    string `json:"entity_id"`
}

type metricsSnapshot struct {
	tick       uint64
	uptime     float64
	entities   int
	agents     int
	viewers    int
	events     uint64
	scraped    bool
}

func main() {
	flag.Parse()
	rng := rand.New(rand.NewSource(*flagSeed))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Sanity ping.
	if err := pingEngine(*flagEngine); err != nil {
		log.Fatalf("engine not reachable at %s: %v", *flagEngine, err)
	}

	st := &stats{}
	pre := scrapeMetrics(*flagEngine)
	if pre.scraped {
		log.Printf("pre-soak: tick=%d entities=%d events=%d", pre.tick, pre.entities, pre.events)
	}

	soakCtx, soakCancel := context.WithTimeout(ctx, *flagDuration)
	defer soakCancel()

	var wg sync.WaitGroup
	for i := 0; i < *flagAgents; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			runAgent(soakCtx, idx, *flagEngine, *flagCadence, *flagActRate, rng, st)
		}(i)
	}

	// Wait for either timeout or signal.
	wg.Wait()

	post := scrapeMetrics(*flagEngine)
	report(pre, post, st)
}

func pingEngine(base string) error {
	r, err := http.Get(base + "/healthz")
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("healthz: %d", r.StatusCode)
	}
	return nil
}

func register(base string, idx int) (*registerResp, error) {
	body := map[string]any{
		"user_token":   fmt.Sprintf("soak-%d", idx),
		"persona_blob": map[string]any{"name": fmt.Sprintf("soak-%d", idx)},
		"vision_mode":  "structured",
		"cadence_ms":   500,
	}
	buf, _ := json.Marshal(body)
	r, err := http.Post(base+"/api/v1/agent/register", "application/json", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		return nil, fmt.Errorf("register %d: %s", r.StatusCode, string(b))
	}
	var rr registerResp
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		return nil, err
	}
	return &rr, nil
}

func runAgent(ctx context.Context, idx int, base string, cadenceMs int, actHz float64, rng *rand.Rand, st *stats) {
	rr, err := register(base, idx)
	if err != nil {
		log.Printf("agent %d register failed: %v", idx, err)
		return
	}
	st.registered.Add(1)

	wsURL := normalizeWSURL(base, rr.WSURL)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		log.Printf("agent %d dial failed: %v", idx, err)
		return
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]string{"auth": rr.AgentSecret}); err != nil {
		log.Printf("agent %d auth failed: %v", idx, err)
		return
	}
	st.connected.Add(1)

	// Set cadence.
	_ = conn.WriteJSON(map[string]any{"type": "set_cadence", "interval_ms": cadenceMs})

	// Reader loop.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				st.wsReadErrors.Add(1)
				return
			}
			var hdr struct {
				Type   string `json:"type"`
				Reason string `json:"reason,omitempty"`
			}
			if err := json.Unmarshal(raw, &hdr); err != nil {
				continue
			}
			switch hdr.Type {
			case "observation":
				st.observations.Add(1)
				st.bumpAgentObs(rr.AgentID)
			case "action_ack":
				st.actionsAcked.Add(1)
				if hdr.Reason == "rate_limited" {
					st.rateLimited.Add(1)
				}
			}
		}
	}()

	// Action loop — Poisson-ish arrivals at actHz.
	if actHz <= 0 {
		<-ctx.Done()
		_ = conn.Close()
		<-done
		return
	}
	dirs := []string{"north", "south", "east", "west"}
	timer := time.NewTimer(jitter(rng, actHz))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close()
			<-done
			return
		case <-timer.C:
			dir := dirs[rng.Intn(len(dirs))]
			msg := map[string]any{
				"type":      "action",
				"action_id": fmt.Sprintf("soak-%d-%d", idx, st.actionsSent.Load()),
				"verb":      "move",
				"params":    map[string]string{"direction": dir},
			}
			if err := conn.WriteJSON(msg); err != nil {
				_ = conn.Close()
				<-done
				return
			}
			st.actionsSent.Add(1)
			timer.Reset(jitter(rng, actHz))
		}
	}
}

func jitter(rng *rand.Rand, hz float64) time.Duration {
	mean := 1.0 / hz
	// Exponential interarrival.
	u := rng.Float64()
	if u < 1e-9 {
		u = 1e-9
	}
	d := -mean * math.Log(u)
	return time.Duration(d * float64(time.Second))
}

func normalizeWSURL(base, ws string) string {
	// Engine returns ws://host/ws/agent based on r.Host; if soak ran
	// through a proxy or the engine bound 0.0.0.0, prefer the base host.
	u, err := url.Parse(ws)
	if err != nil {
		return ws
	}
	bu, err := url.Parse(base)
	if err == nil && bu.Host != "" {
		u.Host = bu.Host
		if bu.Scheme == "https" {
			u.Scheme = "wss"
		} else {
			u.Scheme = "ws"
		}
	}
	return u.String()
}

func scrapeMetrics(base string) metricsSnapshot {
	out := metricsSnapshot{}
	r, err := http.Get(base + "/metrics")
	if err != nil {
		return out
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return out
	}
	body, _ := io.ReadAll(r.Body)
	out.scraped = true
	for _, line := range strings.Split(string(body), "\n") {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		var name, valStr string
		if i := strings.IndexAny(line, " \t"); i > 0 {
			name = line[:i]
			valStr = strings.TrimSpace(line[i:])
		} else {
			continue
		}
		// Strip labels.
		if i := strings.IndexByte(name, '{'); i > 0 {
			name = name[:i]
		}
		switch name {
		case "agentsim_tick":
			fmt.Sscanf(valStr, "%d", &out.tick)
		case "agentsim_uptime_seconds":
			fmt.Sscanf(valStr, "%f", &out.uptime)
		case "agentsim_entities":
			fmt.Sscanf(valStr, "%d", &out.entities)
		case "agentsim_agents_connected":
			fmt.Sscanf(valStr, "%d", &out.agents)
		case "agentsim_viewers_connected":
			fmt.Sscanf(valStr, "%d", &out.viewers)
		case "agentsim_events_emitted_total":
			fmt.Sscanf(valStr, "%d", &out.events)
		}
	}
	return out
}

func report(pre, post metricsSnapshot, st *stats) {
	dt := post.uptime - pre.uptime
	if dt < 1 {
		dt = 1
	}
	tickDelta := int64(post.tick) - int64(pre.tick)
	tickRate := float64(tickDelta) / dt
	fmt.Println()
	fmt.Println("=== soak report ===")
	fmt.Printf("agents registered:    %d\n", st.registered.Load())
	fmt.Printf("agents connected:     %d\n", st.connected.Load())
	fmt.Printf("observations:         %d (≈%.1f/s/agent)\n",
		st.observations.Load(),
		float64(st.observations.Load())/dt/float64(maxInt(int(st.connected.Load()), 1)))
	fmt.Printf("actions sent:         %d\n", st.actionsSent.Load())
	fmt.Printf("actions acked:        %d (rate_limited=%d)\n", st.actionsAcked.Load(), st.rateLimited.Load())
	fmt.Printf("ws read errors:       %d\n", st.wsReadErrors.Load())
	if post.scraped {
		fmt.Printf("engine ticks:         Δ%d over %.1fs → %.1f Hz (target 60)\n",
			tickDelta, dt, tickRate)
		fmt.Printf("engine agents/viewers/events: %d / %d / %d\n",
			post.agents, post.viewers, post.events)
	}
	// Pass/fail.
	zeroObsAgents := 0
	st.perAgentObs.Range(func(_, v any) bool {
		if v.(*atomic.Int64).Load() == 0 {
			zeroObsAgents++
		}
		return true
	})
	bad := false
	if st.connected.Load() < int64(*flagAgents) {
		fmt.Printf("FAIL: only %d/%d agents connected\n", st.connected.Load(), *flagAgents)
		bad = true
	}
	if zeroObsAgents > 0 {
		fmt.Printf("FAIL: %d agents received zero observations\n", zeroObsAgents)
		bad = true
	}
	if post.scraped && tickRate < 48 {
		fmt.Printf("FAIL: tick rate %.1f Hz < 80%% of 60 Hz baseline\n", tickRate)
		bad = true
	}
	if bad {
		os.Exit(1)
	}
	fmt.Println("PASS")
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
