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

from agents.baselines import Avenger, Killer, Manipulator, Scavenger, Survivor

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


def test_survivor_forages_tree_when_hungry():
    from agent_sim_sdk import Forage
    bot = Survivor(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.7, "inventory": [], "gold": 0},
        entities=[vent("oak-1", "tree", (11, 10))],
    )
    act = bot.decide(obs)
    assert isinstance(act, Forage) and act.target == "oak-1", (act, bot.state)


def test_survivor_buys_food_when_hungry_with_gold_no_food():
    from agent_sim_sdk import BuyFood
    bot = Survivor(creds=make_creds())
    obs = make_obs(extras={"hp": 100, "hunger": 0.7, "inventory": [], "gold": 20})
    act = bot.decide(obs)
    assert isinstance(act, BuyFood), (act, bot.state)


def test_survivor_cannot_buy_food_when_broke():
    bot = Survivor(creds=make_creds())
    obs = make_obs(extras={"hp": 100, "hunger": 0.7, "inventory": [], "gold": 1})
    act = bot.decide(obs)
    # Broke + no food in sight → falls through to a roaming step, not BuyFood.
    from agent_sim_sdk import BuyFood
    assert not isinstance(act, BuyFood), act


def test_survivor_flees_infamous_unarmed_agent():
    bot = Survivor(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.1, "inventory": []},
        entities=[vent("killer-9", "killer", (12, 10),
                       extras_summary={"rep_bucket": "infamous"})],  # unarmed but notorious
    )
    act = bot.decide(obs)
    assert bot.state == "FLEEING", bot.state
    assert act is None
    assert bot.goal.kind == "flee" and bot.goal.entity_id == "killer-9"


def test_avenger_moves_on_infamous_without_witnessing():
    from agent_sim_sdk import Attack
    bot = Avenger(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.0, "inventory": [],
                "equipped": {"weapon": "item:sword_short#1"}},
        entities=[vent("villain-1", "killer", (11, 10),
                       extras_summary={"rep_bucket": "infamous"})],
    )
    act = bot.decide(obs)
    assert bot.state == "AVENGING", bot.state
    assert bot.grudge_target == "villain-1"
    assert isinstance(act, Attack) and act.target == "villain-1", act


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
        extras={"hp": 12, "hunger": 0.0, "inventory": []},  # below retreat threshold (18)
        entities=[vent("victim-3", "survivor", (11, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "RETREATING", bot.state
    # Flees the only visible threat (the victim) via a flee goal.
    assert act is None
    assert bot.goal.kind == "flee" and bot.goal.entity_id == "victim-3"


# ----- Avenger -------------------------------------------------------

def _kill_witnessed(killer, victim, pos):
    from agent_sim_sdk import AudibleEvent
    import json
    return AudibleEvent(
        event_id="kw1", kind="sound", from_entity="",
        from_pos=list(pos), sound_kind="kill_witnessed",
        text=json.dumps({"killer": killer, "victim": victim}), tick=100,
    )


def test_avenger_idle_forages():
    bot = Avenger(creds=make_creds())
    obs = make_obs(extras={"hp": 100, "hunger": 0.0, "inventory": []})
    act = bot.decide(obs)
    assert bot.state == "IDLE", bot.state
    assert act is None or isinstance(act, Step)


def test_avenger_holds_grudge_and_pursues_killer_on_witness():
    bot = Avenger(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.0, "inventory": [],
                "equipped": {"weapon": "item:sword_short#1"}},  # already armed
        entities=[vent("hunter-1", "killer", (14, 10))],
        audible=[_kill_witnessed("hunter-1", "poor-soul", (16, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "AVENGING", bot.state
    assert bot.grudge_target == "hunter-1"
    # Killer visible but not adjacent → pursue via standing goal.
    assert act is None
    assert bot.goal.kind == "pursue" and bot.goal.entity_id == "hunter-1"


def test_avenger_attacks_killer_in_range():
    bot = Avenger(creds=make_creds())
    obs = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.0, "inventory": [],
                "equipped": {"weapon": "item:sword_short#1"}},
        entities=[vent("hunter-2", "killer", (11, 10))],
        audible=[_kill_witnessed("hunter-2", "poor-soul", (11, 10))],
    )
    act = bot.decide(obs)
    assert bot.state == "AVENGING", bot.state
    assert isinstance(act, Attack) and act.target == "hunter-2", act


def test_avenger_grudge_decays_back_to_idle():
    bot = Avenger(creds=make_creds())
    # Witness a kill, but the killer is never visible again.
    obs0 = make_obs(
        pos=(10, 10),
        extras={"hp": 100, "hunger": 0.0, "inventory": [],
                "equipped": {"weapon": "item:sword_short#1"}},
        audible=[_kill_witnessed("ghost", "victim", (40, 40))],
    )
    bot.decide(obs0)
    assert bot.state == "AVENGING", bot.state
    # No more sightings; pump past the grudge TTL.
    clean = make_obs(pos=(10, 10),
                     extras={"hp": 100, "hunger": 0.0, "inventory": [],
                             "equipped": {"weapon": "item:sword_short#1"}})
    for _ in range(200):
        bot.decide(clean)
        if bot.state == "IDLE":
            break
    assert bot.state == "IDLE", bot.state
    assert bot.grudge_target is None


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
