"""Tests for the LLM-as-judge scaffold (SUB-11)."""

from __future__ import annotations

import json
import sqlite3
import sys
from pathlib import Path

import pytest

_REPO = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(_REPO))

from tools.judge.judge import (  # noqa: E402
    AnthropicJudge, JudgeReport, StubJudge, build_context_from_sqlite, run_judge,
)


SCHEMA = """
CREATE TABLE events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tick INTEGER NOT NULL, seq INTEGER NOT NULL,
    kind TEXT NOT NULL, category TEXT, payload TEXT NOT NULL
);
CREATE TABLE reasoning_traces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tick INTEGER, seq INTEGER,
    entity_id TEXT, action_id TEXT, verb TEXT, reasoning TEXT
);
"""


@pytest.fixture
def fixture_db(tmp_path):
    db_path = tmp_path / "d.sqlite"
    db = sqlite3.connect(db_path)
    db.executescript(SCHEMA)
    db.executemany(
        "INSERT INTO events (tick, seq, kind, category, payload) VALUES (?, ?, ?, ?, ?)",
        [
            (1, 0, "DamageDealt", "combat",  json.dumps({"Amount": 10})),
            (2, 1, "Speech",      "social",  json.dumps({"Text": "trust me on this", "Speaker": "alice"})),
            (3, 2, "GoldTransferred", "economy", json.dumps({"From": "alice", "To": "bob", "Amount": 5, "Cause": "pay"})),
        ],
    )
    db.executemany(
        "INSERT INTO reasoning_traces (tick, seq, entity_id, action_id, verb, reasoning) VALUES (?, ?, ?, ?, ?, ?)",
        [
            (2, 1, "alice", "a1", "speak", "convincing bob to ally with me"),
            (3, 2, "alice", "a2", "pay",   "sealing the alliance with a gift"),
        ],
    )
    db.commit()
    db.close()
    return str(db_path)


def test_build_context_includes_metrics_and_samples(fixture_db):
    ctx = build_context_from_sqlite(fixture_db)
    assert "CATEGORY COUNTS" in ctx
    assert "TOP EVENT KINDS" in ctx
    assert "SAMPLE REASONING TRACES" in ctx
    assert "SAMPLE SPEECH" in ctx
    # Spot-check actual data made it in.
    assert "social" in ctx
    assert "alice" in ctx


def test_stub_judge_returns_neutral_scores():
    j = StubJudge(score_per_criterion=3)
    out = j.judge(["did agents ally?", "was there scheming?"], "ctx")
    assert len(out["scores"]) == 2
    for s in out["scores"]:
        assert s["score_1_to_5"] == 3
        assert "stub" in s["evidence"].lower()
    assert "stub" in out["summary"].lower()


def test_anthropic_judge_refuses_until_wired():
    with pytest.raises(NotImplementedError):
        AnthropicJudge().judge(["x"], "ctx")


def test_run_judge_produces_structured_report(fixture_db):
    report = run_judge(
        fixture_db,
        rubric=["did agents form alliances?", "was scheming present?"],
        llm=StubJudge(score_per_criterion=4),
    )
    assert isinstance(report, JudgeReport)
    assert report.judge_model == "stub"
    assert len(report.scores) == 2
    assert all(s["score_1_to_5"] == 4 for s in report.scores)


def test_judge_report_to_markdown_shape(fixture_db):
    report = run_judge(
        fixture_db,
        rubric=["did agents trade?"],
        llm=StubJudge(),
    )
    md = report.to_markdown()
    assert md.startswith("# Judge report")
    assert "## Summary" in md
    assert "## Per-criterion scores" in md
    assert "| Criterion | Score (1-5) | Evidence |" in md


def test_judge_report_to_dict_round_trip(fixture_db):
    report = run_judge(fixture_db, rubric=["x"], llm=StubJudge())
    d = report.to_dict()
    assert set(d.keys()) >= {"rubric", "scores", "summary", "judge_model"}
    json.dumps(d)  # serializes cleanly
