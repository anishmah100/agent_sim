// Package wire handles the WebSocket protocol for viewers (browser) and
// agents (LLM/rule bots). Hot path is the broadcast loop in ViewerHub
// which sends World snapshots at ~30Hz to every connected viewer.
//
// v0 uses JSON over WS so we can debug in browser devtools. The schema
// freezes at milestone 5 when we move to FlatBuffers + delta encoding.
package wire

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// PushHz is how often we broadcast world state to viewers. 30 is the
// MMO standard. Cheap when there are no viewers (zero-allocation
// fast-path check below).
const PushHz = 30
const pushInterval = time.Second / PushHz

// upgrader: accept any origin in v0. Real CORS lockdown lands when
// the deploy story is fixed.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ViewerHub owns the set of connected viewer WebSockets and the
// broadcast goroutine that pushes snapshots to all of them.
type ViewerHub struct {
	w *world.World
	// hub — multi-map hub; lets the viewer broadcast building-interior
	// occupants so the frontend can render agents inside (portal sub-map model).
	hub *world.MultiMapHub

	mu      sync.Mutex
	clients map[*viewerConn]struct{}
}

type viewerConn struct {
	conn *websocket.Conn
	send chan []byte
	hub  *ViewerHub
	addr string
	// done is closed by the read pump's defer once teardown begins, so
	// the broadcast loop's non-blocking sends can bail instead of racing
	// against an in-progress channel close.
	done     chan struct{}
	closedAt atomic.Bool
}

// trySend pushes a frame into c.send without blocking and without
// racing against teardown. Returns true on success.
func (c *viewerConn) trySend(data []byte) (ok bool) {
	if c.closedAt.Load() {
		return false
	}
	// Same TOCTOU as agentConn.trySend: a racing teardown can close c.send
	// between the check and the select; recover so a late send fails (false)
	// instead of panicking the viewer broadcast goroutine (audit MEDIUM).
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

// NewViewerHub starts the broadcast goroutine and returns a hub ready
// to accept HTTP handlers for new connections.
func NewViewerHub(ctx context.Context, w *world.World, hub *world.MultiMapHub) *ViewerHub {
	h := &ViewerHub{
		w:       w,
		hub:     hub,
		clients: make(map[*viewerConn]struct{}),
	}
	go h.broadcastLoop(ctx)
	return h
}

// Handle is the http.Handler for /ws/viewer. Upgrades, registers,
// blocks until disconnect.
func (h *ViewerHub) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("viewer upgrade: %v", err)
		return
	}
	c := &viewerConn{
		conn: conn,
		send: make(chan []byte, 8),
		hub:  h,
		addr: r.RemoteAddr,
		done: make(chan struct{}),
	}
	h.register(c)
	log.Printf("viewer connect: %s (total=%d)", c.addr, h.count())

	// Send an immediate full snapshot so the page doesn't render empty
	// while it waits for the next broadcast tick. Uses trySend so a
	// hostile client that ignores frames can't wedge us at connect.
	if msg, err := h.encodeSnapshot(); err == nil {
		c.trySend(msg)
	}

	// Two goroutines per connection: the writer drains the send chan
	// to the socket; the reader drains incoming messages so the
	// gorilla state machine stays healthy (we don't process viewer
	// commands yet — that lands in milestone 7+).
	go c.writePump()
	c.readPump()
}

