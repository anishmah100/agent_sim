#!/usr/bin/env python3
"""Phase 0.3 — structural INVARIANTS over the engine tape.

Conservation / referential-integrity laws that must hold over any run. Like
the referee, a stream-pure offline consumer of the tape — runs on a live tail
or a recorded run, retroactively over any past run. Complements the referee
(which checks perception fidelity); this checks world-state consistency.

  I1 enter/exit pairing — no entity exits a building it never entered; report
     the net still-inside count (should match entities alive inside at end).
  I2 gold-transfer integrity — every GoldTransferred has a POSITIVE amount;
     a PEER transfer (no source/sink Cause) names both From and To. (Cause-
     tagged source/sinks like pickup_coin/work_for_pay/buy_food legitimately
     leave From or To empty — gold minted from / paid to the world.)
  I3 no post-mortem activity — an entity that EntityDied must not act again
     (speak/attack/move/transfer). A "ghost" acting after death is a husk /
     id-reuse corruption (entity ids are not reused).

Usage: python3 tools/audit/invariants.py <tape.jsonl>
Exit 0 = clean; 1 = invariant violations.
"""
import json
import sys
from collections import defaultdict


def load(path):
    rows = []
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                rows.append(json.loads(line))
            except json.JSONDecodeError:
                pass
    return rows


# Which payload field names name the ACTOR for each event kind.
ACTOR_FIELDS = {
    "Speech": "Speaker", "Whisper": "Speaker", "Shout": "Speaker",
    "DamageDealt": "Killer", "EntityMoved": "EntityID",
    "ActionAccepted": "EntityID", "GoldTransferred": "From",
    "ItemPicked": "Picker", "EnteredBuilding": "Builder",
}


def actor_of(r):
    p = r.get("payload") or {}
    f = ACTOR_FIELDS.get(r.get("kind"))
    if f and p.get(f):
        return p.get(f)
    # generic fallbacks
    for k in ("EntityID", "entity_id", "Builder", "Speaker"):
        if p.get(k):
            return p.get(k)
    return None


def run(path):
    rows = load(path)
    violations = []

    # I1 — enter/exit pairing.
    inside = defaultdict(int)  # entity -> (#enter - #exit)
    i1_events = 0
    for r in rows:
        p = r.get("payload") or {}
        ent = p.get("Builder") or p.get("EntityID") or p.get("entity_id")
        if r.get("kind") == "EnteredBuilding" and ent:
            inside[ent] += 1; i1_events += 1
        elif r.get("kind") == "ExitedBuilding" and ent:
            inside[ent] -= 1; i1_events += 1
            if inside[ent] < 0:
                violations.append(f"I1 exit-without-enter: {ent} has more ExitedBuilding than EnteredBuilding")
    still_inside = sum(1 for v in inside.values() if v > 0)

    # I2 — gold-transfer integrity.
    i2_checked = 0
    for r in rows:
        if r.get("kind") != "GoldTransferred":
            continue
        i2_checked += 1
        p = r.get("payload") or {}
        amt = p.get("Amount")
        cause = p.get("Cause") or ""
        if not isinstance(amt, (int, float)) or amt <= 0:
            violations.append(f"I2 non-positive gold transfer: {p}")
        elif not cause and (not p.get("From") or not p.get("To")):
            # A PEER transfer (no source/sink cause) must name both parties.
            violations.append(f"I2 malformed peer GoldTransferred (missing From/To): {p}")

    # I3 — no post-mortem activity.
    death_tick = {}   # entity -> tick it died
    for r in rows:
        if r.get("kind") == "EntityDied":
            p = r.get("payload") or {}
            ent = p.get("EntityID") or p.get("entity_id")
            if ent is not None:
                death_tick[ent] = min(death_tick.get(ent, 1 << 62), int(r.get("tick", 0)))
    i3_checked = 0
    for r in rows:
        kind = r.get("kind")
        if kind in ("EntityDied", "PerceptionDelivered", "Spawned"):
            continue
        actor = actor_of(r)
        if actor in death_tick:
            i3_checked += 1
            if int(r.get("tick", 0)) > death_tick[actor] + 1:  # +1 tick grace for same-tick ordering
                violations.append(
                    f"I3 post-mortem activity: {actor} emitted {kind} at tick {r.get('tick')} "
                    f"but died at tick {death_tick[actor]}")

    print(f"INVARIANTS over {path}  ({len(rows)} records)")
    print(f"  I1 enter/exit: {i1_events} events · {still_inside} entities still inside at end")
    print(f"  I2 gold-transfer integrity: {i2_checked} transfers checked")
    print(f"  I3 post-mortem: {len(death_tick)} deaths · {i3_checked} post-death actor refs checked")
    if violations:
        print(f"\n  {len(violations)} INVARIANT VIOLATION(S):")
        for v in violations[:40]:
            print(f"    - {v}")
        return 1
    print("\n  CLEAN — all invariants hold.")
    return 0


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("usage: invariants.py <tape.jsonl>", file=sys.stderr)
        sys.exit(2)
    sys.exit(run(sys.argv[1]))
