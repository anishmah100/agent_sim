"""Phase AGENT-A5 — verify Qwen client wiring without needing the
server up. The actual live-run smoke is gated to the Wave 6 phase."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from unittest.mock import MagicMock

import pytest

_REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(_REPO))

from examples.qwen_agent.qwen_llm import QwenLLM, is_local_qwen_up  # noqa: E402


def test_grammar_files_exist_and_nonempty():
    qwen = QwenLLM()
    assert qwen._persona_grammar, "persona.gbnf empty"
    assert qwen._reflective_grammar, "reflective.gbnf empty"
    assert qwen._tactical_grammar, "tactical.gbnf empty"


def test_tactical_grammar_mentions_known_verbs():
    qwen = QwenLLM()
    g = qwen._tactical_grammar
    for verb in ("move", "speak", "wait", "look_at", "pay"):
        assert verb in g, f"tactical grammar missing verb {verb!r}"


def test_is_local_qwen_up_returns_bool_on_unreachable():
    # Point at a likely-empty port — function should return False
    # cleanly (no exception).
    assert is_local_qwen_up("http://127.0.0.1:9") is False


def test_qwen_call_posts_grammar(monkeypatch):
    qwen = QwenLLM(base_url="http://stub:1234/v1")
    # Patch httpx.Client to capture the request body.
    captured: dict = {}

    class _FakeResponse:
        def raise_for_status(self):
            pass

        def json(self):
            return {
                "choices": [
                    {"message": {"content": '{"reasoning":"r","actions":[]}'}}
                ]
            }

    class _FakeClient:
        def __init__(self, *a, **k):
            pass

        def __enter__(self):
            return self

        def __exit__(self, *a):
            return False

        def post(self, url, json):
            captured["url"] = url
            captured["body"] = json
            return _FakeResponse()

    import examples.qwen_agent.qwen_llm as mod
    monkeypatch.setattr(mod, "httpx", MagicMock(Client=_FakeClient))

    out = qwen.tactical("prompt here")
    assert out == {"reasoning": "r", "actions": []}
    assert captured["url"].endswith("/chat/completions")
    assert "grammar" in captured["body"]
    # Tactical grammar should be supplied; spot-check.
    assert "speak" in captured["body"]["grammar"]


def test_persona_uses_persona_grammar(monkeypatch):
    qwen = QwenLLM()
    captured: dict = {}

    class _FakeResponse:
        def raise_for_status(self):
            pass

        def json(self):
            return {
                "choices": [
                    {"message": {"content":
                        '{"long_term_values":["x"],'
                        '"initial_goals":[{"goal":"g","why":"w"}]}'
                    }}
                ]
            }

    class _FakeClient:
        def __init__(self, *a, **k):
            pass

        def __enter__(self):
            return self

        def __exit__(self, *a):
            return False

        def post(self, url, json):
            captured["body"] = json
            return _FakeResponse()

    import examples.qwen_agent.qwen_llm as mod
    monkeypatch.setattr(mod, "httpx", MagicMock(Client=_FakeClient))
    out = qwen.persona("hi")
    assert "long_term_values" in out
    assert "long_term_values" in captured["body"]["grammar"]
