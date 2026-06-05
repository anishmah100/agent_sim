"""Tests for the iteration loop orchestrator (SUB-14)."""

from __future__ import annotations

import json
import sys
from pathlib import Path

import pytest

_REPO = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(_REPO))

from tools.loop.orchestrator import Batch, apply_tunings_patch, run_batch  # noqa: E402


def _fake_runner(run_dir: Path) -> None:
    """A test runner that writes a tiny synthetic logs.jsonl instead
    of actually spawning the engine."""
    (run_dir / "logs.jsonl").write_text(
        '{"tick":1,"seq":0,"kind":"DamageDealt","category":"combat","payload":{"Amount":10}}\n'
        '{"tick":2,"seq":1,"kind":"Speech","category":"social","payload":{"Text":"hi"}}\n'
    )


def test_apply_tunings_patch_appends(tmp_path):
    rules = tmp_path / "rules.star"
    rules.write_text('register_tuning("attack_damage", 10)\n')
    apply_tunings_patch(rules, {"attack_damage": 15, "hunger_per_tick": 0.0012})
    body = rules.read_text()
    # Original line still there.
    assert 'register_tuning("attack_damage", 10)' in body
    # New lines appended.
    assert 'register_tuning("attack_damage", 15)' in body
    assert 'register_tuning("hunger_per_tick", 0.0012)' in body


def test_run_batch_single_iteration(tmp_path):
    # Use eldoria (its bundle exists in the repo).
    batch = Batch(
        world="eldoria",
        parent_id=None,
        runs=[("hunger-bump", {"hunger_per_tick": 0.002})],
        rubric=["did things happen?"],
    )
    results = run_batch(
        batch, runner=_fake_runner, exp_root=tmp_path, today="20260605",
    )
    assert len(results) == 1
    r = results[0]
    assert r.succeeded, r.note
    assert r.run_id.startswith("20260605-hunger-bump-")
    # The patch landed.
    rules = (r.run_dir / "bundle" / "rules.star").read_text()
    assert "hunger_per_tick" in rules
    assert "0.002" in rules
    # Finalize wrote the standard outputs.
    for f in ("derived.sqlite", "metrics.json", "judge_report.md", "REPORT.md"):
        assert (r.run_dir / f).exists(), f"missing {f}"
    # Journals updated.
    assert (tmp_path / "INDEX.md").exists()
    assert (tmp_path / "eldoria" / "WORLD_JOURNAL.md").exists()


def test_run_batch_chains_parent(tmp_path):
    batch = Batch(
        world="eldoria",
        runs=[
            ("baseline", {}),
            ("then-tune", {"attack_damage": 20}),
        ],
    )
    results = run_batch(
        batch, runner=_fake_runner, exp_root=tmp_path, today="20260605",
    )
    assert len(results) == 2
    # Second iteration's parent should be the first's run id.
    meta_2 = json.loads((results[1].run_dir / "metadata.json").read_text())
    assert meta_2["parent"] == results[0].run_id


def test_run_batch_records_runner_failure(tmp_path):
    def bad_runner(_dir):
        raise RuntimeError("simulated engine crash")
    batch = Batch(world="eldoria", runs=[("crash", {})])
    results = run_batch(
        batch, runner=bad_runner, exp_root=tmp_path, today="20260605",
    )
    assert results[0].succeeded is False
    assert "simulated engine crash" in results[0].note
