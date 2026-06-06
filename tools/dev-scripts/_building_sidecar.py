"""Long-lived sidecar for the visual building probe.

A single process holds the agent's WebSocket open for the lifetime of
the test. The JS probe spawns it once, reads the meta line on stdout,
then pipes "enter", "exit", and "quit" commands on stdin. Each command
sends the corresponding interact verb over the WS and prints OK back
to stdout (so the JS probe can wait for confirmation).

Protocol (line-oriented):
  stdout: META {"entity_id":..., "agent_id":..., "pos":[x,y]}
  stdin:  enter
  stdout: OK enter
  stdin:  exit
  stdout: OK exit
  stdin:  quit
  (process exits 0)

Keeping the WS open is the whole point — closing it makes
/api/v1/agents drop the agent, which breaks the JS-side viewport pan
that depends on knowing where the probe entity is.
"""
from __future__ import annotations
import asyncio
import json
import sys
import urllib.request

ENGINE = "http://127.0.0.1:8080"


def http_json(path: str) -> dict:
    with urllib.request.urlopen(ENGINE + path, timeout=5) as r:
        return json.loads(r.read())


async def stdin_lines():
    """Async-read lines from stdin without blocking the event loop."""
    loop = asyncio.get_running_loop()
    reader = asyncio.StreamReader()
    protocol = asyncio.StreamReaderProtocol(reader)
    await loop.connect_read_pipe(lambda: protocol, sys.stdin)
    while True:
        line = await reader.readline()
        if not line:
            return
        yield line.decode().strip()


async def main() -> int:
    from agent_sim_sdk import (
        Agent, ActionBatch, Interact, register_agent, VisionMode,
    )

    creds = await register_agent(
        ENGINE, user_token="dev",
        persona={"name": "visual_probe", "bio": "Building visual probe."},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500,
    )

    # Find the closest building to the spawn hub so the interact verb
    # has a sensible target. The engine accepts any 'bld:' target so
    # the exact one isn't critical for the engine side; for the visual
    # side it helps to pick something near the agent's spawn position.
    world = http_json("/worlds/eldoria.json")
    bldgs = [d for d in (world.get("decorations") or [])
             if isinstance(d.get("sprite"), str)
             and d["sprite"].startswith("bld:")]
    if not bldgs:
        print("ERR no buildings", flush=True)
        return 2

    async with Agent(creds) as a:
        # Drain the first obs to learn our position and entity_id.
        first = None
        async for o in a.observations():
            first = o
            break
        if first is None:
            print("ERR no observation", flush=True)
            return 3
        eid = first.self.entity_id
        agx, agy = first.self.pos
        # Pick the building closest to the agent.
        bldgs.sort(key=lambda b: max(abs(b["x"] - agx), abs(b["y"] - agy)))
        bld = bldgs[0]["sprite"]

        print("META " + json.dumps({
            "entity_id":    eid,
            "agent_id":     creds.agent_id,
            "pos":          [agx, agy],
            "building":     bld,
        }), flush=True)

        async for line in stdin_lines():
            cmd = line.strip().lower()
            if cmd == "quit":
                print("OK quit", flush=True)
                return 0
            if cmd not in ("enter", "exit"):
                print(f"ERR unknown_command {cmd}", flush=True)
                continue
            results = await a.act_batch(
                ActionBatch(actions=[Interact(
                    target=bld, affordance=cmd,
                )]),
                wait_for_acks=True, timeout=5.0,
            )
            ack = results[0]
            if not ack or not ack.accepted:
                print(f"ERR {cmd}_rejected reason={ack and ack.reason}", flush=True)
            else:
                print(f"OK {cmd}", flush=True)

    return 0


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
