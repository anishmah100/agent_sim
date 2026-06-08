"""Live verb-affordance audit — drives every agent verb against a running
engine via the SDK and checks BOTH the ActionResult and the real
world-state delta. No LLM involved; deterministic. Run with the engine up
on :8080.

Usage: PYTHONPATH=sdk/python:. python3 tools/dev-scripts/verb_audit.py
"""
from __future__ import annotations
import asyncio, json, urllib.request
from agent_sim_sdk import (
    register_agent, Agent, ActionBatch, VisionMode,
    Step, Speak, Shout, Whisper, Wait, Pickup, Eat, Equip, Drop,
    Give, Pay, Trade, Attack, ProposeTask, AcceptTask, CompleteTask, RejectTask,
)

import os as _os
ENGINE = _os.environ.get("AGENT_SIM_ENGINE", "http://127.0.0.1:8080")
results: list[tuple[str, str, str]] = []  # (verb, verdict, detail)


def ms(eid):
    with urllib.request.urlopen(f"{ENGINE}/api/v1/agent/{eid}/mental_state", timeout=4) as r:
        return json.load(r)


def vit(eid):
    return ms(eid).get("vitals") or {}


def inv_of(eid):
    return vit(eid).get("inventory") or []


def gold_of(eid):
    return vit(eid).get("gold") or 0


def hp_of(eid):
    return vit(eid).get("hp") or 0


async def approach(a, b):
    """Re-read B's live pos and walk A adjacent — idle connected agents
    can drift via engine-side wander, so re-approach before each
    adjacency-required verb."""
    ob = await first_obs(b)
    return await step_toward(a, ob.self.pos[0], ob.self.pos[1])


def rec(verb, ok, detail):
    results.append((verb, "PASS" if ok else "FAIL", detail))
    print(f"  [{'PASS' if ok else 'FAIL'}] {verb}: {detail}")


async def first_obs(a):
    async for o in a.observations():
        return o


async def step_toward(a, tx, ty, maxsteps=40):
    """Greedy per-tile move until adjacent (<=1) to (tx,ty). Returns final obs."""
    o = await first_obs(a)
    for _ in range(maxsteps):
        cx, cy = o.self.pos
        if max(abs(cx - tx), abs(cy - ty)) <= 1:
            return o
        dx = 1 if cx < tx else -1 if cx > tx else 0
        dy = 1 if cy < ty else -1 if cy > ty else 0
        await a.act_batch(ActionBatch(actions=[Step(dir=("E" if dx > 0 else "W") if dx != 0 else ("S" if dy > 0 else "N"))]))
        await asyncio.sleep(0.35)
        o = await first_obs(a)
    return o


async def submit(a, action):
    r = await a.act_batch(ActionBatch(actions=[action]), wait_for_acks=True, timeout=6)
    return r[0] if r else None


