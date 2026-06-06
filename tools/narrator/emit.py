"""NarratorSummary writer — appends one JSON line per emission.

Output schema (one event per line):

    {
      "tick": int,
      "level": "L1"|"L2"|"L3"|"L4",
      "scope": "agent_id"|"cluster_id"|"society"|"world",
      "actors": ["...", "..."],          # cluster/society only
      "text": "...",                     # narrator output
      "n_events": int,                   # how many events were summarized
      "llm": "qwen"|"claude"|"stub"|"skipped",
      "reason": "ok"|"budget_exhausted"|"no_events"|"...",
    }
"""
from __future__ import annotations

import json
from dataclasses import dataclass
from pathlib import Path
from typing import Optional


@dataclass
class NarratorOutput:
    path: Path

    def __post_init__(self) -> None:
        self.path.parent.mkdir(parents=True, exist_ok=True)
        # Open in line-buffered append mode so writes flush per line.
        self._f = open(self.path, "a", encoding="utf-8", buffering=1)

    def emit(
        self,
        *,
        tick: int,
        level: str,
        scope: str,
        text: str,
        n_events: int,
        llm: str,
        reason: str = "ok",
        actors: Optional[list[str]] = None,
    ) -> None:
        rec = {
            "tick":     tick,
            "level":    level,
            "scope":    scope,
            "actors":   actors or [],
            "text":     text,
            "n_events": n_events,
            "llm":      llm,
            "reason":   reason,
        }
        self._f.write(json.dumps(rec, separators=(",", ":")) + "\n")

    def close(self) -> None:
        if self._f and not self._f.closed:
            self._f.close()

    def __enter__(self) -> "NarratorOutput":
        return self

    def __exit__(self, *_) -> None:
        self.close()
