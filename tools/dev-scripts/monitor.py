"""Live world-activity monitor. Samples every few seconds and prints a
compact line so we SEE whether the world is dynamic instead of waiting
blind. Flags STALE when nothing is happening.

Usage: python3 tools/dev-scripts/monitor.py [seconds] [interval]
"""
import json, sys, time, urllib.request
from collections import deque

ENGINE = "http://127.0.0.1:8080"
DUR = int(sys.argv[1]) if len(sys.argv) > 1 else 600
IVL = float(sys.argv[2]) if len(sys.argv) > 2 else 3.0
EVENTS = ".runlog/events.jsonl"

def get(path):
    try:
        return json.load(urllib.request.urlopen(ENGINE + path, timeout=4))
    except Exception:
        return None

def social_totals():
    d = get("/api/v1/social") or {"edges": []}
    atk = sum(e["attack"] for e in d["edges"])
    ctr = sum(e["contract"] for e in d["edges"])
    pay = sum(e["pay"] for e in d["edges"])
    trd = sum(e["trade"] for e in d["edges"])
    return atk, ctr, pay, trd

def positions():
    d = get("/api/v1/agents") or {"agents": []}
    return {a["entity_id"]: tuple(a["pos"]) for a in d.get("agents", []) if a.get("pos")}

def event_counts():
    # tail the event log, count interesting kinds in the last chunk
    c = {"pickup": 0, "death": 0}
    try:
        with open(EVENTS) as f:
            lines = deque(f, maxlen=4000)
    except Exception:
        return c
    for ln in lines:
        s = ln.lower()
        if '"verb":"pickup"' in s or '"pickup"' in s: c["pickup"] += 1
        if "entitydied" in s or "death_scream" in s: c["death"] += 1
    return c

prev_pos = positions()
a0, c0, p0, t0 = social_totals()
e0 = event_counts()
start = time.time()
print(f"{'t':>4} {'agents':>6} {'moving':>6} {'+atk':>5} {'+ctr':>5} {'+pay':>5} {'+trd':>5} {'+pick':>6} {'+death':>6}  note")
stale = 0
while time.time() - start < DUR:
    time.sleep(IVL)
    pos = positions()
    moving = sum(1 for k, v in pos.items() if k in prev_pos and prev_pos[k] != v)
    a1, c1, p1, t1 = social_totals()
    e1 = event_counts()
    datk, dctr, dpay, dtrd = a1-a0, c1-c0, p1-p0, t1-t0
    dpick, ddeath = e1["pickup"]-e0["pickup"], e1["death"]-e0["death"]
    activity = moving + datk + dctr + dpay + dtrd + max(0,dpick) + max(0,ddeath)
    note = ""
    if activity == 0:
        stale += 1
        note = f"STALE x{stale}  <-- nothing happening"
    else:
        stale = 0
    el = int(time.time()-start)
    print(f"{el:>4} {len(pos):>6} {moving:>6} {datk:>5} {dctr:>5} {dpay:>5} {dtrd:>5} {dpick:>6} {ddeath:>6}  {note}", flush=True)
    prev_pos = pos
    a0, c0, p0, t0 = a1, c1, p1, t1
    e0 = e1
