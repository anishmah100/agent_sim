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
	crand "crypto/rand"
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
	// hub — the multi-map hub. The overworld is `w`; building interiors are
	// additional maps. Per-entity observation/action calls route through
	// worldFor() so an agent that warped into an interior is served by that
	// interior's World. nil-safe: falls back to single-world `w`.
	hub *world.MultiMapHub

	// Experiment-level flag. When false, the engine drops the
	// `reasoning` field even if the agent opted in. Both must be true
	// for traces to land in the historian. See
	// docs/EXPERIMENT_SYSTEM_PLAN.md §8 + §11.
	captureReasoning bool

	// Optional callback the bus owner registers — receives the
	// (entityID, actionID, verb, reasoning) tuple when both flags align.
	// The wire layer doesn't import the historian directly, so main.go
	// glues them together. nil = no capture even if flags are on.
	OnReasoning func(entityID, actionID, verb, reasoning string)
	// OnReflection — installed by main.go so reflective notes (the
	// brain's slower "step back and think" output) land in the
	// historian under the agent_reasoning category. Layered opt-in
	// is identical to OnReasoning: needs both the engine flag and
	// the per-agent share_reasoning toggle.
	OnReflection func(entityID, note string)
	// OnMentalNote — D14. Generic architecture-agnostic mental-state
	// channel. Subsumes the legacy ReasoningTrace + ReflectiveNote
	// shapes into one shape any bot can emit on its own cadence.
	// Same layered opt-in (engine capture + per-agent share_reasoning).
	OnMentalNote func(entityID, text, tag string, slots map[string]string)
	// OnPerception — installed by main.go (only when -log-perceptions is
	// set) to record each agent's DELIVERED perception (the audible
	// channel) onto the tape. The referee (Phase 0.5) diffs emitted
	// directed events against these to catch silently-dropped
	// interactions. Called only on a successful send, and only when the
	// agent actually heard/witnessed something. nil = no perception log.
	OnPerception func(entityID string, tick uint64, audible []world.AudibleEvent)

	mu       sync.Mutex
	// agentSecret → agentRecord
	registry map[string]*agentRecord
	// agentID → live connection (only one at a time)
	live map[string]*agentConn
}

// SetCaptureReasoning toggles the engine-level reasoning capture flag.
// Call once at boot from main.go based on the experiment config.
func (h *AgentHub) SetCaptureReasoning(on bool) { h.captureReasoning = on }

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
	// AutoSpawned — true when the engine spawned a fresh body for
	// this agent at register time. Currently informational; the
	// disconnect cleanup keys on BoundExplicit instead.
	AutoSpawned bool
	// BoundExplicit — true when the caller passed a non-empty
	// bind_entity in the register payload. Disconnect cleanup leaves
	// the body alone for these (the caller asked for an existing
	// entity by id and expects it to persist); all other bodies are
	// removed so 1 agent = 1 visible body.
	BoundExplicit bool
	// RegisteredAt — unix milliseconds when HandleRegister created this
	// record. The reaper evicts records that never dialed /ws/agent within
	// a grace window (previously such records — and their auto-spawned
	// bodies — leaked forever; no TTL).
	RegisteredAt int64
	// ConnectedAt — unix milliseconds when the WS connect succeeded.
	// Zero before connect. Surfaced by /api/v1/agents so the picker UI
	// can show recency.
	ConnectedAt int64
	// LastVerb — last action verb the agent submitted (any verb,
	// accepted or not). Updated on every action frame so the picker
	// shows what the agent is currently trying to do.
	LastVerb    string
	// LastSpeech — most recent speak/shout/whisper text the agent
	// emitted. Updated when a speech-class action is received.
	LastSpeech  string
	// Guards LastVerb/LastSpeech across the handler + the picker
	// snapshot read.
	infoMu sync.Mutex

	// ShareReasoning is the per-agent opt-in for capturing the
	// `reasoning` trace attached to actions. Engine-level capture
	// (-capture-reasoning) must ALSO be true for traces to land in
	// the historian. See docs/EXPERIMENT_SYSTEM_PLAN.md §8.
	ShareReasoning bool
}

