"""Unit tests for the focal agent's pure functions: prompt rendering
and action mapping. No engine, no LLM."""
from __future__ import annotations

from agent_sim_sdk import (
    Observation, SelfState, VisibleEntity, VisibleItem, AudibleEvent, LocalView,
    WorldClock, Step, Speak, Whisper, Eat, Pickup, Pay, Attack,
    ProposeTask,
)

from agents.common.motor import Goal
from agents.llm.actions import to_action, to_goal
from agents.llm.prompt import build_prompt, render_self, render_visible, render_local_view


def make_obs(*, pos=(10, 10), extras=None, entities=None, items=None,
             audible=None):
    return Observation(
        obs_id=1, world_tick=100,
        self=SelfState(entity_id="me", pos=list(pos), facing="S",
                       extras=extras or {"hp": 100, "hunger": 0.3,
                                         "gold": 25, "inventory": []}),
        visible_entities=entities or [],
        visible_objects=[],
        visible_items=items or [],
        audible=audible or [],
        recent_self_results=[],
        world_clock=WorldClock(tick=100, day_phase="midday"),
    )


def vit(eid, sprite, pos):
    return VisibleItem(entity_id=eid, sprite=sprite, pos=list(pos))


def vent(eid, arch, pos, summ=None):
    return VisibleEntity(entity_id=eid, apparent_label=eid, pos=list(pos),
                         facing="S", archetype=arch, extras_summary=summ or {})


# ---- action mapping ----

def test_map_all_direct_verbs():
    cases = [
        ({"verb": "step", "dir": "N"}, Step),
        ({"verb": "speak", "text": "hi"}, Speak),
        ({"verb": "whisper", "target": "spawn_2", "text": "psst"}, Whisper),
        ({"verb": "eat", "item": "item:apple#1"}, Eat),
        ({"verb": "pickup", "target": "item_5"}, Pickup),
        ({"verb": "pay", "target": "spawn_2", "amount": 10}, Pay),
        ({"verb": "attack", "target": "spawn_3"}, Attack),
        ({"verb": "propose_task", "target": "spawn_2", "terms": "bring food",
          "reward": "10g"}, ProposeTask),
    ]
    for d, cls in cases:
        a = to_action(d)
        assert isinstance(a, cls), f"{d} -> {a}"


def test_movement_verbs_map_to_goals_not_actions():
    # pursue/flee/goto are GOALS handled by the motor, not one-shot actions.
    assert to_action({"verb": "pursue", "target": "spawn_3"}) is None
    g = to_goal({"verb": "pursue", "target": "spawn_3"})
    assert g is not None and g.kind == "pursue" and g.entity_id == "spawn_3"
    g = to_goal({"verb": "flee", "target": "spawn_9"})
    assert g.kind == "flee" and g.entity_id == "spawn_9"
    g = to_goal({"verb": "goto", "target": ["3", "4"]})
    assert g.kind == "goto" and tuple(g.pos) == (3, 4)


def test_to_goal_none_for_non_movement():
    assert to_goal({"verb": "speak", "text": "hi"}) is None
    assert to_goal({"verb": "goto"}) is None  # missing target


def test_map_unknown_verb_returns_none():
    assert to_action({"verb": "teleport", "target": [1, 2]}) is None


def test_map_malformed_returns_none():
    assert to_action({"verb": "pay", "target": "x"}) is None  # missing amount
    assert to_action({"verb": "step"}) is None                # missing dir
    assert to_action("not a dict") is None


def test_render_local_view_shows_self_and_terrain():
    lv = LocalView(radius=2, origin=(8, 8),
                   rows=[".....", ".###.", ".#@#.", ".###.", "....~"],
                   legend={"@": "you", "#": "blocked"})
    obs = make_obs(pos=(10, 10))
    obs.local_view = lv
    out = render_local_view(obs, max_radius=2)
    assert "@" in out and "#" in out
    assert "Local map" in out


# ---- prompt rendering ----

def test_render_self_includes_vitals_and_inventory():
    obs = make_obs(extras={"hp": 80, "hunger": 0.55, "gold": 40,
                           "inventory": ["item:apple#1", "item:apple#2",
                                         "item:sword_short#9"],
                           "equipped": {"weapon": "item:dagger#3"}})
    s = render_self(obs)
    assert "hp=80" in s
    assert "hunger=0.55" in s
    assert "gold=40" in s
    assert "apple x2" in s.replace("applex2", "apple x2") or "applex2" in s
    assert "dagger" in s


def test_render_visible_marks_adjacency():
    obs = make_obs(pos=(10, 10),
                   entities=[vent("spawn_2", "survivor", (11, 10))],
                   items=[vit("item_1", "item:coin_pouch", (10, 10)),
                          vit("item_2", "item:apple", (20, 20))])
    v = render_visible(obs, (10, 10))
    assert "spawn_2" in v and "ADJACENT" in v
    assert "coin_pouch" in v
    # Far item shows tile distance, not ADJACENT on its line.
    assert "item_2" in v


def test_render_visible_armed_tag():
    obs = make_obs(entities=[
        vent("k1", "killer", (11, 10),
             summ={"hp_bucket": "wounded", "equipped_slot": "weapon"})])
    v = render_visible(obs, (10, 10))
    assert "wounded" in v and "armed" in v


def test_build_prompt_has_menu_and_goal():
    obs = make_obs()
    p = build_prompt(obs, "You are a merchant.", "Get rich.")
    assert "You are a merchant." in p
    assert "Get rich." in p
    assert "propose_task" in p   # menu present
    assert "entity_id" in p      # the targeting rule
    assert '"reasoning"' in p    # output format


def test_build_prompt_surfaces_death_scream():
    obs = make_obs(audible=[AudibleEvent(
        event_id="e1", kind="sound", from_entity="", from_pos=[20, 20],
        sound_kind="death_scream", tick=99)])
    p = build_prompt(obs, "persona", "goal")
    assert "death scream" in p.lower()
