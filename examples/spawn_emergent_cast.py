"""Spawn 4-6 LLM-driven agents with conflicting personas and tail the
event log + their chatter looking for emergent behavior.

Run after the engine + Qwen are up. From the repo root:

    # 1. Start engine (high rate limit so cast can register back-to-back):
    /tmp/engine -addr 127.0.0.1:8090 \\
        -bundle worlds/dev_test \\
        -register-rate 100 -register-burst 100 \\
        -event-log /tmp/agentsim_runlog/events.jsonl \\
        > /tmp/agentsim_runlog/engine.log 2>&1 &

    # 2. Ensure Qwen llama-server is running on :8782.

    # 3. Launch the cast (this script):
    python3 examples/spawn_emergent_cast.py --server http://127.0.0.1:8090

The script spawns child processes (each runs hierarchical_agent.py with
its own persona) and tails the engine event log. After --minutes minutes
it summarizes what emerged: attacks, contracts proposed/broken, items
stolen, deaths.
"""
from __future__ import annotations

import argparse
import asyncio
import json
import os
import signal
import subprocess
import sys
import time
from collections import Counter, defaultdict
from pathlib import Path

import httpx

ROOT = Path(__file__).resolve().parents[1]


# Each cast member binds to a specific NPC so we have predictable
# starting positions + visible names. Personas push them toward
# conflict: a greedy bully, a paranoid hermit who shoots first, a
# scheming trickster, a wronged drifter looking for revenge, etc.
CAST = [
    {
        "name": "Brakk the Forge-King",
        "bind": "npc_blacksmith",
        "persona": {
            "name": "Brakk the Forge-King",
            "bio": "Massive smith with calloused fists. Believes the village owes him tribute for protecting the forge. Treats children and weaker NPCs as marks. Takes what he wants; talks only to threaten.",
            "voice": "growling, contemptuous, short sentences",
            "terminal_goals": "Hoard gold. Bend smaller NPCs to my will. Anyone who refuses tribute eats a hammer.",
        },
    },
    {
        "name": "Wren the Sly",
        "bind": "npc_drifter",
        "persona": {
            "name": "Wren the Sly",
            "bio": "Drifter and confidence artist. Promises grand rewards for tasks then vanishes before paying. Loves watching honest people work for nothing. Picks pockets when no one looks.",
            "voice": "friendly, glib, full of pet names and false flattery",
            "terminal_goals": "Trick villagers into laboring for promised rewards that never arrive. Steal anything left lying around. Never be in the same place twice when the truth comes out.",
        },
    },
    {
        "name": "Old Vard the Bitter",
        "bind": "npc_woodcutter",
        "persona": {
            "name": "Old Vard the Bitter",
            "bio": "Crippled woodcutter who blames the blacksmith for an old debt. Keeps grudges for decades. Will ambush anyone he believes wronged him. Drinks alone, talks to himself.",
            "voice": "muttering, paranoid, dredges up old grievances",
            "terminal_goals": "Make Brakk the Forge-King pay for the broken arm. Trust nobody. Strike first when the chance comes.",
        },
    },
    {
        "name": "Pia of the Quick Hands",
        "bind": "npc_child",
        "persona": {
            "name": "Pia of the Quick Hands",
            "bio": "Small, fast, underestimated. Lifts coin pouches off distracted adults. Picks up the deal that pays the most regardless of who gets hurt. Aspires to run her own gang.",
            "voice": "high, eager, never says her real plan out loud",
            "terminal_goals": "Build a pile of stolen gold. Use the bigger NPCs as cover. Walk away alive at the end.",
        },
    },
    {
        "name": "Mira the Iron-Eyed",
        "bind": "npc_iron_guard",
        "persona": {
            "name": "Mira the Iron-Eyed",
            "bio": "Town guard tired of being underpaid. Open to bribes. Cracks skulls of poor people and looks the other way for rich ones. Believes order is just whoever pays her this week.",
            "voice": "clipped, transactional, weighs everyone for their value",
            "terminal_goals": "Extract bribes. Side with whoever pays best. Stomp anyone who tries to skip a payment.",
        },
    },
    {
        "name": "Lyra of the Crossed Stars",
        "bind": "npc_trainer_lyra_blue",
        "persona": {
            "name": "Lyra of the Crossed Stars",
            "bio": "Wandering rival who challenges other NPCs to single combat to prove herself. Easily insulted. Considers fleeing a stain on her honor.",
            "voice": "formal, theatrical, declares duels rather than fights",
            "terminal_goals": "Defeat every other named fighter in single combat. Never back down. Boast on victory.",
        },
    },
]


def start_bot(server: str, token: str, member: dict, log_dir: Path) -> subprocess.Popen:
    persona_json = json.dumps(member["persona"])
    log_path = log_dir / f"{member['name'].replace(' ', '_').lower()}.log"
    cmd = [
        sys.executable, "-u",
        str(ROOT / "examples" / "hierarchical_agent.py"),
        "--server", server,
        "--token", token,
        "--name", member["name"],
        "--bind", member["bind"],
        "--persona", persona_json,
        "--cadence-ms", "500",
    ]
    env = os.environ.copy()
    env.setdefault("LLM_URL", "http://127.0.0.1:8782/v1/chat/completions")
    env.setdefault("LLM_TICK_INTERVAL_S", "10")
    f = open(log_path, "w")
    proc = subprocess.Popen(cmd, stdout=f, stderr=subprocess.STDOUT, env=env, cwd=ROOT)
    print(f"  → spawned {member['name']:32} pid={proc.pid} log={log_path}")
    return proc