type agentConn struct {
	hub      *AgentHub
	rec      *agentRecord
	conn     *websocket.Conn
	send     chan []byte
	cadence  atomic.Int64
	lastObs  atomic.Uint64

	// Closed exactly once by readPump's defer to signal "stop sending".
	// All senders (observation loop, ack) must select on this; sending
	// to a closed `send` channel would otherwise panic.
	done     chan struct{}
	closedAt atomic.Bool

	actionsMu  sync.Mutex
	actionsWin []int64 // ms timestamps of recent actions; trimmed to last 1s
}

// trySend pushes data onto c.send without blocking and without racing
// against teardown. Returns true on success, false if the buffer is
// full or the connection is shutting down.
func (c *agentConn) trySend(data []byte) (ok bool) {
	if c.closedAt.Load() {
		return false
	}
	// TOCTOU: teardown can close c.send between the check above and the
	// select below (closedAt.Load may read false microseconds before the
	// defer's Swap+close completes). Sending on a closed channel panics —
	// and trySend runs on the SHARED observation loop, so one late send
	// would crash observations for EVERY agent (audit MEDIUM). Recover so a
	// racing send just fails (false) instead of taking the loop down.
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	select {
	case <-c.done:
		return false
	case c.send <- data:
		return true
	default:
		return false
	}
}

func NewAgentHub(ctx context.Context, w *world.World, hub *world.MultiMapHub) *AgentHub {
	h := &AgentHub{
		w:        w,
		hub:      hub,
		registry: make(map[string]*agentRecord),
		live:     make(map[string]*agentConn),
	}
	go h.observationLoop(ctx)
	go h.reapLoop(ctx)
	return h
}

// reapLoop periodically evicts registrations that were created but never
// connected (no /ws/agent dial within the grace window). Without this, a
// register that crashes/aborts before dialing leaks BOTH the registry entry
// and its auto-spawned body forever — there is no TTL on the register path,
// and the only other cleanup (readPump's defer) requires a successful
// connect+disconnect (audit MEDIUM).
func (h *AgentHub) reapLoop(ctx context.Context) {
	const graceMs = 60_000
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			h.reapStaleRegistrations(graceMs)
		}
	}
}

func (h *AgentHub) reapStaleRegistrations(graceMs int64) {
	now := nowMs()
	type victim struct {
		entityID string
		remove   bool
	}
	var victims []victim
	h.mu.Lock()
	for secret, rec := range h.registry {
		if rec.ConnectedAt != 0 {
			continue // connected (or once was); the disconnect path owns cleanup
		}
		if _, live := h.live[rec.AgentID]; live {
			continue // a connection is active right now
		}
		if now-rec.RegisteredAt < graceMs {
			continue // still within grace; the agent may yet connect
		}
		delete(h.registry, secret)
		victims = append(victims, victim{
			entityID: rec.EntityID,
			remove:   rec.AutoSpawned && !rec.BoundExplicit && rec.EntityID != "",
		})
	}
	h.mu.Unlock()
	// Remove leaked auto-spawned bodies OUTSIDE the hub lock (takes the world
	// write lock — mirrors readPump's disconnect cleanup). Never remove an
	// explicitly bound body (the caller owns it).
	removed := 0
	for _, v := range victims {
		if v.remove {
			wf := h.worldFor(v.entityID)
			wf.LockWrite()
			wf.RemoveEntity(v.entityID)
			wf.UnlockWrite()
			removed++
		}
	}
	if len(victims) > 0 {
		log.Printf("reaper: evicted %d never-connected registration(s); removed %d leaked bodies", len(victims), removed)
	}
}

// worldFor returns the World currently holding the entity (overworld or an
// interior). Falls back to the overworld when the hub is absent or the entity
// isn't found on any loaded map (caller handles a nil/gone entity).
func (h *AgentHub) worldFor(entityID string) *world.World {
	if h.hub != nil {
		if w := h.hub.WorldOf(entityID); w != nil {
			return w
		}
	}
	return h.w
}

