"""Slice 2b — the egocentric ASCII local view round-trips through the
Observation model (engine ships it; the SDK must parse it)."""

from __future__ import annotations

from agent_sim_sdk import LocalView, Observation, SelfState, WorldClock, Facing


def test_local_view_parses_from_payload():
    # The exact JSON shape the engine wire layer emits.
    payload = {
        "obs_id": 1,
        "world_tick": 100,
        "self": {"entity_id": "you", "pos": [5, 5], "facing": "S"},
        "world_clock": {"tick": 100, "day_phase": "midday", "weather": "clear"},
        "local_view": {
            "radius": 2,
            "origin": [3, 3],
            "rows": [".....", ".###.", ".#@#.", ".###.", "....~"],
            "legend": {"@": "you", "#": "blocked", "~": "water"},
        },
    }
    obs = Observation.model_validate(payload)
    lv = obs.local_view
    assert isinstance(lv, LocalView)
    assert lv.radius == 2
    assert tuple(lv.origin) == (3, 3)
    assert len(lv.rows) == 5
    # Self glyph sits at (pos - origin) = (2, 2).
    assert lv.rows[2][2] == "@"
    assert lv.legend["~"] == "water"


def test_local_view_optional_when_absent():
    obs = Observation(
        obs_id=1, world_tick=1,
        self=SelfState(entity_id="you", pos=(0, 0), facing=Facing.S),
        world_clock=WorldClock(tick=1, day_phase="dawn"),
    )
    assert obs.local_view is None
