#!/usr/bin/env python3
"""Audit harness S6 — event census against the live runlog.

Cross-references the 23 event types DECLARED in the live affordance manifest
against the event kinds actually observed in the engine's events.jsonl. Flags:
  - declared events never observed (candidate dead/untriggered events), and
  - observed events NOT declared (undocumented emissions).

This is a census, not an exhaustive trigger test: some events (e.g. building
construction completion) only fire under specific play. Pair with verb_matrix
(which exercises the verbs) + a populated demo runlog for best coverage.

Usage: python3 tools/audit/events_check.py [events.jsonl] [engine_url]
"""
import json
import sys
from collections import Counter

from dump_inventory import fetch, inventory


def main() -> None:
    log_path = sys.argv[1] if len(sys.argv) > 1 else "/tmp/doccap_events.jsonl"
    engine = sys.argv[2] if len(sys.argv) > 2 else "http://127.0.0.1:8090"

    declared = set(inventory(fetch(engine))["events"])
    # Engine also emits world/system events not tied to a verb manifest
    # (movement/spawn/etc.); treat these as known-good infrastructure events.
    INFRA = {"ActionAccepted", "Spawned", "Speech", "Whisper", "HungerSpike"}

    seen = Counter()
    try:
        for ln in open(log_path):
            ln = ln.strip()
            if not ln:
                continue
            try:
                d = json.loads(ln)
            except json.JSONDecodeError:
                continue
            k = d.get("kind") or d.get("type") or "?"
            seen[k] += 1
    except FileNotFoundError:
        print(f"no runlog at {log_path}"); sys.exit(2)

    seen_kinds = set(seen)
    print(f"runlog: {sum(seen.values())} events, {len(seen_kinds)} distinct kinds\n")
    for k, n in seen.most_common():
        tag = "" if (k in declared or k in INFRA) else "  <-- UNDOCUMENTED"
        print(f"  {n:7d}  {k}{tag}")

    never = sorted(declared - seen_kinds)
    undocumented = sorted(seen_kinds - declared - INFRA)
    print(f"\ndeclared events observed: {len(declared & seen_kinds)}/{len(declared)}")
    if never:
        print(f"declared but NOT observed in this runlog ({len(never)}):")
        print("  " + ", ".join(never))
        print("  (trigger these via targeted play to confirm they fire)")
    if undocumented:
        print(f"\n!!! UNDOCUMENTED emitted events ({len(undocumented)}): {undocumented}")
    # Only undocumented emissions are a hard failure; never-observed are TODOs.
    sys.exit(1 if undocumented else 0)


if __name__ == "__main__":
    main()
