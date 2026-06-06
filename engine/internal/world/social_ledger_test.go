package world

import "testing"

// D19 — per-pair social ledger contract.

func TestSocialLedger_Bump_Bidirectional(t *testing.T) {
	l := newSocialLedger()
	l.Bump("a", "b", SocialWhisper)
	l.Bump("a", "b", SocialPay)

	ab := l.CountsFor("a", "b")
	if ab.Whisper != 1 || ab.Pay != 1 {
		t.Fatalf("(a,b) want whisper=1 pay=1, got %+v", ab)
	}
	ba := l.CountsFor("b", "a")
	if ba.Whisper != 1 || ba.Pay != 1 {
		t.Fatalf("(b,a) want whisper=1 pay=1, got %+v — bump must be bidirectional", ba)
	}
}

func TestSocialLedger_PeersOf_Copy(t *testing.T) {
	l := newSocialLedger()
	l.Bump("a", "b", SocialTrade)
	l.Bump("a", "c", SocialAttack)

	peers := l.PeersOf("a")
	if len(peers) != 2 {
		t.Fatalf("PeersOf(a) want 2 entries, got %d", len(peers))
	}
	if peers["b"].Trade != 1 {
		t.Fatalf("peers[b].Trade want 1, got %d", peers["b"].Trade)
	}
	if peers["c"].Attack != 1 {
		t.Fatalf("peers[c].Attack want 1, got %d", peers["c"].Attack)
	}

	// Mutating the returned map must NOT affect the ledger.
	peers["b"] = SocialCounts{Trade: 999}
	if got := l.CountsFor("a", "b").Trade; got != 1 {
		t.Fatalf("returned map mutation leaked into ledger: got Trade=%d", got)
	}
}

func TestSocialLedger_Bump_SelfAndEmpty_NoOp(t *testing.T) {
	l := newSocialLedger()
	l.Bump("", "b", SocialWhisper)
	l.Bump("a", "", SocialWhisper)
	l.Bump("a", "a", SocialWhisper)
	if len(l.PeersOf("a")) != 0 || len(l.PeersOf("b")) != 0 {
		t.Fatal("self / empty bumps must be ignored")
	}
}
