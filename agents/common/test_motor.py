"""Unit tests for the motor/reflex layer (slices 4+5)."""
from __future__ import annotations

from agent_sim_sdk import Observation, SelfState, VisibleEntity, WorldClock, Facing
from agents.common.nav import NavGrid
from agents.common.motor import Goal, MotorController


def _grid(rows):
    return NavGrid(len(rows[0]), len(rows), rows)


def _obs(pos, tick=100, visible=None):
    return Observation(
        obs_id=1, world_tick=tick,
        self=SelfState(entity_id="me", pos=pos, facing=Facing.S),
        visible_entities=visible or [],
        world_clock=WorldClock(tick=tick, day_phase="midday"),
    )


def _ent(eid, pos):
    return VisibleEntity(entity_id=eid, apparent_label=eid, pos=pos,
                         facing="S", archetype="trainer")


# Open 10x10 grid.
OPEN = ["." * 10 for _ in range(10)]


def test_goto_steps_toward_target():
    m = MotorController(nav=_grid(OPEN))
    act = m.next_step(Goal.goto(5, 0), _obs((0, 0)))
    assert act is not None and act.dir == "E"  # purely east first on a Manhattan path


def test_goto_routes_around_wall():
    # Row 0 is walled off except the agent's start cell (0,0), so the agent
    # cannot step E onto the wall — the only way to (4,1) is to go S first.
    rows = [
        ".#########",
        "..........",
        "..........",
        "..........",
        "..........",
        "..........",
        "..........",
        "..........",
        "..........",
        "..........",
    ]
    m = MotorController(nav=_grid(rows))
    act = m.next_step(Goal.goto(4, 1), _obs((0, 0)))
    assert act is not None
    assert act.dir == "S"  # E is a wall, so it detours south


def test_pursue_uses_last_seen_when_target_leaves_view():
    m = MotorController(nav=_grid(OPEN))
    # Tick 100: target visible at (5,0). Memory records it.
    o1 = _obs((0, 0), tick=100, visible=[_ent("prey", (5, 0))])
    m.observe(o1)
    a1 = m.next_step(Goal.pursue("prey"), o1)
    assert a1 is not None and a1.dir == "E"
    # Tick 110: target no longer visible. We should still head to last-seen.
    o2 = _obs((1, 0), tick=110, visible=[])
    m.observe(o2)
    a2 = m.next_step(Goal.pursue("prey"), o2)
    assert a2 is not None and a2.dir == "E"


def test_pursue_returns_none_when_memory_stale():
    m = MotorController(nav=_grid(OPEN), memory_ticks=5)
    o1 = _obs((0, 0), tick=100, visible=[_ent("prey", (5, 0))])
    m.observe(o1)
    # Far in the future, the memory has expired → target considered lost.
    o2 = _obs((0, 0), tick=200, visible=[])
    m.observe(o2)
    assert m.next_step(Goal.pursue("prey"), o2) is None


def test_pursue_stops_adjacent():
    m = MotorController(nav=_grid(OPEN))
    # Already adjacent to prey → no step (we hold reach, mind attacks).
    o = _obs((4, 0), tick=100, visible=[_ent("prey", (5, 0))])
    m.observe(o)
    assert m.next_step(Goal.pursue("prey"), o) is None


def test_flee_increases_distance():
    m = MotorController(nav=_grid(OPEN))
    # Threat to the east at (5,5); we're at (5,4). Fleeing should go N (away).
    o = _obs((5, 4), tick=100, visible=[_ent("hunter", (5, 5))])
    m.observe(o)
    act = m.next_step(Goal.flee("hunter"), o)
    assert act is not None and act.dir == "N"


def test_idle_is_noop():
    m = MotorController(nav=_grid(OPEN))
    assert m.next_step(Goal.idle(), _obs((0, 0))) is None
