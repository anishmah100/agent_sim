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
	"sync"
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

	mu      sync.Mutex
	clients map[*viewerConn]struct{}
}

type viewerConn struct {
	conn  *websocket.Conn
	send  chan []byte
	hub   *ViewerHub
	addr  string
}

// NewViewerHub starts the broadcast goroutine and returns a hub ready
// to accept HTTP handlers for new connections.
func NewViewerHub(ctx context.Context, w *world.World) *ViewerHub {
	h := &ViewerHub{
		w:       w,
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
	}
	h.register(c)
	log.Printf("viewer connect: %s (total=%d)", c.addr, h.count())

	// Send an immediate full snapshot so the page doesn't render empty
	// while it waits for the next broadcast tick.
	if msg, err := h.encodeSnapshot(); err == nil {
		select {
		case c.send <- msg:
		default:
		}
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
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
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
			h.mu.Lock()
			for c := range h.clients {
				// Non-blocking send. If the client is slow, drop the
				// frame rather than back-pressuring the broadcast.
				select {
				case c.send <- msg:
				default:
				}
			}
			h.mu.Unlock()
		}
	}
}

type viewerMessage struct {
	Type     string                `json:"type"`
	Snapshot *world.WorldSnapshot  `json:"snapshot,omitempty"`
	Audible  []world.AudibleEvent  `json:"audible,omitempty"`
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
		Type:     "world_snapshot",
		Snapshot: &snap,
		Audible:  audible,
	})
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
