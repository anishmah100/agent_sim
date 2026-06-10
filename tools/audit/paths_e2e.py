#!/usr/bin/env python3
"""Audit harness S9 — happy-path CHAINS, live (the paths the verb matrix
only rejection-tests).

Each chain drives scripted SDK agents through a full multi-step flow and
asserts BOTH the acks and the engine-emitted events, closing the events-census
gap (16/23 declared events never observed in normal runs):

  C1 contract     propose -> accept -> complete   (+ reject path, + two
                  authorization NEGATIVE checks: proposer can't accept,
                  target can't complete)
  C2 transfer     pickup -> give -> drop          (ItemTransferred, ItemDropped)
  C3 pay          pay 5 gold                      (GoldTransferred)
  C4 survival     forage -> eat                   (ResourceHarvested, AteFood)
  C5 resources    chop tree to depletion          (ResourceHarvested, ResourceDepleted)
  C6 property     claim -> lock -> stranger enter REJECTED -> unlock -> enter
                  (OwnershipChanged, BuildingLocked, BuildingUnlocked)
  C7 construction chop wood -> place shed -> advance -> demolish
                  (ConstructionStarted/Advanced[/Completed], Demolished)
  C8 state-dependent verbs (buy_food, work_for_pay) ack sanely either way.

Usage (fresh sidecar engine, default :8090, event log at /tmp/doccap_events.jsonl):
    python3 tools/audit/paths_e2e.py [engine_url] [event_log]
"""
import asyncio
import json
import sys
import time
from pathlib import Path

from harness import connect

ENGINE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
EVLOG = Path(sys.argv[2] if len(sys.argv) > 2 else "/tmp/doccap_events.jsonl")

RESULTS = []  # (chain, status, detail)


def report(chain: str, ok, detail: str = ""):
    status = "PASS" if ok is True else ("SKIP" if ok is None else "FAIL")
    RESULTS.append((chain, status, detail))
    print(f"  [{status}] {chain}{' — ' + detail if detail else ''}")


class EvWatch:
    """Tail the engine event log and assert kinds appear (with payload pred)."""

    def __init__(self):
        self.start = EVLOG.stat().st_size if EVLOG.exists() else 0

    def mark(self):
        self.start = EVLOG.stat().st_size if EVLOG.exists() else 0

    async def expect(self, kind: str, pred=None, timeout: float = 8.0):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            with open(EVLOG) as f:
                f.seek(self.start)
                for line in f:
                    try:
                        e = json.loads(line)
                    except json.JSONDecodeError:
                        continue
                    if e.get("kind") == kind and (pred is None or pred(e.get("payload") or {})):
                        return e
            await asyncio.sleep(0.4)
        return None


async def fresh_obs(c):
    """Skip queued frames; return a current observation."""
    for _ in range(3):
        await c.observe()
    return c.obs


def visible_items(c, want=None):
    out = []
    for it in c.obs.get("visible_items", []):
        if want is None or want in (it.get("sprite") or ""):
            out.append(it)
    return out


def snapshot_items(want=None):
    """Item positions from the static world snapshot (fallback when none are
    in view at spawn)."""
    import urllib.request
    try:
        with urllib.request.urlopen(f"{ENGINE}/worlds/eldoria.json", timeout=5) as r:
            w = json.loads(r.read())
    except Exception:
        return []
    return [e for e in (w.get("entities") or [])
            if e.get("archetype") == "item"
            and (want is None or want in (e.get("sprite") or ""))]


async def acquire_item(c, want=None, max_tries=8):
    """Walk to + pick up a visible item (optionally sprite-substring match).
    Returns the picked item id or None."""
    for _ in range(max_tries):
        await fresh_obs(c)
        cands = visible_items(c, want)
        if not cands:
            # nothing in view: head for the nearest snapshot-declared item
            pos = tuple(c.obs["self"]["pos"])
            known = snapshot_items(want)
            if not known:
                return None
            known.sort(key=lambda e: max(abs(e["pos"][0] - pos[0]), abs(e["pos"][1] - pos[1])))
            if not await c.step_to(tuple(known[0]["pos"]), max_steps=400):
                return None
            continue
        pos = tuple(c.obs["self"]["pos"])
        cands.sort(key=lambda it: max(abs(it["pos"][0] - pos[0]), abs(it["pos"][1] - pos[1])))
        it = cands[0]
        if not await c.step_to(tuple(it["pos"])):
            continue
        ack = await c.act("pickup", target=it["entity_id"])
        if ack.get("accepted"):
            return it["entity_id"]
        if ack.get("reason") == "inventory_full":
            return None
    return None


