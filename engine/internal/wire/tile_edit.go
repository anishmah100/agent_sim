package wire

import (
	"encoding/json"
	"net/http"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

// TileEditHandler — POST /api/v1/world/edit
//
// Body: {"x":int, "y":int, "glyph":"."}
// Response: {"ok":bool, "kind":"grass"} or {"ok":false, "reason":"..."}
//
// Wired by main.go after the world + system stack are up. Mutates the
// world's in-memory tile grid (so observation + viewer pick it up next
// tick) and appends to the bundle's tile_edits.json overlay so a
// restart sees the same edits.
func TileEditHandler(w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.Header().Set("Allow", "POST")
			http.Error(rw, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			X     int    `json:"x"`
			Y     int    `json:"y"`
			Glyph string `json:"glyph"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondEdit(rw, http.StatusBadRequest,
				editResp{OK: false, Reason: "bad_json"})
			return
		}
		// Hold the world write lock around SetTile so the next tick
		// builds its snapshot with the new tile baked in.
		w.LockWrite()
		kind, err := w.SetTile(body.X, body.Y, body.Glyph)
		w.UnlockWrite()
		if err != nil {
			respondEdit(rw, http.StatusBadRequest,
				editResp{OK: false, Reason: err.Error()})
			return
		}
		// Persist to overlay best-effort. A persist failure doesn't
		// undo the in-memory edit — the user sees the paint land
		// immediately; failures only surface on restart.
		if err := w.AppendTileEditOverlay(world.TileEdit{
			X: body.X, Y: body.Y, Glyph: body.Glyph,
		}); err != nil {
			respondEdit(rw, http.StatusOK, editResp{
				OK:     true,
				Kind:   kind,
				Reason: "persist_failed:" + err.Error(),
			})
			return
		}
		respondEdit(rw, http.StatusOK, editResp{OK: true, Kind: kind})
	}
}

type editResp struct {
	OK     bool   `json:"ok"`
	Kind   string `json:"kind,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func respondEdit(rw http.ResponseWriter, status int, body editResp) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(body)
}

// DecorationEditHandler — POST /api/v1/world/edit_deco
//
// Body: same as world.DecorationEdit (x, y, sprite, height_tiles,
// footprint_w, footprint_h, walkable).
// Response: {"ok":bool, "reason":"..."}
//
// Adds a decoration to the live world AND persists it to the bundle's
// decoration_edits.json overlay so a restart sees the same placement.
// Updates walkability + (for bld:* sprites) door registration so the
// new building behaves like a baked-in one.
func DecorationEditHandler(w *world.World) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			rw.Header().Set("Allow", "POST")
			http.Error(rw, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		var body world.DecorationEdit
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			respondEdit(rw, http.StatusBadRequest,
				editResp{OK: false, Reason: "bad_json"})
			return
		}
		var err error
		w.LockWrite()
		switch body.Op {
		case "remove":
			_, err = w.RemoveDecorationAt(body.X, body.Y)
		default:
			err = w.AddDecoration(body)
		}
		w.UnlockWrite()
		if err != nil {
			respondEdit(rw, http.StatusBadRequest,
				editResp{OK: false, Reason: err.Error()})
			return
		}
		if err := w.AppendDecorationEditOverlay(body); err != nil {
			respondEdit(rw, http.StatusOK, editResp{
				OK:     true,
				Reason: "persist_failed:" + err.Error(),
			})
			return
		}
		respondEdit(rw, http.StatusOK, editResp{OK: true})
	}
}
