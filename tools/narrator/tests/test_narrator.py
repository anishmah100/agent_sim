"""Narrator unit + end-to-end tests with StubLLM (no network)."""
from __future__ import annotations

import json
from pathlib import Path

import pytest

from tools.narrator.buckets import Bucketizer, actor_of, cluster_agents
from tools.narrator.config import NarratorConfig
from tools.narrator.emit import NarratorOutput
from tools.narrator.llm import BudgetExceeded, StubLLM
from tools.narrator.main import NarratorRun
from tools.narrator.source import iter_events


# ----- helpers ---------------------------------------------------------

def write_events(path: Path, events: list[dict]) -> None:
    with path.open("w", encoding="utf-8") as f:
        for ev in events:
            f.write(json.dumps(ev) + "\n")


def ev(kind: str, tick: int, **payload) -> dict:
    return {
        "tick":     tick,
        "kind":     kind,
        "category": "social",
        "payload":  payload,
    }


# ----- bucketizer ------------------------------------------------------

def test_actor_of_extracts_correct_key():
    e = ev("Speech", 10, Speaker="alice", Text="hi", Mode="speak")
    assert actor_of(e) == "alice"
    e2 = ev("ActionAccepted", 10, EntityID="bob", Verb="move")
    # ActionAccepted is filtered by SKIP_KINDS.
    assert actor_of(e2) is None
    e3 = ev("EntityDied", 50, VictimID="charlie")
    assert actor_of(e3) == "charlie"


def test_bucketizer_per_agent_and_global():
    b = Bucketizer()
    b.ingest(ev("Speech", 1, Speaker="alice", Text="hi"))
    b.ingest(ev("Whisper", 2, Speaker="bob", Target="alice", Text="psst"))
    b.ingest(ev("Spawned", 3, EntityID="carol"))
    assert b.seen == 3
    assert set(b.all_actors_with_activity()) == {"alice", "bob", "carol"}
    a = b.drain_agent("alice")
    assert len(a) == 1 and a[0]["kind"] == "Speech"
    # Re-draining is empty.
    assert b.drain_agent("alice") == []
    g = b.peek_global()
    assert len(g) == 3
    b.drain_global()
    assert b.peek_global() == []


def test_cluster_detects_whisper_pair():
    events = [
        ev("Whisper", 1, Speaker="alice", Target="bob", Text="x"),
        ev("Speech", 2, Speaker="carol", Text="hi"),
        ev("TaskProposed", 3, Proposer="dave", Target="erin", ID="ct1",
           Terms="x", Reward=""),
    ]
    clusters = cluster_agents(events, cluster_radius_tiles=10)
    cluster_sets = [frozenset(c) for c in clusters]
    assert frozenset({"alice", "bob"}) in cluster_sets
    assert frozenset({"dave", "erin"}) in cluster_sets
    # Solo speakers are NOT in any cluster.
    assert not any("carol" in c for c in cluster_sets)


# ----- iter_events one-shot mode --------------------------------------

def test_iter_events_one_shot(tmp_path):
    path = tmp_path / "events.jsonl"
    events = [ev("Spawned", 1, EntityID="alice"),
              ev("Speech", 2, Speaker="alice", Text="hi")]
    write_events(path, events)
    got = list(iter_events(path, follow=False))
    assert len(got) == 2
    assert got[0]["payload"]["EntityID"] == "alice"


def test_iter_events_idle_exit(tmp_path):
    path = tmp_path / "events.jsonl"
    write_events(path, [ev("Spawned", 1, EntityID="alice")])
    got = list(iter_events(path, follow=True, idle_exit_seconds=0.5,
                           poll_interval=0.1))
    assert len(got) == 1


# ----- LLM stub --------------------------------------------------------

def test_stub_llm_increments_and_refuses():
    s = StubLLM(refuse_after=2)
    s.summarize("a")
    s.summarize("b")
    with pytest.raises(BudgetExceeded):
        s.summarize("c")


# ----- emit ------------------------------------------------------------

def test_emit_writes_jsonl(tmp_path):
    p = tmp_path / "narrator.jsonl"
    with NarratorOutput(p) as out:
        out.emit(tick=100, level="L1", scope="alice", text="walked",
                 n_events=4, llm="stub")
    raw = p.read_text().strip().splitlines()
    assert len(raw) == 1
    rec = json.loads(raw[0])
    assert rec["level"] == "L1"
    assert rec["scope"] == "alice"
    assert rec["text"] == "walked"
    assert rec["llm"] == "stub"