def find_entities(c, archetype):
    return [e for e in c.obs.get("visible_entities", [])
            if e.get("archetype") == archetype]


async def walk_to_entity(c, archetype, max_scans=6):
    """Find nearest visible entity of archetype and walk adjacent to it."""
    for _ in range(max_scans):
        await fresh_obs(c)
        ents = find_entities(c, archetype)
        if not ents:
            return None
        pos = tuple(c.obs["self"]["pos"])
        ents.sort(key=lambda e: max(abs(e["pos"][0] - pos[0]), abs(e["pos"][1] - pos[1])))
        ent = ents[0]
        if await c.step_to(tuple(ent["pos"])):
            return ent
    return None


# ───────────────────────── chains ─────────────────────────

async def c1_contract(a, b, ev):
    aid = a.obs["self"]["entity_id"]
    bid = b.obs["self"]["entity_id"]

    ev.mark()
    ack = await a.act("propose_task", target=bid, terms="bring one apple", reward="5 gold")
    if not ack.get("accepted"):
        return report("C1 contract propose", False, f"rejected: {ack.get('reason')}")
    evt = await ev.expect("TaskProposed", lambda p: p.get("Proposer") == aid and p.get("Target") == bid)
    if not evt:
        return report("C1 contract propose", False, "no TaskProposed event")
    cid = (evt.get("payload") or {}).get("ID")
    report("C1 contract propose (TaskProposed)", True, f"id={cid}")

    # NEGATIVE: proposer may not accept own proposal
    ack = await a.act("accept_task", id=cid)
    report("C1 auth: proposer can't accept",
           ack.get("accepted") is False and ack.get("reason") == "not_authorized",
           f"reason={ack.get('reason')}")

    ev.mark()
    ack = await b.act("accept_task", id=cid)
    ok = bool(ack.get("accepted")) and (await ev.expect("TaskAccepted", lambda p: p.get("ID") == cid)) is not None
    report("C1 contract accept (TaskAccepted)", ok, f"ack={ack.get('accepted')}")

    # NEGATIVE: target may not complete
    ack = await b.act("complete_task", id=cid)
    report("C1 auth: target can't complete",
           ack.get("accepted") is False and ack.get("reason") == "not_authorized",
           f"reason={ack.get('reason')}")

    ev.mark()
    ack = await a.act("complete_task", id=cid)
    ok = bool(ack.get("accepted")) and (await ev.expect("TaskCompleted", lambda p: p.get("ID") == cid)) is not None
    report("C1 contract complete (TaskCompleted)", ok, f"ack={ack.get('accepted')}")

    # reject path on a second proposal
    ev.mark()
    ack = await a.act("propose_task", target=bid, terms="second deal", reward="1 gold")
    evt = await ev.expect("TaskProposed", lambda p: p.get("Proposer") == aid and p.get("ID") != cid)
    if not (ack.get("accepted") and evt):
        return report("C1 contract reject", False, "second proposal failed")
    cid2 = (evt.get("payload") or {}).get("ID")
    ev.mark()
    ack = await b.act("reject_task", id=cid2)
    ok = bool(ack.get("accepted")) and (await ev.expect("TaskRejected", lambda p: p.get("ID") == cid2)) is not None
    report("C1 contract reject (TaskRejected)", ok)


async def c2_transfer(a, b, ev):
    item = await acquire_item(a)
    if not item:
        return report("C2 give/drop", None, "no item acquirable near spawn")
    await fresh_obs(b)
    if not await a.step_to(tuple(b.obs["self"]["pos"])):
        return report("C2 give/drop", None, "could not reach partner")
    ev.mark()
    ack = await a.act("give", target=b.obs["self"]["entity_id"], item=item)
    ok = bool(ack.get("accepted")) and (await ev.expect("ItemTransferred")) is not None
    report("C2 give (ItemTransferred)", ok, f"ack={ack.get('accepted')} reason={ack.get('reason')}")
    if not ok:
        return
    ev.mark()
    ack = await b.act("drop", item=item)
    ok = bool(ack.get("accepted")) and (await ev.expect("ItemDropped")) is not None
    report("C2 drop (ItemDropped)", ok, f"ack={ack.get('accepted')} reason={ack.get('reason')}")


