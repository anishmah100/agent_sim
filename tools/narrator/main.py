"""Narrator entry point — wires the source → bucketizer → LLMs → output.

Run alongside the engine:

    python -m tools.narrator \
        --events .runlog/events.jsonl \
        --out    .runlog/narrator.jsonl \
        --max-claude-calls 8 \
        --max-qwen-calls 200

The narrator follows the events file forever (or until ``--idle-exit``
seconds pass with no new events). Each tier fires when the in-engine
tick crosses its cadence threshold.

Closing L4 summary: send SIGINT (Ctrl-C) or set ``--max-l4`` and the
narrator will, on shutdown, run the L4 summarizer over the entire
global buffer + emitted L3 summaries and exit.
"""
from __future__ import annotations

import argparse
import logging
import signal
import sys
from pathlib import Path
from typing import Optional

from .buckets import Bucketizer, ACTOR_KEYS, cluster_agents
from .config import NarratorConfig
from .emit import NarratorOutput
from .llm import (
    BudgetExceeded,
    ClaudeClient,
    LLMUnavailable,
    QwenClient,
    StubLLM,
)
from .source import iter_events


log = logging.getLogger("agent_sim.narrator")


# --- Prompt formatters ---


def fmt_event(ev: dict) -> str:
    """Compact one-line representation of an event."""
    kind = ev.get("kind", "?")
    payload = ev.get("payload") or {}
    tick = ev.get("tick", "?")
    # Compress payload: drop empties + verbose internals.
    interesting = {
        k: v for k, v in payload.items()
        if v not in (None, "", [], {}) and k not in ("Tick",)
    }
    return f"t={tick} {kind} {interesting}"


def l1_prompt(actor: str, events: list[dict]) -> str:
    lines = [f"Agent: {actor}",
             f"Recent events ({len(events)}):"]
    for ev in events[-30:]:
        lines.append("  " + fmt_event(ev))
    lines.append("\nSummarize what this agent has been doing.")
    return "\n".join(lines)


def l2_prompt(cluster: set[str], events: list[dict]) -> str:
    actor_list = ", ".join(sorted(cluster))
    cluster_events = [
        ev for ev in events
        if any(
            (ev.get("payload") or {}).get(k) in cluster
            for k in ACTOR_KEYS.values()
        )
    ]
    lines = [f"Cluster of agents: {actor_list}",
             f"Interactions in this window ({len(cluster_events)}):"]
    for ev in cluster_events[-40:]:
        lines.append("  " + fmt_event(ev))
    lines.append("\nSummarize the dynamic in this group.")
    return "\n".join(lines)


def l3_prompt(events: list[dict], l2_excerpts: list[str]) -> str:
    lines = [f"Society-level summary. Raw events in window: {len(events)}."]
    if l2_excerpts:
        lines.append("Recent cluster summaries:")
        for s in l2_excerpts[-6:]:
            lines.append("- " + s)
    else:
        lines.append("(no L2 cluster summaries yet — agents have not "
                     "formed interaction clusters in this window)")
    salient = [ev for ev in events if ev.get("kind") in (
        "EntityDied", "TaskProposed", "TaskAccepted", "TaskRejected",
        "TaskCompleted", "Whisper", "ItemTransferred", "MentalNote",
        "DamageDealt",
    )]
    if salient:
        lines.append("\nSalient events:")
        for ev in salient[-20:]:
            lines.append("  " + fmt_event(ev))
    else:
        # No salient events at all — fall back to a slice of the
        # global stream so Claude has something concrete to summarize
        # instead of a near-empty prompt.
        lines.append("\nSample events from window (no contracts, "
                     "deaths, or whispers occurred):")
        for ev in events[-25:]:
            lines.append("  " + fmt_event(ev))
    lines.append(
        "\nSummarize the state of this society: factions, conflicts, "
        "ongoing deals, anything emergent. If the window is uneventful "
        "say so plainly — do not invent drama. 3-5 sentences."
    )
    return "\n".join(lines)


def l4_prompt(l1_excerpts: list[str], l2_excerpts: list[str],
              l3_excerpts: list[str], n_total: int) -> str:
    lines = [
        f"Final closing summary. Total raw events: {n_total}.",
        "Recent L3 society snapshots:",
    ]
    for s in l3_excerpts[-3:]:
        lines.append("- " + s)
    lines.append("\nRecent L2 cluster snapshots:")
    for s in l2_excerpts[-6:]:
        lines.append("- " + s)
    lines.append("\nRecent L1 agent snapshots:")
    for s in l1_excerpts[-8:]:
        lines.append("- " + s)
    lines.append(
        "\nWrite the final narrative of this experiment: what kind of "
        "society formed, who the central characters were, what was "
        "remembered and what was lost. 5-8 sentences. Vivid but accurate."
    )
    return "\n".join(lines)


