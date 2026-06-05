"""AlphaEvolve-style iteration loop orchestrator (Phase SUB-14).

Reads JOURNAL.md for stable findings, applies a batch of ruleset
mutations, runs the engine per mutation, calls `exp finalize`, and
appends each result to JOURNAL.md.

Three execution modes:

  manual   — list of (slug, tunings_dict) tuples supplied to the
             orchestrator. No LLM. Useful for sweeps and the unit
             tests in this file.

  scripted — same as manual but the deltas come from a YAML/JSON
             file the user wrote. Same flow, just a thin frontend.

  llm      — placeholder. Real implementation calls an LLM with
             JOURNAL.md + INDEX.md + the last N run reports + the
             target properties, then asks for a proposed delta.
             Raises NotImplementedError until SUB-15 wires Claude.

The engine run itself is delegated through a runner callable, so tests
substitute a no-op runner and assert the loop structure without
actually spawning a 60-second engine subprocess.
"""

from __future__ import annotations

import json
import shutil
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Callable, Iterable, Optional


REPO = Path(__file__).resolve().parents[2]
EXP_ROOT_DEFAULT = REPO / "experiments"


@dataclass
class IterationResult:
    run_id: str
    run_dir: Path
    slug: str
    tunings_applied: dict[str, Any]
    succeeded: bool
    note: str = ""


@dataclass
class Batch:
    world: str
    parent_id: Optional[str] = None
    runs: list[tuple[str, dict[str, Any]]] = field(default_factory=list)
    rubric: list[str] = field(default_factory=lambda: [
        "did agents form alliances?",
        "was scheming present?",
        "did anything resembling enforcement emerge?",
    ])


# ---- Runner protocol ----

EngineRunner = Callable[[Path], None]
"""Signature: runner(run_dir) — must produce logs.jsonl inside run_dir.
The default implementation invokes the engine binary; tests pass a
stub that writes a synthetic logs.jsonl."""


def default_engine_runner(run_dir: Path, runtime_seconds: int = 60,
                           addr: str = "127.0.0.1:8089") -> None:
    """Build + run the engine for `runtime_seconds`, then SIGTERM."""
    bin_path = REPO / ".runlog" / "engine_loop"
    bin_path.parent.mkdir(parents=True, exist_ok=True)
    subprocess.check_call(
        ["go", "build", "-o", str(bin_path), "./cmd/engine"],
        cwd=str(REPO / "engine"),
    )
    log_path = run_dir / "logs.jsonl"
    bundle_dir = run_dir / "bundle"
    proc = subprocess.Popen(
        [str(bin_path),
         "-addr", addr,
         "-bundle", str(bundle_dir),
         "-event-log", str(log_path),
         "-capture-reasoning",
         "-register-rate", "100", "-register-burst", "100"],
        cwd=str(REPO),
        stdout=open(run_dir / "engine.log", "w"),
        stderr=subprocess.STDOUT,
    )
    try:
        proc.wait(timeout=runtime_seconds)
    except subprocess.TimeoutExpired:
        proc.terminate()
        try:
            proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            proc.kill()


# ---- Ruleset mutation ----

def apply_tunings_patch(rules_star: Path, patch: dict[str, Any]) -> None:
    """Append register_tuning() calls for each (k, v) in patch.

    The latest binding wins in Starlark, so appending is semantically
    a "set" not an "add" — letting us layer a tweak on top of the
    parent's ruleset without needing a real Starlark parser.
    """
    if not patch:
        return
    lines = ["\n# ---- iteration patch ----"]
    for k, v in patch.items():
        if isinstance(v, bool):
            sval = "True" if v else "False"
        elif isinstance(v, (int, float)):
            sval = repr(v)
        elif isinstance(v, str):
            sval = repr(v)
        else:
            sval = repr(v)
        lines.append(f'register_tuning("{k}", {sval})')
    body = "\n".join(lines) + "\n"
    with open(rules_star, "a", encoding="utf-8") as f:
        f.write(body)


# ---- Loop ----

def run_batch(
    batch: Batch,
    runner: EngineRunner = default_engine_runner,
    exp_root: Path = EXP_ROOT_DEFAULT,
    today: Optional[str] = None,
) -> list[IterationResult]:
    """Execute each (slug, tunings) in batch.runs sequentially. Each
    iteration creates a new run dir, patches rules.star, runs the
    engine via the supplied runner, finalizes, and updates the journals.
    """
    from tools.exp.cli import RunMetadata, _next_run_id

    bundle_src = REPO / "worlds" / batch.world
    if not (bundle_src / "bundle.toml").exists():
        raise FileNotFoundError(f"no bundle for world {batch.world}")
    out: list[IterationResult] = []
    parent_id = batch.parent_id
    for slug, tunings in batch.runs:
        world_dir = exp_root / batch.world
        world_dir.mkdir(parents=True, exist_ok=True)
        run_id = _next_run_id(world_dir, slug, today=today)
        run_dir = world_dir / run_id
        run_dir.mkdir()

        # Snapshot the bundle + apply the patch.
        bundle_snap = run_dir / "bundle"
        shutil.copytree(bundle_src, bundle_snap)
        rules_star = bundle_snap / "rules.star"
        if rules_star.exists() and tunings:
            apply_tunings_patch(rules_star, tunings)

        # Metadata.
        from datetime import datetime, timezone
        meta = RunMetadata(
            run_id=run_id, world=batch.world, slug=slug,
            parent=parent_id,
            created_at=datetime.now(timezone.utc).isoformat(),
            rubric=batch.rubric,
        )
        (run_dir / "metadata.json").write_text(
            json.dumps(meta.to_dict(), indent=2), encoding="utf-8",
        )

        succeeded = True
        note = ""
        try:
            runner(run_dir)
        except Exception as e:
            succeeded = False
            note = f"runner failed: {e}"

        if succeeded and (run_dir / "logs.jsonl").exists():
            # Finalize the run.
            try:
                from tools.exp.cli import cmd_finalize
                import argparse
                args = argparse.Namespace(run_dir=str(run_dir))
                rc = cmd_finalize(args)
                if rc != 0:
                    succeeded = False
                    note = f"finalize rc={rc}"
            except Exception as e:
                succeeded = False
                note = f"finalize failed: {e}"

        out.append(IterationResult(
            run_id=run_id, run_dir=run_dir, slug=slug,
            tunings_applied=dict(tunings),
            succeeded=succeeded, note=note,
        ))
        parent_id = run_id  # the next iteration descends from this run
    return out
