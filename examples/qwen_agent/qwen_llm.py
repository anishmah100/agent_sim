"""QwenLLM — OpenAI-compatible client for the local llama-server at
:8782. Uses GBNF grammar-constrained decoding so the JSON output is
always schema-valid even with reasoning-budget=0.

Implements the LLMClient Protocol declared in
examples.claude_agent.harness, so the same Harness class drives both
backends.
"""

from __future__ import annotations

import json
import logging
import os
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional

import httpx


_log = logging.getLogger("qwen_llm")


GRAMMAR_DIR = Path(__file__).parent / "grammar"


def _load(name: str) -> str:
    return (GRAMMAR_DIR / name).read_text(encoding="utf-8")


@dataclass
class QwenLLM:
    """Talks OpenAI-compat (the surface llama.cpp's `--api` serves).

    Per the maintainer's reference_local_llm memory:
      ./llama.cpp/build/bin/llama-server -m models/Qwen3.6-27B-Q4_K_M.gguf \
          -t 32 --reasoning-budget 0 --port 8782
    """

    base_url: str = "http://127.0.0.1:8782/v1"
    model: str = "qwen3.6-27b"
    timeout_s: float = 180.0
    # Cached grammars.
    _persona_grammar: str = field(default_factory=lambda: _load("persona.gbnf"))
    _reflective_grammar: str = field(default_factory=lambda: _load("reflective.gbnf"))
    _tactical_grammar: str = field(default_factory=lambda: _load("tactical.gbnf"))

    # ---- LLMClient protocol ----

    def persona(self, prompt: str, max_tokens: int = 500) -> dict:
        return self._call(prompt, self._persona_grammar, max_tokens, layer="persona")

    def reflect(self, prompt: str, max_tokens: int = 500) -> dict:
        return self._call(prompt, self._reflective_grammar, max_tokens, layer="reflect")

    def tactical(self, prompt: str, max_tokens: int = 600) -> dict:
        # 200 was too tight: a single reasoning sentence eats ~150-180
        # tokens by itself, leaving the 1-3 action objects + their
        # nested keys to be truncated mid-stream, and the grammar can't
        # rescue a cut-off output. 600 gives generous headroom and the
        # grammar still stops generation early when the JSON closes.
        return self._call(prompt, self._tactical_grammar, max_tokens, layer="tactical")

    # ---- internals ----

    def _call(self, prompt: str, grammar: str, max_tokens: int,
              layer: str = "?") -> dict:
        """Submit the prompt with the GBNF grammar attached. Returns
        the parsed JSON object.

        The llama.cpp OpenAI-compat layer accepts an extra `grammar`
        field outside the OpenAI spec — that's what enforces the
        schema during decoding.
        """
        body: dict[str, Any] = {
            "model": self.model,
            "messages": [
                {"role": "user", "content": prompt},
            ],
            "max_tokens": max_tokens,
            "temperature": 0.7,
            "grammar": grammar,
        }
        url = f"{self.base_url.rstrip('/')}/chat/completions"
        t0 = time.monotonic()
        with httpx.Client(timeout=self.timeout_s) as h:
            resp = h.post(url, json=body)
            resp.raise_for_status()
            data = resp.json()
        dt_ms = int((time.monotonic() - t0) * 1000)
        text = data["choices"][0]["message"]["content"]
        # Per-call diagnostic: layer + wall-clock + token usage if the
        # server emitted it. The smoke scorer uses these to compute
        # p50/p99 tactical latencies + spot Qwen overload at scale.
        usage = data.get("usage") or {}
        _log.info("llm[%s] %dms prompt=%dch resp=%dch usage=%s",
                  layer, dt_ms, len(prompt), len(text),
                  {k: usage.get(k) for k in ("prompt_tokens", "completion_tokens")} if usage else "{}")
        try:
            return json.loads(text)
        except json.JSONDecodeError as e:
            # Grammar should make this impossible; surface loudly if not.
            raise RuntimeError(
                f"Qwen output failed JSON parse despite grammar: {text!r}"
            ) from e


def is_local_qwen_up(base_url: Optional[str] = None) -> bool:
    """Probe the llama-server's /v1/models endpoint. Returns False if
    the server isn't reachable. Used by smoke tests to skip cleanly."""
    url = (base_url or "http://127.0.0.1:8782/v1").rstrip("/") + "/models"
    try:
        with httpx.Client(timeout=1.0) as h:
            return h.get(url).status_code < 500
    except Exception:
        return False


def env_base_url(default: str = "http://127.0.0.1:8782/v1") -> str:
    return os.environ.get("QWEN_BASE_URL", default)
