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

# MAJ-6: sentinel pushed into the inbox when the WS reader ends, so
# observations() can return cleanly instead of blocking forever.
_STREAM_END = object()


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
        # Coalesce to the freshest observation. A slow brain (e.g. an LLM
        # that takes 3-8s/decision) falls behind the engine's observation
        # cadence; queued observations are stale snapshots of the world and
        # acting on them is worse than useless (the agent chases positions
        # that have moved). So we block for at least one observation, then
        # drain any others already waiting and yield only the most recent.
        # Fast brains (rule-based bots) never have a backlog, so this is a
        # no-op for them.
        while True:
            obs = await self._inbox.get()
            # MAJ-6: end-of-stream sentinel. When the WS drops, _read_loop
            # pushes _STREAM_END so this generator RETURNS cleanly instead
            # of blocking forever on inbox.get() (which silently froze the
            # agent — the keepalive fix prevents self-inflicted drops, but
            # a server/network drop still ends the reader). The bot's
            # `async for obs in agent.observations()` then exits its loop.
            if obs is _STREAM_END:
                return
            while True:
                try:
                    obs = self._inbox.get_nowait()
                except asyncio.QueueEmpty:
                    break
                if obs is _STREAM_END:
                    return
            yield obs

    async def act(self, action: Action, *, timeout: float = 5.0) -> Optional[ActionResult]:
        """Send ONE action and return the engine's ack (or None on timeout).

        Convenience wrapper around `act_batch` with `wait_for_acks=True` — so
        callers that inspect the result (`res.accepted`) actually get one. The
        previous implementation passed fire-and-forget, so it ALWAYS returned
        None regardless of whether the action landed, which made single-action
        probes look like the agent never moved. New brain code should still
        prefer `act_batch` so the engine's per-tick ordering applies across a
        whole receding-horizon plan.
        """
        results = await self.act_batch(
            ActionBatch(actions=[action]), wait_for_acks=True, timeout=timeout)
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

    async def note(
        self,
        text: str,
        tag: Optional[str] = None,
        slots: Optional[dict[str, str]] = None,
    ) -> None:
        """D14 — emit a generic mental note. Private; never relayed to
        other agents. The optional ``slots`` dict carries
        goal/plan/beliefs/emotion (any subset; other keys ignored by
        the inspector UI). Fire-and-forget; no ack. Subject to the
        same layered opt-in as ``reflect()`` / per-action reasoning.

        Example::

            await agent.note(
                "Heading to market",
                slots={"goal": "buy bread", "plan": "approach baker stall"},
            )
        """
        if not self._ws:
            raise RuntimeError("not connected")
        if not text:
            return
        payload: dict = {"type": "mental_note", "text": text}
        if tag:
            payload["tag"] = tag
        if slots:
            payload["slots"] = slots
        await self._ws.send(json.dumps(payload))

    async def _read_loop(self) -> None:
        # MAJ-6: whatever ends the reader (clean close, server drop,
        # network error, cancel), signal end-of-stream so observations()
        # returns instead of hanging forever on inbox.get().
        try:
            await self._read_loop_impl()
        finally:
            try:
                self._inbox.put_nowait(_STREAM_END)
            except asyncio.QueueFull:
                try:
                    self._inbox.get_nowait()
                except asyncio.QueueEmpty:
                    pass
                try:
                    self._inbox.put_nowait(_STREAM_END)
                except asyncio.QueueFull:
                    pass

    async def _read_loop_impl(self) -> None:
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
            for key in ("visible_entities", "visible_objects", "visible_items",
                        "audible", "recent_self_results"):
                if payload.get(key) is None:
                    payload[key] = []
            # Defensive: non-agent entities (items, decorations) may
            # have empty `facing` because they have no orientation.
            # The pydantic Facing enum only accepts N/S/E/W, so
            # coerce empties to "S" before validation. Same for
            # self.facing in case the engine ever sends an empty
            # string for a body that hasn't been oriented yet.
            for v in payload.get("visible_entities") or []:
                if not v.get("facing"):
                    v["facing"] = "S"
            if isinstance(payload.get("self"), dict) and not payload["self"].get("facing"):
                payload["self"]["facing"] = "S"
            # M4: coerce an unknown audible `kind` to "sound" before
            # validation. The engine field is a free string; if any emitter
            # ever sends a kind outside the SDK enum, pydantic would reject
            # the WHOLE observation and the brain would silently go blind
            # (agent "looks dead"). Better to keep the obs and degrade one
            # field. Same defensive spirit as the facing coercion.
            _AUD_KINDS = {"speech", "shout", "whisper", "sound"}
            for ev in payload.get("audible") or []:
                if isinstance(ev, dict) and ev.get("kind") not in _AUD_KINDS:
                    ev["kind"] = "sound"
            try:
                obs = _ObservationAdapter.validate_python(payload)
            except Exception as e:
                import logging
                # ERROR (not WARNING) with the full error: a persistent
                # validation failure silently blinds the agent, so it must
                # be loud. (M4)
                logging.getLogger("agent_sim_sdk").error(
                    "observation DROPPED — validation failed: %s — payload keys=%s",
                    e, list(payload.keys()),
                )
                continue
            # NEVER block the read loop. If the consumer is slow and the
            # inbox is full, `await put()` would stall this loop — which
            # also stalls the websockets recv path that answers server
            # pings, so the connection dies with a 1011 keepalive-ping
            # timeout (this silently killed every LLM agent ~2min into a
            # run: 3-8s/decision filled the 64-slot inbox, the reader
            # blocked, pings stopped, the body got orphan-cleaned). Drop
            # the oldest observation to make room and keep draining the
            # socket; observations() coalesces to the freshest anyway.
            try:
                self._inbox.put_nowait(obs)
            except asyncio.QueueFull:
                try:
                    self._inbox.get_nowait()
                except asyncio.QueueEmpty:
                    pass
                try:
                    self._inbox.put_nowait(obs)
                except asyncio.QueueFull:
                    pass

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
