package world

import "sync"

// D19 — per-pair social interaction counters. Engine-tracked raw
// counts: every trade/whisper/pay/attack/contract between a pair of
// agents increments. Read by the inspector's Relationships tab and
// fed into D2's quantitative-emergence metric layer.
//
// Pairs are stored bidirectionally — bumping (A, B) increments
// counters in BOTH (A→B) and (B→A) directions of the same logical
// pair entry. Lookup is by either order.

type SocialKind int

const (
	SocialTrade SocialKind = iota
	SocialWhisper
	SocialPay
	SocialAttack
	SocialContract
)

// SocialCounts — per-pair interaction tallies.
type SocialCounts struct {
	Trade    int `json:"trade"`
	Whisper  int `json:"whisper"`
	Pay      int `json:"pay"`
	Attack   int `json:"attack"`
	Contract int `json:"contract"`
}

type socialLedger struct {
	mu     sync.RWMutex
	counts map[string]map[string]*SocialCounts // a → b → counts
}

func newSocialLedger() *socialLedger {
	return &socialLedger{counts: map[string]map[string]*SocialCounts{}}
}

// Bump increments the counter for the (a, b) pair (and its mirror
// (b, a)). Safe for concurrent callers.
func (l *socialLedger) Bump(a, b string, kind SocialKind) {
	if a == "" || b == "" || a == b {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, pair := range [2][2]string{{a, b}, {b, a}} {
		inner, ok := l.counts[pair[0]]
		if !ok {
			inner = map[string]*SocialCounts{}
			l.counts[pair[0]] = inner
		}
		c, ok := inner[pair[1]]
		if !ok {
			c = &SocialCounts{}
			inner[pair[1]] = c
		}
		switch kind {
		case SocialTrade:
			c.Trade++
		case SocialWhisper:
			c.Whisper++
		case SocialPay:
			c.Pay++
		case SocialAttack:
			c.Attack++
		case SocialContract:
			c.Contract++
		}
	}
}

// CountsFor returns a copy of the (a, b) pair's counters, or a zero
// SocialCounts if the pair has no interactions yet.
func (l *socialLedger) CountsFor(a, b string) SocialCounts {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if inner, ok := l.counts[a]; ok {
		if c, ok := inner[b]; ok {
			return *c
		}
	}
	return SocialCounts{}
}

// PeersOf returns every entity_id that has at least one interaction
// with `a`, mapped to their counts. The map is a fresh copy safe to
// iterate after release.
func (l *socialLedger) PeersOf(a string) map[string]SocialCounts {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := map[string]SocialCounts{}
	if inner, ok := l.counts[a]; ok {
		for peer, c := range inner {
			out[peer] = *c
		}
	}
	return out
}

// SocialEdge — one undirected relationship edge for the Society-Pulse
// overlay. A < B (string order) so each pair appears once.
type SocialEdge struct {
	A string `json:"a"`
	B string `json:"b"`
	SocialCounts
}

// AllEdges returns every interaction pair exactly once (deduped across
// the mirrored (a,b)/(b,a) entries). Fresh copy, safe after release.
func (l *socialLedger) AllEdges() []SocialEdge {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := []SocialEdge{}
	for a, inner := range l.counts {
		for b, c := range inner {
			if a >= b {
				continue // keep only a<b so each undirected pair appears once
			}
			out = append(out, SocialEdge{A: a, B: b, SocialCounts: *c})
		}
	}
	return out
}
