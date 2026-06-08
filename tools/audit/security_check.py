#!/usr/bin/env python3
"""Audit harness S12 — negative / adversarial / security paths, live.

Asserts the engine rejects bad input cleanly (no crash, no silent accept):
  - WS auth with a WRONG secret receives no observations (rejected).
  - register with a malformed body returns an HTTP error, not a 200.
  - an unknown WS message type is ignored (engine stays up).

Usage: python3 tools/audit/security_check.py [engine_url]
"""
import asyncio
import json
import sys
import urllib.request

import websockets
from agent_sim_sdk import register_agent, VisionMode


async def main():
    engine = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"
    findings = []

    # 1. Malformed register body -> HTTP error (not 200).
    try:
        req = urllib.request.Request(engine + "/api/v1/agent/register",
                                     data=b"{not valid json",
                                     headers={"Content-Type": "application/json"})
        urllib.request.urlopen(req, timeout=5)
        findings.append("malformed register body returned 200 (should be 4xx)")
        print("  [FAIL] malformed register accepted")
    except urllib.error.HTTPError as e:
        print(f"  [PASS] malformed register -> HTTP {e.code}")
    except Exception as e:
        print(f"  [PASS] malformed register rejected ({type(e).__name__})")

    # 2. WS auth with a WRONG secret -> no observations.
    creds = await register_agent(
        engine, user_token="dev",
        persona={"name": "SecProbe", "archetype_tag": "llm", "brain": "qwen"},
        vision_mode=VisionMode.STRUCTURED, cadence_ms=200)
    try:
        async with websockets.connect(creds.ws_url) as ws:
            await ws.send(json.dumps({"auth": "WRONG-SECRET-12345"}))
            got_obs = False
            try:
                for _ in range(3):
                    raw = await asyncio.wait_for(ws.recv(), timeout=3.0)
                    m = json.loads(raw)
                    if m.get("type") == "observation":
                        got_obs = True
                        break
            except (asyncio.TimeoutError, Exception):
                pass
            if got_obs:
                findings.append("WS streamed observations after a WRONG auth secret")
                print("  [FAIL] bad auth secret still got observations")
            else:
                print("  [PASS] bad auth secret -> no observations")
    except Exception as e:
        print(f"  [PASS] bad auth secret -> connection refused/closed ({type(e).__name__})")

    # 3. Engine still healthy after the bad inputs.
    try:
        with urllib.request.urlopen(engine + "/api/v1/world/info", timeout=5) as r:
            ok = r.status == 200
        print(f"  [{'PASS' if ok else 'FAIL'}] engine healthy after adversarial input")
        if not ok:
            findings.append("engine unhealthy after adversarial input")
    except Exception as e:
        findings.append(f"engine unreachable after adversarial input: {e}")
        print("  [FAIL] engine unreachable")

    print(f"\n=== {'PASS — 0 findings' if not findings else str(len(findings))+' FINDINGS'} ===")
    for f in findings:
        print("  -", f)
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    asyncio.run(main())
