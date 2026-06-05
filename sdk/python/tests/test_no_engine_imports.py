"""Phase AGENT-LINT — the SDK + reference agents must never reach
into Go engine internals.

A researcher should be able to clone the repo, open `examples/claude_agent/`,
and grok the architecture without reading any `engine/internal/...`
file. This test enforces that wall.
"""

from __future__ import annotations

import ast
import sys
from pathlib import Path

import pytest

_REPO = Path(__file__).resolve().parents[3]

# Roots that ARE allowed to import from "engine" — currently none.
# Test-only utilities that need to read engine output are routed via
# stdlib subprocess + HTTP, not Go-package imports.

ALLOWED_ROOTS: set[Path] = set()

# Roots that must NOT import engine.
SCAN_ROOTS = [
    _REPO / "sdk" / "python",
    _REPO / "examples",
]


def _all_python_files() -> list[Path]:
    files: list[Path] = []
    for root in SCAN_ROOTS:
        if not root.exists():
            continue
        for p in root.rglob("*.py"):
            if "__pycache__" in p.parts:
                continue
            files.append(p)
    return files


def _imports_engine(src: str) -> list[str]:
    """Return offending lines if the source imports anything starting
    with 'engine.' — empty list when clean."""
    try:
        tree = ast.parse(src)
    except SyntaxError:
        return []
    bad: list[str] = []
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                if alias.name == "engine" or alias.name.startswith("engine."):
                    bad.append(f"import {alias.name}")
        elif isinstance(node, ast.ImportFrom):
            mod = node.module or ""
            if mod == "engine" or mod.startswith("engine."):
                bad.append(f"from {mod} import ...")
    return bad


@pytest.mark.parametrize("py_file", _all_python_files(), ids=lambda p: str(p.relative_to(_REPO)))
def test_no_engine_imports(py_file: Path):
    if py_file in ALLOWED_ROOTS:
        pytest.skip("file is explicitly allow-listed")
    src = py_file.read_text(encoding="utf-8")
    offenses = _imports_engine(src)
    assert not offenses, (
        f"{py_file.relative_to(_REPO)} imports from the Go engine package: "
        f"{offenses}. Researchers reading examples/ shouldn't need engine internals."
    )


def test_scan_covered_nonempty_set():
    # Sanity: the scan picked up SOMETHING, otherwise the test is silently
    # a no-op.
    assert len(_all_python_files()) > 5