# ----- end-to-end with stubs ------------------------------------------

def test_narrator_run_emits_all_levels(tmp_path):
    events_path = tmp_path / "events.jsonl"
    out_path = tmp_path / "narrator.jsonl"
    # Construct an event stream that crosses L1, L2, and L3 cadences.
    L1_TICKS = 100
    L2_TICKS = 200
    L3_TICKS = 400
    events = []
    # Pre-L1: alice + bob interact (drives a cluster).
    events.append(ev("Spawned", 5, EntityID="alice"))
    events.append(ev("Spawned", 5, EntityID="bob"))
    events.append(ev("Whisper", 10, Speaker="alice", Target="bob",
                     Text="meet me at the market"))
    events.append(ev("Speech", 30, Speaker="alice", Text="hi"))
    # Cross L1 (tick 100) → L1 emits.
    events.append(ev("Speech", 100, Speaker="bob", Text="hello"))
    # Drive activity between L1 and L2.
    events.append(ev("TaskProposed", 150, Proposer="alice", Target="bob",
                     ID="ct1", Terms="bring me food", Reward="10g"))
    # Cross L2 (200).
    events.append(ev("TaskAccepted", 200, Proposer="alice", Target="bob",
                     ID="ct1"))
    # More for L3.
    events.append(ev("EntityDied", 300, VictimID="charlie", KillerID="dave"))
    # Cross L3 (400).
    events.append(ev("Speech", 400, Speaker="dave", Text="cleanup"))
    write_events(events_path, events)

    cfg = NarratorConfig(
        events_path=events_path,
        output_path=out_path,
        l1_cadence_ticks=L1_TICKS,
        l2_cadence_ticks=L2_TICKS,
        l3_cadence_ticks=L3_TICKS,
        max_qwen_calls=100,
        max_claude_calls=10,
        idle_exit_seconds=0.5,
    )
    run = NarratorRun(cfg, qwen=StubLLM("qwen"), claude=StubLLM("claude"))
    run.run()
    lines = [json.loads(l) for l in out_path.read_text().splitlines() if l.strip()]
    levels = {l["level"] for l in lines}
    assert "L1" in levels, lines
    assert "L2" in levels, lines
    assert "L3" in levels, lines
    assert "L4" in levels, lines  # closing summary
    # L1 emissions reference real agent scopes.
    l1s = [l for l in lines if l["level"] == "L1"]
    assert any(l["scope"] in {"alice", "bob"} for l in l1s), l1s
    # L2 emissions include a cluster of alice + bob.
    l2s = [l for l in lines if l["level"] == "L2"]
    assert any(set(l["actors"]) >= {"alice", "bob"} for l in l2s), l2s
    # L4 ran with the stub claude budget intact.
    l4s = [l for l in lines if l["level"] == "L4"]
    assert l4s and l4s[0]["llm"] == "claude"


def test_narrator_respects_budget(tmp_path):
    events_path = tmp_path / "events.jsonl"
    out_path = tmp_path / "narrator.jsonl"
    write_events(events_path, [
        ev("Spawned", 5, EntityID="alice"),
        ev("Speech", 100, Speaker="alice", Text="y"),  # crosses L1
    ])
    cfg = NarratorConfig(
        events_path=events_path, output_path=out_path,
        l1_cadence_ticks=100, l2_cadence_ticks=10_000,
        l3_cadence_ticks=10_000, idle_exit_seconds=0.5,
        max_qwen_calls=0, max_claude_calls=10,
    )
    # refuse_after=0 forces the first L1 call to raise BudgetExceeded.
    run = NarratorRun(cfg,
                      qwen=StubLLM("qwen", refuse_after=0),
                      claude=StubLLM("claude"))
    run.run()
    lines = [json.loads(l) for l in out_path.read_text().splitlines() if l.strip()]
    skipped = [l for l in lines
               if l["level"] == "L1" and l["llm"] == "skipped"]
    assert skipped, "expected at least one L1 skipped after budget exhaust"
    assert any(l["reason"] == "qwen_budget_exhausted" for l in skipped)