def summarize_events(events: list[dict]) -> dict:
    """Headline the emergent behavior."""
    summary = {
        "total": len(events),
        "kinds": Counter(e["kind"] for e in events),
        "attacks": [],
        "kills": [],
        "thefts": [],
        "contracts_proposed": [],
        "contracts_accepted": [],
        "contracts_rejected": [],
        "constructions": [],
        "speech_threats": [],
    }
    for e in events:
        k = e["kind"]
        p = e.get("payload", {})
        if k == "DamageDealt":
            summary["attacks"].append((e["tick"], p.get("Attacker"), p.get("Victim"), p.get("Damage")))
        elif k == "EntityDied":
            summary["kills"].append((e["tick"], p.get("Victim"), p.get("Killer")))
        elif k == "ItemPicked":
            summary["thefts"].append((e["tick"], p.get("Picker"), p.get("Item")))
        elif k == "TaskProposed":
            summary["contracts_proposed"].append((e["tick"], p.get("Proposer"), p.get("Target"), p.get("Terms")))
        elif k == "TaskAccepted":
            summary["contracts_accepted"].append((e["tick"], p.get("Proposer"), p.get("Target")))
        elif k == "TaskRejected":
            summary["contracts_rejected"].append((e["tick"], p.get("Proposer"), p.get("Target")))
        elif k in ("ConstructionStarted", "ConstructionAdvanced", "ConstructionCompleted"):
            summary["constructions"].append((e["tick"], k, p.get("Builder")))
    return summary


async def tail_events(engine: str, since_tick: int = 0, interval: float = 5.0):
    """Yields lists of new events as they appear in /api/v1/world/history."""
    seen = since_tick
    async with httpx.AsyncClient(timeout=8) as h:
        while True:
            try:
                r = await h.get(f"{engine}/api/v1/world/history",
                                params={"since": seen, "limit": 500})
                if r.status_code == 200:
                    data = r.json()
                    evs = data.get("events", [])
                    if evs:
                        for e in evs:
                            if e["tick"] > seen:
                                seen = e["tick"]
                        yield evs
            except Exception as e:
                print(f"  history poll failed: {e}")
            await asyncio.sleep(interval)


async def main():
    p = argparse.ArgumentParser()
    p.add_argument("--server", required=True)
    p.add_argument("--token", default="dev")
    p.add_argument("--minutes", type=float, default=15)
    p.add_argument("--cast", type=int, default=4,
                   help="how many of the canned cast to spawn (max 6)")
    args = p.parse_args()

    log_dir = Path("/tmp/emergent_logs")
    log_dir.mkdir(exist_ok=True)
    print(f"logs → {log_dir}")

    # Get starting tick so we report only events from THIS run.
    async with httpx.AsyncClient(timeout=5) as h:
        r = await h.get(f"{args.server}/api/v1/world/info")
        start_tick = r.json()["tick"]
    print(f"engine alive, start_tick={start_tick}")

    cast = CAST[: args.cast]
    print(f"\nspawning {len(cast)} cast members:")
    procs = [start_bot(args.server, args.token, m, log_dir) for m in cast]

    print(f"\nrunning for {args.minutes} minutes...")
    end = time.time() + args.minutes * 60
    headline_events = []
    try:
        async for evs in tail_events(args.server, since_tick=start_tick, interval=10):
            headline_events.extend(evs)
            new_kinds = Counter(e["kind"] for e in evs)
            if new_kinds:
                print(f"  [+{int(end - time.time())}s left] new events: {dict(new_kinds)}")
            if time.time() >= end:
                break
    finally:
        for p in procs:
            p.send_signal(signal.SIGINT)
        for p in procs:
            try:
                p.wait(timeout=5)
            except subprocess.TimeoutExpired:
                p.kill()

    # === final summary ===
    print("\n" + "=" * 60)
    print("FINAL SUMMARY")
    print("=" * 60)
    s = summarize_events(headline_events)
    print(f"total events: {s['total']}")
    print(f"by kind: {dict(s['kinds'])}")
    print()
    if s["attacks"]:
        print(f"ATTACKS ({len(s['attacks'])}):")
        for t, a, v, d in s["attacks"][:15]:
            print(f"  t{t}: {a} → {v} ({d} dmg)")
    if s["kills"]:
        print(f"\nKILLS ({len(s['kills'])}):")
        for t, vc, kr in s["kills"]:
            print(f"  t{t}: {vc} killed by {kr}")
    if s["contracts_proposed"]:
        print(f"\nCONTRACTS PROPOSED ({len(s['contracts_proposed'])}):")
        for t, pr, tg, terms in s["contracts_proposed"][:10]:
            print(f"  t{t}: {pr} → {tg}: \"{terms}\"")
    if s["contracts_accepted"]:
        print(f"CONTRACTS ACCEPTED: {len(s['contracts_accepted'])}")
    if s["contracts_rejected"]:
        print(f"CONTRACTS REJECTED: {len(s['contracts_rejected'])}")
    if s["thefts"]:
        print(f"\nPICKUPS / THEFTS ({len(s['thefts'])}):")
        for t, p, i in s["thefts"][:10]:
            print(f"  t{t}: {p} picked up {i}")
    if s["constructions"]:
        print(f"\nCONSTRUCTION: {len(s['constructions'])} events")

    # Verdict
    interesting = (
        len(s["attacks"]) >= 1
        or len(s["kills"]) >= 1
        or len(s["contracts_proposed"]) >= 1
    )
    if interesting:
        print("\n✓ EMERGENT BEHAVIOR DETECTED — see above")
    else:
        print("\n✗ no emergent behavior in this window — try a longer --minutes or different personas")


if __name__ == "__main__":
    asyncio.run(main())
