"""Unit tests for the rule-based archetype FSMs.

We test by handing each bot a synthetic Observation that matches the
SDK's pydantic schema, then asserting on the Action it returns and the
state it transitions to. No engine, no WebSocket — just the decide()
function under a microscope.

Each archetype gets:
- baseline behaviour (does nothing crazy in a clean tick),
- the canonical transition the design doc names,
- the safety transition (FLEEING / RETREATING).
"""
from __future__ import annotations

from agent_sim_sdk import (
    Attack, Eat, Give, Step, Observation, Pickup, ProposeTask, Speak,
    SelfState, VisibleEntity, VisibleItem, WorldClock, AgentCredentials,
)

from agents.baselines import Killer, Manipulator, Scavenger, Survivor

# Movement is now a standing GOAL the motor executes, not a one-shot action:
# a bot that wants to move sets self.goal (pursue/flee/goto) and decide()
# returns None (the harness motor steps toward the goal). So movement tests
# assert on bot.goal, not on a returned Move action.


# ----- builders ---------------------------------------------------------

def make_creds(agent_id="bot-1") -> AgentCredentials:
    return AgentCredentials(
        agent_id=agent_id,
        agent_secret="x",
        ws_url="ws://test",
    )


def make_obs(
    *,
    entity_id="bot-1",
    pos=(10, 10),
    extras=None,
    entities=None,
    items=None,
    audible=None,
) -> Observation:
    return Observation(
        obs_id=1,
        world_tick=100,
        self=SelfState(
            entity_id=entity_id,
            pos=list(pos),
            facing="S",
            extras=extras or {"hp": 100, "hunger": 0.0, "inventory": []},
        ),
        visible_entities=entities or [],
        visible_objects=[],
        visible_items=items or [],
        audible=audible or [],
        recent_self_results=[],
        world_clock=WorldClock(tick=100, day_phase="midday"),
    )


def vit(eid, sprite, pos):
    return VisibleItem(entity_id=eid, sprite=sprite, pos=list(pos))


def vent(eid, archetype, pos, extras_summary=None):
    return VisibleEntity(
        entity_id=eid,
        apparent_label=eid,
        pos=list(pos),
        facing="S",
        archetype=archetype,
        extras_summary=extras_summary or {},
    )


# ----- Survivor -------------------------------------------------------

def test_survivor_idle_does_nothing_or_walks():
    bot = Survivor(creds=make_creds())
    obs = make_obs(extras={"hp": 100, "hunger": 0.1, "inventory": []})
    # IDLE may emit a random walk (Step) or no action; either is fine.
    act = bot.decide(obs)
    assert bot.state == "IDLE"
    assert act is None or isinstance(act, Step)


def test_survivor_hungry_eats_from_inventory():
    bot = Survivor(creds=make_creds())
    obs = make_obs(extras={
        "hp": 100, "hunger": 0.7, "inventory": ["item:apple#7"],
    })
    act = bot.decide(obs)
    assert isinstance(act, Eat) and act.item == "item:apple#7", (act, bot.state)
    assert bot.state == "EATING", bot.state


def test_survivor_hungry_walks_toward_visible_food():
    bot = Survivor(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.7, "inventory": []},
        items=[vit("apple-1", "item:apple", (15, 10))],
    )
    act = bot.decide(obs)
    # Sets a goto goal toward the apple; motor walks there.
    assert act is None
    assert bot.goal.kind == "goto" and tuple(bot.goal.pos) == (15, 10)
    assert bot.state == "HUNGRY"


def test_survivor_picks_up_adjacent_food():
    bot = Survivor(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.7, "inventory": []},
        items=[vit("apple-2", "item:apple", (11, 10))],
    )
    act = bot.decide(obs)
    assert isinstance(act, Pickup) and act.target == "apple-2", act


def test_survivor_flees_armed_entity():
    bot = Survivor(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.1, "inventory": []},
        entities=[vent("killer-1", "killer", (12, 10),
                       extras_summary={"equipped_slot": "weapon",
                                       "equipped_sprite": "item:sword_short"})],
    )
    act = bot.decide(obs)
    assert bot.state == "FLEEING"
    # Sets a flee goal on the armed threat; motor steps away.
    assert act is None
    assert bot.goal.kind == "flee" and bot.goal.entity_id == "killer-1"


def test_survivor_desperate_when_starving_no_food():
    bot = Survivor(creds=make_creds())
    obs = make_obs(extras={
        "hp": 100, "hunger": 0.9, "inventory": [],
    })
    bot.decide(obs)
    assert bot.state == "DESPERATE"


