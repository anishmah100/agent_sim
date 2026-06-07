"""Tests for the social-emergence run scorer."""
from __future__ import annotations

from tools.metrics.score_run import score_events, gini


def ev(kind, tick=1, **payload):
    return {"kind": kind, "tick": tick, "payload": payload}


def test_gini_equality_and_inequality():
    assert gini([10, 10, 10, 10]) == 0.0
    assert gini([]) == 0.0
    assert gini([0, 0, 0]) == 0.0
    # One person has everything → near 1.
    g = gini([0, 0, 0, 100])
    assert 0.7 < g < 1.0


def test_contract_state_machine():
    events = [
        ev("TaskProposed", ID="c1", Proposer="a", Target="b", Terms="x"),
        ev("TaskAccepted", ID="c1", Proposer="a", Target="b"),
        ev("TaskCompleted", ID="c1", Proposer="a", Target="b"),
        ev("TaskProposed", ID="c2", Proposer="a", Target="c", Terms="y"),
        ev("TaskAccepted", ID="c2", Proposer="a", Target="c"),
        # c2 never completed → broken.
        ev("TaskProposed", ID="c3", Proposer="d", Target="e", Terms="z"),
        ev("TaskRejected", ID="c3", Proposer="d", Target="e"),
    ]
    s = score_events(events)
    assert s.contracts_proposed == 3
    assert s.contracts_accepted == 2
    assert s.contracts_completed == 1
    assert s.contracts_rejected == 1
    assert s.contracts_broken == 1   # c2


def test_kills_and_damage():
    events = [
        ev("DamageDealt", Target="v", Killer="k", Amount=10, NewHP=90),
        ev("DamageDealt", Target="v", Killer="k", Amount=90, NewHP=0),
        ev("EntityDied", EntityID="v", Killer="k", Cause="attack"),
    ]
    s = score_events(events)
    assert s.kills == 1
    assert s.damage_events == 2
    assert s.kill_pairs[0]["killer"] == "k"
    assert s.kill_pairs[0]["victim"] == "v"


def test_manipulator_defection_dedups_raw_attacks():
    events = [
        ev("TaskProposed", ID="c1", Proposer="manip", Target="mark", Terms="deal"),
        ev("TaskAccepted", ID="c1", Proposer="manip", Target="mark"),
        # Manipulator attacks its contract partner MANY times — that's
        # ONE betrayal, not 3.
        ev("DamageDealt", Target="mark", Killer="manip", Amount=12, NewHP=88),
        ev("DamageDealt", Target="mark", Killer="manip", Amount=12, NewHP=76),
        ev("DamageDealt", Target="mark", Killer="manip", Amount=12, NewHP=64),
    ]
    s = score_events(events, manipulators={"manip"})
    assert s.manipulator_contracts == 1
    assert s.manipulator_defections == 1  # deduped, not 3


def test_no_false_defection_for_unrelated_attack():
    events = [
        ev("TaskProposed", ID="c1", Proposer="manip", Target="mark", Terms="deal"),
        ev("DamageDealt", Target="stranger", Killer="manip", Amount=5, NewHP=95),
    ]
    s = score_events(events, manipulators={"manip"})
    assert s.manipulator_defections == 0  # attacked a non-partner


def test_gold_gini_from_snapshot():
    s = score_events([], gold_by_agent={"a": 100, "b": 0, "c": 0, "d": 0})
    assert s.gold_total_end == 100
    assert s.gold_gini_end is not None and s.gold_gini_end > 0.7


def test_communication_counts():
    events = [
        ev("Speech", Speaker="a", Text="hi", Mode="speak"),
        ev("Speech", Speaker="a", Text="psst", Mode="whisper"),
        ev("Speech", Speaker="b", Text="HEY", Mode="shout"),
        ev("MentalNote", entity_id="a", text="thinking"),
    ]
    s = score_events(events)
    assert s.speech_count == 1
    assert s.whisper_count == 1
    assert s.shout_count == 1
    assert s.mental_notes == 1
    assert s.top_speakers[0][0] == "a"  # a spoke twice
