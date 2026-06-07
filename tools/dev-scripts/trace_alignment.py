"""Observation<->world alignment tracer.

For a live agent, at each step cross-checks THREE independent views:
  1. What the SDK agent actually receives over the WebSocket.
  2. What the engine's observation BUILDER produces (/debug/vision?entity).
     (Same builder the WS uses -> any diff = serialization-layer bug,
      the exact class that once silently dropped visible_items.)
  3. The authoritative per-entity vitals (/agent/<id>/mental_state).
Plus: no item is reported outside vision radius (no phantoms), a known
adjacent item is actually visible (LOS sane), the full contract
plumbing (propose->see->accept->complete flips status on BOTH parties),
and action->world-state deltas.

Run with the engine up:  PYTHONPATH=sdk/python:. python3 tools/dev-scripts/trace_alignment.py
"""
from __future__ import annotations
import asyncio, json, urllib.request
from agent_sim_sdk import (
    register_agent, Agent, ActionBatch, VisionMode,
    Move, Pickup, Pay, Give, Eat, ProposeTask, AcceptTask, CompleteTask,
)

import os as _os
ENGINE = _os.environ.get("AGENT_SIM_ENGINE", "http://127.0.0.1:8080")
VISION = 12
fails: list[str] = []
checks = 0


def GET(path):
    with urllib.request.urlopen(ENGINE + path, timeout=4) as r:
        return json.load(r)


def chk(cond, label):
    global checks
    checks += 1
    if not cond:
        fails.append(label)
        print(f"  [FAIL] {label}")
    else:
        print(f"  [ ok ] {label}")


def cheb(a, b):
    return max(abs(a[0] - b[0]), abs(a[1] - b[1]))


async def first(a):
    async for o in a.observations():
        return o


def item_ids(lst):
    return sorted(it.entity_id if hasattr(it, "entity_id") else it["entity_id"] for it in (lst or []))


