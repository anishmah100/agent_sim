"""Definitive attack test: attacker navigates adjacent to a stationary
victim and attacks; we poll the victim's HP to see if damage actually
lands. Isolates the attack affordance from the chase/flee dynamics."""
import asyncio, json, urllib.request
from agent_sim_sdk import Agent, Step, Attack, Wait, VisionMode, register_agent
from agents.common.nav import NavGrid
from agents.baselines._common import nearest, chebyshev

E = "http://127.0.0.1:8080"

def hp_of(eid):
    try:
        ms = json.load(urllib.request.urlopen(f"{E}/api/v1/agent/{eid}/mental_state", timeout=4))
        return (ms.get("vitals") or {}).get("hp")
    except Exception as ex:
        return f"err{ex}"

async def victim(c):
    async with Agent(c) as ag:
        async for obs in ag.observations():
            await ag.act(Wait(ticks=60))

async def attacker(c, grid, veid):
    async with Agent(c) as ag:
        it = ag.observations().__aiter__(); obs = await it.__anext__()
        for i in range(40):
            here = tuple(obs.self.pos)
            prey = [e for e in (obs.visible_entities or []) if e.entity_id != obs.self.entity_id]
            if prey:
                t = nearest(prey, here); d = chebyshev(here, tuple(t.pos))
                if d <= 1:
                    res = await ag.act(Attack(target=t.entity_id))
                    print(f"[{i}] ATTACK {t.entity_id} d={d} victim_hp={hp_of(veid[0])} result={res}", flush=True)
                else:
                    dr = grid.next_dir(here, tuple(t.pos), dynamic_blocked=[tuple(e.pos) for e in prey], stop_adjacent=True)
                    if dr:
                        await ag.act(Step(dir=dr))
            await asyncio.sleep(0.5); obs = await it.__anext__()

async def main():
    grid = NavGrid.fetch(E)
    vc = await register_agent(E, user_token="dev", persona={"name":"Victim","bio":"v","archetype_tag":"survivor"}, vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    ac = await register_agent(E, user_token="dev", persona={"name":"Attacker","bio":"a","archetype_tag":"killer"}, vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    veid = [None]
    await asyncio.sleep(1)
    ags = json.load(urllib.request.urlopen(f"{E}/api/v1/agents", timeout=5))["agents"]
    for a in ags:
        if a.get("persona_name") == "Victim":
            veid[0] = a["entity_id"]
    print("victim entity:", veid[0], "hp0=", hp_of(veid[0]), flush=True)
    await asyncio.gather(victim(vc), attacker(ac, grid, veid))

asyncio.run(main())
