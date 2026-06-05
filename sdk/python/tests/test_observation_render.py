"""Phase AGENT-A2 — layered observation renderer."""

from __future__ import annotations

from agent_sim_sdk import (
    AudibleEvent, Facing, Observation, SelfState, VisibleEntity, VisibleObject,
    WorldClock,
    render_layered_observation, render_self, render_nearby, render_audible,
    render_minimap, rank_nearby, relative_compass,
)


def _obs(
    self_pos=(24, 17),
    extras=None,
    last=None,
    visible=None,
    objects=None,
    audible=None,
):
    return Observation(
        obs_id=1,
        world_tick=100,
        self=SelfState(
            entity_id="you",
            pos=self_pos,
            facing=Facing.S,
            extras=extras or {"hp": 100, "gold": 14, "hunger": 0.62},
            last_action_result=last,
        ),
        visible_entities=visible or [],
        visible_objects=objects or [],
        audible=audible or [],
        world_clock=WorldClock(tick=100, day_phase="midday"),
    )


# ---- Salience ranking ----

def test_rank_promotes_whisper_target():
    you = SelfState(entity_id="you", pos=(0, 0), facing=Facing.S)
    visible = [
        VisibleEntity(
            entity_id="gren", apparent_label="gren", pos=(20, 20),
            facing=Facing.W, archetype="trainer",
        ),
        VisibleEntity(
            entity_id="cara", apparent_label="cara", pos=(8, 8),
            facing=Facing.W, archetype="trainer",
        ),
    ]
    audible = [
        AudibleEvent(
            event_id="ev-1", kind="whisper",
            from_entity="cara", from_pos=(8, 8),
            text="meet me at noon", tick=99,
        ),
    ]
    ranked = rank_nearby(you, visible, audible)
    assert ranked[0].e.entity_id == "cara", "whisper sender should outrank closer-by-distance gren"
    assert "SPOKE_TO_YOU" in ranked[0].flags


def test_rank_promotes_adjacent():
    you = SelfState(entity_id="you", pos=(0, 0), facing=Facing.S)
    visible = [
        VisibleEntity(
            entity_id="far_friend", apparent_label="...", pos=(10, 0),
            facing=Facing.S, archetype="trainer",
        ),
        VisibleEntity(
            entity_id="adj_stranger", apparent_label="...", pos=(1, 0),
            facing=Facing.W, archetype="trainer",
        ),
    ]
    ranked = rank_nearby(you, visible, [])
    assert ranked[0].e.entity_id == "adj_stranger"
    assert "ADJACENT" in ranked[0].flags


def test_rank_limits_to_top_k():
    you = SelfState(entity_id="you", pos=(0, 0), facing=Facing.S)
    visible = [
        VisibleEntity(
            entity_id=f"e{i}", apparent_label=f"e{i}", pos=(i, 0),
            facing=Facing.S, archetype="trainer",
        )
        for i in range(30)
    ]
    ranked = rank_nearby(you, visible, [], top_k=5)
    assert len(ranked) == 5


# ---- Compass ----

def test_relative_compass_basic():
    # +x = E, -y = N (screen Y grows south)
    assert relative_compass((0, 0), (5, 0)) == "E"
    assert relative_compass((0, 0), (0, -5)) == "N"
    assert relative_compass((0, 0), (-5, 0)) == "W"
    assert relative_compass((0, 0), (0, 5)) == "S"
    assert relative_compass((0, 0), (5, -5)) == "NE"
    assert relative_compass((1, 1), (1, 1)) == "HERE"


# ---- Self block ----

def test_render_self_includes_stats_and_last():
    s = SelfState(
        entity_id="you",
        pos=(24, 17), facing=Facing.S,
        extras={"hp": 90, "gold": 14, "hunger": 0.625},
        last_action_result={"verb": "move", "accepted": False, "reason": "blocked_by_entity"},
    )
    txt = render_self(s, goal="reach blacksmith")
    assert "pos: (24, 17)" in txt
    assert "hp:90" in txt
    assert "gold:14" in txt
    assert "hunger:0.62" in txt
    assert "reach blacksmith" in txt
    assert "DENIED" in txt
    assert "blocked_by_entity" in txt


