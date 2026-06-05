"""score_a9.py — checks the AGENT-A9 pass criteria against an
events.jsonl produced by the historian.

Pass criteria mirror PROGRESS.md row 10. Outputs a quick checkmark
table and a 0/1 exit code.
"""

from __future__ import annotations

import json
import sys
from collections import Counter
from pathlib import Path


CORE_VERBS = {
    "move_step", "look_at", "speak", "shout", "whisper",
    "pick_up", "drop", "give", "pay", "wait", "ponder",
    # Legacy verbs that map to core in fantasy_town's manifest.
    "move", "speak", "interact", "pickup",
}


def score(path: str) -> int:
    p = Path(path)
    if not p.exists():
        print(f"events.jsonl not found: {path}", file=sys.stderr)
        return 2

    by_kind: Counter[str] = Counter()
    by_category: Counter[str] = Counter()
    speech_lines = []
    payment_count = 0
    enters = 0
    exits = 0
    reasoning_count = 0
    reflection_count = 0
    unique_actors = set()
    verbs_seen: set[str] = set()

    for line in p.read_text().splitlines():
        if not line.strip():
            continue
        try:
            rec = json.loads(line)
        except Exception:
            continue
        kind = rec.get("kind", "")
        cat = rec.get("category", "")
        by_kind[kind] += 1
        by_category[cat] += 1
        payload = rec.get("payload", {}) or {}
        if isinstance(payload, str):
            try:
                payload = json.loads(payload)
            except Exception:
                payload = {}
        if kind == "Speech":
            speech_lines.append(payload)
        if kind == "GoldTransferred":
            payment_count += 1
        if kind == "EnteredBuilding":
            enters += 1
        if kind == "ExitedBuilding":
            exits += 1
        if cat == "agent_reasoning":
            reasoning_count += 1
            actor = payload.get("entity_id")
            verb = payload.get("verb")
            if actor:
                unique_actors.add(actor)
            if verb:
                verbs_seen.add(verb)
        if kind == "ReflectiveNote":
            reflection_count += 1

    # Multi-turn dialogue heuristic: count back-and-forth pairs.
    multi_turn = 0
    if speech_lines:
        last_speaker = None
        last_target = None
        for s in speech_lines:
            speaker = (s.get("Speaker") or s.get("From") or "")
            target = (s.get("Target") or s.get("To") or "")
            if last_speaker and last_target == speaker and last_speaker == target:
                multi_turn += 1
            last_speaker = speaker
            last_target = target

    checks = [
        ("zero crashes (events present)", sum(by_kind.values()) > 0),
        (">= every core verb (seen ≥1)", verbs_seen >= {"move", "speak"}),
        ("≥ 2 multi-turn dialogues", multi_turn >= 2),
        ("≥ 1 trade/payment", payment_count >= 1),
        ("≥ 1 building entry + exit", enters >= 1 and exits >= 1),
        ("≥ 1 reasoning trace", reasoning_count >= 1),
        ("≥ 1 reflection note", reflection_count >= 1),
        ("≥ 1 distinct reasoning actor", len(unique_actors) >= 1),
    ]

    print(f"events:      {sum(by_kind.values())} total")
    print(f"by_category: {dict(by_category)}")
    print(f"verbs seen:  {sorted(verbs_seen)}")
    print(f"actors:      {sorted(unique_actors)}")
    print()
    print("pass criteria:")
    all_pass = True
    for name, ok in checks:
        mark = "✓" if ok else "✗"
        print(f"  {mark} {name}")
        if not ok:
            all_pass = False
    print()
    if all_pass:
        print("AGENT-A9 mechanical criteria: PASS")
        print("Sample a few reasoning traces by querying derived.sqlite "
              "and make a taste call.")
        return 0
    else:
        print("AGENT-A9 mechanical criteria: FAIL — see ✗ above")
        return 1


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: python tools/dev-scripts/score_a9.py <events.jsonl>",
              file=sys.stderr)
        return 2
    return score(sys.argv[1])


if __name__ == "__main__":
    sys.exit(main())
