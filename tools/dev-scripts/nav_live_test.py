"""Live end-to-end: agent-side A* + engine step verb. The agent fetches the
real walkability grid, picks a reachable far goal, and navigates there one
N/S/E/W step per tick (re-planning around other agents), proving it routes
around real terrain (incl. the river) and actually arrives."""
import asyncio
from agent_sim_sdk import Agent, Step, VisionMode, register_agent
from agents.common.nav import NavGrid

def cheb(a,b): return max(abs(a[0]-b[0]), abs(a[1]-b[1]))

async def main():
    grid = NavGrid.fetch("http://127.0.0.1:8080")
    print(f"grid {grid.w}x{grid.h}")
    creds = await register_agent("http://127.0.0.1:8080", user_token="dev",
        persona={"name":"Navver","bio":"nav test","archetype_tag":"navver"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=450)
    async with Agent(creds) as ag:
        it = ag.observations().__aiter__()
        obs = await it.__anext__()
        start = tuple(obs.self.pos)
        # pick a reachable goal ~22 tiles away
        goal = None
        for r in (22, 18, 14, 10):
            for dx,dy in [(r,0),(0,r),(-r,0),(0,-r),(r,r),(-r,-r),(r,-r),(-r,r)]:
                cand=(start[0]+dx, start[1]+dy)
                if grid.walkable(*cand) and grid.astar(start, cand):
                    goal=cand; break
            if goal: break
        if not goal:
            print("RESULT: FAIL (no reachable goal found)"); return
        path0 = grid.astar(start, goal)
        print(f"start {start} -> goal {goal}  (straight-line {cheb(start,goal)}, A* path {len(path0)} tiles)")
        DELTA={"N":(0,-1),"S":(0,1),"E":(1,0),"W":(-1,0)}
        last=start; blacklist={}  # tile -> ticks remaining blocked
        for step_i in range(160):
            obs = await it.__anext__()
            here = tuple(obs.self.pos)
            if cheb(here, goal) <= 1:
                print(f"REACHED goal-adjacent at {here} in {step_i} ticks"); print("RESULT: PASS"); return
            blacklist = {t:n-1 for t,n in blacklist.items() if n>1}
            dyn = [tuple(e.pos) for e in (obs.visible_entities or []) if e.entity_id != obs.self.entity_id]
            dyn += list(blacklist)
            d = grid.next_dir(here, goal, dynamic_blocked=dyn, stop_adjacent=True)
            if d is None:
                print(f"no route from {here}; RESULT: FAIL"); return
            await ag.act(Step(dir=d))
            await asyncio.sleep(0.5)
            # if we didn't move, blacklist the tile we tried so we re-route
            if here == last:
                dxy=DELTA[d]; blacklist[(here[0]+dxy[0], here[1]+dxy[1])]=5
            last = here
        print(f"did not arrive in 160 ticks (got to {last}, {cheb(last,goal)} away); RESULT: FAIL")

asyncio.run(main())
