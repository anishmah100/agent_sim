"""Agent client. Async WebSocket connection. Receives observations,
sends actions, lets the caller drive the decision loop."""

from __future__ import annotations

import asyncio
import base64
import json
import uuid
from collections.abc import AsyncIterator, Awaitable, Callable
from dataclasses import dataclass
from typing import Optional

import httpx
import websockets
from pydantic import TypeAdapter

from .models import Action, ActionBatch, ActionResult, Observation, VisionMode

_ObservationAdapter = TypeAdapter(Observation)


@dataclass
class AgentCredentials:
    agent_id: str
    agent_secret: str
    ws_url: str


async def register_agent(
    server: str,
    *,
    user_token: str,
    persona: dict,
    vision_mode: VisionMode = VisionMode.STRUCTURED,
    cadence_ms: int = 1000,
    bind_entity: Optional[str] = None,
    share_reasoning: bool = False,
) -> AgentCredentials:
    """One-shot HTTP registration. Returns credentials to feed into Agent().

    Pass `bind_entity` to claim a specific existing entity (e.g.
    `npc_woodcutter`) instead of letting the engine pick the first
    available agent-archetype body.

    Set `share_reasoning=True` to opt this agent's free-text reasoning
    traces into the experiment's capture stream. The engine ignores
    this unless it was launched with `-capture-reasoning` AS WELL —
    layered opt-in keeps private bots' inner monologue out of the
    log even when the experimenter forgets to flip the bot's flag."""
    body = {
        "user_token": user_token,
        "persona_blob": persona,
        "vision_mode": vision_mode.value,
        "cadence_ms": cadence_ms,
        "share_reasoning": share_reasoning,
    }
    if bind_entity:
        body["bind_entity"] = bind_entity
    async with httpx.AsyncClient() as h:
        r = await h.post(
            f"{server}/api/v1/agent/register",
            json=body,
            timeout=10.0,
        )
        r.raise_for_status()
        data = r.json()
        return AgentCredentials(
            agent_id=data["agent_id"],
            agent_secret=data["agent_secret"],
            ws_url=data["ws_url"],
        )


