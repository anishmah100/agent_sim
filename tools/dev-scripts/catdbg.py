import asyncio
from agent_sim_sdk import Agent, Step, Attack, Wait, VisionMode, register_agent
from agents.common.nav import NavGrid
from agents.baselines._common import nearest, chebyshev

async def mouse(c):
    async with Agent(c) as ag:
        async for obs in ag.observations():
            await ag.act(Wait(ticks=60))

async def cat(c, grid):
    async with Agent(c) as ag:
        it = ag.observations().__aiter__(); obs = await it.__anext__()
        for i in range(30):
            here = tuple(obs.self.pos)
            prey = [e for e in (obs.visible_entities or []) if e.entity_id != obs.self.entity_id]
            if not prey:
                print(f"[{i}] cat={here} NO PREY", flush=True)
                await ag.act(Step(dir="E"))
            else:
                t = nearest(prey, here); d = chebyshev(here, tuple(t.pos))
                if d <= 1:
                    print(f"[{i}] cat={here} prey={tuple(t.pos)} d={d} -> ATTACK", flush=True)
                    await ag.act(Attack(target=t.entity_id))
                else:
                    dr = grid.next_dir(here, tuple(t.pos), dynamic_blocked=[tuple(e.pos) for e in prey], stop_adjacent=True)
                    print(f"[{i}] cat={here} prey={tuple(t.pos)} d={d} -> step {dr}", flush=True)
                    if dr:
                        await ag.act(Step(dir=dr))
            await asyncio.sleep(0.5); obs = await it.__anext__()

async def main():
    grid = NavGrid.fetch("http://127.0.0.1:8080")
    mc = await register_agent("http://127.0.0.1:8080", user_token="dev", persona={"name":"Mouse","bio":"m","archetype_tag":"mouse"}, vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    cc = await register_agent("http://127.0.0.1:8080", user_token="dev", persona={"name":"Cat","bio":"c","archetype_tag":"cat"}, vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    await asyncio.gather(mouse(mc), cat(cc, grid))

asyncio.run(main())
