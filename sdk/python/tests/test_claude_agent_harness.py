"""Phase AGENT-A4 — verify the Claude harness skeleton works
end-to-end against the StubLLM. No real API calls."""

from __future__ import annotations

from agent_sim_sdk import (
    AudibleEvent, Facing, Observation, SelfState, VisibleEntity, WorldClock,
)

import sys
from pathlib import Path

# examples/claude_agent isn't on the import path by default; sneak it on.
_REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(_REPO))

from examples.claude_agent import (  # noqa: E402
    BrainState, Harness, Persona, StubLLM,
)


def _harness():
    state = BrainState(persona=Persona(
        name="Test",
        archetype="trainer",
        bio="A merchant trying to make a living.",
    ))
    return Harness(state=state, llm=StubLLM())


def _obs(self_id="me", self_pos=(10, 10), extras=None, visible=None):
    return Observation(
        obs_id=1,
        world_tick=100,
        self=SelfState(
            entity_id=self_id,
            pos=self_pos, facing=Facing.S,
            extras=extras or {"hp": 100, "gold": 25, "hunger": 0.1},
        ),
        visible_entities=visible or [],
        audible=[],
        world_clock=WorldClock(tick=100, day_phase="midday"),
    )


def test_init_persona_seeds_values_and_goals():
    h = _harness()
    assert h.state.persona.long_term_values == []
    assert h.state.goal_stack == []
    h.init_persona()
    assert len(h.state.persona.long_term_values) > 0
    assert len(h.state.goal_stack) > 0


def test_tactical_returns_valid_batch():
    h = _harness()
    h.init_persona()
    batch = h.tactical(_obs())
    assert batch.actions, "tactical must return at least one action"
    # The stub always returns Wait — that's fine; we just need a
    # valid ActionBatch with a reasoning string.
    assert batch.reasoning, "reasoning should be set on every batch"


def test_tactical_records_a_note():
    h = _harness()
    h.init_persona()
    h.tactical(_obs())
    assert len(h.state.tactical_notes) == 1, "tactical writes one note per cycle"


def test_observe_others_populates_register():
    h = _harness()
    h.init_persona()
    visible = [
        VisibleEntity(
            entity_id="cara", apparent_label="cara",
            pos=(11, 10), facing=Facing.W, archetype="trainer",
        ),
        VisibleEntity(
            entity_id="gren", apparent_label="gren",
            pos=(12, 10), facing=Facing.W, archetype="trainer",
        ),
    ]
    h.tactical(_obs(visible=visible))
    assert "cara" in h.state.agent_register
    assert "gren" in h.state.agent_register
    assert h.state.agent_register["cara"].last_seen_pos == (11, 10)


def test_reflective_runs_after_cadence():
    h = _harness()
    h.reflective_every = 3
    h.init_persona()
    for _ in range(2):
        h.maybe_reflect()
    # Still no reflection note.
    assert len(h.state.reflective_notes) == 0
    h.maybe_reflect()
    # Third call triggers; stub returns a non-empty note.
    assert len(h.state.reflective_notes) == 1


def test_reflex_fires_on_low_hp():
    h = _harness()
    h.init_persona()
    batch = h.reflex(_obs(extras={"hp": 3, "gold": 10, "hunger": 0.1}))
    assert batch is not None
    assert batch.actions[0].verb == "step"
    assert "flee" in (batch.reasoning or "")


def test_reflex_idle_on_normal_hp():
    h = _harness()
    h.init_persona()
    batch = h.reflex(_obs())  # hp=100, no fleeing
    assert batch is None


def test_agent_register_caps_at_register_cap():
    h = _harness()
    h.state.register_cap = 3
    h.init_persona()
    visible = [
        VisibleEntity(
            entity_id=f"e{i}", apparent_label="...",
            pos=(i, 10), facing=Facing.W, archetype="trainer",
        )
        for i in range(10)
    ]
    h.tactical(_obs(visible=visible))
    assert len(h.state.agent_register) == 3
