"""Mechanical metrics catalog (Phase SUB-10).

Reads a derived.sqlite (produced by cmd/jsonl2sqlite) and computes a
fixed set of mechanical, automation-friendly metrics about the run.

Distinct from LLM-as-judge (Phase SUB-11) — those are qualitative.
THESE are cheap, deterministic, and rerunnable. The autoresearch loop's
diagnose step starts here, then escalates to the judge if needed.

Used by:
  - cmd-line: ``python -m tools.metrics.catalog <derived.sqlite> > metrics.json``
  - Phase SUB-12 exp framework will import compute_all() per run.
"""

from __future__ import annotations

import json
import sqlite3
import sys
from dataclasses import dataclass, field, asdict
from typing import Any


@dataclass
class Metrics:
    # Volume.
    total_events: int = 0
    per_category: dict[str, int] = field(default_factory=dict)
    per_kind: dict[str, int] = field(default_factory=dict)

    # Combat.
    deaths: int = 0
    damage_dealt_count: int = 0
    damage_dealt_total: int = 0

    # Economy.
    transactions: int = 0
    gold_transferred_total: int = 0
    transactions_per_cause: dict[str, int] = field(default_factory=dict)

    # Social.
    speech_count: int = 0
    whisper_count: int = 0
    shout_count: int = 0
    sound_count: int = 0

    # Cognition.
    reasoning_traces: int = 0
    unique_reasoning_agents: int = 0

    # Population.
    unique_entities_in_events: int = 0


def compute_all(db_path: str) -> Metrics:
    """Open derived.sqlite and compute the full catalog."""
    db = sqlite3.connect(db_path)
    db.row_factory = sqlite3.Row
    m = Metrics()
    try:
        # Volume.
        m.total_events = _scalar(db, "SELECT COUNT(*) FROM events")
        m.per_category = _grouped(db,
            "SELECT category, COUNT(*) AS c FROM events GROUP BY category")
        m.per_kind = _grouped(db,
            "SELECT kind, COUNT(*) AS c FROM events GROUP BY kind")

        # Combat — payload JSON parsed inline via SQLite's json_extract.
        m.deaths = _scalar(db,
            "SELECT COUNT(*) FROM events WHERE kind='EntityDied'")
        m.damage_dealt_count = _scalar(db,
            "SELECT COUNT(*) FROM events WHERE kind='DamageDealt'")
        m.damage_dealt_total = _scalar(db,
            "SELECT COALESCE(SUM(CAST(json_extract(payload, '$.Amount') AS INTEGER)), 0) "
            "FROM events WHERE kind='DamageDealt'")

        # Economy.
        m.transactions = _scalar(db,
            "SELECT COUNT(*) FROM events WHERE kind='GoldTransferred'")
        m.gold_transferred_total = _scalar(db,
            "SELECT COALESCE(SUM(CAST(json_extract(payload, '$.Amount') AS INTEGER)), 0) "
            "FROM events WHERE kind='GoldTransferred'")
        m.transactions_per_cause = _grouped(db,
            "SELECT json_extract(payload, '$.Cause') AS cause, COUNT(*) AS c "
            "FROM events WHERE kind='GoldTransferred' GROUP BY cause")

        # Social.
        # speak and shout are BOTH emitted as kind='Speech', discriminated by
        # the Mode payload field ('speak' | 'shout'); the engine never emits a
        # 'Shout' kind. Querying kind='Shout' silently returned 0 and let
        # speech_count absorb shouts (audit MEDIUM). Whisper is its own kind.
        m.speech_count  = _scalar(db, "SELECT COUNT(*) FROM events WHERE kind='Speech' AND COALESCE(json_extract(payload,'$.Mode'),'speak')<>'shout'")
        m.whisper_count = _scalar(db, "SELECT COUNT(*) FROM events WHERE kind='Whisper'")
        m.shout_count   = _scalar(db, "SELECT COUNT(*) FROM events WHERE kind='Speech' AND json_extract(payload,'$.Mode')='shout'")
        m.sound_count   = _scalar(db, "SELECT COUNT(*) FROM events WHERE kind='Sound'")

        # Cognition.
        m.reasoning_traces = _scalar(db, "SELECT COUNT(*) FROM reasoning_traces")
        m.unique_reasoning_agents = _scalar(db,
            "SELECT COUNT(DISTINCT entity_id) FROM reasoning_traces")

        # Population. Collect ALL entity_ids referenced in any
        # payload. Cheap UNION since the schema is small.
        m.unique_entities_in_events = _scalar(db,
            "SELECT COUNT(DISTINCT id) FROM ("
            " SELECT json_extract(payload, '$.Killer') AS id FROM events"
            " UNION SELECT json_extract(payload, '$.Target') FROM events"
            " UNION SELECT json_extract(payload, '$.From')   FROM events"
            " UNION SELECT json_extract(payload, '$.To')     FROM events"
            ") WHERE id IS NOT NULL")
    finally:
        db.close()
    return m


def _scalar(db: sqlite3.Connection, sql: str) -> int:
    row = db.execute(sql).fetchone()
    if row is None:
        return 0
    v = row[0]
    return int(v or 0)


def _grouped(db: sqlite3.Connection, sql: str) -> dict[str, int]:
    out: dict[str, int] = {}
    for row in db.execute(sql):
        key = row[0]
        if key is None:
            continue
        out[str(key)] = int(row[1])
    return out


def to_dict(m: Metrics) -> dict[str, Any]:
    return asdict(m)


def main() -> None:
    if len(sys.argv) < 2:
        print("usage: python -m tools.metrics.catalog <derived.sqlite>",
              file=sys.stderr)
        sys.exit(2)
    m = compute_all(sys.argv[1])
    json.dump(to_dict(m), sys.stdout, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