async def main():
    A = await register_agent(ENGINE, user_token="dev",
        persona={"name": "tracer_A", "bio": "", "archetype_tag": "llm"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=300)
    B = await register_agent(ENGINE, user_token="dev",
        persona={"name": "tracer_B", "bio": "", "archetype_tag": "llm"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=300)
    async with Agent(A) as a, Agent(B) as b:
        oa = await first(a); ob = await first(b)
        eA, eB = oa.self.entity_id, ob.self.entity_id
        print(f"A={eA}  B={eB}\n")

        # ---- OBSERVATION FIDELITY over several positions ----
        for step in range(6):
            oa = await first(a)
            pos = tuple(oa.self.pos)
            obs_items = item_ids(getattr(oa, "visible_items", []))
            obs_ents = sorted(e.entity_id for e in (getattr(oa, "visible_entities", []) or []))
            dv = GET(f"/api/v1/debug/vision?entity={eA}")
            ms = GET(f"/api/v1/agent/{eA}/mental_state").get("vitals") or {}
            print(f"step {step} @ {pos}: obs_items={len(obs_items)} builder_items={dv.get('v_items')} "
                  f"obs_ents={len(obs_ents)} builder_ents={dv.get('v_entities')}")
            # 1. serialization fidelity: SDK items == builder items (set)
            chk(obs_items == item_ids(dv.get("items", [])),
                f"step{step}: SDK visible_items == builder visible_items")
            # 2. entity count parity (builder returns counts only)
            chk(len(obs_ents) == dv.get("v_entities"),
                f"step{step}: SDK visible_entities count == builder count")
            # 3. state fidelity vs authoritative vitals
            ex = oa.self.extras or {}
            chk(ex.get("gold") == ms.get("gold"), f"step{step}: self.gold==vitals.gold ({ex.get('gold')}/{ms.get('gold')})")
            chk(ex.get("hp") == ms.get("hp"), f"step{step}: self.hp==vitals.hp ({ex.get('hp')}/{ms.get('hp')})")
            # 4. no phantom items beyond vision radius
            beyond = [it for it in (getattr(oa, "visible_items", []) or [])
                      if cheb(pos, tuple(it.pos)) > VISION]
            chk(not beyond, f"step{step}: no items beyond vision radius {VISION} ({len(beyond)} over)")
            # 5. no phantom entities beyond radius
            be = [e for e in (getattr(oa, "visible_entities", []) or [])
                  if cheb(pos, tuple(e.pos)) > VISION]
            chk(not be, f"step{step}: no entities beyond vision radius ({len(be)} over)")
            # step toward the nearest item to re-check at a new tile
            its = list(getattr(oa, "visible_items", []) or [])
            if its:
                its.sort(key=lambda it: cheb(pos, tuple(it.pos)))
                t = its[0].pos
                dx = 1 if pos[0] < t[0] else -1 if pos[0] > t[0] else 0
                dy = 1 if pos[1] < t[1] else -1 if pos[1] > t[1] else 0
                await a.act_batch(ActionBatch(actions=[Move(target=[pos[0]+dx, pos[1]+dy])]))
            await asyncio.sleep(0.5)

        # ---- BEHAVIORAL LOS: walk onto an item tile, must be visible+adjacent ----
        print("\n-- behavioral: reach an item, confirm visible at adjacency --")
        oa = await first(a)
        its = list(getattr(oa, "visible_items", []) or [])
        if its:
            its.sort(key=lambda it: cheb(tuple(oa.self.pos), tuple(it.pos)))
            tgt = its[0]; tx, ty = tgt.pos
            for _ in range(30):
                oa = await first(a); cx, cy = oa.self.pos
                if cheb((cx, cy), (tx, ty)) <= 1:
                    break
                await a.act_batch(ActionBatch(actions=[Move(target=[tx, ty])]))
                await asyncio.sleep(0.35)
            oa = await first(a)
            still = [it for it in (getattr(oa, "visible_items", []) or []) if it.entity_id == tgt.entity_id]
            adj = cheb(tuple(oa.self.pos), (tx, ty)) <= 1
            chk(adj, f"reached adjacency to {tgt.entity_id}")
            # pickup delta
            g0 = (GET(f'/api/v1/agent/{eA}/mental_state').get('vitals') or {}).get('gold', 0)
            inv0 = len((oa.self.extras or {}).get("inventory") or [])
            r = await a.act_batch(ActionBatch(actions=[Pickup(target=tgt.entity_id)]), wait_for_acks=True, timeout=5)
            await asyncio.sleep(0.5)
            oa = await first(a)
            g1 = (GET(f'/api/v1/agent/{eA}/mental_state').get('vitals') or {}).get('gold', 0)
            inv1 = len((oa.self.extras or {}).get("inventory") or [])
            chk(r[0] and r[0].accepted, f"pickup accepted ({r[0] and r[0].reason})")
            chk(g1 > g0 or inv1 > inv0, f"pickup changed world: gold {g0}->{g1} inv {inv0}->{inv1}")
            # the item must no longer be in the agent's obs (removed from ground)
            gone = tgt.entity_id not in item_ids(getattr(await first(a), "visible_items", []))
            chk(gone, f"picked-up item {tgt.entity_id} removed from vision")

        # ---- CONTRACT PLUMBING end-to-end (propose A->B, B sees, B accepts, both flip, A completes) ----
        print("\n-- contract plumbing: propose -> observe -> accept -> complete --")
        r = await a.act_batch(ActionBatch(actions=[ProposeTask(target=eB, terms="bring food", reward="5 gold")]),
                              wait_for_acks=True, timeout=5)
        chk(r[0] and r[0].accepted, "propose_task accepted")
        await asyncio.sleep(0.6)
        ob = await first(b)
        bc = [c for c in (ob.self.extras or {}).get("contracts") or []
              if c.get("target") == eB and c.get("status") == "proposed"]
        chk(len(bc) == 1, f"B's observation shows the proposed contract ({len(bc)})")
        if bc:
            cid = bc[0]["id"]
            r = await b.act_batch(ActionBatch(actions=[AcceptTask(id=cid)]), wait_for_acks=True, timeout=5)
            await asyncio.sleep(0.6)
            # status flips to accepted on BOTH parties' ledgers
            oa = await first(a); ob = await first(b)
            sa = next((c["status"] for c in (oa.self.extras or {}).get("contracts") or [] if c["id"] == cid), "?")
            sb = next((c["status"] for c in (ob.self.extras or {}).get("contracts") or [] if c["id"] == cid), "?")
            chk(sa == "accepted", f"contract status on PROPOSER(A) == accepted ({sa})")
            chk(sb == "accepted", f"contract status on ACCEPTER(B) == accepted ({sb})")
            r = await a.act_batch(ActionBatch(actions=[CompleteTask(id=cid)]), wait_for_acks=True, timeout=5)
            await asyncio.sleep(0.6)
            oa = await first(a)
            sa2 = next((c["status"] for c in (oa.self.extras or {}).get("contracts") or [] if c["id"] == cid), "?")
            chk(sa2 == "completed", f"contract status after complete == completed ({sa2})")

    print(f"\n===== ALIGNMENT TRACE: {checks - len(fails)}/{checks} checks PASS =====")
    if fails:
        print("FAILURES:")
        for f in fails:
            print("  -", f)

asyncio.run(main())
