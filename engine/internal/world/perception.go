package world

// Perception — LOS vision + hearing distribution.

// AudibleEvent — a speech / shout / whisper / sound event with enough
// context for receivers to localize the speaker. Stored in a small
// ring buffer; perceived through the per-agent observation builder.
type AudibleEvent struct {
	EventID    string `json:"event_id"`
	Kind       string `json:"kind"`           // speech | shout | whisper | sound
	FromEntity string `json:"from_entity"`
	FromPos    [2]int `json:"from_pos"`
	Text       string `json:"text,omitempty"`
	SoundKind  string `json:"sound_kind,omitempty"`
	Tick       uint64 `json:"tick"`
	radius     int    // engine-internal; distance-of-hearing
	whisperTo  string // engine-internal; only this entity hears
}

// emitSpeech pushes a speech event onto the ring buffer with the given
// audible radius (3 for speak, 15 for shout).
func (w *World) emitSpeech(e *Entity, kind, text string, radius int) {
	w.audibleAppend(AudibleEvent{
		EventID:    nextEventID(&w.eventSeq),
		Kind:       kind,
		FromEntity: e.EntityID,
		FromPos:    e.LogicalTile,
		Text:       text,
		Tick:       w.tick,
		radius:     radius,
	})
}

// emitWhisper — heard ONLY by the target. Implemented via a tagged
// event with radius 1 + whisperTo set, filtered in VisibleAudible.
func (w *World) emitWhisper(speaker, target *Entity, text string) {
	w.audibleAppend(AudibleEvent{
		EventID:    nextEventID(&w.eventSeq),
		Kind:       "whisper",
		FromEntity: speaker.EntityID,
		FromPos:    speaker.LogicalTile,
		Text:       text,
		Tick:       w.tick,
		radius:     1,
		whisperTo:  target.EntityID,
	})
}

// WitnessRecord — one notable thing an agent perceived: a kill it saw
// (kind="kill_witnessed", with killer/victim identity) or a death scream
// it heard from somewhere (kind="scream_heard", anonymous). Surfaced by
// the inspector's Witnesses tab so a spectator can read an agent's recent
// first-hand experience of violence in the world.
type WitnessRecord struct {
	Tick   uint64 `json:"tick"`
	Kind   string `json:"kind"`   // kill_witnessed | scream_heard
	Killer string `json:"killer,omitempty"`
	Victim string `json:"victim,omitempty"`
	Pos    [2]int `json:"pos"`
}

// witnessAppend records a perceived event for one witness. Called from
// EmitDeathScream under the world write lock. Keeps the most recent 32
// per entity — enough for the inspector, bounded so a long run can't
// leak memory through a popular witness.
func (w *World) witnessAppend(witnessID string, rec WitnessRecord) {
	if w.witnessLog == nil {
		w.witnessLog = make(map[string][]WitnessRecord)
	}
	list := append(w.witnessLog[witnessID], rec)
	if len(list) > 32 {
		list = list[len(list)-32:]
	}
	w.witnessLog[witnessID] = list
}

// WitnessedBy returns the most recent `limit` things the entity has
// witnessed, newest first. Safe for concurrent HTTP callers.
func (w *World) WitnessedBy(entityID string, limit int) []WitnessRecord {
	w.mu.RLock()
	defer w.mu.RUnlock()
	src := w.witnessLog[entityID]
	out := make([]WitnessRecord, 0, limit)
	for i := len(src) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, src[i])
	}
	return out
}

func (w *World) audibleAppend(ev AudibleEvent) {
	w.audible = append(w.audible, ev)
	// Trim: keep only last 256 events. They're cheap and expire by
	// being out of every receiver's range eventually.
	if len(w.audible) > 256 {
		w.audible = w.audible[len(w.audible)-256:]
	}
}

func nextEventID(seq *uint64) string {
	*seq++
	return formatUint64(*seq)
}

func formatUint64(n uint64) string {
	// minimal allocation-free uint64 → decimal. The fmt package would
	// also work but this is a hot path.
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// reverse in place
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

// RecentAudibleAll returns every public (non-whisper) audible event
// with tick >= sinceTick. The viewer hub broadcasts these to spectators
// so the frontend can render floating speech bubbles above speakers.
// Whispers are excluded — they're 1-on-1 conversation and not for
// public consumption.
func (w *World) RecentAudibleAll(sinceTick uint64) []AudibleEvent {
	out := make([]AudibleEvent, 0, 8)
	for _, ev := range w.audible {
		if ev.Tick < sinceTick {
			continue
		}
		if ev.whisperTo != "" {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// VisibleAudible returns audible events the given entity should hear
// THIS observation. Filters by:
//   - whisperTo (must match e or empty)
//   - chebyshev distance ≤ event.radius
//   - tick within the last 4 seconds (windowed)
func (w *World) VisibleAudible(e *Entity, sinceTick uint64) []AudibleEvent {
	out := make([]AudibleEvent, 0, 4)
	for _, ev := range w.audible {
		if ev.Tick < sinceTick {
			continue
		}
		if ev.whisperTo != "" && ev.whisperTo != e.EntityID {
			continue
		}
		if chebyshev(e.LogicalTile, ev.FromPos) > ev.radius {
			continue
		}
		out = append(out, ev)
	}
	return out
}

// SeesEntity returns true if `e` can see `other`. Default radius is
// VisionRadius; at night (day phase = night) it drops to NightRadius
// (caller passes the radius; we don't track time-of-day in engine
// yet).
//
// LOS is checked by bresenham: any wall tile on the line blocks vision.
func (w *World) SeesEntity(e *Entity, other *Entity, radius int) bool {
	if chebyshev(e.LogicalTile, other.LogicalTile) > radius {
		return false
	}
	return w.lineOfSight(e.LogicalTile, other.LogicalTile)
}

// SeesTile returns true if the given tile is visible to entity e.
func (w *World) SeesTile(e *Entity, t Tile, radius int) bool {
	if chebyshev(e.LogicalTile, t) > radius {
		return false
	}
	return w.lineOfSight(e.LogicalTile, t)
}

// lineOfSight — bresenham line walk. Returns true if NO tile on the
// open segment blocks vision. Endpoints don't block themselves.
func (w *World) lineOfSight(a, b Tile) bool {
	x0, y0 := a[0], a[1]
	x1, y1 := b[0], b[1]
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if x0 == x1 && y0 == y1 {
			return true
		}
		// Skip the source tile.
		if !(x0 == a[0] && y0 == a[1]) {
			if w.tileBlocksVision(Tile{x0, y0}) {
				return false
			}
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// tileBlocksVision — true if the tile has a vision-blocking element
// (wall tile or a tall opaque decoration).
func (w *World) tileBlocksVision(t Tile) bool {
	if t[0] < 0 || t[0] >= w.WidthTiles || t[1] < 0 || t[1] >= w.HeightTiles {
		return true
	}
	return w.visionBlocks[t[1]][t[0]]
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
