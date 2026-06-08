"""Live check for the egocentric ASCII local view (slice 2b).

Registers one agent against the running engine, pulls its first real
observation, and prints the local_view: its geometry plus a trimmed
central window so we can eyeball that terrain ('.'/'#'/'~') and the '@'
self-marker render correctly against the live world.
"""
import asyncio
from agent_sim_sdk import Agent, Wait, VisionMode, register_agent

E = "http://127.0.0.1:8080"


async def main():
    c = await register_agent(
        E, user_token="dev",
        persona={"name": "LVProbe", "bio": "p", "archetype_tag": "survivor"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500,
    )
    async with Agent(c) as ag:
        it = ag.observations().__aiter__()
        obs = await it.__anext__()
        lv = obs.local_view
        sx, sy = obs.self.pos
        print(f"self pos = ({sx},{sy})")
        if lv is None:
            print("FAIL: local_view is None")
            return
        print(f"radius={lv.radius} origin={lv.origin} rows={len(lv.rows)}x"
              f"{len(lv.rows[0]) if lv.rows else 0}")
        # center glyph must be '@'
        cx = sx - lv.origin[0]
        cy = sy - lv.origin[1]
        center = lv.rows[cy][cx]
        print(f"center glyph (self) = {center!r}  (expect '@')")
        # Trim to a 21x21 central window for readability.
        r = 10
        print("--- central 21x21 window (N up) ---")
        for ry in range(cy - r, cy + r + 1):
            if 0 <= ry < len(lv.rows):
                print(lv.rows[ry][max(0, cx - r):cx + r + 1])
        # glyph histogram over the whole view
        hist = {}
        for row in lv.rows:
            for ch in row:
                hist[ch] = hist.get(ch, 0) + 1
        print("glyph counts:", {k: hist[k] for k in sorted(hist)})
        await ag.act(Wait(ticks=10))


asyncio.run(main())
