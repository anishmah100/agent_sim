"""experiment CLI (Phase SUB-12).

Implements the folder-per-experiment layout the auto-research loop
consumes:

  experiments/<world>/<run-id>/
    metadata.json          parent + rubric + created_at
    bundle/                snapshot of the world bundle the run used
      world.json
      bundle.toml
      rules.star           run-specific tweaks
    logs.jsonl             written by the engine (target of -event-log)
    derived.sqlite         produced by `exp finalize`
    metrics.json           produced by `exp finalize`
    judge_report.md        produced by `exp finalize`
    REPORT.md              produced by `exp finalize`

CLI:
    python -m tools.exp.cli new      eldoria  --slug hunger-tweak
    python -m tools.exp.cli finalize experiments/eldoria/<id>/

Sample run script (the user runs this themselves):
    bin/engine -addr :8080 \
        -bundle experiments/eldoria/<id>/bundle \
        -event-log experiments/eldoria/<id>/logs.jsonl \
        -capture-reasoning

Then `exp finalize` post-processes the logs.

This is a thin orchestrator; the heavy lifting (sqlite, metrics, judge)
lives in tools/metrics, tools/judge, engine/cmd/jsonl2sqlite.
"""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional


REPO = Path(__file__).resolve().parents[2]
EXP_ROOT_DEFAULT = REPO / "experiments"


@dataclass
class RunMetadata:
    run_id: str
    world: str
    slug: str
    parent: Optional[str] = None
    created_at: str = ""
    rubric: list[str] = field(default_factory=list)

    def to_dict(self) -> dict:
        return {
            "run_id":     self.run_id,
            "world":      self.world,
            "slug":       self.slug,
            "parent":     self.parent,
            "created_at": self.created_at,
            "rubric":     self.rubric,
        }


# ---- new ----

def _next_run_id(world_dir: Path, slug: str, today: Optional[str] = None) -> str:
    """Assign a fresh <YYYYMMDD>-<slug>-NNN id under world_dir."""
    today = today or datetime.now(timezone.utc).strftime("%Y%m%d")
    existing = [d.name for d in world_dir.glob(f"{today}-{slug}-*") if d.is_dir()]
    n = 1
    while f"{today}-{slug}-{n:03d}" in existing:
        n += 1
    return f"{today}-{slug}-{n:03d}"


def cmd_new(args: argparse.Namespace) -> int:
    bundles_root = REPO / "worlds"
    bundle_src = bundles_root / args.world
    if not (bundle_src / "bundle.toml").exists():
        print(f"no bundle at {bundle_src}", file=sys.stderr)
        return 2
    world_dir = (EXP_ROOT_DEFAULT if args.exp_root is None else Path(args.exp_root)) / args.world
    world_dir.mkdir(parents=True, exist_ok=True)
    run_id = _next_run_id(world_dir, args.slug or "run", today=args.today)
    run_dir = world_dir / run_id
    run_dir.mkdir()

    # Snapshot the bundle.
    bundle_snap = run_dir / "bundle"
    shutil.copytree(bundle_src, bundle_snap)

    meta = RunMetadata(
        run_id=run_id, world=args.world, slug=args.slug or "run",
        parent=args.parent,
        created_at=datetime.now(timezone.utc).isoformat(),
        rubric=list(args.rubric or []),
    )
    (run_dir / "metadata.json").write_text(
        json.dumps(meta.to_dict(), indent=2), encoding="utf-8",
    )
    print(f"created {run_dir}")
    return 0


# ---- finalize ----

def cmd_finalize(args: argparse.Namespace) -> int:
    run_dir = Path(args.run_dir)
    if not (run_dir / "metadata.json").exists():
        print(f"not a run directory (no metadata.json): {run_dir}", file=sys.stderr)
        return 2
    meta = json.loads((run_dir / "metadata.json").read_text())
    logs = run_dir / "logs.jsonl"
    if not logs.exists():
        print(f"no logs.jsonl in {run_dir} — run engine first with "
              f"-event-log {logs}", file=sys.stderr)
        return 3

    derived = run_dir / "derived.sqlite"
    metrics = run_dir / "metrics.json"
    judge   = run_dir / "judge_report.md"
    report  = run_dir / "REPORT.md"

    # 1. jsonl2sqlite
    _run_go_tool(
        ["go", "run", "./cmd/jsonl2sqlite", "-in", str(logs), "-out", str(derived)],
        cwd=REPO / "engine",
    )

    # 2. metrics catalog
    from tools.metrics.catalog import compute_all, to_dict
    m = compute_all(str(derived))
    metrics.write_text(json.dumps(to_dict(m), indent=2), encoding="utf-8")

    # 3. judge (stub by default)
    from tools.judge.judge import StubJudge, run_judge
    rubric: list[str] = meta.get("rubric") or [
        "did agents form alliances?",
        "was there scheming or deception?",
        "did anything resembling enforcement emerge?",
    ]
    rep = run_judge(str(derived), rubric, llm=StubJudge())
    judge.write_text(rep.to_markdown(), encoding="utf-8")

    # 4. REPORT.md — punchy 1-page synthesis
    report.write_text(_render_report(meta, m, rep), encoding="utf-8")

    # 5. Journal pipeline — append a row to INDEX.md and a section to
    # the per-world WORLD_JOURNAL.md so SUB-14's diagnose step can
    # find this run later.
    from tools.journal.update import update_all
    update_all(run_dir, EXP_ROOT_DEFAULT)

    print(f"finalized {run_dir}")
    return 0


def _run_go_tool(cmd: list[str], cwd: Path) -> None:
    subprocess.check_call(cmd, cwd=str(cwd))


def _render_report(meta: dict, metrics, judge_report) -> str:
    lines = [
        f"# Run {meta['run_id']}",
        "",
        f"- World: `{meta['world']}`",
        f"- Slug: `{meta['slug']}`",
        f"- Parent: `{meta['parent'] or '(root)'}`",
        f"- Created: {meta['created_at']}",
        "",
        "## Headline metrics",
        f"- Total events: {metrics.total_events}",
        f"- Per category: {dict(sorted(metrics.per_category.items()))}",
        f"- Deaths: {metrics.deaths}  ·  Transactions: {metrics.transactions}",
        f"- Speech / Whisper / Shout: "
        f"{metrics.speech_count} / {metrics.whisper_count} / {metrics.shout_count}",
        f"- Reasoning traces: {metrics.reasoning_traces} "
        f"(from {metrics.unique_reasoning_agents} agents)",
        "",
        "## Judge",
        f"_{judge_report.judge_model}_",
        "",
        judge_report.summary,
        "",
    ]
    return "\n".join(lines)


# ---- argparse ----

def main() -> int:
    p = argparse.ArgumentParser(prog="exp")
    sub = p.add_subparsers(required=True)

    pn = sub.add_parser("new", help="create a fresh run directory")
    pn.add_argument("world", help="bundle name under worlds/")
    pn.add_argument("--slug", default="run")
    pn.add_argument("--parent", default=None)
    pn.add_argument("--rubric", nargs="*", default=None,
                    help="one rubric line per arg")
    pn.add_argument("--exp-root", default=None)
    pn.add_argument("--today", default=None,
                    help="override the YYYYMMDD stamp (used by tests)")
    pn.set_defaults(func=cmd_new)

    pf = sub.add_parser("finalize", help="post-process a finished run")
    pf.add_argument("run_dir")
    pf.set_defaults(func=cmd_finalize)

    args = p.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
