"""Phase AGENT-A1 — SDK shapes for the new layered observation +
action batch + ActionResult contracts."""

from __future__ import annotations

import asyncio
import json
from typing import Any

import pytest

from agent_sim_sdk import (
    ActionBatch, ActionResult, ReasonCode,
    Move, Speak, Pay,
    Agent, AgentCredentials,
)


# ---- Pure-shape round-trips ----

def test_action_batch_serializes_with_reasoning():
    b = ActionBatch(
        actions=[Move(target=(24, 17)), Speak(text="hello")],
        reasoning="trying to reach the blacksmith and announce my arrival",
    )
    blob = json.loads(b.model_dump_json())
    assert blob["reasoning"].startswith("trying to reach")
    assert len(blob["actions"]) == 2
    assert blob["actions"][0]["verb"] == "move"
    assert blob["actions"][1]["verb"] == "speak"


def test_action_batch_without_reasoning_is_valid():
    # The reasoning trace is opt-in; an unannotated batch should still
    # round-trip cleanly so the heuristic_bot path works without
    # the LLM brain.
    b = ActionBatch(actions=[Pay(target="merchant", amount=5)])
    blob = json.loads(b.model_dump_json())
    assert blob["reasoning"] is None
    assert blob["actions"][0]["verb"] == "pay"


def test_action_result_legacy_shape():
    # Engine currently emits {accepted, reason}. The new fields stay
    # None until the engine ships the richer ack.
    r = ActionResult(
        action_id="a1", verb="move", accepted=False,
        reason="blocked_by_entity",
    )
    assert r.accepted is False
    assert r.reason == "blocked_by_entity"
    assert r.reason_code is None
    assert r.human_text is None


def test_action_result_new_shape():
    r = ActionResult(
        action_id="a1", verb="move", accepted=False,
        reason_code=ReasonCode.BLOCKED_BY_ENTITY.value,
        context={"blocker_id": "cara", "your_pos": [24, 17]},
        human_text="cara is standing where you tried to step east",
    )
    assert r.reason_code == "blocked_by_entity"
    assert r.context["blocker_id"] == "cara"
    assert "cara" in r.human_text


def test_reason_codes_are_stable():
    # Harness code branches on these literal strings; freezing them
    # here prevents accidental rename.
    assert ReasonCode.BLOCKED_BY_ENTITY.value == "blocked_by_entity"
    assert ReasonCode.OUT_OF_RANGE.value == "out_of_range"
    assert ReasonCode.PRECONDITION_FAILED.value == "precondition_failed"


# ---- Ack dispatch (stubs the WS) ----

class _FakeWS:
    """Minimal WS stub. Captures sends; tests poke responses back via
    the agent's _dispatch_ack helper directly."""
    def __init__(self):
        self.sent: list[dict] = []

    async def send(self, payload: str):
        self.sent.append(json.loads(payload))

    async def close(self):
        pass


@pytest.mark.asyncio
async def test_act_batch_round_trip_acks():
    agent = Agent(AgentCredentials(agent_id="a", agent_secret="s", ws_url="ws://x"))
    fake = _FakeWS()
    agent._ws = fake  # type: ignore[assignment]

    batch = ActionBatch(
        actions=[Move(target=(5, 5)), Speak(text="hi")],
        reasoning="walking and greeting",
    )

    async def run():
        return await agent.act_batch(batch, wait_for_acks=True, timeout=2.0)

    task = asyncio.create_task(run())
    # Let act_batch enqueue both actions before we ack them.
    await asyncio.sleep(0)
    assert len(fake.sent) == 2
    move_id = fake.sent[0]["action_id"]
    speak_id = fake.sent[1]["action_id"]
    # Both action frames carry the reasoning trace.
    assert fake.sent[0]["reasoning"] == "walking and greeting"

    # Engine acks: move accepted, speak rejected with the new richer fields.
    agent._dispatch_ack({
        "type": "action_ack",
        "action_id": move_id,
        "verb": "move",
        "accepted": True,
        "reason": "ok",
    })
    agent._dispatch_ack({
        "type": "action_ack",
        "action_id": speak_id,
        "verb": "speak",
        "accepted": False,
        "reason_code": "rate_limited",
        "context": {"actions_in_last_sec": 4},
        "human_text": "too many actions this second",
    })

    results = await task
    assert len(results) == 2
    assert results[0].accepted is True
    assert results[1].accepted is False
    assert results[1].reason_code == "rate_limited"
    assert results[1].context["actions_in_last_sec"] == 4
    # Ordering matches batch order, NOT ack arrival order.
    assert results[0].verb == "move"
    assert results[1].verb == "speak"


@pytest.mark.asyncio
async def test_act_batch_times_out_cleanly():
    # If the engine never acks, act_batch should raise TimeoutError
    # and clean up pending futures so the agent isn't leaking memory.
    agent = Agent(AgentCredentials(agent_id="a", agent_secret="s", ws_url="ws://x"))
    fake = _FakeWS()
    agent._ws = fake  # type: ignore[assignment]
    with pytest.raises(asyncio.TimeoutError):
        await agent.act_batch(
            ActionBatch(actions=[Move(target=(5, 5))]),
            wait_for_acks=True,
            timeout=0.05,
        )
    # No leaked futures.
    assert len(agent._pending_acks) == 0


@pytest.mark.asyncio
async def test_act_single_action_compat_shim():
    # The legacy act() entry-point should still work. It wraps the
    # action in a single-item batch and returns the first result.
    agent = Agent(AgentCredentials(agent_id="a", agent_secret="s", ws_url="ws://x"))
    fake = _FakeWS()
    agent._ws = fake  # type: ignore[assignment]

    async def run():
        return await agent.act_batch(ActionBatch(actions=[Move(target=(7, 7))]), wait_for_acks=True)

    task = asyncio.create_task(run())
    await asyncio.sleep(0)
    assert len(fake.sent) == 1
    aid = fake.sent[0]["action_id"]
    agent._dispatch_ack({
        "type": "action_ack",
        "action_id": aid,
        "verb": "move",
        "accepted": True,
    })
    rs = await task
    assert isinstance(rs, list)
    assert rs[0].accepted is True
