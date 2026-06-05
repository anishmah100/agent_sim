"""LLM-as-judge scaffold (Phase SUB-11).

Reads a derived.sqlite + a rubric (list of criteria the experiment
wants graded), and produces a structured judge report:

  {
    "rubric":      ["did agents form alliances?", ...],
    "scores":      [{"criterion": "...", "score_1_to_5": 3,
                     "evidence": "tick 1400 — Alice + Bob made trade ring"}],
    "summary":     "Free-text 1-paragraph overall.",
    "judge_model": "stub" | "claude-3-5-sonnet" | ...
  }

The LLMJudge Protocol lets us swap implementations:
  - StubJudge   — deterministic, used in tests + no-API runs
  - AnthropicJudge — placeholder, raises until key + plumbing land

CLI:
    python -m tools.judge.judge derived.sqlite criteria.txt > report.md

(criteria.txt has one rubric line per row)
"""

from __future__ import annotations

import json
import sqlite3
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional, Protocol


# ---- Protocol + implementations ----

class LLMJudge(Protocol):
    name: str

    def judge(self, rubric: list[str], context: str) -> dict: ...


@dataclass
class StubJudge:
    name: str = "stub"
    score_per_criterion: int = 3  # neutral by default

    def judge(self, rubric: list[str], context: str) -> dict:
        return {
            "scores": [
                {
                    "criterion": c,
                    "score_1_to_5": self.score_per_criterion,
                    "evidence": "[stub] cannot evaluate without a live LLM",
                }
                for c in rubric
            ],
            "summary": (
                "[stub judge] No qualitative judgement available. "
                "Set ANTHROPIC_API_KEY + --enable-claude to upgrade."
            ),
        }


@dataclass
class AnthropicJudge:
    name: str = "claude"

    def judge(self, rubric: list[str], context: str) -> dict:
        raise NotImplementedError(
            "Anthropic-backed judge not wired yet — see "
            "docs/EXPERIMENT_SYSTEM_PLAN.md §11. Use StubJudge for now."
        )


# ---- Context builder ----

def build_context_from_sqlite(db_path: str, sample_traces: int = 30,
                              sample_dialogue: int = 30) -> str:
    """Produce a compact prose context the judge LLM reads. Pulls
    representative dialogue + reasoning traces + headline metrics."""
    db = sqlite3.connect(db_path)
    db.row_factory = sqlite3.Row
    parts: list[str] = []
    try:
        # Headline event counts.
        cats = db.execute(
            "SELECT category, COUNT(*) AS c FROM events GROUP BY category"
        ).fetchall()
        if cats:
            parts.append("CATEGORY COUNTS:")
            for r in cats:
                parts.append(f"  {r['category']:<18} {r['c']}")
        # Top kinds.
        kinds = db.execute(
            "SELECT kind, COUNT(*) AS c FROM events GROUP BY kind ORDER BY c DESC LIMIT 12"
        ).fetchall()
        if kinds:
            parts.append("\nTOP EVENT KINDS:")
            for r in kinds:
                parts.append(f"  {r['kind']:<22} {r['c']}")
        # Reasoning sample.
        traces = db.execute(
            f"SELECT entity_id, verb, reasoning FROM reasoning_traces "
            f"ORDER BY RANDOM() LIMIT {int(sample_traces)}"
        ).fetchall()
        if traces:
            parts.append(f"\nSAMPLE REASONING TRACES (n={len(traces)}):")
            for r in traces:
                snippet = r["reasoning"][:120]
                parts.append(f"  [{r['entity_id']} verb={r['verb']}] {snippet}")
        # Speech sample.
        speech = db.execute(
            "SELECT tick, payload FROM events WHERE kind='Speech' "
            f"ORDER BY RANDOM() LIMIT {int(sample_dialogue)}"
        ).fetchall()
        if speech:
            parts.append(f"\nSAMPLE SPEECH (n={len(speech)}):")
            for r in speech:
                try:
                    j = json.loads(r["payload"])
                    text = j.get("Text") or j.get("text") or ""
                    speaker = j.get("Speaker") or j.get("From") or "?"
                except Exception:
                    text, speaker = "(unparseable)", "?"
                parts.append(f"  t={r['tick']} {speaker}: {text[:100]}")
    finally:
        db.close()
    return "\n".join(parts)


# ---- Top-level ----

@dataclass
class JudgeReport:
    rubric: list[str]
    scores: list[dict[str, Any]]
    summary: str
    judge_model: str

    def to_dict(self) -> dict:
        return {
            "rubric": self.rubric,
            "scores": self.scores,
            "summary": self.summary,
            "judge_model": self.judge_model,
        }

    def to_markdown(self) -> str:
        lines = [
            f"# Judge report ({self.judge_model})",
            "",
            "## Summary",
            self.summary,
            "",
            "## Per-criterion scores",
            "",
            "| Criterion | Score (1-5) | Evidence |",
            "|---|---|---|",
        ]
        for s in self.scores:
            crit = s.get("criterion", "?").replace("|", "\\|")
            score = s.get("score_1_to_5", "—")
            evidence = (s.get("evidence", "") or "").replace("|", "\\|")
            lines.append(f"| {crit} | {score} | {evidence} |")
        return "\n".join(lines) + "\n"


def run_judge(
    db_path: str,
    rubric: list[str],
    llm: Optional[LLMJudge] = None,
) -> JudgeReport:
    judge = llm or StubJudge()
    ctx = build_context_from_sqlite(db_path)
    raw = judge.judge(rubric, ctx)
    return JudgeReport(
        rubric=rubric,
        scores=raw.get("scores", []),
        summary=raw.get("summary", ""),
        judge_model=judge.name,
    )


def main() -> None:
    if len(sys.argv) < 3:
        print(
            "usage: python -m tools.judge.judge <derived.sqlite> <criteria.txt>",
            file=sys.stderr,
        )
        sys.exit(2)
    db_path = sys.argv[1]
    rubric_file = Path(sys.argv[2])
    rubric = [line.strip() for line in rubric_file.read_text().splitlines() if line.strip()]
    report = run_judge(db_path, rubric)
    sys.stdout.write(report.to_markdown())


if __name__ == "__main__":
    main()