def test_survivor_chases_visible_coins_when_idle():
    bot = Survivor(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.1, "inventory": []},
        items=[vit("coin-1", "item:coin_pouch", (13, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "IDLE", bot.state
    assert act is None
    assert bot.goal.kind == "goto" and tuple(bot.goal.pos) == (13, 10)


def test_survivor_picks_up_adjacent_gem():
    bot = Survivor(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.1, "inventory": []},
        items=[vit("gem-1", "item:gem_emerald", (10, 11))],
    )
    act = bot.decide(obs)
    assert isinstance(act, Pickup) and act.target == "gem-1", act


# ----- Scavenger -----------------------------------------------------

def test_scavenger_races_on_death_scream():
    from agent_sim_sdk import AudibleEvent
    bot = Scavenger(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        audible=[AudibleEvent(
            event_id="e1",
            kind="sound",
            from_entity="",
            from_pos=[30, 10],
            sound_kind="death_scream",
            tick=100,
        )],
    )
    act = bot.decide(obs)
    assert bot.state == "RACING", bot.state
    # Races to the scream tile via a goto goal.
    assert act is None
    assert bot.goal.kind == "goto" and tuple(bot.goal.pos) == (30, 10)


def test_scavenger_loots_when_at_corpse():
    bot = Scavenger(creds=make_creds())
    bot.state = "RACING"
    bot.target_pos = (10, 10)
    obs = make_obs(
        pos=(10, 10),
        items=[
            vit("coin-1", "item:coin_pouch", (10, 10)),
            vit("apple-1", "item:apple", (10, 10)),
        ],
    )
    act = bot.decide(obs)
    # State should land on LOOTING and pick the gold pile first.
    assert bot.state == "LOOTING", bot.state
    assert isinstance(act, Pickup) and act.target == "coin-1", act


def test_scavenger_retreats_when_armed_agent_at_corpse():
    bot = Scavenger(creds=make_creds())
    bot.state = "LOOTING"
    bot.target_pos = (10, 10)
    obs = make_obs(
        pos=(10, 10),
        items=[vit("coin-1", "item:coin_pouch", (10, 10))],
        entities=[vent(
            "k", "killer", (11, 10),
            extras_summary={"equipped_slot": "weapon"})],
    )
    act = bot.decide(obs)
    assert bot.state == "RETREATING", bot.state
    assert act is None
    assert bot.goal.kind == "flee" and bot.goal.entity_id == "k"


# ----- Killer --------------------------------------------------------

def test_killer_pursues_unarmed_target():
    bot = Killer(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.0, "inventory": []},
        entities=[vent("victim-1", "survivor", (13, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "PURSUING", bot.state
    assert bot.target_id == "victim-1"
    # Pursues via a standing goal; motor closes the gap (not adjacent yet).
    assert act is None
    assert bot.goal.kind == "pursue" and bot.goal.entity_id == "victim-1"


def test_killer_attacks_in_range():
    bot = Killer(creds=make_creds())
    bot.state = "PURSUING"
    bot.target_id = "victim-2"
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.0, "inventory": []},
        entities=[vent("victim-2", "survivor", (11, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "ATTACKING", bot.state
    # The next decide() emits Attack; this one just transitions, no act yet.
    # Run a second tick:
    act2 = bot.decide(obs)
    assert isinstance(act2, Attack) and act2.target == "victim-2", act2


def test_killer_retreats_low_hp():
    bot = Killer(creds=make_creds())
    bot.state = "ATTACKING"
    bot.target_id = "victim-3"
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 20, "hunger": 0.0, "inventory": []},
        entities=[vent("victim-3", "survivor", (11, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "RETREATING", bot.state
    # Flees the only visible threat (the victim) via a flee goal.
    assert act is None
    assert bot.goal.kind == "flee" and bot.goal.entity_id == "victim-3"


# ----- Manipulator ---------------------------------------------------

def test_manipulator_approaches_target():
    bot = Manipulator(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        entities=[vent("mark-1", "survivor", (14, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "APPROACHING", bot.state
    assert bot.target_id == "mark-1"
    # Approaches the mark via a pursue goal; motor closes to adjacent.
    assert act is None
    assert bot.goal.kind == "pursue" and bot.goal.entity_id == "mark-1"


def test_manipulator_gifts_then_speaks_when_adjacent():
    bot = Manipulator(creds=make_creds())
    bot.state = "APPROACHING"
    bot.target_id = "mark-2"
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.0, "inventory": ["item:apple#1"]},
        entities=[vent("mark-2", "survivor", (11, 10))],
    )
    act1 = bot.decide(obs)
    # First tick: transitions APPROACHING → BUILDING_TRUST → gifts.
    assert bot.state == "BUILDING_TRUST", bot.state
    assert isinstance(act1, Give) and act1.target == "mark-2" and act1.item == "item:apple#1"
    act2 = bot.decide(obs)
    assert isinstance(act2, Speak) and "friend" in act2.text.lower()


def test_manipulator_eventually_betrays():
    bot = Manipulator(creds=make_creds())
    bot.state = "WAITING"
    bot.target_id = "mark-3"
    bot.waiting_ticks = 0
    obs = make_obs(
        pos=(10, 10),
        entities=[vent("mark-3", "survivor", (11, 10))],
    )
    # Pump WAITING ticks past the deadline.
    for _ in range(40):
        bot.decide(obs)
        if bot.state.startswith("DEFECTING"):
            break
    assert bot.state.startswith("DEFECTING"), bot.state