async def c3_pay(a, b, ev):
    await fresh_obs(b)
    if not await a.step_to(tuple(b.obs["self"]["pos"])):
        return report("C3 pay", None, "could not reach partner")
    ev.mark()
    ack = await a.act("pay", target=b.obs["self"]["entity_id"], amount=5)
    ok = bool(ack.get("accepted")) and (await ev.expect("GoldTransferred")) is not None
    report("C3 pay (GoldTransferred)", ok, f"ack={ack.get('accepted')} reason={ack.get('reason')}")


async def c4_survival(a, ev):
    node = await walk_to_entity(a, "tree") or await walk_to_entity(a, "bush")
    if not node:
        return report("C4 forage+eat", None, "no tree/bush visible")
    ev.mark()
    ack = await a.act("forage", target=node["entity_id"])
    if not ack.get("accepted"):
        return report("C4 forage", False, f"reason={ack.get('reason')}")
    ok = (await ev.expect("ResourceHarvested")) is not None
    report("C4 forage (ResourceHarvested)", ok)
    await fresh_obs(a)
    inv = (a.obs["self"].get("extras") or {}).get("inventory") or []
    food = next((i for i in inv if any(k in i for k in ("apple", "bread", "berry", "fish"))), None)
    if not food:
        return report("C4 eat", None, "forage yielded no recognizable food")
    ev.mark()
    ack = await a.act("eat", item=food)
    if ack.get("accepted"):
        ok = (await ev.expect("AteFood")) is not None
        report("C4 eat (AteFood)", ok)
    else:
        report("C4 eat (AteFood)", None if ack.get("reason") == "not_hungry" else False,
               f"reason={ack.get('reason')}")


async def c5_chop_deplete(a, ev):
    node = await walk_to_entity(a, "tree")
    if not node:
        return report("C5 chop->deplete", None, "no tree visible")
    ev.mark()
    harvested = depleted = False
    for _ in range(30):
        ack = await a.act("chop", target=node["entity_id"])
        if ack.get("accepted"):
            harvested = True
        elif ack.get("reason") in ("unknown_target", "inventory_full"):
            break  # tree gone (depleted+removed) or we can't carry more
    if harvested:
        harvested = (await ev.expect("ResourceHarvested")) is not None
        depleted = (await ev.expect("ResourceDepleted", timeout=4)) is not None
    report("C5 chop (ResourceHarvested)", harvested)
    report("C5 chop to depletion (ResourceDepleted)", depleted if harvested else None,
           "" if depleted else "tree survived 30 swings or inventory filled first")


async def c6_property(a, b, ev):
    bld = await walk_to_entity(a, "building")
    if not bld:
        return report("C6 property", None, "no building entity visible")
    tid = bld["entity_id"]
    ev.mark()
    ack = await a.act("claim_ownership", target=tid)
    if not ack.get("accepted"):
        return report("C6 claim", None if ack.get("reason") == "already_owned" else False,
                      f"reason={ack.get('reason')}")
    ok = (await ev.expect("OwnershipChanged")) is not None
    report("C6 claim (OwnershipChanged)", ok)

    ev.mark()
    ack = await a.act("lock", target=tid)
    ok = bool(ack.get("accepted")) and (await ev.expect("BuildingLocked")) is not None
    report("C6 lock (BuildingLocked)", ok, f"ack={ack.get('accepted')} reason={ack.get('reason')}")

    if await b.step_to(tuple(bld["pos"])):
        ack = await b.act("enter", target=tid)
        report("C6 locked blocks stranger",
               ack.get("accepted") is False and ack.get("reason") == "locked",
               f"reason={ack.get('reason')}")
    else:
        report("C6 locked blocks stranger", None, "stranger couldn't reach building")

    ev.mark()
    ack = await a.act("unlock", target=tid)
    ok = bool(ack.get("accepted")) and (await ev.expect("BuildingUnlocked")) is not None
    report("C6 unlock (BuildingUnlocked)", ok)

    ack = await b.act("enter", target=tid)
    if ack.get("accepted"):
        await fresh_obs(b)
        await b.act("exit")
        report("C6 unlocked admits stranger", True)
    else:
        report("C6 unlocked admits stranger", False, f"reason={ack.get('reason')}")


