"""Prompt templates for the Claude harness. Plain f-strings so the
shape is inspectable; no Jinja or template engine.

A real Claude integration would feed these into the Anthropic Messages
API with tool use to emit structured action batches. The shape here
matches what the stub expects, so swapping in the real client is a
one-line change.
"""

from __future__ import annotations

from .state import BrainState


def persona_prompt(state: BrainState) -> str:
    p = state.persona
    return f"""You are {p.name}, a {p.archetype} in this world.

Bio: {p.bio}

The world's rulebook has been provided separately. Your job right now:
distill 3-5 long-term values you commit to (one sentence each), and
1-3 initial top-level goals derived from your persona.

Output JSON with keys: long_term_values, initial_goals.
"""


def reflective_prompt(state: BrainState, recent_tactical: list[str]) -> str:
    notes = "\n".join(f"tactical_note: {n}" for n in recent_tactical[-12:])
    values = "\n".join(f"- {v}" for v in state.persona.long_term_values)
    return f"""Step back. You are {state.persona.name}.

Long-term values:
{values}

Current goals (top first):
{_goals_str(state)}

Recent tactical notes (last ~12):
{notes}

Reflect on the past ~minute. What changed? Are your goals still right?
Output JSON with keys: reflective_note (one paragraph), goal_updates
(list of {{action, goal, why}}), theory_of_mind_updates
(map entity_id -> new theory_of_me string).
"""


def tactical_prompt(state: BrainState, observation_block: str) -> str:
    return f"""You are {state.persona.name}.

CURRENT TOP GOAL: {(state.top_goal() or "wander").goal if state.top_goal() else "wander"}

OBSERVATION:
{observation_block}

RECENT TACTICAL NOTES:
{chr(10).join(state.tactical_notes)}

Propose 1–3 actions to make progress on the top goal. Each action MUST
be a valid verb from the rulebook. Include a brief reasoning string
explaining WHY this batch.

Output JSON with keys: reasoning (1-2 sentences), actions (list of
{{verb, ...args}}).
"""


def _goals_str(state: BrainState) -> str:
    return "\n".join(
        f"- ({g.status}) {g.goal} — {g.why}"
        for g in state.goal_stack
    ) or "(none)"
