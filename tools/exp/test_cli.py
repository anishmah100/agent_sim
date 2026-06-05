"""Tests for the experiment CLI (SUB-12)."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

import pytest

_REPO = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(_REPO))

from tools.exp.cli import _next_run_id, cmd_finalize, cmd_new  # noqa: E402


def test_new_creates_run_dir_with_snapshot(tmp_path):
    args = argparse.Namespace(
        world="eldoria",
        slug="hunger-tweak",
        parent=None,
        rubric=["did agents form alliances?"],
        exp_root=str(tmp_path),
        today="20260605",
    )
    code = cmd_new(args)
    assert code == 0
    runs = list((tmp_path / "eldoria").iterdir())
    assert len(runs) == 1
    run = runs[0]
    assert run.name == "20260605-hunger-tweak-001"
    # Metadata round-trips.
    meta = json.loads((run / "metadata.json").read_text())
    assert meta["world"] == "eldoria"
    assert meta["slug"] == "hunger-tweak"
    assert meta["rubric"] == ["did agents form alliances?"]
    # Bundle was snapshotted.
    assert (run / "bundle" / "world.json").exists()
    assert (run / "bundle" / "bundle.toml").exists()
    # rules.star should also have come along (eldoria has one).
    assert (run / "bundle" / "rules.star").exists()


def test_next_run_id_increments_when_dup(tmp_path):
    (tmp_path / "20260605-x-001").mkdir()
    (tmp_path / "20260605-x-002").mkdir()
    nid = _next_run_id(tmp_path, "x", today="20260605")
    assert nid == "20260605-x-003"


def test_finalize_refuses_without_metadata(tmp_path):
    args = argparse.Namespace(run_dir=str(tmp_path))
    assert cmd_finalize(args) == 2


def test_finalize_refuses_without_logs(tmp_path):
    (tmp_path / "metadata.json").write_text(json.dumps({"world": "eldoria", "slug": "x"}))
    args = argparse.Namespace(run_dir=str(tmp_path))
    assert cmd_finalize(args) == 3


def test_finalize_round_trip(tmp_path):
    # Set up a minimal run dir with metadata + a tiny logs.jsonl.
    (tmp_path / "metadata.json").write_text(json.dumps({
        "run_id": "test", "world": "eldoria", "slug": "x",
        "parent": None, "created_at": "2026-06-05T00:00:00Z",
        "rubric": ["did things happen?"],
    }))
    (tmp_path / "logs.jsonl").write_text(
        '{"tick":1,"seq":0,"kind":"DamageDealt","category":"combat","payload":{"Amount":10}}\n'
        '{"tick":2,"seq":1,"kind":"Speech","category":"social","payload":{"Text":"hi"}}\n'
    )
    args = argparse.Namespace(run_dir=str(tmp_path))
    rc = cmd_finalize(args)
    assert rc == 0
    # Outputs land.
    for f in ("derived.sqlite", "metrics.json", "judge_report.md", "REPORT.md"):
        assert (tmp_path / f).exists(), f"finalize missed {f}"
    # Metrics JSON has the expected categories.
    m = json.loads((tmp_path / "metrics.json").read_text())
    assert m["per_category"]["combat"] == 1
    assert m["per_category"]["social"] == 1
    # Judge report mentions stub model.
    assert "stub" in (tmp_path / "judge_report.md").read_text().lower()
    # Headline report has the run id.
    report = (tmp_path / "REPORT.md").read_text()
    assert "# Run test" in report
