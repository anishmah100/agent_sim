"""Deterministic substrate exercise.

Drives the harness against a LIVE engine by hand — no LLM in the
loop — so we can isolate substrate failures from model failures.

For each affordance the A9 scorer cares about, we:
  1. Walk the agent to where the affordance is meaningful.
  2. Fire the typed SDK Action(s) via agent.act_batch(...).
  3. Tail the engine's events.jsonl for the expected historian event.

Result is printed as a PASS/FAIL table. If this script PASSES the
substrate is sound and any remaining Qwen smoke failures are model
or prompt issues. If it FAILS we know the engine/harness still has
holes.

Run BEFORE/ALONGSIDE a Qwen smoke. The engine is shared between
this script and the smoke as long as you point them at the same
--server.
"""

from __future__ import annotations

import argparse
import asyncio
import json
import time
from pathlib import Path
from typing import Optional

from agent_sim_sdk import (
    Agent, ActionBatch, VisionMode, register_agent,
    Move, Speak, Shout, Whisper, Interact, Wait,
    WorkForPay, Pay, Give,
)


# Helpers ---------------------------------------------------------------

def tail_for_kind(events_path: Path, kind: str, after_seq: int,
                  timeout_s: float = 30.0) -> Optional[dict]:
    """Block until a Record with .kind == kind and .seq > after_seq
    appears in events.jsonl, or timeout. Returns the matching record
    dict, or None on timeout."""
    deadline = time.monotonic() + timeout_s
    last_size = 0
    while time.monotonic() < deadline:
        if events_path.exists():
            sz = events_path.stat().st_size
            if sz > last_size:
                # Re-read entire file (small enough for smoke window).
                for line in events_path.read_text().splitlines():
                    try:
                        rec = json.loads(line)
                    except Exception:
                        continue
                    if rec.get("seq", -1) <= after_seq:
                        continue
                    if rec.get("kind") == kind:
                        return rec
                last_size = sz
        time.sleep(0.2)
    return None


def cur_max_seq(events_path: Path) -> int:
    if not events_path.exists():
        return -1
    last = -1
    for line in events_path.read_text().splitlines():
        try:
            rec = json.loads(line)
        except Exception:
            continue
        s = rec.get("seq", -1)
        if s > last:
            last = s
    return last


def step_toward(a, b, n: int = 1) -> tuple[int, int]:
    dx = max(-n, min(n, b[0] - a[0]))
    dy = max(-n, min(n, b[1] - a[1]))
    return (a[0] + dx, a[1] + dy)


# Driver ----------------------------------------------------------------