# --- Tier runners ---


def run_l1(*, bucket: Bucketizer, qwen, out: NarratorOutput,
           tick: int, history: list[str]) -> int:
    fired = 0
    for actor in bucket.all_actors_with_activity():
        events = bucket.drain_agent(actor)
        if not events:
            continue
        prompt = l1_prompt(actor, events)
        try:
            text = qwen.summarize(prompt, max_tokens=120)
            out.emit(tick=tick, level="L1", scope=actor, text=text,
                     n_events=len(events), llm="qwen")
            history.append(text)
            fired += 1
        except BudgetExceeded:
            out.emit(tick=tick, level="L1", scope=actor, text="",
                     n_events=len(events), llm="skipped",
                     reason="qwen_budget_exhausted")
            return fired
        except LLMUnavailable as e:
            log.warning("qwen unavailable: %s — falling back to stub", e)
            text = StubLLM("qwen-stub").summarize(prompt)
            out.emit(tick=tick, level="L1", scope=actor, text=text,
                     n_events=len(events), llm="stub",
                     reason="qwen_unavailable")
            history.append(text)
            fired += 1
    return fired


def run_l2(*, bucket: Bucketizer, qwen, out: NarratorOutput,
           tick: int, history: list[str], cluster_radius: int) -> int:
    events = bucket.peek_global()
    if not events:
        return 0
    clusters = cluster_agents(events, cluster_radius)
    fired = 0
    for cluster in clusters:
        if not cluster:
            continue
        prompt = l2_prompt(cluster, events)
        try:
            text = qwen.summarize(prompt, max_tokens=160)
            out.emit(tick=tick, level="L2",
                     scope="cluster:" + ",".join(sorted(cluster)),
                     actors=sorted(cluster), text=text,
                     n_events=len(events), llm="qwen")
            history.append(text)
            fired += 1
        except BudgetExceeded:
            out.emit(tick=tick, level="L2", scope="cluster", text="",
                     n_events=len(events), llm="skipped",
                     actors=sorted(cluster),
                     reason="qwen_budget_exhausted")
            return fired
        except LLMUnavailable as e:
            log.warning("qwen unavailable: %s — falling back to stub", e)
            text = StubLLM("qwen-stub").summarize(prompt)
            out.emit(tick=tick, level="L2",
                     scope="cluster:" + ",".join(sorted(cluster)),
                     actors=sorted(cluster), text=text,
                     n_events=len(events), llm="stub",
                     reason="qwen_unavailable")
            history.append(text)
            fired += 1
    return fired


def run_l3(*, bucket: Bucketizer, claude, out: NarratorOutput,
           tick: int, l2_history: list[str],
           l3_history: list[str]) -> int:
    events = bucket.drain_global()
    if not events:
        return 0
    prompt = l3_prompt(events, l2_history)
    try:
        text = claude.summarize(prompt, max_tokens=400)
        out.emit(tick=tick, level="L3", scope="society", text=text,
                 n_events=len(events), llm="claude")
        l3_history.append(text)
        return 1
    except BudgetExceeded:
        out.emit(tick=tick, level="L3", scope="society", text="",
                 n_events=len(events), llm="skipped",
                 reason="claude_budget_exhausted")
        return 0
    except LLMUnavailable as e:
        log.warning("claude unavailable: %s — falling back to stub", e)
        text = StubLLM("claude-stub").summarize(prompt)
        out.emit(tick=tick, level="L3", scope="society", text=text,
                 n_events=len(events), llm="stub",
                 reason="claude_unavailable")
        l3_history.append(text)
        return 1


def run_l4(*, claude, out: NarratorOutput, tick: int,
           l1_history: list[str], l2_history: list[str],
           l3_history: list[str], total_events: int) -> int:
    prompt = l4_prompt(l1_history, l2_history, l3_history, total_events)
    try:
        text = claude.summarize(prompt, max_tokens=600)
        out.emit(tick=tick, level="L4", scope="world", text=text,
                 n_events=total_events, llm="claude")
        return 1
    except BudgetExceeded:
        out.emit(tick=tick, level="L4", scope="world", text="",
                 n_events=total_events, llm="skipped",
                 reason="claude_budget_exhausted")
        return 0
    except LLMUnavailable as e:
        text = StubLLM("claude-stub").summarize(prompt)
        out.emit(tick=tick, level="L4", scope="world", text=text,
                 n_events=total_events, llm="stub",
                 reason="claude_unavailable")
        return 1


# --- Driver ---


