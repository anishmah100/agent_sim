"""Bucketization: shard incoming events by tier so the right summarizer
can grab them when its cadence fires.

L1 (per-agent): one bucket per `EntityID`/`Speaker`. Cleared after
each L1 emission.

L2 (cluster): clusters of agents currently interacting. Detected by
proximity + speech overlap within the L2 window.

L3 (society): the full per-window event stream + L2 summaries since
the last L3 fire. We emit a society summary every L3 cadence.

L4 (closing): the entire run, fed to the final summarizer.
"""
from __future__ import annotations

from collections import defaultdict, deque
from dataclasses import dataclass, field
from typing import Iterable, Optional


# Event kinds that carry a single primary actor we can attribute the
# event to. Used by the L1 bucketizer.
ACTOR_KEYS = {
    "ActionAccepted":   "EntityID",
    "Speech":           "Speaker",
    "Whisper":          "Speaker",
    "MentalNote":       "entity_id",
    "ReflectiveNote":   "entity_id",
    "ReasoningTrace":   "EntityID",
    "EnteredBuilding":  "EntityID",
    "ExitedBuilding":   "EntityID",
    "Spawned":          "EntityID",
    "EntityDied":       "VictimID",
    "ItemTransferred":  "From",
    "TaskProposed":     "Proposer",
    "TaskAccepted":     "Target",
    "TaskRejected":     "Target",
    "TaskCompleted":    "Proposer",
}

# Kinds we deliberately ignore (low signal for narration).
SKIP_KINDS = {"ActionAccepted"}


def _is_item_spawn(ev: dict) -> bool:
    """True for periodic ITEM respawns (Sprite='item:...'). The world
    respawns food/coins/weapons as `Spawned` events sharing the same
    kind as agent appearances; without this filter the narrator wrote
    'agent spawn_42 appeared as a bread loaf', flooding the emergence
    story with respawn noise and miscategorising loot as characters."""
    if ev.get("kind") != "Spawned":
        return False
    sprite = str((ev.get("payload") or {}).get("Sprite", ""))
    return sprite.startswith("item:")


def actor_of(ev: dict) -> Optional[str]:
    """Extract the primary actor id from an event, or None if the
    event has no single actor (or is in SKIP_KINDS)."""
    kind = ev.get("kind", "")
    if kind in SKIP_KINDS:
        return None
    key = ACTOR_KEYS.get(kind)
    if key is None:
        return None
    payload = ev.get("payload") or {}
    val = payload.get(key)
    if not val:
        return None
    return str(val)


@dataclass
class Bucketizer:
    """Maintains per-tier event lists, ready to be drained by the
    summarizers. Cap controls the maximum number of events per bucket
    (preserves tail when overrun)."""

    cap_per_agent: int = 256
    cap_global: int = 4096

    # Per-agent buffer of events touching that agent.
    per_agent: dict[str, deque] = field(default_factory=lambda: defaultdict(deque))
    # Global rolling buffer for L2 + L3.
    global_buf: deque = field(default_factory=deque)
    # Total event count seen (informational).
    seen: int = 0

    def ingest(self, ev: dict) -> None:
        self.seen += 1
        if ev.get("kind") in SKIP_KINDS:
            return
        if _is_item_spawn(ev):
            return
        actor = actor_of(ev)
        if actor is not None:
            buf = self.per_agent[actor]
            buf.append(ev)
            while len(buf) > self.cap_per_agent:
                buf.popleft()
        self.global_buf.append(ev)
        while len(self.global_buf) > self.cap_global:
            self.global_buf.popleft()

    def drain_agent(self, actor: str) -> list[dict]:
        """Return + clear the per-actor buffer."""
        buf = self.per_agent.get(actor)
        if not buf:
            return []
        out = list(buf)
        buf.clear()
        return out

    def all_actors_with_activity(self) -> list[str]:
        return [a for a, b in self.per_agent.items() if b]

    def drain_global(self) -> list[dict]:
        out = list(self.global_buf)
        self.global_buf.clear()
        return out

    def peek_global(self) -> list[dict]:
        return list(self.global_buf)


# --- L2 cluster detection ---


def cluster_agents(events: Iterable[dict], cluster_radius_tiles: int) -> list[set[str]]:
    """Group agents who interacted in the same window. Two agents are
    in the same cluster if either:

    1. one whispered to the other, OR
    2. they exchanged speech in the same tick range, OR
    3. they took accepted actions targeting each other.

    Position-based proximity is NOT used here — we rely on the
    *behavioral* signal (interaction) instead of just nearness. This
    avoids creating a cluster from two strangers who happened to walk
    past each other.

    Returns a list of cluster sets. Singletons are dropped. Agents
    present in multiple interactions are unioned via simple disjoint-
    set merge.
    """
    # Union-find. Every node we touch is registered as its own parent
    # first so the collection pass at the bottom sees it.
    parent: dict[str, str] = {}

    def ensure(x: str) -> None:
        if x not in parent:
            parent[x] = x

    def find(x: str) -> str:
        ensure(x)
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    def union(a: str, b: str) -> None:
        ra, rb = find(a), find(b)
        if ra == rb:
            return
        parent[ra] = rb

    for ev in events:
        kind = ev.get("kind", "")
        payload = ev.get("payload") or {}
        if kind == "Whisper":
            spk = payload.get("Speaker") or payload.get("speaker")
            tgt = payload.get("Target") or payload.get("target")
            if spk and tgt:
                union(spk, tgt)
        elif kind == "Speech":
            # Speech is broadcast; we don't form clusters from it alone.
            pass
        elif kind in ("TaskProposed", "TaskAccepted", "TaskCompleted",
                      "TaskRejected"):
            a, b = payload.get("Proposer"), payload.get("Target")
            if a and b:
                union(a, b)
        elif kind == "ItemTransferred":
            a, b = payload.get("From"), payload.get("To")
            if a and b:
                union(a, b)
        elif kind == "EntityDied":
            victim = payload.get("VictimID")
            killer = payload.get("KillerID")
            if victim and killer:
                union(victim, killer)
    # Collect clusters.
    by_root: dict[str, set[str]] = defaultdict(set)
    for k in list(parent.keys()):
        r = find(k)
        by_root[r].add(k)
    return [c for c in by_root.values() if len(c) >= 2]
