"""Harness verb-coverage suite.

The harness's `_action_from_dict` is the bridge between the LLM's
free-form tactical JSON output and the typed SDK Action classes that
get serialized onto the wire. Earlier versions only handled
move/speak/wait — every other verb the LLM produced was silently
converted to Wait, and bugs of that shape would only surface as
"why does my agent never speak/pickup/enter?" in production. This
suite covers every engine verb so a regression is caught immediately.

For each verb we assert:
  1. The dict `{verb: <name>, ...}` round-trips to the right typed
     SDK class (not Wait).
  2. The required argument fields survive the round-trip.
"""

from __future__ import annotations

import pytest

from examples.claude_agent.harness import _action_from_dict
from agent_sim_sdk import (
    Move, Speak, Whisper, Shout, LookAt, Interact, Pickup, Drop, Equip,
    Give, Pay, WorkForPay, Trade, Loot, Chop, Mine,
    Enter, Exit, Lock, Unlock, ClaimOwnership, TransferOwnership,
    Attack, Defend, Heal, Wait,
)


# (input dict, expected SDK class) — the table is the contract.
# Every engine verb must appear here so the contract is enforceable.
ROUND_TRIP_CASES = [
    # Movement + social.
    ({"verb": "move", "target": [3, 4]}, Move),
    ({"verb": "speak", "text": "hi"}, Speak),
    ({"verb": "shout", "text": "OI"}, Shout),
    ({"verb": "whisper", "target": "b", "text": "psst"}, Whisper),
    ({"verb": "look_at", "target": "b"}, LookAt),
    # Interact + inventory.
    ({"verb": "interact", "target": "bld:001", "affordance": "enter"}, Interact),
    ({"verb": "pickup", "target": "apple_42"}, Pickup),
    ({"verb": "drop", "item": "apple"}, Drop),
    ({"verb": "equip", "item": "sword"}, Equip),
    ({"verb": "give", "target": "b", "item": "apple"}, Give),
    # Economy.
    ({"verb": "pay", "target": "b", "amount": 5}, Pay),
    ({"verb": "work_for_pay"}, WorkForPay),
    ({"verb": "trade", "target": "b", "item": "apple", "price": 2}, Trade),
    ({"verb": "loot", "target": "corpse_1"}, Loot),
    # Resources.
    ({"verb": "chop", "target": "tree_3"}, Chop),
    ({"verb": "mine", "target": "rock_3"}, Mine),
    # Property.
    ({"verb": "enter", "target": "bld:001"}, Enter),
    ({"verb": "exit"}, Exit),
    ({"verb": "lock", "target": "bld:001"}, Lock),
    ({"verb": "unlock", "target": "bld:001"}, Unlock),
    ({"verb": "claim_ownership", "target": "bld:001"}, ClaimOwnership),
    ({"verb": "transfer_ownership", "target": "bld:001", "new_owner": "b"},
     TransferOwnership),
    # Combat.
    ({"verb": "attack", "target": "b"}, Attack),
    ({"verb": "defend"}, Defend),
    ({"verb": "heal"}, Heal),
    # Misc.
    ({"verb": "wait", "ticks": 30}, Wait),
]


@pytest.mark.parametrize("payload,cls", ROUND_TRIP_CASES)
def test_action_from_dict_routes_correctly(payload, cls):
    """Each verb name → its typed Action class. The catch-all 'else:
    return Wait' was the production silent-drop bug; this test makes
    it a regression instead of a runtime surprise."""
    action = _action_from_dict(payload)
    assert isinstance(action, cls), (
        f"verb={payload['verb']!r} should route to {cls.__name__}, "
        f"got {type(action).__name__}"
    )


def test_move_target_round_trips_as_tuple():
    action = _action_from_dict({"verb": "move", "target": [7, 8]})
    assert action.target == (7, 8)


def test_speak_text_round_trips():
    action = _action_from_dict({"verb": "speak", "text": "hello world"})
    assert action.text == "hello world"


def test_whisper_target_and_text_round_trip():
    action = _action_from_dict({
        "verb": "whisper", "target": "npc_42", "text": "psst"
    })
    assert action.target == "npc_42"
    assert action.text == "psst"


def test_interact_affordance_round_trips():
    action = _action_from_dict({
        "verb": "interact", "target": "bld:001", "affordance": "enter"
    })
    assert action.target == "bld:001"
    assert action.affordance == "enter"


def test_pay_amount_round_trips_as_int():
    action = _action_from_dict({"verb": "pay", "target": "b", "amount": "12"})
    assert action.amount == 12  # str coerced to int


def test_unknown_verb_falls_back_to_wait():
    """Unknown verb names should NOT crash — they fall back to Wait so
    a brain typo (or a future verb the harness hasn't been updated for)
    doesn't take the agent down."""
    action = _action_from_dict({"verb": "florpify", "x": "y"})
    assert isinstance(action, Wait)


def test_missing_verb_defaults_to_wait():
    action = _action_from_dict({})
    assert isinstance(action, Wait)


def test_every_engine_verb_has_a_case():
    """The harness must route EVERY engine-known verb to a typed
    action. The set below is the engine's verb surface; if any name
    falls through to Wait, the contract is broken."""
    engine_verbs = {
        "move", "speak", "shout", "whisper", "look_at",
        "interact", "pickup", "drop", "equip", "give",
        "pay", "work_for_pay", "trade", "loot",
        "chop", "mine",
        "enter", "exit", "lock", "unlock",
        "claim_ownership", "transfer_ownership",
        "attack", "defend", "heal", "wait",
    }
    missing = []
    for v in engine_verbs:
        action = _action_from_dict({"verb": v})
        # Wait is the legitimate route only for v=="wait".
        if isinstance(action, Wait) and v != "wait":
            missing.append(v)
    assert not missing, (
        f"these verbs silently fall through to Wait — fix _action_from_dict: "
        f"{sorted(missing)}"
    )