func (h *ViewerHub) register(c *viewerConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *ViewerHub) unregister(c *viewerConn) {
	// Mark closed FIRST so any in-flight broadcast can bail out via
	// trySend's atomic check before we close the channel.
	if !c.closedAt.Swap(true) {
		close(c.done)
	}
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

func (h *ViewerHub) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// Count is the public version for the metrics exporter.
func (h *ViewerHub) Count() int { return h.count() }

// broadcastLoop pushes a JSON-encoded WorldSnapshot to every connected
// viewer at PushHz. Fast-paths when nobody is listening so we don't
// pay the JSON encode cost during quiet periods.
func (h *ViewerHub) broadcastLoop(ctx context.Context) {
	tk := time.NewTicker(pushInterval)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			// Cheap check: no viewers, skip the encode.
			h.mu.Lock()
			n := len(h.clients)
			h.mu.Unlock()
			if n == 0 {
				continue
			}
			msg, err := h.encodeSnapshot()
			if err != nil {
				log.Printf("encode snapshot: %v", err)
				continue
			}
			// Snapshot the client set under the lock, then fan out the
			// non-blocking sends OUTSIDE the lock. That keeps the
			// register/unregister fast path responsive even when the
			// fan-out has many viewers — critical at 1k+ concurrent
			// connections.
			h.mu.Lock()
			conns := make([]*viewerConn, 0, len(h.clients))
			for c := range h.clients {
				conns = append(conns, c)
			}
			h.mu.Unlock()
			for _, c := range conns {
				c.trySend(msg)
			}
		}
	}
}

type viewerMessage struct {
	Type     string               `json:"type"`
	Snapshot *world.WorldSnapshot `json:"snapshot,omitempty"`
	Audible  []world.AudibleEvent `json:"audible,omitempty"`
	// Interiors — one entry per currently-occupied building interior
	// (portal sub-map model). Lets the frontend render the live agents inside a
	// building when the viewer follows/opens it. Empty/absent when nobody
	// is inside anything.
	Interiors []interiorView `json:"interiors,omitempty"`
}

// interiorView — a building interior's live occupants for the viewer. Keyed by
// the building instance (sprite + overworld door tile) so the frontend can
// match it to the building the user clicked/followed.
type interiorView struct {
	MapID    string          `json:"map_id"`
	Sprite   string          `json:"sprite"`
	Door     [2]int          `json:"door"`
	Width    int             `json:"width_tiles"`
	Height   int             `json:"height_tiles"`
	Entities []*world.Entity `json:"entities"`
}

func (h *ViewerHub) encodeSnapshot() ([]byte, error) {
	snap := h.w.Snapshot()
	// Include all public audible events from the recent window so the
	// frontend can render speech bubbles. We use a 2-second window —
	// at 60Hz that's 120 ticks — to give the bubbles time to fade.
	since := uint64(0)
	if snap.Tick > 120 {
		since = snap.Tick - 120
	}
	audible := h.w.RecentAudibleAll(since)
	return json.Marshal(viewerMessage{
		Type:      "world_snapshot",
		Snapshot:  &snap,
		Audible:   audible,
		Interiors: h.interiorViews(),
	})
}

// interiorViews collects the live occupants of every loaded building interior.
func (h *ViewerHub) interiorViews() []interiorView {
	if h.hub == nil {
		return nil
	}
	var out []interiorView
	for _, id := range h.hub.Maps() {
		if !strings.HasPrefix(id, "interior:") {
			continue
		}
		iw := h.hub.Get(id)
		if iw == nil {
			continue
		}
		isnap := iw.Snapshot()
		if len(isnap.Entities) == 0 {
			continue // empty room — nothing to render
		}
		sprite, door := world.ParseInteriorMapID(id)
		out = append(out, interiorView{
			MapID:    id,
			Sprite:   sprite,
			Door:     door,
			Width:    isnap.WidthTiles,
			Height:   isnap.HeightTiles,
			Entities: isnap.Entities,
		})
	}
	return out
}

func (c *viewerConn) writePump() {
	pingTicker := time.NewTicker(30 * time.Second)
	defer func() {
		pingTicker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Hub closed the channel — connection is unregistered.
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-pingTicker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *viewerConn) readPump() {
	defer func() {
		c.hub.unregister(c)
		_ = c.conn.Close()
		log.Printf("viewer disconnect: %s (total=%d)", c.addr, c.hub.count())
	}()
	c.conn.SetReadLimit(64 * 1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(70 * time.Second))
		return nil
	})
	for {
		// v0: just drain. Real viewer commands (subscribe_chunks,
		// inspect_entity) land in milestone 7+.
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
