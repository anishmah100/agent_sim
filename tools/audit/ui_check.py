#!/usr/bin/env python3
"""Audit harness S8 — UI render gate (wraps the Playwright ui_smoke).

Runs tools/dev-scripts/ui_smoke.mjs (engine reachable, frontend loads with no
page/console errors, Pixi renders, no error banner) AS PART of the audit suite
rather than only manually. The smoke needs the full stack up (engine :8080 +
frontend dev server :5173); if the frontend isn't running this SKIPS cleanly
(exit 0) so run_all stays green on headless/CI boxes where only the audit
sidecar is up.

Usage: python3 tools/audit/ui_check.py
"""
import subprocess
import sys
import urllib.request
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
SMOKE = REPO / "tools" / "dev-scripts" / "ui_smoke.mjs"
FRONTEND = "http://127.0.0.1:5173/"


def reachable(url: str) -> bool:
    try:
        urllib.request.urlopen(url, timeout=2)
        return True
    except Exception:
        return False


def main() -> None:
    if not reachable(FRONTEND):
        print("  [SKIP] frontend dev server (:5173) not running — S8 UI smoke skipped")
        sys.exit(0)  # not a failure: the UI stack just isn't up
    p = subprocess.run(["node", str(SMOKE)], cwd=str(SMOKE.parent),
                       capture_output=True, text=True, timeout=120)
    print(p.stdout.strip())
    if p.returncode != 0:
        print(p.stderr.strip()[-400:])
        print("  [FAIL] UI smoke failed")
        sys.exit(1)
    print("  [PASS] UI smoke")
    sys.exit(0)


if __name__ == "__main__":
    main()
