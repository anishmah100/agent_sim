"""Instrumented hunter vs stationary victim — isolates the closing+attack
mechanic. Logs hunter pos, victim pos, distance, action, victim hp each
tick so we see EXACTLY why attacks do/don't land."""
from __future__ import annotations
import asyncio
from agent_sim_sdk import Agent, Attack, Move, Wait, VisionMode, register_agent

def cheb(a,b): return max(abs(a[0]-b[0]),abs(a[1]-b[1]))
def step(h,t):
    dx=(t[0]>h[0])-(t[0]<h[0]); dy=(t[1]>h[1])-(t[1]<h[1]); return [h[0]+dx,h[1]+dy]

async def victim(creds, state):
    async with Agent(creds) as ag:
        async for obs in ag.observations():
            state['vid']=obs.self.entity_id; state['vpos']=tuple(obs.self.pos)
            state['vhp']=(obs.self.extras or {}).get('hp')
            await ag.act(Wait(ticks=30))

async def hunter(creds, state):
    async with Agent(creds) as ag:
        n=0
        async for obs in ag.observations():
            n+=1
            if n>40: return
            here=tuple(obs.self.pos)
            # find the victim among visible entities
            vid=state.get('vid')
            tgt=None
            for e in obs.visible_entities:
                if e.entity_id==vid: tgt=e
            if tgt is None:
                # victim not visible — move toward last known
                vp=state.get('vpos')
                print(f"[{n}] hunter={here} victim NOT VISIBLE (known={vp}) -> move")
                if vp: await ag.act(Move(target=list(vp)))
                continue
            d=cheb(here,tuple(tgt.pos))
            if d<=1:
                print(f"[{n}] hunter={here} victim={tuple(tgt.pos)} d={d} vhp={state.get('vhp')} -> ATTACK")
                await ag.act(Attack(target=vid))
            else:
                print(f"[{n}] hunter={here} victim={tuple(tgt.pos)} d={d} -> MOVE")
                await ag.act(Move(target=list(tgt.pos)))

async def main():
    eng="http://127.0.0.1:8080"
    vc=await register_agent(eng,user_token="dev",persona={"name":"Victim","bio":"v","archetype_tag":"victim"},vision_mode=VisionMode.STRUCTURED,cadence_ms=500)
    hc=await register_agent(eng,user_token="dev",persona={"name":"Hunter","bio":"h","archetype_tag":"hunter"},vision_mode=VisionMode.STRUCTURED,cadence_ms=500)
    print("victim",vc.agent_id,"hunter",hc.agent_id)
    state={}
    await asyncio.gather(victim(vc,state), hunter(hc,state))

asyncio.run(main())