class Agent:
    """Long-lived WS connection. Exposes an async iterator of typed
    Observation objects + a coroutine for posting actions.

    Typical usage:

        async with Agent(creds) as agent:
            async for obs in agent.observations():
                action = my_brain(obs)
                if action:
                    await agent.act(action)
    """

    def __init__(self, creds: AgentCredentials):
        self._creds = creds
        self._ws: Optional[websockets.WebSocketClientProtocol] = None
        self._inbox: asyncio.Queue[Observation] = asyncio.Queue(maxsize=64)
        self._reader_task: Optional[asyncio.Task] = None
        # Per-action-id futures the harness can await for the engine's ack.
        # Populated by act_batch (and the legacy act); resolved by
        # _read_loop when an "action_ack" frame arrives.
        self._pending_acks: dict[str, asyncio.Future[ActionResult]] = {}

    async def __aenter__(self) -> "Agent":
        await self.connect()
        return self

    async def __aexit__(self, *args) -> None:
        await self.close()

    async def connect(self) -> None:
        self._ws = await websockets.connect(self._creds.ws_url)
        # First message authenticates.
        await self._ws.send(json.dumps({"auth": self._creds.agent_secret}))
        self._reader_task = asyncio.create_task(self._read_loop())

    async def close(self) -> None:
        if self._reader_task:
            self._reader_task.cancel()
            try:
                await self._reader_task
            except (asyncio.CancelledError, Exception):
                pass
        if self._ws:
            await self._ws.close()

    async def observations(self) -> AsyncIterator[Observation]:
        while True:
            yield await self._inbox.get()

    async def act(self, action: Action) -> Optional[ActionResult]:
        """Send ONE action. Returns engine's ack if it arrives within
        the timeout, None otherwise — the brain stays unblocked.

        Preserved for backward compatibility. New brain code should
        use `act_batch` so the engine's per-tick ordering applies
        across the whole receding-horizon plan.
        """
        results = await self.act_batch(ActionBatch(actions=[action]))
        return results[0] if results and results[0] is not None else None

    async def act_batch(
        self,
        batch: ActionBatch,
        *,
        wait_for_acks: bool = False,
        timeout: float = 5.0,
    ) -> list[Optional[ActionResult]]:
        """Send a 1–3-action plan + reasoning trace.

        By default this is fire-and-forget: the actions are sent over
        the WS immediately and the call returns once the bytes are out.
        Returns a list of None matching batch.actions length. The
        engine's acks still arrive on the WS and are routed into a
        side dict — pass `wait_for_acks=True` to block on them.

        The fire-and-forget default matters at scale: a slow engine
        tick (or a busy queue) can delay the ack by hundreds of ms,
        and we don't want the brain blocked behind those round-trips.
        The receding-horizon plan re-runs on the next observation
        anyway, so a missed ack is information the brain naturally
        recovers from.
        """
        if not self._ws:
            raise RuntimeError("not connected")
        import logging
        loop = asyncio.get_running_loop()
        futures: list[asyncio.Future[ActionResult]] = []
        sent_action_ids: list[str] = []
        for action in batch.actions:
            action_id = str(uuid.uuid4())
            f: asyncio.Future[ActionResult] = loop.create_future()
            self._pending_acks[action_id] = f
            futures.append(f)
            sent_action_ids.append(action_id)
            payload = {
                "type": "action",
                "action_id": action_id,
                **action.model_dump(),
            }
            if batch.reasoning:
                payload["reasoning"] = batch.reasoning
            try:
                await self._ws.send(json.dumps(payload))
            except Exception as e:
                logging.getLogger("agent_sim_sdk").warning(
                    "act_batch(%s) send failed: %s", payload.get("verb"), e,
                )
                for fid in sent_action_ids:
                    self._pending_acks.pop(fid, None)
                raise
        if not wait_for_acks:
            # Fire-and-forget. Don't keep the futures around — they'd
            # just leak. The engine's ack will land at _dispatch_ack
            # but find no future to resolve, and silently drop.
            for fid in sent_action_ids:
                self._pending_acks.pop(fid, None)
            return [None] * len(batch.actions)
        try:
            return await asyncio.wait_for(
                asyncio.gather(*futures), timeout=timeout,
            )
        except asyncio.TimeoutError:
            futures_set = set(map(id, futures))
            for action_id, fut in list(self._pending_acks.items()):
                if id(fut) in futures_set:
                    if not fut.done():
                        fut.cancel()
                    self._pending_acks.pop(action_id, None)
            raise

    async def set_cadence(self, interval_ms: int) -> None:
        if not self._ws:
            raise RuntimeError("not connected")
        await self._ws.send(json.dumps({"type": "set_cadence", "interval_ms": interval_ms}))

    async def reflect(self, note: str) -> None:
        """Send a reflective note for historian capture.

        Engine-side fan-out is gated by `-capture-reasoning` AND the
        per-agent `share_reasoning=True` opt-in (same layered consent
        as per-action reasoning). Fire-and-forget; no ack.
        """
        if not self._ws:
            raise RuntimeError("not connected")
        if not note:
            return
        await self._ws.send(json.dumps({"type": "reflection", "note": note}))

    async def _read_loop(self) -> None:
        assert self._ws
        async for raw in self._ws:
            try:
                msg = json.loads(raw) if isinstance(raw, str) else json.loads(raw.decode())
            except Exception:
                continue
            mtype = msg.get("type")
            if mtype == "action_ack":
                self._dispatch_ack(msg)
                continue
            if mtype != "observation":
                # Future: dispatch world_event_notify here.
                continue
            payload = msg
            # Decode view_image if present.
            vi = payload.get("view_image")
            if vi and isinstance(vi.get("data"), str):
                vi["data"] = base64.b64decode(vi["data"])
            # Defensive: older engine builds (or future bare-engine modes)
            # may serialize empty slices as null. The pydantic Observation
            # model types these as list[...], so coerce None → [] before
            # validation. Engine v0.0.3+ initializes empty slices but we
            # keep the coercion for forward + backward compat.
            for key in ("visible_entities", "visible_objects", "audible",
                        "recent_self_results"):
                if payload.get(key) is None:
                    payload[key] = []
            try:
                obs = _ObservationAdapter.validate_python(payload)
            except Exception as e:
                import logging
                logging.getLogger("agent_sim_sdk").warning(
                    "observation validation failed: %s — payload keys=%s",
                    e, list(payload.keys()),
                )
                continue
            await self._inbox.put(obs)

    def _dispatch_ack(self, msg: dict) -> None:
        """Resolve the pending future for an action_ack frame.

        Handles both the legacy ack shape (accepted + reason) and the
        forthcoming richer shape (reason_code + context + human_text)
        — see ActionResult.
        """
        action_id = msg.get("action_id")
        if not action_id:
            return
        fut = self._pending_acks.pop(action_id, None)
        if fut is None or fut.done():
            return
        try:
            result = ActionResult(
                action_id=action_id,
                verb=msg.get("verb", ""),
                accepted=bool(msg.get("accepted", False)),
                reason=msg.get("reason"),
                reason_code=msg.get("reason_code"),
                context=msg.get("context"),
                human_text=msg.get("human_text"),
            )
            fut.set_result(result)
        except Exception as e:
            fut.set_exception(e)


async def register_and_connect(
    server: str,
    *,
    user_token: str,
    persona: dict,
    vision_mode: VisionMode = VisionMode.STRUCTURED,
    cadence_ms: int = 1000,
    bind_entity: Optional[str] = None,
    share_reasoning: bool = False,
    brain: Optional[Callable[[Observation], Awaitable[Optional[Action]]]] = None,
) -> Agent:
    """Convenience: register + connect + optionally run a brain loop.

    If `brain` is provided, an internal task drives obs → action; otherwise
    the caller drives the loop themselves via agent.observations()."""
    creds = await register_agent(
        server, user_token=user_token, persona=persona,
        vision_mode=vision_mode, cadence_ms=cadence_ms,
        bind_entity=bind_entity,
        share_reasoning=share_reasoning,
    )
    agent = Agent(creds)
    await agent.connect()
    if brain:
        async def loop() -> None:
            async for obs in agent.observations():
                try:
                    act = await brain(obs)
                except Exception as e:
                    import logging
                    logging.getLogger("agent_sim_sdk").warning(
                        "brain raised %s — skipping this obs", e,
                    )
                    continue
                if act is not None:
                    try:
                        await agent.act(act)
                    except Exception as e:
                        import logging
                        logging.getLogger("agent_sim_sdk").warning(
                            "act(%s) failed: %s", type(act).__name__, e,
                        )
        # CRITICAL: keep the task reference on the Agent so it isn't
        # silently garbage-collected mid-run. Without this, the brain
        # appears to stop responding after a few ticks.
        agent._brain_task = asyncio.create_task(loop())  # type: ignore[attr-defined]
    return agent
