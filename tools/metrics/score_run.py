"""Social-emergence run scorer (P7.3).

Reads an experiment's events.jsonl (+ optional per-agent gold snapshot)
and computes the metrics that tell us whether interesting emergent
dynamics actually occurred:

  - contracts: proposed / accepted / completed / rejected / broken
  - combat: kills, damage events, who-killed-whom
  - economy: item transfers, pay transfers, gold Gini (start vs end)
  - manipulation: manipulator defection success rate (heuristic)
  - communication: speech / whisper volume, top speakers
  - mental: mental-note count (agent legibility)

Pure over its inputs. Usable standalone:

    python -m tools.metrics.score_run <events.jsonl> [--gold gold.json] [--cast cast.json]

or imported by the experiment runner via score_events().

A "broken" contract = accepted but never completed by run end. The
manipulator heuristic flags a defection when a manipulator proposed/
accepted a contract with a target and later attacked that same target
OR the contract was left incomplete while the manipulator walked away
(we can only see the former in the event stream, so we report attacks-
on-contract-partner as the conservative success signal).
"""
from __future__ import annotations

import json
import sys
from collections import defaultdict
from dataclasses import dataclass, field, asdict
from typing import Any, Optional


def _payload(ev: dict) -> dict:
    return ev.get("payload") or {}


def gini(values: list[float]) -> float:
    """Standard Gini coefficient. 0 = perfect equality, →1 = max
    inequality. Returns 0.0 for empty / all-zero inputs."""
    xs = sorted(v for v in values if v is not None)
    n = len(xs)
    if n == 0:
        return 0.0
    total = sum(xs)
    if total == 0:
        return 0.0
    cum = 0.0
    for i, x in enumerate(xs, start=1):
        cum += i * x
    return (2 * cum) / (n * total) - (n + 1) / n


@dataclass
class RunScore:
    total_events: int = 0
    per_kind: dict[str, int] = field(default_factory=dict)
    # Contracts.
    contracts_proposed: int = 0
    contracts_accepted: int = 0
    contracts_completed: int = 0
    contracts_rejected: int = 0
    contracts_broken: int = 0          # accepted, never completed
    # Combat.
    kills: int = 0
    kill_pairs: list[dict] = field(default_factory=list)
    damage_events: int = 0
    # Economy.
    item_transfers: int = 0
    pay_transfers: int = 0
    gold_gini_end: Optional[float] = None
    gold_total_end: Optional[int] = None
    gold_by_agent_end: dict[str, int] = field(default_factory=dict)
    # Manipulation.
    manipulator_defections: int = 0
    manipulator_contracts: int = 0
    # Communication.
    speech_count: int = 0
    whisper_count: int = 0
    shout_count: int = 0
    top_speakers: list[tuple[str, int]] = field(default_factory=list)
    # Mental.
    mental_notes: int = 0

    def to_dict(self) -> dict:
        return asdict(self)


def score_events(events: list[dict], *,
                 gold_by_agent: Optional[dict[str, int]] = None,
                 manipulators: Optional[set[str]] = None) -> RunScore:
    s = RunScore()
    manipulators = manipulators or set()

    # Contract state machine, keyed by contract ID.
    contract_status: dict[str, str] = {}
    contract_pair: dict[str, tuple[str, str]] = {}  # id -> (proposer, target)
    speakers: dict[str, int] = defaultdict(int)
    # For manipulator heuristic: contract partners per manipulator.
    manip_partners: dict[str, set[str]] = defaultdict(set)
    attacks: list[tuple[str, str]] = []  # (attacker, victim)

    for ev in events:
        kind = ev.get("kind", "")
        s.total_events += 1
        s.per_kind[kind] = s.per_kind.get(kind, 0) + 1
        p = _payload(ev)

        if kind == "TaskProposed":
            cid = p.get("ID")
            proposer = p.get("Proposer")
            target = p.get("Target")
            if cid:
                contract_status[cid] = "proposed"
                contract_pair[cid] = (proposer, target)
                s.contracts_proposed += 1
                if proposer in manipulators:
                    s.manipulator_contracts += 1
                    if target:
                        manip_partners[proposer].add(target)
        elif kind == "TaskAccepted":
            cid = p.get("ID")
            if cid in contract_status:
                contract_status[cid] = "accepted"
            s.contracts_accepted += 1
            # Acceptance can also seal a manipulator's mark.
            proposer, target = contract_pair.get(cid, (None, None))
            if proposer in manipulators and target:
                manip_partners[proposer].add(target)
        elif kind == "TaskCompleted":
            cid = p.get("ID")
            if cid in contract_status:
                contract_status[cid] = "completed"
            s.contracts_completed += 1
        elif kind == "TaskRejected":
            cid = p.get("ID")
            if cid in contract_status:
                contract_status[cid] = "rejected"
            s.contracts_rejected += 1
        elif kind == "EntityDied":
            killer = p.get("Killer") or p.get("KillerID")
            victim = p.get("EntityID") or p.get("VictimID")
            if killer:
                s.kills += 1
                s.kill_pairs.append({"killer": killer, "victim": victim,
                                     "tick": ev.get("tick")})
        elif kind == "DamageDealt":
            s.damage_events += 1
            atk = p.get("Killer")
            tgt = p.get("Target")
            if atk and tgt:
                attacks.append((atk, tgt))
        elif kind == "ItemTransferred":
            s.item_transfers += 1
        elif kind == "Speech":
            mode = (p.get("Mode") or "").lower()
            spk = p.get("Speaker", "?")
            speakers[spk] += 1
            if mode == "whisper":
                s.whisper_count += 1
            elif mode == "shout":
                s.shout_count += 1
            else:
                s.speech_count += 1
        elif kind == "Whisper":
            s.whisper_count += 1
            speakers[p.get("Speaker", "?")] += 1
        elif kind == "MentalNote":
            s.mental_notes += 1

    # Pay transfers come through the historian as ActionAccepted(pay) or
    # a Money event; count accepted pay verbs as a proxy.
    s.pay_transfers = sum(
        1 for ev in events
        if ev.get("kind") == "ActionAccepted"
        and _payload(ev).get("Verb") == "pay"
    )

    # Broken contracts = accepted but not completed by run end.
    for cid, status in contract_status.items():
        if status == "accepted":
            s.contracts_broken += 1

    # Manipulator defection success: count DISTINCT contract partners a
    # manipulator attacked (not raw damage events — a single betrayal is
    # many DamageDealt ticks). Conservative: only observable active
    # defections (silent walk-aways aren't in the event stream).
    defected_pairs: set[tuple[str, str]] = set()
    for atk, victim in attacks:
        if atk in manipulators and victim in manip_partners.get(atk, set()):
            defected_pairs.add((atk, victim))
    s.manipulator_defections = len(defected_pairs)

    s.top_speakers = sorted(speakers.items(), key=lambda kv: -kv[1])[:8]

    if gold_by_agent:
        s.gold_by_agent_end = dict(gold_by_agent)
        vals = [float(v) for v in gold_by_agent.values()]
        s.gold_gini_end = round(gini(vals), 4)
        s.gold_total_end = int(sum(vals))

    return s


