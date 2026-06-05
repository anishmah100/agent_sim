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

Available verbs (use the one that BEST fits the situation, not the
default "move" or "wait"):
  move:    {{verb:"move",    target:[x,y]}}          step toward an adjacent walkable tile (≤1 tile from you)
  speak:   {{verb:"speak",   text:"..."}}            broadcast within 3 tiles, hear anyone, no target
  shout:   {{verb:"shout",   text:"..."}}            broadcast within 15 tiles — use for help / alerts
  whisper: {{verb:"whisper", target:"<entity_id>", text:"..."}}  ≤1 tile target, only they hear
  enter:   {{verb:"enter",   target:"<building_id>"}}  use when you see a door object adjacent — door IDs look like "door:<sprite>:..."
  exit:    {{verb:"exit"}}                            use immediately after entering if you want to leave
  pickup:  {{verb:"pickup",  target:"<item_id>"}}    grab any visible item by its ID
  give:    {{verb:"give",    target:"<entity_id>", item:"<item_id>"}}
  pay:     {{verb:"pay",     target:"<entity_id>", amount:<int>}}  use after a "give" or to seal a deal
  look_at: {{verb:"look_at", target:"<entity_id_or_direction>"}}  free, social signal
  wait:    {{verb:"wait", ticks:60}}                  ONLY when you genuinely have no better move

PRIORITY: if you can speak to / whisper to / enter / pickup / pay
something, DO IT instead of wait. Repeating wait or move without a
specific reason is wasted progress. Strong preference:
  - see a door nearby? enter it.
  - see another entity within 1 tile? whisper them; otherwise speak.
  - hear someone speak nearby? speak back to keep the dialogue going.
  - see an item? pickup.

Move targets must be at most 1 tile away from your current pos (move
is one step). If you want to go further, plan a chain of moves in
this batch.

Propose 1–3 actions. Keep reasoning short (1-2 sentences).

Output JSON with keys: reasoning (1-2 sentences), actions (list of
{{verb, ...args}}).
"""


def _goals_str(state: BrainState) -> str:
    return "\n".join(
        f"- ({g.status}) {g.goal} — {g.why}"
        for g in state.goal_stack
    ) or "(none)"
