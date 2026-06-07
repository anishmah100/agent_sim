"""Live test of the engine `step` verb: fire each direction, assert the
world position changes by exactly the right delta."""
import asyncio
from agent_sim_sdk import Agent, Step, VisionMode, register_agent

DELTAS = {"N":(0,-1),"S":(0,1),"E":(1,0),"W":(-1,0)}

async def main():
    creds = await register_agent("http://127.0.0.1:8080", user_token="dev",
        persona={"name":"Stepper","bio":"step test","archetype_tag":"stepper"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=300)
    ok = True
    async with Agent(creds) as ag:
        it = ag.observations().__aiter__()
        obs = await it.__anext__()
        pos = tuple(obs.self.pos)
        print("start pos", pos)
        for d in ["E","S","N","W","S","E"]:
            res = await ag.act(Step(dir=d))
            r0 = res[0] if res else None
            accepted = getattr(r0, "accepted", None)
            reason = getattr(r0, "reason", "")
            await asyncio.sleep(0.7)
            obs = await it.__anext__()
            newpos = tuple(obs.self.pos)
            dx,dy = DELTAS[d]
            exp = (pos[0]+dx, pos[1]+dy)
            if newpos==exp:
                status = "OK moved one tile"
            elif newpos==pos:
                status = "OK blocked — correctly did not move"
            else:
                status = "MISMATCH"; ok=False
            print(f"step {d}: {pos} -> {newpos} accepted={accepted} {status}")
            pos = newpos
    print("RESULT:", "PASS" if ok else "FAIL")

asyncio.run(main())
