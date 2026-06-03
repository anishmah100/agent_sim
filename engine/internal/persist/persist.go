// Package persist writes/reads world snapshots to disk (and later a
// Postgres-backed store). v1 is JSON on local disk so we can validate
// the round-trip without provisioning a DB.
package persist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anishmah100/agent_sim/engine/internal/world"
)

type Snapshot struct {
	MapID       string                       `json:"map_id"`
	Tick        uint64                       `json:"tick"`
	Timestamp   time.Time                    `json:"timestamp"`
	Entities    []EntitySnapshot             `json:"entities"`
	Decorations []world.DecorationRef        `json:"decorations,omitempty"`
}

type EntitySnapshot struct {
	EntityID       string                 `json:"entity_id"`
	Archetype      string                 `json:"archetype"`
	DisplayName    string                 `json:"display_name,omitempty"`
	Pos            [2]int                 `json:"pos"`
	Facing         string                 `json:"facing"`
	Extras         map[string]interface{} `json:"extras,omitempty"`
	InsideBuilding string                 `json:"inside_building,omitempty"`
}

// Write writes the current world state to a JSON snapshot file under
// dir/. Returns the file path.
func Write(w *world.World, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	name := fmt.Sprintf("%s_%d.json", w.MapID, now.UnixNano())
	path := filepath.Join(dir, name)
	snap := buildSnapshot(w, now)
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	// Also update a "latest" pointer for cold-boot load.
	_ = os.WriteFile(filepath.Join(dir, "latest.json"), data, 0o644)
	return path, nil
}

// LatestPath returns the most recent snapshot path, or "" if none.
func LatestPath(dir string) string {
	p := filepath.Join(dir, "latest.json")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// Restore reads a snapshot file and applies entity state on top of the
// already-loaded world. (Walkability and map dims come from the world
// JSON; this just overlays the dynamic state — gold, hp, inventory,
// inside_building.)
func Restore(w *world.World, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.MapID != "" && snap.MapID != w.MapID {
		return fmt.Errorf("snapshot map_id %q != world map_id %q", snap.MapID, w.MapID)
	}
	for _, es := range snap.Entities {
		w.ApplySnapshot(es.EntityID, es.Extras, es.InsideBuilding)
	}
	return nil
}

func buildSnapshot(w *world.World, ts time.Time) Snapshot {
	ws := w.Snapshot()
	out := Snapshot{
		MapID:     ws.MapID,
		Tick:      ws.Tick,
		Timestamp: ts,
	}
	for _, e := range ws.Entities {
		out.Entities = append(out.Entities, EntitySnapshot{
			EntityID:       e.EntityID,
			Archetype:      e.Archetype,
			DisplayName:    e.DisplayName,
			Pos:            e.LogicalTile,
			Facing:         string(e.Facing),
			Extras:         e.Extras,
			InsideBuilding: e.InsideBuilding,
		})
	}
	return out
}

// RunAutoSave spawns a goroutine that writes a snapshot every interval.
// Stops when ctx is canceled.
func RunAutoSave(w *world.World, dir string, interval time.Duration, done <-chan struct{}) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-done:
			if _, err := Write(w, dir); err != nil {
				return
			}
			return
		case <-t.C:
			if _, err := Write(w, dir); err != nil {
				// log + keep going
				_ = err
			}
		}
	}
}

// PathFor returns the canonical save dir for the given map ID.
func PathFor(root, mapID string) string {
	return filepath.Join(root, mapID)
}

var ErrNoSnapshot = errors.New("no snapshot")
