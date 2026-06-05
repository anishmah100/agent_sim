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

from .models import Action, Observation, VisionMode

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
) -> AgentCredentials:
    """One-shot HTTP registration. Returns credentials to feed into Agent().

    Pass `bind_entity` to claim a specific existing entity (e.g.
    `npc_woodcutter`) instead of letting the engine pick the first
    available agent-archetype body."""
    body = {
        "user_token": user_token,
        "persona_blob": persona,
        "vision_mode": vision_mode.value,
        "cadence_ms": cadence_ms,
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

    async def act(self, action: Action) -> None:
        """Send a typed action. The engine returns an action_ack on the
        same stream; this SDK collects acks into observations'
        recent_self_results for now."""
        if not self._ws:
            raise RuntimeError("not connected")
        payload = {
            "type": "action",
            "action_id": str(uuid.uuid4()),
            **action.model_dump(),
        }
        try:
            await self._ws.send(json.dumps(payload))
        except Exception as e:
            # Bubble up but log first so silent send failures show in
            # bot logs (instrumentation added during emergence debugging).
            import logging
            logging.getLogger("agent_sim_sdk").warning(
                "act(%s) send failed: %s", payload.get("verb"), e,
            )
            raise

    async def set_cadence(self, interval_ms: int) -> None:
        if not self._ws:
            raise RuntimeError("not connected")
        await self._ws.send(json.dumps({"type": "set_cadence", "interval_ms": interval_ms}))

    async def _read_loop(self) -> None:
        assert self._ws
        async for raw in self._ws:
            try:
                msg = json.loads(raw) if isinstance(raw, str) else json.loads(raw.decode())
            except Exception:
                continue
            if msg.get("type") != "observation":
                # Future: dispatch action_ack / world_event_notify here.
                continue
            payload = msg
            # Decode view_image if present.
            vi = payload.get("view_image")
            if vi and isinstance(vi.get("data"), str):
                vi["data"] = base64.b64decode(vi["data"])
            try:
                obs = _ObservationAdapter.validate_python(payload)
            except Exception:
                continue
            await self._inbox.put(obs)


async def register_and_connect(
    server: str,
    *,
    user_token: str,
    persona: dict,
    vision_mode: VisionMode = VisionMode.STRUCTURED,
    cadence_ms: int = 1000,
    bind_entity: Optional[str] = None,
    brain: Optional[Callable[[Observation], Awaitable[Optional[Action]]]] = None,
) -> Agent:
    """Convenience: register + connect + optionally run a brain loop.

    If `brain` is provided, an internal task drives obs → action; otherwise
    the caller drives the loop themselves via agent.observations()."""
    creds = await register_agent(
        server, user_token=user_token, persona=persona,
        vision_mode=vision_mode, cadence_ms=cadence_ms,
        bind_entity=bind_entity,
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
