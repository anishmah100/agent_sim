"""Phase AGENT-A3 — verify the heuristic bot's pick_action logic
without needing a live engine."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

import pytest

from agent_sim_sdk import (
    AudibleEvent, Facing, Observation, SelfState, VisibleEntity,
    VisibleObject, WorldClock,
)


def _load_bot():
    repo_root = Path(__file__).resolve().parents[3]
    src = repo_root / "examples" / "heuristic_bot.py"
    spec = importlib.util.spec_from_file_location("heuristic_bot", src)
    mod = importlib.util.module_from_spec(spec)
    # Ensure the module has its own STATE dict per test invocation.
    sys.modules["heuristic_bot"] = mod
    spec.loader.exec_module(mod)
    # Reset state so tests are independent.
    mod.STATE["greeted"].clear()
    mod.STATE["rendered_obs_once"] = False
    return mod


def _obs(self_pos=(10, 10), extras=None, visible=None, objects=None):
    return Observation(
        obs_id=1,
        world_tick=100,
        self=SelfState(
            entity_id="me",
            pos=self_pos, facing=Facing.S,
            extras=extras or {"hp": 100, "gold": 10, "hunger": 0.1},
        ),
        visible_entities=visible or [],
        visible_objects=objects or [],
        audible=[],
        world_clock=WorldClock(tick=100, day_phase="midday"),
    )


def test_greets_a_new_neighbor_first():
    bot = _load_bot()
    obs = _obs(
        visible=[
            VisibleEntity(
                entity_id="cara", apparent_label="cara",
                pos=(11, 10), facing=Facing.W, archetype="trainer",
            ),
        ],
    )
    batch, label = bot.pick_action(obs)
    assert label.startswith("GREET")
    assert batch.actions[0].verb == "speak"
    assert "cara" in batch.actions[0].text


def test_does_not_greet_same_neighbor_twice():
    bot = _load_bot()
    visible = [
        VisibleEntity(
            entity_id="cara", apparent_label="cara",
            pos=(11, 10), facing=Facing.W, archetype="trainer",
        ),
    ]
    obs = _obs(visible=visible)
    bot.pick_action(obs)  # first turn — greets
    # Second turn — same Cara, no food, low hunger → should wander.
    batch, label = bot.pick_action(obs)
    assert label == "WANDER"
    assert batch.actions[0].verb == "step"


def test_walks_toward_food_when_hungry():
    bot = _load_bot()
    obs = _obs(
        self_pos=(10, 10),
        extras={"hunger": 0.8},
        objects=[
            VisibleObject(object_id="apple-1", kind="apple", pos=(13, 10)),
        ],
    )
    batch, label = bot.pick_action(obs)
    assert label.startswith("WALK_TO_FOOD")
    # Step should be toward the food (east).
    assert batch.actions[0].dir == "E"


def test_wanders_when_idle():
    bot = _load_bot()
    obs = _obs()  # no visible, no hunger, nothing to do
    batch, label = bot.pick_action(obs)
    assert label == "WANDER"
    assert batch.actions[0].verb == "step"


def test_reasoning_trace_attached_to_every_batch():
    bot = _load_bot()
    obs = _obs(visible=[
        VisibleEntity(
            entity_id="cara", apparent_label="cara",
            pos=(11, 10), facing=Facing.W, archetype="trainer",
        ),
    ])
    batch, _ = bot.pick_action(obs)
    assert batch.reasoning, "every ActionBatch should carry a reasoning string"
