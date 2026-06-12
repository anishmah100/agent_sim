#!/usr/bin/env python3
"""Audit harness — shared live-engine helpers.

Thin async wrappers over the agent WebSocket so every audit script talks to a
*live* engine the same way (register -> auth -> observe/act). See
docs/ENVIRONMENT_AUDIT_PLAN.md. NO mocks: everything here hits a real engine.
"""
import asyncio
import json
import uuid

import websockets
from agent_sim_sdk import register_agent, VisionMode


class Conn:
    def __init__(self, ws, creds, first_obs):
        self.ws = ws
        self.creds = creds
        self.obs = first_obs

    async def observe(self, timeout: float = 10.0) -> dict:
        while True:
            raw = await asyncio.wait_for(self.ws.recv(), timeout=timeout)
            m = json.loads(raw if isinstance(raw, str) else raw.decode())
            if m.get("type") == "observation":
                self.obs = m
                return m

    async def act(self, verb: str, *, wait_ack: bool = True, timeout: float = 4.0,
                  **params) -> dict:
        """Submit one action; return the action_ack frame (or {} if not waited).
        If no matching ack arrives within `timeout`, returns a sentinel
        {"accepted": None, "reason": "__no_ack__"} instead of hanging — a verb
        that never acks is itself an audit finding."""
        aid = str(uuid.uuid4())
        msg = {"type": "action", "action_id": aid, "verb": verb, **params}
        await self.ws.send(json.dumps(msg))
        if not wait_ack:
            return {}
        # Wall-clock deadline across the whole loop: observations stream in
        # every cadence_ms, so a per-recv timeout never fires while frames
        # keep arriving. A verb that never acks must still bail out.
        loop = asyncio.get_running_loop()
        deadline = loop.time() + timeout
        while True:
            remaining = deadline - loop.time()
            if remaining <= 0:
                return {"accepted": None, "reason": "__no_ack__"}
            try:
                raw = await asyncio.wait_for(self.ws.recv(), timeout=remaining)
            except asyncio.TimeoutError:
                return {"accepted": None, "reason": "__no_ack__"}
            m = json.loads(raw if isinstance(raw, str) else raw.decode())
            if m.get("type") == "action_ack" and m.get("action_id") == aid:
                return m
            # ignore interleaved observations

    async def step_to(self, target, *, max_steps: int = 250) -> bool:
        """Obstacle-aware walk toward a world tile using local_view glyphs.
        Returns True if it reached chebyshev<=1 of target."""
        DELTA = {"N": (0, -1), "S": (0, 1), "E": (1, 0), "W": (-1, 0)}

        def glyph(lv, wx, wy):
            if not lv:
                return None
            ox, oy = lv["origin"]
            rows = lv["rows"]
            col, row = wx - ox, wy - oy
            if 0 <= row < len(rows) and 0 <= col < len(rows[row]):
                return rows[row][col]
            return None

        for _ in range(max_steps):
            pos = tuple(self.obs["self"]["pos"])
            if max(abs(pos[0] - target[0]), abs(pos[1] - target[1])) <= 1:
                return True
            cand = []
            if target[0] > pos[0]: cand.append("E")
            if target[0] < pos[0]: cand.append("W")
            if target[1] > pos[1]: cand.append("S")
            if target[1] < pos[1]: cand.append("N")
            cand.sort(key=lambda d: -abs((target[0]-pos[0]) if d in "EW" else (target[1]-pos[1])))
            for d in ("N", "S", "E", "W"):
                if d not in cand:
                    cand.append(d)
            lv = self.obs.get("local_view")
            chosen = next((d for d in cand
                           if glyph(lv, pos[0]+DELTA[d][0], pos[1]+DELTA[d][1]) not in ("#", "~")),
                          cand[0])
            await self.act("step", dir=chosen, wait_ack=False)
            await self.observe()
        return False


async def connect(engine: str, name: str = "Auditor",
                  vision: VisionMode = VisionMode.STRUCTURED,
                  cadence_ms: int = 200) -> Conn:
    creds = await register_agent(
        engine, user_token="dev",
        persona={"name": name, "bio": "audit probe", "archetype_tag": "llm", "brain": "qwen"},
        vision_mode=vision, cadence_ms=cadence_ms,
    )
    # ping_interval=None: disable the client keepalive. The engine doesn't
    # always pong idle agent sockets (known WS-keepalive gap), and audit
    # probes sit idle while OTHER agents navigate — the default 20s ping
    # timeout then kills an idle probe mid-test. The engine pushes
    # observations on its own cadence, so the link stays live regardless.
    ws = await websockets.connect(creds.ws_url, ping_interval=None)
    await ws.send(json.dumps({"auth": creds.agent_secret}))
    c = Conn(ws, creds, None)
    await c.observe()
    return c