# ---- Nearby ----

def test_render_nearby_compass_mode():
    you = SelfState(entity_id="you", pos=(0, 0), facing=Facing.S)
    visible = [
        VisibleEntity(
            entity_id="gren", apparent_label="gren", pos=(5, -3),
            facing=Facing.W, archetype="trainer",
        ),
    ]
    ranked = rank_nearby(you, visible, [])
    txt = render_nearby(ranked, you.pos, coord_style="compass")
    assert "gren" in txt
    assert "NE" in txt or "E" in txt  # depending on the bucket; both acceptable


# ---- Audible ----

def test_render_audible_renders_whisper_with_text_for_target():
    aud = [
        AudibleEvent(
            event_id="x", kind="whisper",
            from_entity="cara", from_pos=(25, 17), text="meet me", tick=100,
        ),
    ]
    txt = render_audible(aud, "you", (24, 17))
    assert "whisper" in txt
    assert "meet me" in txt


def test_render_audible_shows_sound_kind():
    aud = [
        AudibleEvent(
            event_id="x", kind="sound",
            from_entity="env", from_pos=(28, 15),
            sound_kind="metal_clang", tick=100,
        ),
    ]
    txt = render_audible(aud, "you", (24, 17), coord_style="compass")
    assert "metal_clang" in txt


def test_render_audible_limits_to_n():
    aud = [
        AudibleEvent(
            event_id=str(i), kind="speech",
            from_entity="mari", from_pos=(0, 0), text=f"line {i}", tick=i,
        )
        for i in range(20)
    ]
    txt = render_audible(aud, "you", (0, 0), limit=3)
    assert "line 17" in txt
    assert "line 19" in txt
    assert "line 0" not in txt


# ---- Minimap ----

def test_render_minimap_marks_self_and_neighbors():
    you_pos = (10, 10)
    visible = [
        VisibleEntity(
            entity_id="cara", apparent_label="cara", pos=(11, 10),
            facing=Facing.W, archetype="trainer",
        ),
    ]
    objects = [
        VisibleObject(object_id="tree-1", kind="tree", pos=(10, 9)),
    ]
    txt = render_minimap(you_pos, visible, objects, radius=3)
    # "@" should be center; first letter of cara should be present.
    assert "@" in txt
    assert "C" in txt
    assert "T" in txt


# ---- Top-level ----

def test_render_layered_observation_has_all_sections():
    obs = _obs(
        visible=[
            VisibleEntity(
                entity_id="cara", apparent_label="cara", pos=(25, 17),
                facing=Facing.W, archetype="trainer",
            ),
        ],
        audible=[
            AudibleEvent(
                event_id="x", kind="speech",
                from_entity="cara", from_pos=(25, 17),
                text="watch it", tick=100,
            ),
        ],
        last={"verb": "move", "accepted": False, "reason_code": "blocked_by_entity"},
    )
    txt = render_layered_observation(obs, goal="reach blacksmith")
    for section in ("self:", "nearby", "audible", "map ("):
        assert section in txt, f"section {section!r} missing from layered block:\n{txt}"


def test_render_layered_observation_token_frugal_for_qwen():
    # Compass mode should NOT have raw (x,y) coords in the nearby /
    # audible blocks — Qwen saves tokens by reading direction only.
    obs = _obs(
        visible=[
            VisibleEntity(
                entity_id="cara", apparent_label="cara", pos=(25, 17),
                facing=Facing.W, archetype="trainer",
            ),
        ],
        audible=[
            AudibleEvent(
                event_id="s", kind="shout",
                from_entity="mari", from_pos=(50, 17),
                text="apples 2g", tick=100,
            ),
        ],
    )
    txt = render_layered_observation(obs, coord_style="compass")
    # Self block still has absolute pos (that's what the agent IS).
    assert "(24, 17)" in txt
    # But the nearby + audible blocks should use compass not raw coords
    # for OTHER entities.
    nearby_block = txt.split("nearby", 1)[1].split("audible", 1)[0]
    assert "(25, 17)" not in nearby_block
    assert "E" in nearby_block or "NE" in nearby_block or "SE" in nearby_block