async def main_async(args) -> None:
    events_path = Path(args.events).resolve()
    print(f"==> exercising substrate against {args.server}")
    print(f"    tailing events at {events_path}")

    creds = await register_agent(
        args.server,
        user_token=args.token,
        persona={"name": "substrate_tester", "archetype": "tester",
                 "bio": "Deterministic affordance exerciser."},
        vision_mode=VisionMode.STRUCTURED,
        share_reasoning=True,
    )
    agent = Agent(creds)
    await agent.connect()

    # Collect the first observation so we know where we are.
    obs = await asyncio.wait_for(agent._inbox.get(), timeout=10.0)
    pos = obs.self.pos
    print(f"==> bound to entity={obs.self.entity_id} pos={pos}")

    results: list[tuple[str, bool, str]] = []

    def record(name: str, ok: bool, detail: str = ""):
        mark = "PASS" if ok else "FAIL"
        results.append((name, ok, detail))
        print(f"    [{mark}] {name}{(' — ' + detail) if detail else ''}")

    # 1) speak -> Speech event
    seq0 = cur_max_seq(events_path)
    await agent.act_batch(ActionBatch(
        actions=[Speak(text="hello from substrate tester")],
        reasoning="exercise: speak",
    ))
    rec = tail_for_kind(events_path, "Speech", seq0, timeout_s=15.0)
    record("speak -> Speech",
           ok=(rec is not None),
           detail=("seq=" + str(rec.get("seq")) if rec else "no event seen"))

    # 2) shout -> Speech event
    seq0 = cur_max_seq(events_path)
    await agent.act_batch(ActionBatch(
        actions=[Shout(text="OI EVERYONE")],
        reasoning="exercise: shout",
    ))
    rec = tail_for_kind(events_path, "Speech", seq0, timeout_s=15.0)
    record("shout -> Speech", ok=(rec is not None),
           detail=("seq=" + str(rec.get("seq")) if rec else "no event seen"))

    # 3) interact-affordance enter+exit -> EnteredBuilding + ExitedBuilding
    # Find a nearby door from the latest observation.
    obs = await asyncio.wait_for(agent._inbox.get(), timeout=10.0)
    door = next((o for o in (obs.visible_objects or [])
                 if o.kind == "door" and "enter" in (o.affordances or [])),
                None)
    if door is None:
        record("enter -> EnteredBuilding", ok=False,
               detail="no door in vision radius, skipping")
        record("exit -> ExitedBuilding", ok=False,
               detail="depends on enter, skipping")
    else:
        # Walk one step at a time toward the door until adjacent.
        cur = obs.self.pos
        for _ in range(20):
            if max(abs(cur[0] - door.pos[0]), abs(cur[1] - door.pos[1])) <= 1:
                break
            tgt = step_toward(cur, door.pos)
            await agent.act_batch(ActionBatch(actions=[Move(target=tgt)],
                                              reasoning="walk to door"))
            try:
                obs = await asyncio.wait_for(agent._inbox.get(), timeout=5.0)
                cur = obs.self.pos
            except asyncio.TimeoutError:
                break
        bld = door.object_id
        if bld.startswith("door:"):
            bld = bld[len("door:"):]
        seq0 = cur_max_seq(events_path)
        await agent.act_batch(ActionBatch(
            actions=[Interact(target=bld, affordance="enter")],
            reasoning="exercise: enter via interact",
        ))
        rec = tail_for_kind(events_path, "EnteredBuilding", seq0, timeout_s=15.0)
        record("interact-enter -> EnteredBuilding", ok=(rec is not None),
               detail=("seq=" + str(rec.get("seq")) if rec else "no event seen"))
        if rec is not None:
            seq0 = cur_max_seq(events_path)
            await agent.act_batch(ActionBatch(
                actions=[Interact(target=bld, affordance="exit")],
                reasoning="exercise: exit via interact",
            ))
            rec2 = tail_for_kind(events_path, "ExitedBuilding", seq0, timeout_s=15.0)
            record("interact-exit -> ExitedBuilding", ok=(rec2 is not None),
                   detail=("seq=" + str(rec2.get("seq")) if rec2 else "no event seen"))
        else:
            record("interact-exit -> ExitedBuilding", ok=False,
                   detail="enter failed, can't exercise exit")

    # 4) ActionAccepted breadcrumb (move).
    seq0 = cur_max_seq(events_path)
    obs = await asyncio.wait_for(agent._inbox.get(), timeout=10.0)
    tgt = (obs.self.pos[0], obs.self.pos[1] + 1)
    await agent.act_batch(ActionBatch(actions=[Move(target=tgt)],
                                      reasoning="exercise: move"))
    rec = tail_for_kind(events_path, "ActionAccepted", seq0, timeout_s=15.0)
    record("move -> ActionAccepted", ok=(rec is not None),
           detail=("seq=" + str(rec.get("seq")) if rec else "no event seen"))

    # 5) work_for_pay -> GoldTransferred (Grant variant).
    # Money system credits the worker and queues GoldTransferred with
    # From="" + Cause="work_for_pay". This is the cleanest way to
    # prove the economy verbs work without needing a counter-party.
    seq0 = cur_max_seq(events_path)
    await agent.act_batch(ActionBatch(actions=[WorkForPay()],
                                      reasoning="exercise: work_for_pay"))
    rec = tail_for_kind(events_path, "GoldTransferred", seq0, timeout_s=15.0)
    record("work_for_pay -> GoldTransferred (Grant)",
           ok=(rec is not None),
           detail=("seq=" + str(rec.get("seq")) if rec
                   else "no event — money system not firing Grant"))

    # 6) pay -> GoldTransferred (Pay variant).
    # Requires an adjacent target and self balance ≥ amount. After
    # work_for_pay above, balance should be > 0. If no entity is
    # already within 1 tile, walk toward the closest visible one for
    # up to 20 steps and then pay them.
    try:
        obs = await asyncio.wait_for(agent._inbox.get(), timeout=5.0)
    except asyncio.TimeoutError:
        obs = None
    adjacent = None
    if obs is not None:
        my_pos = obs.self.pos
        for ent in (obs.visible_entities or []):
            d = max(abs(ent.pos[0] - my_pos[0]),
                    abs(ent.pos[1] - my_pos[1]))
            if d <= 1:
                adjacent = ent
                break
        if adjacent is None and obs.visible_entities:
            # Walk to nearest visible entity.
            nearest = min(obs.visible_entities,
                          key=lambda e: max(abs(e.pos[0] - my_pos[0]),
                                            abs(e.pos[1] - my_pos[1])))
            for _ in range(20):
                d = max(abs(nearest.pos[0] - my_pos[0]),
                        abs(nearest.pos[1] - my_pos[1]))
                if d <= 1:
                    adjacent = nearest
                    break
                step = step_toward(my_pos, nearest.pos)
                await agent.act_batch(ActionBatch(
                    actions=[Move(target=step)],
                    reasoning="walk to entity to pay",
                ))
                try:
                    obs = await asyncio.wait_for(agent._inbox.get(), timeout=5.0)
                    my_pos = obs.self.pos
                    # Refresh nearest position; the entity may have moved.
                    nearest = next((e for e in (obs.visible_entities or [])
                                    if e.entity_id == nearest.entity_id),
                                   nearest)
                except asyncio.TimeoutError:
                    break
    if adjacent is None:
        record("pay -> GoldTransferred (Pay)", ok=False,
               detail="no entity within 1 tile to pay")
    else:
        seq0 = cur_max_seq(events_path)
        await agent.act_batch(ActionBatch(
            actions=[Pay(target=adjacent.entity_id, amount=1)],
            reasoning="exercise: pay 1 gold to adjacent",
        ))
        rec = tail_for_kind(events_path, "GoldTransferred", seq0, timeout_s=15.0)
        # We accept either "GoldTransferred" with non-empty From OR
        # rejection (logged via engine, not on the event stream). To
        # disambiguate, check the latest action_ack: tail won't see
        # rejections, so a missing event here means the pay was
        # silently dropped at dispatch — which would be a substrate
        # bug.
        record("pay -> GoldTransferred (Pay)",
               ok=(rec is not None),
               detail=("seq=" + str(rec.get("seq")) if rec
                       else "pay didn't produce GoldTransferred — check "
                            "adjacency / balance / verb routing"))

    # Teardown.
    await agent.close()

    # Summary.
    print()
    n_pass = sum(1 for _, ok, _ in results if ok)
    print(f"==> SUBSTRATE EXERCISE: {n_pass}/{len(results)} passed")
    for name, ok, detail in results:
        print(f"   {'✓' if ok else '✗'} {name}{(' — ' + detail) if detail else ''}")
    if n_pass == len(results):
        print()
        print("If this is GREEN, the engine + harness substrate is sound. "
              "Any remaining Qwen smoke gaps are model/prompt issues.")


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--server", default="http://127.0.0.1:8088")
    p.add_argument("--token", default="dev")
    p.add_argument("--events", required=True,
                   help="Path to engine's events.jsonl (the file the engine "
                        "writes via -event-log).")
    args = p.parse_args()
    asyncio.run(main_async(args))


if __name__ == "__main__":
    main()