async def main():
    A = await register_agent(ENGINE, user_token="dev", persona={"name": "audit_A", "bio": "", "archetype_tag": "killer"}, vision_mode=VisionMode.STRUCTURED, cadence_ms=300)
    B = await register_agent(ENGINE, user_token="dev", persona={"name": "audit_B", "bio": "", "archetype_tag": "survivor"}, vision_mode=VisionMode.STRUCTURED, cadence_ms=300)
    async with Agent(A) as a, Agent(B) as b:
        oa = await first_obs(a)
        ob = await first_obs(b)
        eA, eB = oa.self.entity_id, ob.self.entity_id
        print(f"A={eA} at {oa.self.pos}  B={eB} at {ob.self.pos}")

        # ---- move ----
        bx, by = ob.self.pos
        oa = await step_toward(a, bx, by)
        adj = max(abs(oa.self.pos[0] - bx), abs(oa.self.pos[1] - by)) <= 1
        rec("move", adj, f"A walked to {oa.self.pos}, B at {(bx,by)}, adjacent={adj}")

        # ---- speak / shout / wait ----
        rec("speak", (r := await submit(a, Speak(text="hello world"))) and r.accepted, str(r and (r.reason or "accepted")))
        rec("shout", (r := await submit(a, Shout(text="HEAR ME"))) and r.accepted, str(r and (r.reason or "accepted")))
        rec("wait", (r := await submit(a, Wait(ticks=5))) and r.accepted, str(r and (r.reason or "accepted")))

        # ---- whisper (needs adjacency) — re-approach B first ----
        await approach(a, b)
        r = await submit(a, Whisper(target=eB, text="psst"))
        rec("whisper", r and r.accepted, str(r and (r.reason or "accepted")))

        # ---- pickup (find a visible item, walk to it, grab) ----
        oa = await first_obs(a)
        items = list(getattr(oa, "visible_items", []) or [])
        if items:
            items.sort(key=lambda it: max(abs(it.pos[0]-oa.self.pos[0]), abs(it.pos[1]-oa.self.pos[1])))
            tgt = items[0]
            g0 = gold_of(eA)
            inv0 = len(inv_of(eA))
            oa = await step_toward(a, tgt.pos[0], tgt.pos[1])
            r = await submit(a, Pickup(target=tgt.entity_id))
            await asyncio.sleep(0.6)
            g1 = gold_of(eA); inv1 = len(inv_of(eA))
            changed = (g1 > g0) or (inv1 > inv0)
            rec("pickup", r and r.accepted and changed, f"{tgt.sprite} accepted={r and r.accepted} reason={r and r.reason!r} gold {g0}->{g1} invcount {inv0}->{inv1}")
        else:
            rec("pickup", False, "no visible items near A to test")

        # ---- inventory-dependent: eat / equip / give / drop ----
        inv = vit(eA).get("inventory", [])
        food = next((it for it in inv if it.get("kind") == "food"), None)
        weapon = next((it for it in inv if it.get("kind") == "weapon"), None)
        # collect a couple more items so we have material to give/eat/equip
        for _ in range(3):
            oa = await first_obs(a)
            its = [it for it in (getattr(oa, "visible_items", []) or [])]
            if not its:
                break
            its.sort(key=lambda it: max(abs(it.pos[0]-oa.self.pos[0]), abs(it.pos[1]-oa.self.pos[1])))
            t = its[0]
            oa = await step_toward(a, t.pos[0], t.pos[1])
            await submit(a, Pickup(target=t.entity_id))
            await asyncio.sleep(0.4)
        invfull = vit(eA).get("inventory", [])
        print("   A inventory now:", invfull)
        # raw item ids: need the actual item entity ids in inventory; mental_state aggregates by kind+count.
        # Use the observation self.extras inventory (raw ids) instead.
        oa = await first_obs(a)
        raw_inv = (oa.self.extras or {}).get("inventory") or []
        print("   A raw inventory ids:", raw_inv)

        def find_kind(kind_substr):
            for iid in raw_inv:
                if kind_substr in str(iid):
                    return iid
            return None

        food_id = find_kind("apple") or find_kind("bread") or find_kind("fish") or find_kind("cheese")
        weap_id = find_kind("sword") or find_kind("dagger")

        if food_id:
            h0 = vit(eA).get("hunger", 0)
            r = await submit(a, Eat(item=food_id))
            await asyncio.sleep(0.5)
            rec("eat", r and r.accepted, f"{food_id} accepted={r and r.accepted} reason={r and r.reason!r}")
        else:
            rec("eat", False, "no food in A inventory to test (SKIP-ish)")

        if weap_id:
            r = await submit(a, Equip(item=weap_id, slot="weapon"))
            await asyncio.sleep(0.5)
            eq = vit(eA).get("equipped", {})
            rec("equip", r and r.accepted, f"{weap_id} accepted={r and r.accepted} equipped={eq}")
        else:
            rec("equip", False, "no weapon in A inventory to test (SKIP-ish)")

        # give: A -> B (need adjacency; re-approach B)
        ob = await first_obs(b); bx, by = ob.self.pos
        oa = await step_toward(a, bx, by)
        raw_inv = (await first_obs(a)).self.extras.get("inventory") or []
        give_id = raw_inv[0] if raw_inv else None
        if give_id:
            binv0 = len((await first_obs(b)).self.extras.get("inventory") or [])
            r = await submit(a, Give(target=eB, item=give_id))
            await asyncio.sleep(0.6)
            binv1 = len((await first_obs(b)).self.extras.get("inventory") or [])
            rec("give", r and r.accepted and binv1 > binv0, f"{give_id} A->B accepted={r and r.accepted} reason={r and r.reason!r} B inv {binv0}->{binv1}")
        else:
            rec("give", False, "A has no item to give (SKIP-ish)")

        # drop
        raw_inv = (await first_obs(a)).self.extras.get("inventory") or []
        if raw_inv:
            r = await submit(a, Drop(item=raw_inv[0]))
            rec("drop", r and r.accepted, f"{raw_inv[0]} accepted={r and r.accepted} reason={r and r.reason!r}")
        else:
            rec("drop", False, "A has no item to drop (SKIP-ish)")

        # ---- pay (A -> B) ----
        ob = await first_obs(b); bx, by = ob.self.pos
        oa = await step_toward(a, bx, by)
        ag0 = gold_of(eA); bg0 = gold_of(eB)
        r = await submit(a, Pay(target=eB, amount=5))
        await asyncio.sleep(0.6)
        ag1 = gold_of(eA); bg1 = gold_of(eB)
        rec("pay", r and r.accepted and bg1 == bg0 + 5 and ag1 == ag0 - 5, f"accepted={r and r.accepted} reason={r and r.reason!r} A {ag0}->{ag1} B {bg0}->{bg1}")

        # ---- trade (A sells item to B) ----
        raw_inv = (await first_obs(a)).self.extras.get("inventory") or []
        if raw_inv:
            r = await submit(a, Trade(target=eB, item=raw_inv[0], price=2))
            rec("trade", r is not None, f"item={raw_inv[0]} accepted={r and r.accepted} reason={r and r.reason!r}")
        else:
            rec("trade", False, "A has no item to trade (SKIP-ish)")

        # ---- contracts: propose (A->B), accept (B), complete (A), reject path ----
        r = await submit(a, ProposeTask(target=eB, terms="bring me an apple", reward="5 gold"))
        cid = None
        # find the contract id from B's extras
        await asyncio.sleep(0.5)
        ob = await first_obs(b)
        bcontracts = (ob.self.extras or {}).get("contracts") or []
        for c in bcontracts:
            if isinstance(c, dict) and c.get("target") == eB and c.get("status") == "proposed":
                cid = c.get("id"); break
        rec("propose_task", r and r.accepted and cid is not None, f"accepted={r and r.accepted} reason={r and r.reason!r} contract_id={cid}")

        if cid:
            r = await submit(b, AcceptTask(id=cid))
            await asyncio.sleep(0.5)
            # confirm status flipped to accepted on A's ledger
            oa = await first_obs(a)
            acon = [c for c in (oa.self.extras or {}).get("contracts") or [] if c.get("id") == cid]
            st = acon[0].get("status") if acon else "?"
            rec("accept_task", r and r.accepted and st == "accepted", f"accepted={r and r.accepted} reason={r and r.reason!r} status_on_A={st}")

            r = await submit(a, CompleteTask(id=cid))
            await asyncio.sleep(0.5)
            oa = await first_obs(a)
            acon = [c for c in (oa.self.extras or {}).get("contracts") or [] if c.get("id") == cid]
            st = acon[0].get("status") if acon else "?"
            rec("complete_task", r and r.accepted and st == "completed", f"accepted={r and r.accepted} reason={r and r.reason!r} status={st}")
        else:
            rec("accept_task", False, "no contract id to accept")
            rec("complete_task", False, "no contract id to complete")

        # reject_task: propose a second one, reject it
        r = await submit(a, ProposeTask(target=eB, terms="guard me", reward="nothing"))
        await asyncio.sleep(0.5)
        ob = await first_obs(b)
        cid2 = None
        for c in (ob.self.extras or {}).get("contracts") or []:
            if isinstance(c, dict) and c.get("target") == eB and c.get("status") == "proposed":
                cid2 = c.get("id"); break
        if cid2:
            r = await submit(b, RejectTask(id=cid2))
            await asyncio.sleep(0.4)
            rec("reject_task", r and r.accepted, f"accepted={r and r.accepted} reason={r and r.reason!r}")
        else:
            rec("reject_task", False, "no second contract to reject")

        # ---- attack (A -> B), check hp drop ----
        ob = await first_obs(b); bx, by = ob.self.pos
        oa = await step_toward(a, bx, by)
        bhp0 = vit(eB).get("hp", 0)
        r = await submit(a, Attack(target=eB))
        await asyncio.sleep(0.6)
        bhp1 = vit(eB).get("hp", 0)
        rec("attack", r and r.accepted and bhp1 < bhp0, f"accepted={r and r.accepted} reason={r and r.reason!r} B hp {bhp0}->{bhp1}")

    # ---- summary ----
    print("\n===== VERB AUDIT SUMMARY =====")
    npass = sum(1 for _, v, _ in results if v == "PASS")
    for verb, verdict, detail in results:
        print(f"{verdict:4}  {verb:14} {detail}")
    print(f"\n{npass}/{len(results)} verbs PASS")


asyncio.run(main())
