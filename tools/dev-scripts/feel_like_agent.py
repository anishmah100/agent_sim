"""Print exactly what an agent sees + the literal LLM prompt for one tick."""
import asyncio
from agent_sim_sdk import Agent, VisionMode, register_agent
from agents.llm.prompt import build_prompt, render_self, render_visible

async def main():
    creds = await register_agent("http://127.0.0.1:8080", user_token="dev",
        persona={"name":"Watcher","bio":"observer","archetype_tag":"watcher"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=500)
    async with Agent(creds) as ag:
        async for obs in ag.observations():
            s = obs.self
            print("="*70)
            print("RAW STRUCTURED OBSERVATION (the agent's 'senses'):")
            print("="*70)
            print(f"self.pos = {tuple(s.pos)}   facing={getattr(s,'facing','?')}")
            print(f"self.extras hp/hunger/gold = {(s.extras or {}).get('hp')}/{(s.extras or {}).get('hunger')}/{(s.extras or {}).get('gold')}")
            print(f"map_dims = {getattr(obs,'map_dims',None)}")
            print(f"# visible_entities = {len(obs.visible_entities or [])}")
            for e in (obs.visible_entities or [])[:6]:
                print(f"   entity {e.entity_id} archetype={e.archetype} pos={tuple(e.pos)} summary={e.extras_summary}")
            print(f"# visible_items = {len(obs.visible_items or [])}")
            for it in (obs.visible_items or [])[:6]:
                print(f"   item {it.entity_id} sprite={it.sprite} pos={tuple(it.pos)}")
            objs = getattr(obs,'visible_objects',[]) or []
            print(f"# visible_objects (doors etc) = {len(objs)}")
            for o in objs[:4]:
                print(f"   object {o.object_id} kind={o.kind} pos={tuple(o.pos)}")
            print()
            print("="*70)
            print("THE LITERAL PROMPT THE LLM READS (its 'eyes' as text):")
            print("="*70)
            try:
                p = build_prompt(obs, "You are Watcher, a curious soul.", "Survive and explore.")
                print(p)
            except Exception as ex:
                print("build_prompt err:", ex)
                print(render_self(obs)); print(render_visible(obs, tuple(s.pos)))
            return

asyncio.run(main())
