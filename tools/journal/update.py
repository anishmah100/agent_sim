"""JOURNAL.md + INDEX.md maintenance pipeline (Phase SUB-13).

The auto-research loop persists what it has learned across runs. Three
files at the experiments root:

  experiments/JOURNAL.md             — cross-world meta-learnings,
                                       hand-curated. The script just
                                       makes sure a header exists.
  experiments/INDEX.md               — auto-generated table of every
                                       run (run_id + world + slug +
                                       headline metrics + judge model).
  experiments/<world>/WORLD_JOURNAL.md — per-world findings; each
                                       finalize() appends a new entry
                                       with the run id and a one-line
                                       takeaway.

Called by `exp finalize` (tools/exp/cli.py) after a run's REPORT.md is
written. Also runnable manually:

    python -m tools.journal.update <run-dir>
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Iterable


EXP_ROOT_DEFAULT = Path(__file__).resolve().parents[2] / "experiments"

TOP_JOURNAL_HEADER = (
    "# Experiment journal — cross-world learnings\n\n"
    "Hand-curated. The auto-loop adds run entries to INDEX.md and\n"
    "WORLD_JOURNAL.md but never edits this file.\n\n"
)


def ensure_top_journal(exp_root: Path) -> None:
    f = exp_root / "JOURNAL.md"
    if not f.exists():
        exp_root.mkdir(parents=True, exist_ok=True)
        f.write_text(TOP_JOURNAL_HEADER, encoding="utf-8")


def append_index_row(exp_root: Path, run_dir: Path) -> None:
    """Add a row to INDEX.md if not already there."""
    idx = exp_root / "INDEX.md"
    exp_root.mkdir(parents=True, exist_ok=True)
    if not idx.exists():
        idx.write_text(
            "# Run index\n\n"
            "| Run | World | Slug | Parent | Events | Deaths | Trades | Reasoning | Judge |\n"
            "|---|---|---|---|---|---|---|---|---|\n",
            encoding="utf-8",
        )
    meta = json.loads((run_dir / "metadata.json").read_text())
    metrics = json.loads((run_dir / "metrics.json").read_text()) \
        if (run_dir / "metrics.json").exists() else {}
    judge = ""
    if (run_dir / "judge_report.md").exists():
        # First line after the title is "# Judge report (<model>)" — pull
        # the model.
        first = (run_dir / "judge_report.md").read_text().splitlines()[:1]
        if first and "(" in first[0]:
            judge = first[0].split("(", 1)[1].rstrip(")")
    row = (
        f"| `{meta['run_id']}` "
        f"| {meta['world']} "
        f"| {meta['slug']} "
        f"| {meta.get('parent') or '—'} "
        f"| {metrics.get('total_events', '?')} "
        f"| {metrics.get('deaths', '?')} "
        f"| {metrics.get('transactions', '?')} "
        f"| {metrics.get('reasoning_traces', '?')} "
        f"| {judge or 'stub'} |\n"
    )
    existing = idx.read_text()
    if f"`{meta['run_id']}`" in existing:
        return  # already indexed
    idx.write_text(existing + row, encoding="utf-8")


def append_world_journal(exp_root: Path, run_dir: Path) -> None:
    meta = json.loads((run_dir / "metadata.json").read_text())
    world_dir = exp_root / meta["world"]
    world_dir.mkdir(parents=True, exist_ok=True)
    journal = world_dir / "WORLD_JOURNAL.md"
    if not journal.exists():
        journal.write_text(
            f"# {meta['world']} journal\n\n"
            "Append-only per-run takeaways. Hand-edit the prose;\n"
            "the auto-loop appends the section headers.\n\n",
            encoding="utf-8",
        )
    # One stub entry per run; the human writes the takeaway.
    entry = (
        f"\n## {meta['run_id']} — {meta['slug']}\n"
        f"\n- Parent: `{meta.get('parent') or '(root)'}`\n"
        f"- Rubric: {meta.get('rubric') or '(default)'}\n"
        f"- Report: [`REPORT.md`]({meta['run_id']}/REPORT.md)\n"
        f"\n**Takeaway:** _TODO — fill in what this run taught us._\n"
    )
    existing = journal.read_text()
    if f"## {meta['run_id']}" in existing:
        return
    journal.write_text(existing + entry, encoding="utf-8")


def update_all(run_dir: Path, exp_root: Path = EXP_ROOT_DEFAULT) -> None:
    if not (run_dir / "metadata.json").exists():
        raise SystemExit(f"not a run dir (missing metadata.json): {run_dir}")
    ensure_top_journal(exp_root)
    append_index_row(exp_root, run_dir)
    append_world_journal(exp_root, run_dir)


def main(argv: Iterable[str] = None) -> int:
    p = argparse.ArgumentParser()
    p.add_argument("run_dir")
    p.add_argument("--exp-root", default=str(EXP_ROOT_DEFAULT))
    args = p.parse_args(list(argv) if argv is not None else None)
    update_all(Path(args.run_dir), Path(args.exp_root))
    print(f"updated journal index for {args.run_dir}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
