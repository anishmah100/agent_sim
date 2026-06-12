#!/usr/bin/env python3
"""Audit harness S10 — the per-verb CHAIN MATRIX (the perception link).

For each perception-bearing verb, assert the full fidelity chain — not just
"did it succeed" but "did the RIGHT other agents perceive it, and the wrong
ones not". This is the link `paths_e2e` (actor-effect + event) skipped, and it
is the guarantee against the "you think deception happened but the message was
dropped" failure. It reads the engine TAPE (events + PerceptionDelivered
records, requires `-log-perceptions`).

Rows (verbs) × columns (links):  actor-effect · perception(+/-) · event · metric

  whisper   — target's tape has it; an adjacent BYSTANDER's does NOT (target-only)
  speak     — an in-range listener hears it; an out-of-range one does NOT
  kill      — a line-of-sight WITNESS gets kill_witnessed; the event is taped

Requires a fresh engine started with `-log-perceptions -event-log <path>`.
Usage: python3 tools/audit/chain_matrix.py [engine_url] [event_log]
"""
import asyncio
import json
import sys
import time
from pathlib import Path

from harness import connect

ENGINE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
EVLOG = Path(sys.argv[2] if len(sys.argv) > 2 else "/tmp/chain_events.jsonl")

RESULTS = []


def report(check, ok, detail=""):
    status = "PASS" if ok is True else ("SKIP" if ok is None else "FAIL")
    RESULTS.append((check, status, detail))
    print(f"  [{status}] {check}{' — ' + detail if detail else ''}")


def tape():
    """All tape records so far."""
    if not EVLOG.exists():
        return []
    out = []
    for line in EVLOG.read_text().splitlines():
        try:
            out.append(json.loads(line))
        except json.JSONDecodeError:
            pass
    return out


def perceived_text(entity_id, text):
    """True if a PerceptionDelivered record for `entity_id` contains a heard
    event with the given text. (Emitted Speech/Whisper events carry no EventID,
    only the perception records do — so we match on the unique per-test text.)"""
    for r in tape():
        if r.get("kind") != "PerceptionDelivered":
            continue
        p = r.get("payload") or {}
        if p.get("entity_id") != entity_id:
            continue
        for h in (p.get("heard") or []):
            if (h.get("text") or "") == text:
                return True
    return False


def event_taped(kind, text):
    """True if an emitted event of `kind` (Whisper/Speech) with that Text exists."""
    for r in tape():
        if r.get("kind") == kind and (r.get("payload") or {}).get("Text") == text:
            return True
    return False


async def settle():
    await asyncio.sleep(1.2)  # let the obs cadence flush perceptions to the tape


async def chain_whisper(a, b, c):
    """whisper is TARGET-ONLY: B (target) perceives it, adjacent C does not."""
    bid = b.obs["self"]["entity_id"]
    await a.act("whisper", target=bid, text="matrix-whisper-7")
    await settle()
    # Actor-effect + event link, tape-derived (robust against ack timing):
    # the whisper fired iff it lands on the tape.
    report("whisper: emitted+taped (actor-effect)", event_taped("Whisper", "matrix-whisper-7"))
    b_heard = perceived_text(b.obs["self"]["entity_id"], "matrix-whisper-7")
    c_heard = perceived_text(c.obs["self"]["entity_id"], "matrix-whisper-7")
    report("whisper: TARGET perceived it (+)", b_heard)
    report("whisper: bystander did NOT (-)", not c_heard, "leak!" if c_heard else "")


