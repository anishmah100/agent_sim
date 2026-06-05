"""Deterministic stub LLM. Returns canned responses keyed by
(layer, fixture_name) so tests + offline smoke runs are reproducible.

Real Claude / Qwen clients implement the same protocol:

    class LLM(Protocol):
        def reflect(self, prompt: str, max_tokens: int = 500) -> str: ...
        def tactical(self, prompt: str, max_tokens: int = 200) -> dict: ...
        def persona(self, prompt: str, max_tokens: int = 500) -> dict: ...

When ANTHROPIC_API_KEY is set + --enable-claude is passed, main.py
swaps StubLLM for an Anthropic-backed implementation.
"""

from __future__ import annotations

import re
from dataclasses import dataclass


@dataclass
class StubLLM:
    """Trivial stub: produces a sane-shaped response by inspecting
    keywords in the prompt. Not "AI" — just enough to drive tests."""

    seed: int = 0

    # ---- Persona ----

    def persona(self, prompt: str, max_tokens: int = 500) -> dict:
        # The persona prompt embeds the bio; we pluck a few default
        # values out so the spawned agent has a coherent identity.
        return {
            "long_term_values": [
                "stay safe",
                "make a living",
                "form alliances when useful",
            ],
            "initial_goals": [
                {"goal": "find food when hungry", "why": "hunger pressure kills"},
                {"goal": "earn gold", "why": "needed for trade"},
            ],
        }

    # ---- Reflective ----

    def reflect(self, prompt: str, max_tokens: int = 500) -> dict:
        # Look at recent notes; produce a reflection that consolidates
        # the most recent observations.
        recent = re.findall(r"tactical_note: (.*)", prompt)
        if not recent:
            note = "no notable events; world feels calm"
        else:
            note = "saw " + ", ".join(recent[-3:]) + "; staying alert"
        return {
            "reflective_note": note,
            "goal_updates": [],
            "theory_of_mind_updates": {},
        }

    # ---- Tactical ----

    def tactical(self, prompt: str, max_tokens: int = 200) -> dict:
        # The stub returns a single wait action so tests pass cleanly.
        # Real Claude tactical returns 1-3 actions via tool use.
        return {
            "reasoning": "stub: no obvious action; waiting one tick",
            "actions": [
                {"verb": "wait", "ticks": 60}
            ],
        }
