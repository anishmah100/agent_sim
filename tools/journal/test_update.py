"""Tests for the journal maintenance pipeline (SUB-13)."""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

_REPO = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(_REPO))

from tools.journal.update import (  # noqa: E402
    append_index_row, append_world_journal, ensure_top_journal, update_all,
)


def _make_run(parent: Path, run_id: str, world: str = "eldoria",
              slug: str = "x", with_metrics: bool = True,
              with_judge: bool = True) -> Path:
    run = parent / world / run_id
    run.mkdir(parents=True)
    run.joinpath("metadata.json").write_text(json.dumps({
        "run_id": run_id, "world": world, "slug": slug,
        "parent": None, "created_at": "2026-06-05T00:00:00Z",
        "rubric": ["did agents form alliances?"],
    }))
    if with_metrics:
        run.joinpath("metrics.json").write_text(json.dumps({
            "total_events": 100, "deaths": 2, "transactions": 5,
            "reasoning_traces": 12,
        }))
    if with_judge:
        run.joinpath("judge_report.md").write_text(
            "# Judge report (stub)\n## Summary\nNothing exciting.\n"
        )
    return run


def test_ensure_top_journal_creates_with_header(tmp_path):
    ensure_top_journal(tmp_path)
    f = tmp_path / "JOURNAL.md"
    assert f.exists()
    assert "Hand-curated" in f.read_text()


def test_ensure_top_journal_idempotent(tmp_path):
    (tmp_path).mkdir(exist_ok=True)
    (tmp_path / "JOURNAL.md").write_text("custom prose\n")
    ensure_top_journal(tmp_path)
    assert "custom prose" in (tmp_path / "JOURNAL.md").read_text()


def test_append_index_creates_table(tmp_path):
    run = _make_run(tmp_path, "20260605-x-001")
    append_index_row(tmp_path, run)
    idx = tmp_path / "INDEX.md"
    txt = idx.read_text()
    assert "# Run index" in txt
    assert "20260605-x-001" in txt
    assert "stub" in txt


def test_append_index_idempotent(tmp_path):
    run = _make_run(tmp_path, "20260605-x-002")
    append_index_row(tmp_path, run)
    n1 = (tmp_path / "INDEX.md").read_text().count("20260605-x-002")
    append_index_row(tmp_path, run)
    n2 = (tmp_path / "INDEX.md").read_text().count("20260605-x-002")
    assert n1 == n2 == 1, f"index should not duplicate rows; n1={n1} n2={n2}"


def test_append_world_journal_creates_with_section(tmp_path):
    run = _make_run(tmp_path, "20260605-x-003", world="eldoria")
    append_world_journal(tmp_path, run)
    journal = tmp_path / "eldoria" / "WORLD_JOURNAL.md"
    assert journal.exists()
    body = journal.read_text()
    assert "## 20260605-x-003 — x" in body
    assert "TODO" in body  # human takeaway hook


def test_update_all_runs_three_writers(tmp_path):
    run = _make_run(tmp_path, "20260605-y-001", world="lotr", slug="y")
    update_all(run, exp_root=tmp_path)
    assert (tmp_path / "JOURNAL.md").exists()
    assert (tmp_path / "INDEX.md").exists()
    assert (tmp_path / "lotr" / "WORLD_JOURNAL.md").exists()


def test_update_all_refuses_invalid_run_dir(tmp_path):
    with pytest.raises(SystemExit):
        update_all(tmp_path, exp_root=tmp_path)