class NarratorRun:
    """Owns the in-progress state; survives one engine session."""

    def __init__(
        self,
        cfg: NarratorConfig,
        qwen=None,
        claude=None,
    ) -> None:
        self.cfg = cfg
        self.qwen = qwen if qwen is not None else QwenClient(
            endpoint=cfg.qwen_endpoint, model=cfg.qwen_model,
            refuse_after=cfg.max_qwen_calls,
        )
        self.claude = claude if claude is not None else ClaudeClient(
            model=cfg.claude_l3_model, refuse_after=cfg.max_claude_calls,
        )
        self.bucket = Bucketizer()
        self.out = NarratorOutput(cfg.output_path)
        self.l1_history: list[str] = []
        self.l2_history: list[str] = []
        self.l3_history: list[str] = []
        # Tick counters for each tier.
        self.next_l1 = cfg.l1_cadence_ticks
        self.next_l2 = cfg.l2_cadence_ticks
        self.next_l3 = cfg.l3_cadence_ticks
        self.last_tick = 0
        self.total_events = 0
        self.stopping = False

    def stop(self, *_) -> None:
        self.stopping = True

    def run(self) -> None:
        signal.signal(signal.SIGINT, self.stop)
        signal.signal(signal.SIGTERM, self.stop)
        try:
            for ev in iter_events(
                self.cfg.events_path,
                follow=True,
                idle_exit_seconds=self.cfg.idle_exit_seconds,
            ):
                if self.stopping:
                    break
                self.bucket.ingest(ev)
                self.total_events += 1
                tick = int(ev.get("tick", 0) or 0)
                self.last_tick = max(self.last_tick, tick)
                self._maybe_fire(tick)
        finally:
            # Closing L4 summary.
            self._fire_l4()
            self.out.close()

    def _maybe_fire(self, tick: int) -> None:
        if tick >= self.next_l1:
            run_l1(bucket=self.bucket, qwen=self.qwen, out=self.out,
                   tick=tick, history=self.l1_history)
            self.next_l1 = tick + self.cfg.l1_cadence_ticks
        if tick >= self.next_l2:
            run_l2(bucket=self.bucket, qwen=self.qwen, out=self.out,
                   tick=tick, history=self.l2_history,
                   cluster_radius=self.cfg.cluster_radius_tiles)
            self.next_l2 = tick + self.cfg.l2_cadence_ticks
        if tick >= self.next_l3:
            run_l3(bucket=self.bucket, claude=self.claude, out=self.out,
                   tick=tick, l2_history=self.l2_history,
                   l3_history=self.l3_history)
            self.next_l3 = tick + self.cfg.l3_cadence_ticks

    def _fire_l4(self) -> None:
        try:
            run_l4(claude=self.claude, out=self.out, tick=self.last_tick,
                   l1_history=self.l1_history,
                   l2_history=self.l2_history,
                   l3_history=self.l3_history,
                   total_events=self.total_events)
        except Exception:
            log.exception("L4 closing summary failed")


def parse_args(argv: list[str]) -> NarratorConfig:
    p = argparse.ArgumentParser(prog="narrator")
    p.add_argument("--events", default=".runlog/events.jsonl",
                   help="path to engine's events jsonl")
    p.add_argument("--out", default=".runlog/narrator.jsonl",
                   help="path to write NarratorSummary records")
    p.add_argument("--max-qwen-calls", type=int, default=200)
    p.add_argument("--max-claude-calls", type=int, default=8)
    p.add_argument("--l1-ticks", type=int, default=60 * 60)
    p.add_argument("--l2-ticks", type=int, default=5 * 60 * 60)
    p.add_argument("--l3-ticks", type=int, default=15 * 60 * 60)
    p.add_argument("--idle-exit", type=float, default=None,
                   help="exit after N seconds without new events")
    p.add_argument("--cluster-radius", type=int, default=20)
    p.add_argument("--verbose", action="store_true")
    a = p.parse_args(argv)
    cfg = NarratorConfig(
        events_path=Path(a.events),
        output_path=Path(a.out),
        l1_cadence_ticks=a.l1_ticks,
        l2_cadence_ticks=a.l2_ticks,
        l3_cadence_ticks=a.l3_ticks,
        max_qwen_calls=a.max_qwen_calls,
        max_claude_calls=a.max_claude_calls,
        idle_exit_seconds=a.idle_exit,
        cluster_radius_tiles=a.cluster_radius,
        verbose=a.verbose,
    )
    return cfg


def main(argv: Optional[list[str]] = None) -> int:
    argv = list(argv if argv is not None else sys.argv[1:])
    cfg = parse_args(argv)
    logging.basicConfig(level=logging.DEBUG if cfg.verbose else logging.INFO,
                        format="%(asctime)s %(levelname)s %(message)s")
    NarratorRun(cfg).run()
    return 0


if __name__ == "__main__":
    sys.exit(main())
