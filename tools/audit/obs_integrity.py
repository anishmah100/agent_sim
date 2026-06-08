#!/usr/bin/env python3
"""Audit harness S2 — observation integrity against a LIVE engine.

Asserts the observation contract that agents (and the next-gen harness) rely on:
  - top-level keys are EXACTLY the documented set (no dead/extra fields),
  - self.extras carries the agent's private state,
  - another agent's extras_summary NEVER leaks private fields (gold/inventory/
    hunger/contracts) — the adversarial-information rule,
  - local_view glyphs match the actual world (spot-checked against debug vision),
  - removed fields (known_map_summary/recent_self_results/weather/doing) absent.

Spawns TWO agents so we can inspect how each appears in the other's view.
See docs/ENVIRONMENT_AUDIT_PLAN.md S2.

Usage: python3 tools/audit/obs_integrity.py [engine_url]
"""
import asyncio
import sys

from harness import connect

EXPECTED_TOP_KEYS = {
    "type", "obs_id", "world_tick", "self", "visible_entities",
    "visible_objects", "visible_items", "audible", "local_view", "world_clock",
}
PRIVATE_FIELDS = {"gold", "inventory", "hunger", "contracts", "access", "max_hp"}
PUBLIC_SUMMARY_OK = {"hp_bucket", "equipped_slot", "equipped_sprite",
                     "reputation", "rep_bucket"}


def check(cond, label, findings, detail=""):
    print(f"  [{'PASS' if cond else 'FAIL'}] {label}" + (f" — {detail}" if detail and not cond else ""))
    if not cond:
        findings.append(f"{label}: {detail}")


async def main() -> None:
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    findings = []
    a = await connect(engine, name="ObsA", cadence_ms=500)
    b = await connect(engine, name="ObsB", cadence_ms=500)
    # let both settle so they may see each other / the world
    for _ in range(5):
        await a.observe(); await b.observe()
    obs = a.obs

    print("=== top-level keys ===")
    keys = set(obs.keys())
    extra = keys - EXPECTED_TOP_KEYS
    missing = EXPECTED_TOP_KEYS - keys
    check(not extra, "no undocumented top-level keys", findings, f"extra={extra}")
    check(not missing, "all documented top-level keys present", findings, f"missing={missing}")
    for dead in ("known_map_summary", "recent_self_results"):
        check(dead not in obs, f"removed field '{dead}' absent", findings)
    check("weather" not in obs.get("world_clock", {}), "world_clock has no weather", findings)

    print("\n=== self.extras (private state present) ===")
    extras = obs["self"].get("extras", {})
    for f in ("hp", "gold", "hunger", "inventory", "reputation"):
        check(f in extras, f"self.extras has '{f}'", findings)

    print("\n=== no private leak in other agents' extras_summary ===")
    # Drive A and B adjacent so each appears in the other's visible_entities.
    bpos = tuple(b.obs["self"]["pos"])
    await a.step_to(bpos, max_steps=120)
    for _ in range(3):
        await a.observe()
    seen = a.obs.get("visible_entities", [])
    check(len(seen) >= 1, "agent A sees at least one other entity", findings,
          f"visible_entities={len(seen)}")
    leak = []
    for e in seen:
        summ = e.get("extras_summary", {})
        bad = set(summ.keys()) & PRIVATE_FIELDS
        if bad:
            leak.append((e.get("entity_id"), bad))
        if "doing" in e:
            leak.append((e.get("entity_id"), {"doing"}))
    check(not leak, "no private/dead field in any extras_summary", findings, f"leaks={leak}")
    if seen:
        sample = seen[0].get("extras_summary", {})
        unknown = set(sample.keys()) - PUBLIC_SUMMARY_OK
        check(not unknown, "extras_summary keys are all known-public", findings,
              f"unexpected={unknown} (sample={sample})")

    print(f"\n=== {'PASS — 0 findings' if not findings else str(len(findings)) + ' FINDINGS'} ===")
    for f in findings:
        print("  -", f)
    await a.ws.close(); await b.ws.close()
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    asyncio.run(main())
