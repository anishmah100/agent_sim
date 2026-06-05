"""Harness — the 4-layer brain orchestrator.

Layers (slowest to fastest):
  1. persona   — one-time, sets long_term_values + initial goals
  2. reflective — every ~60-120 sim-seconds, consolidates notes,
                  updates goals + theory_of_mind
  3. tactical   — every ~1-3 sim-seconds, emits an ActionBatch
  4. reflex     — pure Python, no LLM, runs at observation cadence
                  (interrupt detection, local sidestep, etc.)

The LLM client is injected so the same harness drives both a real
Anthropic Claude client and the StubLLM used by tests + no-API mode.
"""

from __future__ import annotations

from dataclasses import dataclass
from typing import Optional, Protocol

from agent_sim_sdk import (
    ActionBatch, Move, Speak, Wait,
    Observation, render_layered_observation,
)

from .prompts import persona_prompt, reflective_prompt, tactical_prompt
from .state import AgentRegisterEntry, BrainState, GoalStackEntry, Persona


class LLMClient(Protocol):
    def persona(self, prompt: str, max_tokens: int = 500) -> dict: ...
    def reflect(self, prompt: str, max_tokens: int = 500) -> dict: ...
    def tactical(self, prompt: str, max_tokens: int = 200) -> dict: ...


@dataclass
class Harness:
    state: BrainState
    llm: LLMClient
    # Coord style passed to the observation renderer. Claude → absolute;
    # Qwen subclass would override to "compass".
    coord_style: str = "absolute"
    # Reflection cadence: every N tactical cycles. Default 60 (≈1 minute
    # at 1Hz tactical).
    reflective_every: int = 60
    # Internal counter.
    _ticks_since_reflection: int = 0

    # ---- Layer 1: persona ----

    def init_persona(self) -> None:
        """Run once at agent registration. Populates long_term_values
        and seeds the goal stack."""
        resp = self.llm.persona(persona_prompt(self.state))
        self.state.persona.long_term_values = list(resp.get("long_term_values", []))
        for entry in resp.get("initial_goals", []):
            self.state.goal_stack.append(GoalStackEntry(
                goal=entry["goal"], why=entry.get("why", ""),
            ))

    # ---- Layer 2: reflective ----

    def maybe_reflect(self) -> Optional[str]:
        """Reflective layer — runs every Nth tactical cycle so we
        amortize the bigger LLM call. Returns the new reflective note
        if one was produced this cycle (caller may ship it to the
        engine for historian capture); None otherwise."""
        self._ticks_since_reflection += 1
        if self._ticks_since_reflection < self.reflective_every:
            return None
        self._ticks_since_reflection = 0
        recent = list(self.state.tactical_notes)
        resp = self.llm.reflect(reflective_prompt(self.state, recent))
        new_note: Optional[str] = None
        if note := resp.get("reflective_note"):
            self.state.push_reflective_note(note)
            new_note = note
        for upd in resp.get("goal_updates", []):
            action = upd.get("action")
            if action == "push":
                self.state.goal_stack.append(GoalStackEntry(
                    goal=upd["goal"], why=upd.get("why", ""),
                ))
            elif action == "complete":
                for g in self.state.goal_stack:
                    if g.goal == upd.get("goal"):
                        g.status = "done"
        for entity_id, new_tom in resp.get("theory_of_mind_updates", {}).items():
            entry = self.state.agent_register.get(entity_id)
            if entry:
                entry.theory_of_me = new_tom
        return new_note

    # ---- Layer 3: tactical ----

    def tactical(self, obs: Observation) -> ActionBatch:
        """The ~1Hz call. Returns an ActionBatch (1-3 actions)."""
        self._observe_others(obs)
        block = render_layered_observation(
            obs,
            goal=(self.state.top_goal().goal if self.state.top_goal() else None),
            coord_style=self.coord_style,
        )
        resp = self.llm.tactical(tactical_prompt(self.state, block))
        # Bind into typed Action objects. Unrecognized verbs fall
        # through to a no-op Wait so the batch is always valid.
        actions = []
        for raw in resp.get("actions", []):
            actions.append(_action_from_dict(raw))
        if not actions:
            actions = [Wait(ticks=60)]
        batch = ActionBatch(actions=actions, reasoning=resp.get("reasoning"))
        # Tactical writes a short note so the next reflection has
        # context to compress.
        if batch.actions:
            verb = batch.actions[0].verb
            self.state.push_tactical_note(f"{verb}")
        return batch

    # ---- Layer 4: reflex (Python only) ----

    def reflex(self, obs: Observation) -> Optional[ActionBatch]:
        """Hooks for sub-second decisions that don't need the LLM.
        Returns a batch to fast-path (override tactical's next cycle),
        or None to defer to tactical.

        Example: if HP < 5%, override with a flee action.
        """
        hp = obs.self.extras.get("hp")
        if isinstance(hp, (int, float)) and hp <= 5:
            # Try to step away from any visible aggressor — for now,
            # just emit a north step as a stand-in. Real heuristics
            # consider the threat vector.
            x, y = obs.self.pos
            return ActionBatch(
                actions=[Move(target=(x, y - 1))],
                reasoning="reflex: critical hp, fleeing north",
            )
        return None

    # ---- Memory updates ----

    def _observe_others(self, obs: Observation) -> None:
        # First-order ToM seed: every visible other goes into the
        # register. Disposition + beliefs + theory_of_me grow as
        # reflection runs.
        for v in obs.visible_entities:
            if v.entity_id == obs.self.entity_id:
                continue
            entry = self.state.agent_register.get(v.entity_id)
            if entry is None:
                entry = AgentRegisterEntry(
                    entity_id=v.entity_id, last_seen_pos=tuple(v.pos),
                )
            else:
                entry.last_seen_pos = tuple(v.pos)
            self.state.remember_other(entry)


def _action_from_dict(d: dict):
    """Best-effort: turn a {verb, ...} dict into a typed SDK Action.
    Falls back to Wait on unknown shapes. Real harness would route
    every supported verb here."""
    verb = d.get("verb", "wait")
    if verb == "move":
        return Move(target=tuple(d.get("target", (0, 0))))
    if verb == "speak":
        return Speak(text=d.get("text", ""))
    return Wait(ticks=d.get("ticks", 60))
