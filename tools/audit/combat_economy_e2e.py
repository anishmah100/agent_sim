#!/usr/bin/env python3
"""Audit harness S4 — combat → death → gold-drop → recovery, live.

Validates the headline HIGH fix [0] end-to-end against a live engine: when an
agent is killed, its gold is DROPPED as recoverable coin items at the corpse
(not destroyed), and another agent can pick them up. Two SDK agents: attacker
walks to victim, attacks until death, then picks up the dropped coins and its
gold rises.

Run against a fresh engine (no other agents fighting): bash restart_sidecar.sh
Usage: python3 tools/audit/combat_economy_e2e.py [engine_url]
"""
import asyncio
import sys

from harness import connect


async def main():
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    findings = []
    atk = await connect(engine, name="Attacker", cadence_ms=150)
    vic = await connect(engine, name="Victim", cadence_ms=150)
    await atk.observe(); await vic.observe()
    vpos = tuple(vic.obs["self"]["pos"])
    vid = vic.obs["self"]["entity_id"]
    vgold = (vic.obs["self"]["extras"] or {}).get("gold", 0)
    a_gold0 = (atk.obs["self"]["extras"] or {}).get("gold", 0)
    print(f"victim={vid} at {vpos} gold={vgold}; attacker gold={a_gold0}")

    # 1. Attacker walks adjacent to the victim.
    reached = await atk.step_to(vpos, max_steps=200)
    print(f"attacker reached victim-adjacent: {reached} at {tuple(atk.obs['self']['pos'])}")
    if not reached:
        print("FAIL: attacker could not reach victim"); sys.exit(1)

    # 2. Attack until the victim dies (gone from this map) or we give up.
    victim_dead = False
    for i in range(60):
        ack = await atk.act("attack", target=vid)
        await atk.observe()
        # victim removed from world once killed
        ents = {e["entity_id"] for e in atk.obs.get("visible_entities", [])}
        if vid not in ents:
            # confirm via a fresh observation a couple ticks later
            await atk.observe()
            ents = {e["entity_id"] for e in atk.obs.get("visible_entities", [])}
            if vid not in ents:
                victim_dead = True
                break
    print(f"victim dead after attacks: {victim_dead}")
    if not victim_dead:
        findings.append("victim never died under sustained attack")

    # 3. Coins should have dropped at the corpse tile — they appear in
    #    visible_items as monetary sprites. Walk onto them + pick up.
    await atk.observe()
    coins = [it for it in atk.obs.get("visible_items", []) if "coin" in it.get("sprite", "")]
    print(f"coin drops visible: {len(coins)} -> {[c['sprite'] for c in coins][:4]}")
    if not coins and victim_dead and vgold > 0:
        findings.append(f"victim had {vgold} gold but NO coins dropped at corpse (gold destroyed)")

    # 4. Pick up a coin and confirm attacker's gold rises (auto-convert).
    picked = False
    for _ in range(40):
        await atk.observe()
        coins = [it for it in atk.obs.get("visible_items", []) if "coin" in it.get("sprite", "")]
        if not coins:
            break
        c = coins[0]
        cpos = tuple(c["pos"])
        if max(abs(cpos[0]-atk.obs["self"]["pos"][0]), abs(cpos[1]-atk.obs["self"]["pos"][1])) <= 1:
            ack = await atk.act("pickup", target=c["entity_id"])
            if ack.get("accepted"):
                picked = True
                break
        else:
            await atk.step_to(cpos, max_steps=10)
    await atk.observe()
    a_gold1 = (atk.obs["self"]["extras"] or {}).get("gold", 0)
    print(f"attacker gold: {a_gold0} -> {a_gold1} (picked a coin: {picked})")
    if victim_dead and vgold > 0 and a_gold1 <= a_gold0:
        findings.append("attacker gold did not rise after looting corpse coins")

    print(f"\n=== {'PASS — gold conserved through death' if not findings else str(len(findings))+' FINDINGS'} ===")
    for f in findings:
        print("  -", f)
    await atk.ws.close(); await vic.ws.close()
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    asyncio.run(main())
