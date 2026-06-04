// Agent WebSocket handler.
//
// Lifecycle:
//   1. Agent POSTs /api/v1/agent/register → engine returns agent_id +
//      agent_secret + ws_url.
//   2. Agent dials ws_url. First message MUST be `{"auth": <secret>}`.
//   3. Engine binds the connection to the agent's entity_id.
//   4. Engine pushes observations at the configured cadence; agent
//      sends action / set_cadence / ping messages.
//
// Cadence + rate limiting:
//   - Default cadence 1000 ms. Min 200 ms (5 Hz max). The model wants
//     "up to 1 Hz" per the vision doc, but we allow short bursts.
//   - Per-connection action rate cap: 30/sec. Excess is dropped with
//     an action_ack reason="rate_limited".

package wire

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	syscore "github.com/anishmah100/agent_sim/engine/internal/core/systems"
	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// AgentHub manages registered agents + their live connections.
type AgentHub struct {
	w *world.World

	mu       sync.Mutex
	// agentSecret → agentRecord
	registry map[string]*agentRecord
	// agentID → live connection (only one at a time)
	live map[string]*agentConn
}

// Count returns the number of currently-connected agents (used by
// the /metrics endpoint).
func (h *AgentHub) Count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.live)
}

type agentRecord struct {
	AgentID   string
	EntityID  string
	Secret    string
	Persona   map[string]any
	VisionMode string
	CadenceMs int
}

type agentConn struct {
	hub      *AgentHub
	rec      *agentRecord
	conn     *websocket.Conn
	send     chan []byte
	cadence  atomic.Int64
	lastObs  atomic.Uint64

	actionsMu  sync.Mutex
	actionsWin []int64 // ms timestamps of recent actions; trimmed to last 1s
}

func NewAgentHub(ctx context.Context, w *world.World) *AgentHub {
	h := &AgentHub{
		w:        w,
		registry: make(map[string]*agentRecord),
		live:     make(map[string]*agentConn),
	}
	go h.observationLoop(ctx)
	return h
}

// HandleRegister responds to POST /api/v1/agent/register.
// Body: { user_token, persona_blob, vision_mode, cadence_ms }
// Resp: { agent_id, agent_secret, ws_url }
func (h *AgentHub) HandleRegister(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserToken   string                 `json:"user_token"`
		PersonaBlob map[string]any         `json:"persona_blob"`
		VisionMode  string                 `json:"vision_mode"`
		CadenceMs   int                    `json:"cadence_ms"`
		BindEntity  string                 `json:"bind_entity,omitempty"` // optional: claim an existing entity
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(rw, "bad json", http.StatusBadRequest)
		return
	}
	if req.VisionMode == "" {
		req.VisionMode = "structured"
	}
	if req.CadenceMs <= 0 {
		req.CadenceMs = 1000
	}
	if req.CadenceMs < 200 {
		req.CadenceMs = 200
	}

	// Bind to an existing entity (e.g. an NPC the user wants to control)
	// or pick the first agent-eligible one for demo flow.
	// Hard rule: an agent can only attach to an agent-archetype entity.
	// Binding to a tree / rock / building / item is rejected — those are
	// world objects, not bodies.
	entityID := req.BindEntity
	if entityID == "" {
		for _, id := range h.w.EntityIDs() {
			e := h.w.EntityByID(id)
			if e == nil {
				continue
			}
			if !syscore.IsAgentArchetype(e.Archetype) {
				continue
			}
			// Skip entities already owned by a live agent.
			h.mu.Lock()
			taken := false
			for _, c := range h.live {
				if c.rec != nil && c.rec.EntityID == id {
					taken = true
					break
				}
			}
			h.mu.Unlock()
			if taken {
				continue
			}
			entityID = id
			break
		}
		if entityID == "" {
			http.Error(rw, "no free agent-eligible entities", http.StatusServiceUnavailable)
			return
		}
	}
	target := h.w.EntityByID(entityID)
	if target == nil {
		http.Error(rw, "unknown bind_entity", http.StatusNotFound)
		return
	}
	if !syscore.IsAgentArchetype(target.Archetype) {
		http.Error(rw, "entity is a world object, not an agent body", http.StatusBadRequest)
		return
	}

	agentID := genID(16)
	secret := genID(32)
	rec := &agentRecord{
		AgentID:    agentID,
		EntityID:   entityID,
		Secret:     secret,
		Persona:    req.PersonaBlob,
		VisionMode: req.VisionMode,
		CadenceMs:  req.CadenceMs,
	}
	h.mu.Lock()
	h.registry[secret] = rec
	h.mu.Unlock()

	scheme := "ws"
	if r.TLS != nil {
		scheme = "wss"
	}
	wsURL := scheme + "://" + r.Host + "/ws/agent"
	resp := map[string]string{
		"agent_id":     agentID,
		"agent_secret": secret,
		"ws_url":       wsURL,
		"entity_id":    entityID,
	}
	rw.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(rw).Encode(resp)
}

