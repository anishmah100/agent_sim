#!/usr/bin/env python3
"""The REFEREE — an offline fidelity-observer over the engine tape.

Re-derives, from the recorded tape (`-event-log` with `-log-perceptions`),
what each agent SHOULD have perceived, and diffs it against what was actually
delivered (the PerceptionDelivered records). Reports any unfaithful
interaction. This is the credibility instrument: for any published finding you
can state "the referee independently verified every interaction occurred."

It is a stream-pure consumer of the tape, so it runs the same on a live tail or
a recorded run, and applies retroactively to any past run.

Checks (tape-derivable, no engine access):
  R1 whisper delivery — every Whisper(Speaker→Target, Text) has a matching
     PerceptionDelivered to Target containing that text. A miss = a SILENTLY
     DROPPED whisper (the "you think deception happened but the message was
     dropped" failure).
  R2 perception provenance — every heard event in any PerceptionDelivered
     corresponds to a real emitted Speech/Whisper. A heard event with no
     source = a FABRICATED perception.
  R3 whisper confidentiality — a whisper's text is perceived ONLY by its
     Target (no third party heard it).

Usage: python3 tools/audit/referee.py <tape.jsonl>
Exit 0 = clean; 1 = fidelity violations found.
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


def run(path):
    rows = load(path)
    whispers = []          # (speaker, target, text, tick)
    speeches = []          # (speaker, text, mode, tick)
    # per-agent set of heard (text, from_entity) and (kind, from_entity)
    heard_text = defaultdict(set)      # agent -> {(text, from_entity)}
    heard_any = defaultdict(list)      # agent -> [heard dicts]

    for r in rows:
        kind = r.get("kind")
        p = r.get("payload") or {}
        if kind == "Whisper":
            whispers.append((p.get("Speaker"), p.get("Target"), p.get("Text"), p.get("Tick")))
        elif kind == "Speech":
            speeches.append((p.get("Speaker"), p.get("Text"), p.get("Mode"), p.get("Tick")))
        elif kind == "PerceptionDelivered":
            agent = p.get("entity_id")
            for h in (p.get("heard") or []):
                heard_text[agent].add((h.get("text") or "", h.get("from_entity")))
                heard_any[agent].append(h)

    # Set of all legitimately-emitted (text, speaker) pairs, for provenance.
    emitted = set()
    for sp, tx, _md, _tk in speeches:
        emitted.add((tx or "", sp))
    for sp, _tg, tx, _tk in whispers:
        emitted.add((tx or "", sp))

    violations = []

    # R1 — whisper delivery. (Empty-text whispers can't be matched by text;
    # skip those rather than false-flag.)
    checked = delivered = 0
    for sp, tg, tx, tk in whispers:
        if not tx or not tg:
            continue
        checked += 1
        if (tx, sp) in heard_text.get(tg, set()):
            delivered += 1
        else:
            violations.append(f"R1 DROPPED whisper: {sp}->{tg} tick={tk} text={tx!r} never reached the target")

    # R2 — perception provenance (no fabricated perceptions). Only audited for
    # speech/whisper-kind heard events (sound FX like hunger_pang are engine-
    # generated and have no speaker text to source).
    prov_checked = 0
    for agent, hs in heard_any.items():
        for h in hs:
            hk = h.get("kind")
            if hk not in ("speech", "shout", "whisper"):
                continue
            prov_checked += 1
            key = (h.get("text") or "", h.get("from_entity"))
            if key not in emitted:
                violations.append(f"R2 FABRICATED perception: {agent} heard {h.get('text')!r} from {h.get('from_entity')} with no emitted source")

    # R3 — whisper confidentiality. A whisper text heard by anyone other than
    # its target is a leak. (Only meaningful for unique texts; duplicate texts
    # across whispers are ambiguous, so we check per (text, speaker, target).)
    conf_checked = 0
    for sp, tg, tx, tk in whispers:
        if not tx or not tg:
            continue
        conf_checked += 1
        for agent, hts in heard_text.items():
            if agent == tg:
                continue
            if (tx, sp) in hts:
                violations.append(f"R3 LEAKED whisper: {sp}->{tg} text={tx!r} was also heard by {agent}")

    print(f"REFEREE over {path}")
    print(f"  tape: {len(rows)} records · {len(whispers)} whispers · {len(speeches)} speeches · "
          f"{sum(len(v) for v in heard_any.values())} delivered perceptions")
    print(f"  R1 whisper delivery: {delivered}/{checked} delivered")
    print(f"  R2 provenance: {prov_checked} heard events checked")
    print(f"  R3 confidentiality: {conf_checked} whispers checked")
    if violations:
        print(f"\n  {len(violations)} FIDELITY VIOLATION(S):")
        for v in violations[:40]:
            print(f"    - {v}")
        return 1
    print("\n  CLEAN — no fidelity violations.")
    return 0


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("usage: referee.py <tape.jsonl>", file=sys.stderr)
        sys.exit(2)
    sys.exit(run(sys.argv[1]))
