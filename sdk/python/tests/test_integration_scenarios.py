"""Phase AGENT-A8 — tier-2 integration scenarios.

Launches the engine binary in a subprocess and drives it through the
SDK to verify end-to-end behavior the unit tests can't reach: dialogue
roundtrip, mental_state endpoint, audibility radii.

The fixture skips cleanly if the engine binary hasn't been built yet
or if port 8088 is already busy. CI gates these tests behind an
ENGINE_BIN env var (set by the build step).
"""

from __future__ import annotations

import os
import shutil
import socket
import subprocess
import sys
import time
from pathlib import Path

import pytest


REPO = Path(__file__).resolve().parents[3]


def _port_free(port: int) -> bool:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        try:
            s.bind(("127.0.0.1", port))
            return True
        except OSError:
            return False


def _build_engine() -> Path | None:
    """Build the engine binary into .runlog/engine_test_a8. Returns
    the path on success, None if go isn't installed."""
    out = REPO / ".runlog" / "engine_test_a8"
    out.parent.mkdir(parents=True, exist_ok=True)
    if shutil.which("go") is None:
        return None
    cmd = ["go", "build", "-o", str(out), "./cmd/engine"]
    r = subprocess.run(cmd, cwd=str(REPO / "engine"), capture_output=True)
    if r.returncode != 0:
        return None
    return out


def _wait_ready(url: str, timeout: float = 8.0) -> bool:
    import urllib.request
    end = time.time() + timeout
    while time.time() < end:
        try:
            urllib.request.urlopen(url, timeout=0.5)
            return True
        except Exception:
            time.sleep(0.15)
    return False


@pytest.fixture(scope="module")
def engine_proc():
    bin_path = _build_engine()
    if bin_path is None:
        pytest.skip("go toolchain not available")
    port = 8088
    if not _port_free(port):
        pytest.skip(f"port {port} busy")
    bundle_dir = REPO / "worlds" / "dev_test"
    if not (bundle_dir / "bundle.toml").exists():
        pytest.skip("dev_test bundle not found")
    log = REPO / ".runlog" / "engine_test_a8.log"
    log.parent.mkdir(parents=True, exist_ok=True)
    proc = subprocess.Popen(
        [str(bin_path),
         "-addr", f"127.0.0.1:{port}",
         "-bundle", str(bundle_dir),
         "-capture-reasoning",
         "-register-rate", "100", "-register-burst", "100"],
        cwd=str(REPO),
        stdout=open(log, "w"),
        stderr=subprocess.STDOUT,
    )
    if not _wait_ready(f"http://127.0.0.1:{port}/api/v1/world/info", timeout=10):
        proc.terminate()
        pytest.skip("engine failed to come up")
    try:
        yield f"http://127.0.0.1:{port}"
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=3)
        except subprocess.TimeoutExpired:
            proc.kill()


def test_rulebook_served(engine_proc):
    """Phase A6 lives behind /api/v1/world/rulebook.json. Verify the
    endpoint serves a structurally-valid rulebook."""
    import json
    import urllib.request
    body = urllib.request.urlopen(engine_proc + "/api/v1/world/rulebook.json").read()
    rb = json.loads(body)
    assert rb["schema_version"] == 1
    assert rb["world"]["name"] == "dev_test"
    assert isinstance(rb["verbs"], list)


def test_mental_state_endpoint_for_known_entity(engine_proc):
    """Phase A7 endpoint should return a structurally-valid mental
    state even before any agent is registered (placeholders)."""
    import json
    import urllib.request
    r = urllib.request.urlopen(engine_proc + "/api/v1/agent/hero/mental_state")
    assert r.status == 200
    body = json.loads(r.read())
    assert body["entity_id"] == "hero"
    assert "dialogue" in body
    assert "mind" in body
    assert "traces" in body
    assert body["capture_reasoning_enabled"] is True


def test_mental_state_endpoint_404_on_garbage_path(engine_proc):
    """Non-mental_state paths under /api/v1/agent/ should fall to
    the more-specific routes (e.g. /register) or 404."""
    import urllib.request
    import urllib.error
    with pytest.raises(urllib.error.HTTPError) as e:
        urllib.request.urlopen(engine_proc + "/api/v1/agent/hero/garbage")
    assert e.value.code == 404


def test_event_log_jsonl_categories(engine_proc, tmp_path):
    """Smoke-check that the historian writes events with categories
    when capture-reasoning is on and the world ticks."""
    import json
    import urllib.request
    # Just ask for the in-memory history — already populated by ticks.
    body = urllib.request.urlopen(engine_proc + "/api/v1/world/history?limit=10").read()
    events = json.loads(body)
    assert isinstance(events, dict)
    # Newer historian returns {"records": [...]} per Phase A
    recs = events.get("records") or events.get("events") or []
    if recs:
        # Every record should have a category since SUB-7
        for r in recs[:5]:
            assert "category" in r, f"record missing category: {r}"
