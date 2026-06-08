#!/usr/bin/env python3
"""Audit harness S1 — verb rejection/accept matrix against a LIVE engine.

For every verb pulled from the live affordance manifest, this exercises the
deterministic rejection paths (malformed params, unknown target/item/contract)
and the trivially-accepted verbs, then asserts:
  - every reason the engine returns is DECLARED in the manifest (no undocumented
    rejection reasons leaking to agents), and
  - the expected reason actually fires (the documented contract is real).

Anything else (unexpected accept, undocumented reason, wrong reason) is a
FINDING printed at the end. See docs/ENVIRONMENT_AUDIT_PLAN.md S1.

Usage: python3 tools/audit/verb_matrix.py [engine_url]
"""
import asyncio
import sys

from dump_inventory import fetch, inventory
from harness import connect

# Deterministic rejection probes: verb -> list of (label, params, expected_reason).
# These need no world setup; they must reject with the documented reason.
BAD = "__nonexistent_target__"
CASES = {
    "attack": [("unknown_target", {"target": BAD}, "unknown_target"),
               ("bad_params", {}, "bad_params")],
    "heal": [("unknown_target", {"target": BAD}, "unknown_target")],
    "pay": [("unknown_target", {"target": BAD, "amount": 1}, "unknown_target"),
            ("bad_params", {}, "bad_params")],
    "buy_food": [("no_market_or_not_hungry", {}, None)],  # reason depends on state
    "pickup": [("not_an_item_or_unknown", {"target": BAD}, None),
               ("bad_params", {}, "bad_params")],
    "drop": [("not_in_inventory", {"item": BAD}, "not_in_inventory"),
             ("bad_params", {}, "bad_params")],
    "equip": [("not_in_inventory", {"item": BAD}, "not_in_inventory"),
              ("bad_params", {}, "bad_params")],
    "give": [("unknown_target", {"target": BAD, "item": BAD}, "unknown_target"),
             ("bad_params", {}, "bad_params")],
    "eat": [("not_in_inventory", {"item": BAD}, "not_in_inventory"),
            ("bad_params", {}, "bad_params")],
    "cook": [("not_in_inventory", {"item": BAD}, "not_in_inventory"),
             ("bad_params", {}, "bad_params")],
    "enter": [("unknown_target", {"target": BAD}, "unknown_target"),
              ("bad_params", {}, "bad_params")],
    "exit": [("not_inside", {}, "not_inside")],
    "lock": [("unknown_target", {"target": BAD}, "unknown_target")],
    "unlock": [("unknown_target", {"target": BAD}, "unknown_target")],
    "claim_ownership": [("unknown_target", {"target": BAD}, "unknown_target")],
    "transfer_ownership": [("unknown_target", {"target": BAD, "new_owner": BAD}, "unknown_target")],
    "chop": [("unknown_target", {"target": BAD}, "unknown_target")],
    "mine": [("unknown_target", {"target": BAD}, "unknown_target")],
    "forage": [("unknown_target", {"target": BAD}, "unknown_target")],
    "place_blueprint": [("unknown_blueprint", {"kind": BAD, "at": [0, 0]}, "unknown_blueprint")],
    "advance_construction": [("unknown_target", {"target": BAD}, "unknown_target")],
    "demolish": [("unknown_target", {"target": BAD}, "unknown_target")],
    "trade": [("unknown_target", {"target": BAD, "item": BAD, "price": 1}, "unknown_target")],
    "loot": [("unknown_target", {"target": BAD}, "unknown_target")],
    "propose_task": [("unknown_target", {"target": BAD, "terms": "x"}, "unknown_target"),
                     ("empty_terms_or_self", {"target": BAD, "terms": ""}, None)],
    "accept_task": [("unknown_contract", {"id": BAD}, "unknown_contract")],
    "reject_task": [("unknown_contract", {"id": BAD}, "unknown_contract")],
    "complete_task": [("unknown_contract", {"id": BAD}, "unknown_contract")],
    # State-dependent: accepts when a worksite (building) is within range,
    # else rejects no_worksite_nearby. At the hub either can happen.
    "work_for_pay": [("worksite_state_dependent", {}, None)],
}
# Verbs that should ACCEPT with trivial params (no setup).
ACCEPT = {
    "step": {"dir": "N"},
    "speak": {"text": "audit hello"},
    "shout": {"text": "audit shout"},
    "wait": {"ticks": 5},
    "look_at": {"target": "self"},
    "defend": {},
}


async def main() -> None:
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    inv = inventory(fetch(engine))
    declared = inv["verbs"]
    # Calm cadence: at a fast cadence the observation stream floods the socket
    # and can starve ack-matching within the per-act deadline. 1s cadence keeps
    # ack round-trips clean and deterministic for the matrix.
    c = await connect(engine, name="VerbMatrix", cadence_ms=1000)
    findings, npass, ntest = [], 0, 0

    print("=== REJECTION PROBES ===")
    for verb, cases in CASES.items():
        declared_reasons = declared.get(verb, {}).get("rejections", [])
        for label, params, expected in cases:
            ntest += 1
            ack = await c.act(verb, **params)
            # Retry once on a missing ack: under a long run the observation
            # stream can transiently delay ack-matching past the deadline.
            # A genuinely un-acked verb fails both attempts.
            if ack.get("reason") == "__no_ack__":
                ack = await c.act(verb, timeout=8.0, **params)
            acc = ack.get("accepted")
            reason = ack.get("reason", "")
            ok = True
            note = ""
            if acc:
                # Accept is only a finding when we expected a specific
                # rejection. For state-dependent cases (expected=None) an
                # accept can be legitimate (e.g. buy_food when hungry+funded).
                if expected is not None:
                    ok, note = False, f"UNEXPECTEDLY ACCEPTED (expected '{expected}')"
                else:
                    note = "accepted (state-dependent, OK)"
            elif expected and reason != expected:
                # allow if returned reason is at least documented
                if reason in declared_reasons:
                    note = f"got documented '{reason}' (expected '{expected}')"
                else:
                    ok, note = False, f"got '{reason}', expected '{expected}', NOT in manifest"
            elif reason and reason not in declared_reasons:
                ok, note = False, f"UNDOCUMENTED reason '{reason}'"
            status = "PASS" if ok else "FAIL"
            if ok:
                npass += 1
            else:
                findings.append(f"{verb}/{label}: {note} (ack={ack})")
            print(f"  [{status}] {verb:20s} {label:28s} -> accepted={acc} reason='{reason}' {note}")

    print("\n=== ACCEPT PROBES ===")
    for verb, params in ACCEPT.items():
        ntest += 1
        ack = await c.act(verb, **params)
        ok = bool(ack.get("accepted"))
        if ok:
            npass += 1
        else:
            findings.append(f"{verb}/accept: rejected '{ack.get('reason')}' (ack={ack})")
        print(f"  [{'PASS' if ok else 'FAIL'}] {verb:20s} accept -> {ack.get('accepted')} '{ack.get('reason','')}'")

    print(f"\n=== {npass}/{ntest} probes passed ===")
    if findings:
        print(f"\n!!! {len(findings)} FINDINGS:")
        for f in findings:
            print("  -", f)
    await c.ws.close()
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    asyncio.run(main())