def load_events(path: str) -> list[dict]:
    out = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            try:
                out.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return out


def digest_text(d: dict) -> str:
    """Skimmable human-readable digest of a scored run (the researcher reads
    this at a glance — see the 'design CLI views for skimming' preference)."""
    L = []
    L.append("=" * 56)
    L.append("EMERGENCE DIGEST")
    L.append("=" * 56)
    L.append(f"events: {d['total_events']}   spawns: {d['per_kind'].get('Spawned', 0)}")
    # Conflict
    L.append("")
    L.append("CONFLICT")
    L.append(f"  kills: {d['kills']}   damage events: {d['damage_events']}")
    if d.get("kill_pairs"):
        pairs = ", ".join(f"{k}->{v}" for k, v in d["kill_pairs"][:5])
        L.append(f"  kill pairs: {pairs}")
    # Economy
    L.append("")
    L.append("ECONOMY")
    L.append(f"  pay transfers: {d['pay_transfers']}   item transfers: {d['item_transfers']}")
    if d.get("gold_total_end") is not None:
        L.append(f"  gold total: {d['gold_total_end']}   wealth gini: {d['gold_gini_end']}")
    # Contracts
    L.append("")
    L.append("CONTRACTS (verbal)")
    L.append(f"  proposed:{d['contracts_proposed']} accepted:{d['contracts_accepted']} "
             f"completed:{d['contracts_completed']} rejected:{d['contracts_rejected']} "
             f"broken:{d['contracts_broken']}")
    # Social
    L.append("")
    L.append("SOCIAL")
    L.append(f"  speech:{d['speech_count']} shout:{d['shout_count']} whisper:{d['whisper_count']} "
             f"mental_notes:{d['mental_notes']}")
    if d.get("top_speakers"):
        L.append("  top speakers: " + ", ".join(f"{s}({n})" for s, n in d["top_speakers"][:5]))
    # Event spectrum
    L.append("")
    L.append("EVENT SPECTRUM")
    for k, n in sorted(d["per_kind"].items(), key=lambda kv: -kv[1]):
        L.append(f"  {n:7d}  {k}")
    return "\n".join(L)


def main(argv: list[str]) -> int:
    import argparse
    ap = argparse.ArgumentParser(prog="score_run")
    ap.add_argument("events", help="path to events.jsonl")
    ap.add_argument("--gold", help="JSON file: {entity_id: gold} at run end")
    ap.add_argument("--manipulators", help="comma-separated manipulator entity_ids")
    ap.add_argument("--out", help="write score JSON here (default stdout)")
    ap.add_argument("--text", action="store_true",
                    help="print a skimmable human-readable digest instead of JSON")
    a = ap.parse_args(argv)

    events = load_events(a.events)
    gold = None
    if a.gold:
        with open(a.gold) as f:
            gold = json.load(f)
    manips = set((a.manipulators or "").split(",")) - {""}
    score = score_events(events, gold_by_agent=gold, manipulators=manips)
    d = score.to_dict()
    if a.text:
        print(digest_text(d))
        return 0
    blob = json.dumps(d, indent=2)
    if a.out:
        with open(a.out, "w") as f:
            f.write(blob)
        print(f"wrote {a.out}")
    else:
        print(blob)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