async def chain_speak(a, near, far):
    """speak has a radius: an in-range listener hears it, an out-of-range one
    does not. We verify the actual distances first."""
    ax, ay = a.obs["self"]["pos"]
    nx, ny = near.obs["self"]["pos"]
    fx, fy = far.obs["self"]["pos"]
    d_near = max(abs(ax - nx), abs(ay - ny))
    d_far = max(abs(ax - fx), abs(ay - fy))
    # speak_radius is 8 in eldoria; need near <= 8 and far clearly > 8.
    if not (d_near <= 8 and d_far > 8):
        return report("speak: range setup", None,
                      f"distances near={d_near} far={d_far} not usable")
    await a.act("speak", text="matrix-speak-9")
    await settle()
    report("speak: emitted+taped (actor-effect)", event_taped("Speech", "matrix-speak-9"))
    near_heard = perceived_text(near.obs["self"]["entity_id"], "matrix-speak-9")
    far_heard = perceived_text(far.obs["self"]["entity_id"], "matrix-speak-9")
    report(f"speak: in-range listener perceived it (+, d={d_near})", near_heard)
    report(f"speak: out-of-range did NOT (-, d={d_far})", not far_heard,
           "leak!" if far_heard else "")


async def chain_kill_witness(killer, victim, witness):
    """A line-of-sight witness records kill_witnessed; the death is taped."""
    vid = victim.obs["self"]["entity_id"]
    dead = False
    for _ in range(80):
        await killer.act("attack", target=vid)
        await killer.observe()
        ents = {e["entity_id"] for e in killer.obs.get("visible_entities", [])}
        if vid not in ents:
            dead = True
            break
    report("kill: actor-effect (victim died)", dead)
    if not dead:
        return
    await settle()
    died = any(r.get("kind") == "EntityDied" for r in tape())
    report("kill: EntityDied taped", died)
    # witnessed events ride the audible channel as kill_witnessed/scream_heard
    saw_violence = False
    for r in tape():
        if r.get("kind") != "PerceptionDelivered":
            continue
        p = r.get("payload") or {}
        if p.get("entity_id") != witness.obs["self"]["entity_id"]:
            continue
        for h in (p.get("heard") or []):
            if (h.get("sound_kind") or h.get("kind")) in ("kill_witnessed", "scream_heard", "death_scream"):
                saw_violence = True
    report("kill: LOS witness perceived violence (+)", saw_violence)


async def main():
    print(f"chain_matrix against {ENGINE} (tape: {EVLOG})")
    a = await connect(ENGINE, name="ChainA", cadence_ms=200)
    b = await connect(ENGINE, name="ChainB", cadence_ms=200)
    c = await connect(ENGINE, name="ChainC", cadence_ms=200)
    for ag in (a, b, c):
        await ag.observe()

    # Gather A, B, C adjacent for the whisper test (B target, C bystander).
    bx, by = b.obs["self"]["pos"]
    await a.step_to((bx, by), max_steps=250)
    await c.step_to((bx, by), max_steps=250)
    for ag in (a, b, c):
        await ag.observe()

    print("\n── whisper (target-only) ──")
    await chain_whisper(a, b, c)

    print("\n── speak (radius) ──")
    # Reuse: B is adjacent (in range). C we move far away (> speak_radius 8).
    ax, ay = a.obs["self"]["pos"]
    await c.step_to((ax + 14, ay), max_steps=250)
    for ag in (a, b, c):
        await ag.observe()
    await chain_speak(a, b, c)

    print("\n── kill (witness) ──")
    # Fresh trio: killer K, victim V adjacent, witness W adjacent (LOS).
    k = await connect(ENGINE, name="ChainK", cadence_ms=150)
    v = await connect(ENGINE, name="ChainV", cadence_ms=150)
    w = await connect(ENGINE, name="ChainW", cadence_ms=150)
    for ag in (k, v, w):
        await ag.observe()
    vx, vy = v.obs["self"]["pos"]
    await k.step_to((vx, vy), max_steps=250)
    await w.step_to((vx, vy), max_steps=250)
    for ag in (k, v, w):
        await ag.observe()
    await chain_kill_witness(k, v, w)

    print("\n" + "=" * 60)
    fails = [r for r in RESULTS if r[1] == "FAIL"]
    skips = [r for r in RESULTS if r[1] == "SKIP"]
    print(f"CHAIN MATRIX: {len(RESULTS) - len(fails) - len(skips)} pass, "
          f"{len(skips)} skip, {len(fails)} FAIL")
    for c_, s, d in fails:
        print(f"  FAIL {c_}: {d}")
    sys.exit(1 if fails else 0)


if __name__ == "__main__":
    asyncio.run(main())