// HandleWS — upgrades, expects auth as first message, then runs the
// duplex loop for the agent connection.
func (h *AgentHub) HandleWS(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Printf("agent upgrade: %v", err)
		return
	}
	// Auth — first frame.
	_, raw, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return
	}
	var auth struct{ Auth string `json:"auth"` }
	if err := json.Unmarshal(raw, &auth); err != nil || auth.Auth == "" {
		_ = conn.WriteJSON(map[string]string{"error": "auth_required"})
		conn.Close()
		return
	}
	h.mu.Lock()
	rec, ok := h.registry[auth.Auth]
	if !ok {
		h.mu.Unlock()
		_ = conn.WriteJSON(map[string]string{"error": "auth_invalid"})
		conn.Close()
		return
	}
	if prev, hasLive := h.live[rec.AgentID]; hasLive {
		_ = prev.conn.Close()
	}
	c := &agentConn{
		hub:  h,
		rec:  rec,
		conn: conn,
		send: make(chan []byte, 16),
	}
	c.cadence.Store(int64(rec.CadenceMs))
	h.live[rec.AgentID] = c
	h.mu.Unlock()

	log.Printf("agent connect: %s (entity=%s)", rec.AgentID, rec.EntityID)
	go c.writePump()
	c.readPump()
}

// observationLoop ticks every 100 ms; for each live agent, if their
// cadence has elapsed since the last observation, build + push a new
// one.
func (h *AgentHub) observationLoop(ctx context.Context) {
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.tickObservations()
		}
	}
}

func (h *AgentHub) tickObservations() {
	now := time.Now().UnixMilli()
	h.mu.Lock()
	conns := make([]*agentConn, 0, len(h.live))
	for _, c := range h.live {
		conns = append(conns, c)
	}
	h.mu.Unlock()
	for _, c := range conns {
		cad := c.cadence.Load()
		if now-int64(c.lastObs.Load()) < cad {
			continue
		}
		obs := h.w.BuildObservationFor(c.rec.EntityID, c.lastObs.Load()+1, nil)
		if obs == nil {
			continue
		}
		msg := map[string]any{
			"type":              "observation",
			"obs_id":            obs.ObsID,
			"world_tick":        obs.WorldTick,
			"self":              obs.Self,
			"visible_entities":  obs.VisibleEntities,
			"visible_objects":   obs.VisibleObjects,
			"audible":           obs.Audible,
			"known_map_summary": obs.KnownMap,
			"world_clock":       obs.WorldClock,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		select {
		case c.send <- data:
			c.lastObs.Store(uint64(now))
		default:
			// Backpressure — drop. Agent will catch up next cadence.
		}
	}
}

func (c *agentConn) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func (c *agentConn) readPump() {
	defer func() {
		c.hub.mu.Lock()
		delete(c.hub.live, c.rec.AgentID)
		c.hub.mu.Unlock()
		close(c.send)
		log.Printf("agent disconnect: %s", c.rec.AgentID)
	}()
	c.conn.SetReadLimit(1 << 16)
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		c.handleMessage(raw)
	}
}

func (c *agentConn) handleMessage(raw []byte) {
	var hdr struct{ Type string `json:"type"` }
	if err := json.Unmarshal(raw, &hdr); err != nil {
		return
	}
	switch hdr.Type {
	case "action":
		if !c.allowAction() {
			c.ack(world.ActionResult{Accepted: false, Reason: "rate_limited"})
			return
		}
		var env world.ActionEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return
		}
		res := c.hub.w.SubmitAction(c.rec.EntityID, &env)
		c.ack(res)
	case "set_cadence":
		var p struct{ IntervalMs int `json:"interval_ms"` }
		if err := json.Unmarshal(raw, &p); err == nil && p.IntervalMs >= 200 {
			c.cadence.Store(int64(p.IntervalMs))
		}
	case "ping":
		c.send <- []byte(`{"type":"pong"}`)
	}
}

func (c *agentConn) ack(res world.ActionResult) {
	msg := map[string]any{
		"type":      "action_ack",
		"action_id": res.ActionID,
		"verb":      res.Verb,
		"accepted":  res.Accepted,
		"reason":    res.Reason,
	}
	data, _ := json.Marshal(msg)
	select {
	case c.send <- data:
	default:
	}
}

func (c *agentConn) allowAction() bool {
	now := time.Now().UnixMilli()
	c.actionsMu.Lock()
	defer c.actionsMu.Unlock()
	cutoff := now - 1000
	i := 0
	for ; i < len(c.actionsWin); i++ {
		if c.actionsWin[i] >= cutoff {
			break
		}
	}
	c.actionsWin = c.actionsWin[i:]
	if len(c.actionsWin) >= 30 {
		return false
	}
	c.actionsWin = append(c.actionsWin, now)
	return true
}

// genID — short URL-safe random ID.
func genID(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[time.Now().UnixNano()%int64(len(alphabet))]
		time.Sleep(time.Nanosecond)
	}
	return string(out)
}
