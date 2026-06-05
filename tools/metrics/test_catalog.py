"""Tests for the mechanical metrics catalog (SUB-10).

Builds an in-memory SQLite that looks like jsonl2sqlite output, then
verifies compute_all() lands the right numbers.
"""

from __future__ import annotations

import json
import sqlite3
import sys
from pathlib import Path

import pytest

_REPO = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(_REPO))

from tools.metrics.catalog import compute_all  # noqa: E402


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
    db_path = tmp_path / "derived.sqlite"
    db = sqlite3.connect(db_path)
    db.executescript(SCHEMA)

    events = [
        (1, 0, "DamageDealt",     "combat",          {"Target": "goblin", "Killer": "hero", "Amount": 10}),
        (2, 1, "DamageDealt",     "combat",          {"Target": "goblin", "Killer": "hero", "Amount": 12}),
        (3, 2, "EntityDied",      "combat",          {"EntityID": "goblin", "Killer": "hero"}),
        (4, 3, "GoldTransferred", "economy",         {"From": "hero", "To": "mari", "Amount": 5, "Cause": "pay"}),
        (5, 4, "GoldTransferred", "economy",         {"From": "hero", "To": "blacksmith", "Amount": 20, "Cause": "trade"}),
        (6, 5, "Speech",          "social",          {}),
        (7, 6, "Whisper",         "social",          {}),
        (8, 7, "Shout",           "social",          {}),
        (9, 8, "EntityMoved",     "movement",        {}),
    ]
    db.executemany(
        "INSERT INTO events (tick, seq, kind, category, payload) VALUES (?, ?, ?, ?, ?)",
        [(t, s, k, c, json.dumps(p)) for (t, s, k, c, p) in events],
    )
    traces = [
        (3, 2, "hero", "a1", "attack", "killing the goblin"),
        (4, 3, "hero", "a2", "pay",    "settling debt with mari"),
        (6, 5, "mari", "a3", "speak",  "thanking hero"),
    ]
    db.executemany(
        "INSERT INTO reasoning_traces (tick, seq, entity_id, action_id, verb, reasoning) VALUES (?, ?, ?, ?, ?, ?)",
        traces,
    )
    db.commit()
    db.close()
    return str(db_path)


def test_volume_counts(fixture_db):
    m = compute_all(fixture_db)
    assert m.total_events == 9
    assert m.per_category["combat"]   == 3
    assert m.per_category["economy"]  == 2
    assert m.per_category["social"]   == 3
    assert m.per_category["movement"] == 1


def test_combat(fixture_db):
    m = compute_all(fixture_db)
    assert m.deaths == 1
    assert m.damage_dealt_count == 2
    assert m.damage_dealt_total == 22


def test_economy(fixture_db):
    m = compute_all(fixture_db)
    assert m.transactions == 2
    assert m.gold_transferred_total == 25
    assert m.transactions_per_cause == {"pay": 1, "trade": 1}


def test_social_counts(fixture_db):
    m = compute_all(fixture_db)
    assert m.speech_count == 1
    assert m.whisper_count == 1
    assert m.shout_count == 1
    assert m.sound_count == 0


def test_cognition(fixture_db):
    m = compute_all(fixture_db)
    assert m.reasoning_traces == 3
    assert m.unique_reasoning_agents == 2


def test_unique_entities_seen(fixture_db):
    m = compute_all(fixture_db)
    # Hero, mari, blacksmith, goblin — referenced across kills + trades.
    assert m.unique_entities_in_events >= 4