// isEntityGone reports whether the entity is absent from EVERY loaded map
// (truly removed — death/loot cleanup), as opposed to merely having warped
// off the overworld into an interior. Used to decide when to close a dead
// agent's socket so the supervisor respawns it.
func (h *AgentHub) isEntityGone(entityID string) bool {
	if h.hub != nil {
		return h.hub.WorldOf(entityID) == nil
	}
	return h.w.EntityByID(entityID) == nil
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
		UserToken      string                 `json:"user_token"`
		PersonaBlob    map[string]any         `json:"persona_blob"`
		VisionMode     string                 `json:"vision_mode"`
		CadenceMs      int                    `json:"cadence_ms"`
		BindEntity     string                 `json:"bind_entity,omitempty"` // optional: claim an existing entity
		ShareReasoning bool                   `json:"share_reasoning,omitempty"`
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
	autoSpawned := false
	boundExplicit := req.BindEntity != ""
	if entityID == "" {
		for _, id := range h.w.EntityIDs() {
			e := h.w.EntityByID(id)
			if e == nil {
				continue
			}
			if !syscore.IsAgentArchetype(e.Archetype) {
				continue
			}
			// Skip entities already owned by a live agent OR by an
			// agent that has registered but not yet connected its WS.
			// Without the h.registry check, two near-simultaneous
			// registrations to a sparse world could both bind to the
			// same auto-spawned entity (a race surfaced when D3
			// stripped the 250 pre-declared bodies and every register
			// goes through SpawnAgentEntity).
			h.mu.Lock()
			taken := false
			for _, c := range h.live {
				if c.rec != nil && c.rec.EntityID == id {
					taken = true
					break
				}
			}
			if !taken {
				for _, r := range h.registry {
					if r.EntityID == id {
						taken = true
						break
					}
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
			// Auto-spawn a fresh wanderer at a random walkable tile. This
			// makes the engine elastic — any number of agents can join even
			// in a sparse world. The spawned entity is removed on agent
			// disconnect (see readPump cleanup).
			spawned, err := h.w.SpawnAgentEntity("wanderer", "")
			if err != nil {
				http.Error(rw, "spawn failed: "+err.Error(),
					http.StatusServiceUnavailable)
				return
			}
			entityID = spawned
			autoSpawned = true
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
		AgentID:        agentID,
		EntityID:       entityID,
		Secret:         secret,
		Persona:        req.PersonaBlob,
		VisionMode:     req.VisionMode,
		CadenceMs:      req.CadenceMs,
		ShareReasoning: req.ShareReasoning,
		AutoSpawned:    autoSpawned,
		BoundExplicit:  boundExplicit,
		RegisteredAt:   nowMs(),
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
		// Stomp the old connection. Signal teardown via done THEN close
		// the TCP; the old readPump will exit, but its defer compares
		// h.live[id] to itself and won't delete OUR new entry.
		prev.closedAt.Store(true)
		select {
		case <-prev.done:
			// already torn down
		default:
			close(prev.done)
		}
		_ = prev.conn.Close()
	}
	c := &agentConn{
		hub:  h,
		rec:  rec,
		conn: conn,
		send: make(chan []byte, 16),
		done: make(chan struct{}),
	}
	c.cadence.Store(int64(rec.CadenceMs))
	rec.ConnectedAt = nowMs()
	h.live[rec.AgentID] = c
	h.mu.Unlock()

	// Mark the entity as player-controlled so the engine's autonomous
	// wander loop stops overriding the bot's move commands.
	h.w.SetPlayerControlled(rec.EntityID, true)

	log.Printf("agent connect: %s (entity=%s)", rec.AgentID, rec.EntityID)
	go c.writePump()
	c.readPump()
	// PlayerControlled is cleared in readPump's defer, guarded by
	// stillOwner — a stomped reconnect must not clear the flag its live
	// replacement just set (audit HIGH: agent left wandering uncontrolled).
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
		obs := h.worldFor(c.rec.EntityID).BuildObservationFor(c.rec.EntityID, c.lastObs.Load()+1, nil)
		if obs == nil {
			// No observation: either the entity was removed (death/loot
			// cleanup) or the published snapshot is briefly behind a fresh
			// spawn. Distinguish via the live entity map — only a TRULY
			// gone entity gets its socket closed. Without this, a dead
			// agent's WS stayed open forever; the bot's observation loop
			// blocked on messages that never came, so supervise()'s
			// `await bot.run()` never returned and the agent was never
			// respawned — the population bled out over a long run
			// (sustain bug). Closing the conn ends observations() → run()
			// returns → the supervisor registers a replacement.
			if h.isEntityGone(c.rec.EntityID) && !c.closedAt.Load() {
				c.conn.Close()
			}
			continue
		}
		msg := map[string]any{
			"type":              "observation",
			"obs_id":            obs.ObsID,
			"world_tick":        obs.WorldTick,
			"self":              obs.Self,
			"visible_entities":  obs.VisibleEntities,
			"visible_objects":   obs.VisibleObjects,
			// D8 — items the agent can see. CRITICAL: this MUST be
			// in the map. The hand-rolled serialization used to drop
			// it, which silently broke every items-aware feature
			// (survivor money-seeking, killer weapon pickup, scavenger
			// looting, wanderer coin grab). Discovered by comparing
			// the /api/v1/debug/vision endpoint output (which uses
			// BuildObservationFor directly) against the WS payload.
			"visible_items":     obs.VisibleItems,
			"audible":           obs.Audible,
			// local_view — the egocentric ASCII terrain window. Like
			// visible_items above, the hand-rolled map MUST list it or the
			// WS payload silently drops it (the struct's json tag only
			// matters for json.Marshal of the whole Observation, which this
			// path does NOT do). Without this agents are terrain-blind.
			"local_view":        obs.LocalView,
			"world_clock":       obs.WorldClock,
		}
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		if c.trySend(data) {
			c.lastObs.Store(uint64(now))
			// Tape the perception half: only on a real delivery, and only
			// when the agent actually heard/witnessed something (keeps the
			// log proportional to social events, not 60Hz × N agents).
			if h.OnPerception != nil && len(obs.Audible) > 0 {
				h.OnPerception(c.rec.EntityID, obs.WorldTick, obs.Audible)
			}
		}
		// Backpressure / shutdown — drop silently; the agent recovers
		// on its next cadence.
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
		// Signal "no more sends" BEFORE anything else; subsequent
		// trySend/ack calls return immediately.
		if !c.closedAt.Swap(true) {
			close(c.done)
		}
		// Compare-and-delete: only remove h.live[id] if it's still US.
		// A new register might have stomped over us with a fresh
		// connection; in that case the new one owns the slot.
		c.hub.mu.Lock()
		stillOwner := false
		if cur, ok := c.hub.live[c.rec.AgentID]; ok && cur == c {
			delete(c.hub.live, c.rec.AgentID)
			stillOwner = true
		}
		// Forget the registry entry too so the entity tile slot is
		// released for future register() calls — without this, the
		// next bot to register sees the dead body as "taken" until
		// the engine restarts. Guarded by stillOwner: a STOMPED
		// connection's teardown must not purge the secret its live
		// replacement is still using (unguarded, same-secret re-auth
		// was one-shot — the next reconnect got auth_invalid).
		if stillOwner {
			delete(c.hub.registry, c.rec.Secret)
		}
		c.hub.mu.Unlock()
		// Clear player-control on a REAL disconnect (stillOwner) so the body
		// resumes autonomous behavior. A STOMPED connection must NOT clear it
		// — its live replacement already set PlayerControlled=true, and an
		// unconditional clear here left the reconnected agent uncontrolled,
		// wandering autonomously instead of obeying its bot (audit HIGH).
		if stillOwner {
			c.hub.worldFor(c.rec.EntityID).SetPlayerControlled(c.rec.EntityID, false)
		}
		// Closing c.send AFTER closedAt+done are set lets any in-flight
		// sender bail out via trySend's c.closedAt check.
		close(c.send)
		// Remove the body on disconnect so it doesn't ghost. The user
		// saw "3 agents on screen, only 2 in connected list" because
		// the register handler binds to any agent-eligible entity it
		// finds — including NPC supervisor bodies — and leaves them
		// dangling on disconnect. Removing the entity here keeps
		// 1-agent-connected == 1-body-on-screen as the invariant.
		// Skip when bind_entity was explicitly passed: the caller
		// claimed an existing entity by id and expects it to persist.
		if stillOwner && !c.rec.BoundExplicit && c.rec.EntityID != "" {
			c.hub.w.LockWrite()
			c.hub.w.RemoveEntity(c.rec.EntityID)
			c.hub.w.UnlockWrite()
		}
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
			// Echo action_id/verb like every other rejection path — an
			// ack without action_id can never resolve the SDK's pending
			// future, so wait-for-ack callers stalled the full timeout.
			var hdr2 struct {
				ActionID string `json:"action_id"`
				Verb     string `json:"verb"`
			}
			_ = json.Unmarshal(raw, &hdr2)
			c.ack(world.ActionResult{ActionID: hdr2.ActionID, Verb: hdr2.Verb,
				Accepted: false, Reason: "rate_limited"})
			return
		}
		var env world.ActionEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return
		}
		// Layered reasoning opt-in: forward to historian iff BOTH the
		// experiment-level capture flag and the per-agent share flag are
		// on, AND main.go installed an OnReasoning callback. Drop the
		// reasoning field on the floor otherwise so it never reaches the
		// trace log.
		if env.Reasoning != "" &&
			c.hub.captureReasoning &&
			c.rec.ShareReasoning &&
			c.hub.OnReasoning != nil {
			c.hub.OnReasoning(c.rec.EntityID, env.ActionID, env.Verb, env.Reasoning)
		}
		// Picker telemetry: track the latest verb + speech so the UI
		// agent picker shows what the agent is currently doing.
		c.rec.infoMu.Lock()
		c.rec.LastVerb = env.Verb
		switch env.Verb {
		case "speak", "shout", "whisper":
			var p struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(env.Raw, &p) == nil && p.Text != "" {
				c.rec.LastSpeech = p.Text
			}
		}
		c.rec.infoMu.Unlock()
		res := c.hub.worldFor(c.rec.EntityID).SubmitAction(c.rec.EntityID, &env)
		// Diagnostic: log every rejection with the dispatcher's reason
		// so the smoke + downstream analysis can see what verbs the
		// agent attempted but the engine refused. Without this, an
		// agent that thinks it's acting but is being rejected (bad
		// target, locked door, target_too_far, ...) looks identical
		// to one that's idle.
		if !res.Accepted {
			log.Printf("action rejected: agent=%s entity=%s verb=%s reason=%q",
				c.rec.AgentID, c.rec.EntityID, env.Verb, res.Reason)
		}
		c.ack(res)
	case "reflection":
		// Per-agent reflective layer output. Routes to the historian
		// the same way reasoning does — gated on both the engine-level
		// capture flag AND the per-agent share_reasoning opt-in, so a
		// quiet agent in a noisy run stays quiet. The hub's
		// OnReflection callback handles the actual write.
		var p struct {
			Note string `json:"note"`
		}
		if err := json.Unmarshal(raw, &p); err != nil || p.Note == "" {
			return
		}
		if c.hub.captureReasoning &&
			c.rec.ShareReasoning &&
			c.hub.OnReflection != nil {
			c.hub.OnReflection(c.rec.EntityID, p.Note)
		}
	case "mental_note":
		// D14 — generic mental-state channel. text + optional tag +
		// optional slots map ({goal, plan, beliefs, emotion} are the
		// recommended keys, but any subset/none is accepted). Same
		// layered opt-in as reflection.
		var p struct {
			Text  string            `json:"text"`
			Tag   string            `json:"tag"`
			Slots map[string]string `json:"slots"`
		}
		if err := json.Unmarshal(raw, &p); err != nil || p.Text == "" {
			return
		}
		if c.hub.captureReasoning &&
			c.rec.ShareReasoning &&
			c.hub.OnMentalNote != nil {
			c.hub.OnMentalNote(c.rec.EntityID, p.Text, p.Tag, p.Slots)
		}
	case "set_cadence":
		var p struct{ IntervalMs int `json:"interval_ms"` }
		if err := json.Unmarshal(raw, &p); err == nil && p.IntervalMs >= 200 {
			c.cadence.Store(int64(p.IntervalMs))
		}
	case "ping":
		c.trySend([]byte(`{"type":"pong"}`))
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
	c.trySend(data)
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

// genID — short URL-safe random ID. H3 FIX: was seeded from
// time.Now().UnixNano() with a 1ns sleep per char, making agent SECRETS
// guessable from wall-clock and collision-prone under coarse clocks
// (auth is the only gate on controlling an entity). Use crypto/rand.
func genID(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, n)
	if _, err := crand.Read(buf); err != nil {
		// Extremely unlikely; fall back to a non-secret but unique-ish id
		// rather than panicking the register handler.
		for i := range buf {
			buf[i] = alphabet[(int(buf[i])+i)%len(alphabet)]
		}
		return string(buf)
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}
