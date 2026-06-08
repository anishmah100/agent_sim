#!/usr/bin/env python3
"""Audit harness S7 — SDK parity (engine wire <-> Python SDK <-> TS SDK).

Asserts the three layers agree, against a LIVE engine:
  1. The Python Observation model parses a real wire frame with NO unknown
     top-level keys and NO missing required keys.
  2. The Python Action verb union == the engine's live verb set
     (system verbs from the affordance manifest + the base/world verbs).
  3. The TS SDK declares the same Observation top-level fields + the same
     Action verbs (parsed out of sdk/typescript/src/models.ts).

Catches the class of bug where a field/verb exists in one layer but not
another (e.g. the TS SDK was missing visible_items). See
docs/ENVIRONMENT_AUDIT_PLAN.md S7.

Usage: python3 tools/audit/sdk_parity.py [engine_url]
"""
import asyncio
import re
import sys
from pathlib import Path

from dump_inventory import fetch, inventory
from harness import connect

REPO = Path(__file__).resolve().parents[2]
TS_MODELS = REPO / "sdk" / "typescript" / "src" / "models.ts"

# Base/world verbs the engine accepts that aren't in the per-system manifest
# (handled in core dispatch / are session-meta). Kept in sync with the SDK.
BASE_VERBS = {"step", "speak", "shout", "whisper", "look_at", "interact", "wait"}


def py_observation_fields() -> set:
    from agent_sim_sdk.models import Observation
    return set(Observation.model_fields.keys())


def py_action_verbs() -> set:
    import agent_sim_sdk.models as m
    verbs = set()
    for name in dir(m):
        cls = getattr(m, name)
        f = getattr(cls, "model_fields", None)
        if not f or "verb" not in f:
            continue
        # the verb literal default
        default = f["verb"].default
        if isinstance(default, str):
            verbs.add(default)
    return verbs


def ts_observation_fields() -> set:
    src = TS_MODELS.read_text()
    m = re.search(r"export const Observation = z\.object\(\{(.*?)\}\);", src, re.S)
    if not m:
        return set()
    return set(re.findall(r"^\s*([a-z_]+):", m.group(1), re.M))


def ts_action_verbs() -> set:
    src = TS_MODELS.read_text()
    return set(re.findall(r'verb:\s*z\.literal\("([a-z_]+)"\)', src))


async def main() -> None:
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    findings = []
    inv = inventory(fetch(engine))
    engine_verbs = set(inv["verbs"]) | BASE_VERBS

    # 1. Live frame parses through the Python model with no key drift.
    c = await connect(engine, name="ParityProbe")
    frame = dict(c.obs)
    frame.pop("type", None)
    await c.ws.close()
    from agent_sim_sdk.models import Observation
    Observation.model_validate(frame)  # raises on schema mismatch
    py_fields = py_observation_fields()
    wire_keys = set(frame.keys())
    extra_on_wire = wire_keys - py_fields
    if extra_on_wire:
        findings.append(f"wire has keys the Python model lacks: {extra_on_wire}")
    print(f"  [{'PASS' if not extra_on_wire else 'FAIL'}] Python model covers all wire keys "
          f"(wire={sorted(wire_keys)})")

    # 2. Python Action verbs == engine verbs.
    pv = py_action_verbs()
    missing_in_py = engine_verbs - pv - {"mental_note"}  # mental_note is a meta channel
    extra_in_py = pv - engine_verbs
    if missing_in_py:
        findings.append(f"engine verbs missing from Python SDK: {sorted(missing_in_py)}")
    if extra_in_py:
        findings.append(f"Python SDK verbs the engine doesn't expose: {sorted(extra_in_py)}")
    print(f"  [{'PASS' if not (missing_in_py or extra_in_py) else 'FAIL'}] Python Action verbs == engine "
          f"({len(pv)} py vs {len(engine_verbs)} engine)")

    # 3. TS observation fields + verbs match Python.
    tf = ts_observation_fields()
    py_obs_missing_in_ts = py_fields - tf
    ts_obs_extra = tf - py_fields
    if py_obs_missing_in_ts:
        findings.append(f"Observation fields in Python but missing in TS: {sorted(py_obs_missing_in_ts)}")
    if ts_obs_extra:
        findings.append(f"Observation fields in TS but missing in Python: {sorted(ts_obs_extra)}")
    print(f"  [{'PASS' if not (py_obs_missing_in_ts or ts_obs_extra) else 'FAIL'}] TS Observation fields == Python")

    tv = ts_action_verbs()
    py_v_missing_ts = pv - tv
    ts_v_extra = tv - pv
    if py_v_missing_ts:
        findings.append(f"Action verbs in Python but missing in TS: {sorted(py_v_missing_ts)}")
    if ts_v_extra:
        findings.append(f"Action verbs in TS but missing in Python: {sorted(ts_v_extra)}")
    print(f"  [{'PASS' if not (py_v_missing_ts or ts_v_extra) else 'FAIL'}] TS Action verbs == Python "
          f"({len(tv)} ts vs {len(pv)} py)")

    print(f"\n=== {'PASS — engine/py/ts in parity' if not findings else str(len(findings))+' FINDINGS'} ===")
    for f in findings:
        print("  -", f)
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    asyncio.run(main())