async def c7_construction(a, ev):
    # gather wood (shed: initial=[wood], advance=[wood] per stage)
    for _ in range(3):
        node = await walk_to_entity(a, "tree")
        if not node:
            break
        for _ in range(10):
            ack = await a.act("chop", target=node["entity_id"])
            if not ack.get("accepted"):
                break
        await fresh_obs(a)
        inv = (a.obs["self"].get("extras") or {}).get("inventory") or []
        if sum(1 for i in inv if "wood" in i) >= 4:
            break
    await fresh_obs(a)
    inv = (a.obs["self"].get("extras") or {}).get("inventory") or []
    if not any("wood" in i for i in inv):
        return report("C7 construction", None, "could not gather wood")

    pos = tuple(a.obs["self"]["pos"])
    lv = a.obs.get("local_view")
    site = None
    if lv:
        ox, oy = lv["origin"]; rows = lv["rows"]
        for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
            wx, wy = pos[0] + dx, pos[1] + dy
            col, row = wx - ox, wy - oy
            if 0 <= row < len(rows) and 0 <= col < len(rows[row]) and rows[row][col] == ".":
                site = [wx, wy]
                break
    if not site:
        return report("C7 construction", None, "no clear adjacent tile")

    ev.mark()
    ack = await a.act("place_blueprint", kind="shed", at=site)
    if not ack.get("accepted"):
        return report("C7 place shed", False, f"reason={ack.get('reason')}")
    evt = await ev.expect("ConstructionStarted")
    if not evt:
        return report("C7 place shed", False, "no ConstructionStarted event")
    bp = (evt.get("payload") or {}).get("Blueprint")
    report("C7 place shed (ConstructionStarted)", True, f"bp={bp}")

    advanced = completed = False
    for _ in range(8):
        ev.mark()
        ack = await a.act("advance_construction", target=bp)
        if ack.get("accepted"):
            if await ev.expect("ConstructionAdvanced", timeout=3):
                advanced = True
            if await ev.expect("ConstructionCompleted", timeout=2):
                completed = True
                break
        elif ack.get("reason") == "missing_materials":
            node = await walk_to_entity(a, "tree")
            if not node:
                break
            for _ in range(6):
                if not (await a.act("chop", target=node["entity_id"])).get("accepted"):
                    break
            if not await a.step_to(tuple(site)):
                break
        else:
            break
    report("C7 advance (ConstructionAdvanced)", advanced, f"completed={completed}")

    await fresh_obs(a)
    target = bp
    if completed:
        blds = [e for e in find_entities(a, "building")
                if max(abs(e["pos"][0] - site[0]), abs(e["pos"][1] - site[1])) <= 2]
        if blds:
            target = blds[0]["entity_id"]
    ev.mark()
    ack = await a.act("demolish", target=target)
    ok = bool(ack.get("accepted")) and (await ev.expect("Demolished")) is not None
    report("C7 demolish (Demolished)", ok, f"ack={ack.get('accepted')} reason={ack.get('reason')}")


async def c8_state_dependent(a):
    for verb in ("buy_food", "work_for_pay"):
        ack = await a.act(verb)
        ok = bool(ack.get("accepted")) or (ack.get("reason") not in (None, "__no_ack__"))
        report(f"C8 {verb} acks sanely", ok, f"accepted={ack.get('accepted')} reason={ack.get('reason')}")


async def main():
    print(f"paths_e2e against {ENGINE} (events: {EVLOG})")
    ev = EvWatch()
    a = await connect(ENGINE, name="PathsA", cadence_ms=150)
    b = await connect(ENGINE, name="PathsB", cadence_ms=150)
    await fresh_obs(a); await fresh_obs(b)

    chains = [
        ("C1", c1_contract(a, b, ev)),
        ("C2", c2_transfer(a, b, ev)),
        ("C3", c3_pay(a, b, ev)),
        ("C4", c4_survival(a, ev)),
        ("C5", c5_chop_deplete(a, ev)),
        ("C6", c6_property(a, b, ev)),
        ("C7", c7_construction(a, ev)),
        ("C8", c8_state_dependent(a)),
    ]
    for name, coro in chains:
        print(f"\n── {name} ──")
        try:
            await coro
        except Exception as e:  # a crashed chain is a finding, not a suite abort
            report(f"{name} (crashed)", False, f"{type(e).__name__}: {e}")

    print("\n" + "=" * 60)
    fails = [r for r in RESULTS if r[1] == "FAIL"]
    skips = [r for r in RESULTS if r[1] == "SKIP"]
    print(f"PATHS E2E: {len(RESULTS) - len(fails) - len(skips)} pass, "
          f"{len(skips)} skip, {len(fails)} FAIL")
    for c, s, d in fails:
        print(f"  FAIL {c}: {d}")
    sys.exit(1 if fails else 0)


if __name__ == "__main__":
    asyncio.run(main())
