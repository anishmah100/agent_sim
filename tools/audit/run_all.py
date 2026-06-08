#!/usr/bin/env python3
"""Audit harness — run every suite and print a PASS/FAIL matrix.

Assumes an engine is already running (default :8090). Runs each suite as a
subprocess, captures its exit code, and prints a one-line-per-suite summary.
This is the permanent regression gate (docs/ENVIRONMENT_AUDIT_PLAN.md §4).

Usage: python3 tools/audit/run_all.py [engine_url]
"""
import subprocess
import sys
import os

ENGINE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
HERE = os.path.dirname(os.path.abspath(__file__))

SUITES = [
    ("S1 verb matrix", "verb_matrix.py", [ENGINE]),
    ("S2 obs integrity", "obs_integrity.py", [ENGINE]),
    ("S3 movement", "movement_check.py", [ENGINE]),
    ("S5 building e2e", "building_e2e.py", [ENGINE]),
    ("S4 combat→econ e2e", "combat_economy_e2e.py", [ENGINE]),
    ("S6 events census", "events_check.py", ["/tmp/doccap_events.jsonl", ENGINE]),
    ("S7 sdk parity", "sdk_parity.py", [ENGINE]),
    ("S12 security", "security_check.py", [ENGINE]),
]


def main() -> None:
    results = []
    for label, script, args in SUITES:
        print(f"\n{'='*60}\nRUN {label} ({script})\n{'='*60}")
        p = subprocess.run([sys.executable, os.path.join(HERE, script), *args],
                           capture_output=True, text=True, timeout=240)
        tail = "\n".join(p.stdout.strip().splitlines()[-4:])
        print(tail)
        if p.returncode != 0 and p.stderr.strip():
            print("STDERR:", p.stderr.strip()[-300:])
        results.append((label, p.returncode))

    print(f"\n{'='*60}\nAUDIT MATRIX\n{'='*60}")
    allgreen = True
    for label, rc in results:
        status = "PASS" if rc == 0 else f"FAIL(rc={rc})"
        if rc != 0:
            allgreen = False
        print(f"  [{status}] {label}")
    print(f"\n{'ALL GREEN' if allgreen else 'SOME SUITES FAILED'}")
    sys.exit(0 if allgreen else 1)


if __name__ == "__main__":
    main()
