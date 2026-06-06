"""Narrator config — cadences, cost caps, model selection."""
from __future__ import annotations

from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Optional


# In-game tick scale: Eldoria runs at 60 Hz, so 60 ticks = 1 in-game sec.
TICKS_PER_INGAME_SECOND = 60
TICKS_PER_INGAME_MINUTE = TICKS_PER_INGAME_SECOND * 60


@dataclass
class NarratorConfig:
    """All-in-one config. Cadences are in in-game ticks (engine ticks);
    the wall clock is irrelevant — narrator runs against engine time so
    the time-multiplier doesn't change the cadence meaning."""

    # ---- IO ----
    events_path: Path = Path(".runlog/events.jsonl")
    output_path: Path = Path(".runlog/narrator.jsonl")

    # ---- Cadences (in ticks) ----
    # 60 sec → L1 every 60 in-game sec.
    l1_cadence_ticks: int = 60 * TICKS_PER_INGAME_SECOND
    # 5 min → L2 every 5 in-game min.
    l2_cadence_ticks: int = 5 * TICKS_PER_INGAME_MINUTE
    # 15 min → L3 every 15 in-game min.
    l3_cadence_ticks: int = 15 * TICKS_PER_INGAME_MINUTE
    # L4 is once at end (manual close call), no periodic cadence.

    # ---- Cost caps (hard ceilings; refuse calls past these) ----
    max_qwen_calls: int = 200
    max_claude_calls: int = 8

    # ---- Model selection ----
    qwen_endpoint: str = "http://127.0.0.1:8782/v1"
    qwen_model: str = "qwen3.6"
    claude_l3_model: str = "claude-haiku-4-5-20251001"
    claude_l4_model: str = "claude-sonnet-4-6"

    # ---- Clustering for L2 ----
    # Two agents are in the same cluster if Chebyshev distance ≤
    # this AND they spoke / interacted within the same L2 window.
    cluster_radius_tiles: int = 20

    # ---- Logging ----
    verbose: bool = False
    # Stop tailing after this many seconds idle (i.e. no new
    # events appear). Useful for one-shot post-run runs.
    idle_exit_seconds: Optional[float] = None

    def with_overrides(self, **kw) -> "NarratorConfig":
        merged = asdict(self)
        merged.update(kw)
        # path fields need to come back as Path
        merged["events_path"] = Path(merged["events_path"])
        merged["output_path"] = Path(merged["output_path"])
        return NarratorConfig(**merged)
