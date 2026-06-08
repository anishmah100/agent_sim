#!/usr/bin/env python3
"""Audit harness — dump the live ground-truth inventory.

Pulls /api/v1/world/affordances from a running engine and prints the
authoritative verb / event / state-field / archetype inventory the rest of
the audit suite tests against. NO assumptions: this is whatever the engine
actually serves right now. See docs/ENVIRONMENT_AUDIT_PLAN.md §2.

Usage: python3 tools/audit/dump_inventory.py [http://127.0.0.1:8090]
"""
import json
import sys
import urllib.request


def fetch(engine: str) -> dict:
    with urllib.request.urlopen(engine + "/api/v1/world/affordances", timeout=5) as r:
        return json.loads(r.read().decode())


def inventory(aff: dict) -> dict:
    verbs, events, fields, archs = {}, set(), [], []
    for s in aff.get("systems", []):
        for v in s.get("verbs") or []:
            verbs[v["verb"]] = {
                "system": s["name"],
                "rejections": v.get("rejection_reasons") or [],
                "emits": v.get("emits_events") or [],
                "params_schema": v.get("params_schema") or {},
            }
            events.update(v.get("emits_events") or [])
        for sf in s.get("state_fields") or []:
            fields.append((s["name"], sf.get("key"), sf.get("owner"),
                           sf.get("public_at_any_distance")))
        for a in s.get("archetypes") or []:
            archs.append((s["name"], a.get("archetype")))
    return {"verbs": verbs, "events": sorted(events), "fields": fields,
            "archetypes": archs,
            "world": aff.get("world"), "scenario": aff.get("scenario")}


def main() -> None:
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    inv = inventory(fetch(engine))
    print(f"world={inv['world']} scenario={inv['scenario']}")
    print(f"{len(inv['verbs'])} verbs · {len(inv['events'])} events · "
          f"{len(inv['fields'])} state fields · {len(inv['archetypes'])} archetypes\n")
    for verb, meta in sorted(inv["verbs"].items()):
        print(f"  {verb:20s} [{meta['system']}] reject={meta['rejections']} "
              f"emits={meta['emits']}")
    print("\nEVENTS:", inv["events"])
    print("\nFIELDS:", inv["fields"])
    print("\nARCHETYPES:", inv["archetypes"])


if __name__ == "__main__":
    main()
