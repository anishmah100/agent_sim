"""LLM clients for the narrator.

Two real backends + a stub:

- ``QwenClient`` talks to local llama.cpp's OpenAI-compat HTTP API
  on port 8782 (per the reference-local-llm memory). No network
  surprises; fast; cheap.
- ``ClaudeClient`` uses the anthropic SDK if installed; reads the
  API key from ANTHROPIC_API_KEY in env (or .env.local via
  python-dotenv). L3 + L4 only.
- ``StubLLM`` returns a deterministic mock; used by tests + dry runs.

All three share a ``summarize(prompt, *, max_tokens)`` surface and a
``calls`` counter so the narrator can refuse calls past the budget cap.
"""
from __future__ import annotations

import json
import logging
import os
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from typing import Optional, Protocol


log = logging.getLogger("agent_sim.narrator.llm")


class LLM(Protocol):
    calls: int
    def summarize(self, prompt: str, *, max_tokens: int = 200) -> str: ...


# --- Stub ---


@dataclass
class StubLLM:
    """Returns the input prompt prefixed with a tag. Deterministic;
    no network. Counts calls so budget caps can still be exercised."""

    name: str = "stub"
    calls: int = 0
    refuse_after: Optional[int] = None

    def summarize(self, prompt: str, *, max_tokens: int = 200) -> str:
        if self.refuse_after is not None and self.calls >= self.refuse_after:
            raise BudgetExceeded(self.name, self.calls)
        self.calls += 1
        # Crude excerpt: first ~3 lines of the prompt's event block.
        lines = [l for l in prompt.splitlines() if l.strip()]
        head = " | ".join(lines[-3:])[:120]
        return f"[{self.name}#{self.calls}] {head}"


# --- Qwen (local llama.cpp) ---


@dataclass
class QwenClient:
    endpoint: str = "http://127.0.0.1:8782/v1"
    model: str = "qwen3.6"
    timeout: float = 30.0
    calls: int = 0
    refuse_after: Optional[int] = None

    def summarize(self, prompt: str, *, max_tokens: int = 200) -> str:
        if self.refuse_after is not None and self.calls >= self.refuse_after:
            raise BudgetExceeded("qwen", self.calls)
        body = {
            "model": self.model,
            "messages": [
                {
                    "role": "system",
                    "content": (
                        "You are a concise narrator of an agent-based "
                        "simulation. Given a stream of events, produce "
                        "one short factual paragraph (under 80 words) "
                        "describing what just happened. Do not invent "
                        "facts. No bullet points; one paragraph."
                    ),
                },
                {"role": "user", "content": prompt},
            ],
            "temperature": 0.3,
            "max_tokens": max_tokens,
        }
        req = urllib.request.Request(
            f"{self.endpoint.rstrip('/')}/chat/completions",
            data=json.dumps(body).encode("utf-8"),
            headers={"Content-Type": "application/json"},
            method="POST",
        )
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as r:
                raw = r.read()
        except (urllib.error.URLError, OSError) as e:
            raise LLMUnavailable("qwen", str(e)) from e
        self.calls += 1
        data = json.loads(raw)
        choices = data.get("choices") or []
        if not choices:
            return ""
        msg = (choices[0].get("message") or {}).get("content") or ""
        return msg.strip()


# --- Claude (Anthropic) ---


@dataclass
class ClaudeClient:
    model: str = "claude-haiku-4-5-20251001"
    timeout: float = 60.0
    calls: int = 0
    refuse_after: Optional[int] = None
    # Optional override; defaults to env.
    api_key: Optional[str] = None
    _client: object = field(default=None, init=False, repr=False)

    def _ensure_client(self) -> None:
        if self._client is not None:
            return
        try:
            import anthropic  # type: ignore[import-not-found]
        except ImportError as e:
            raise LLMUnavailable("claude",
                "anthropic package not installed; pip install anthropic") from e
        key = self.api_key or os.environ.get("ANTHROPIC_API_KEY")
        if not key:
            # Try .env.local one directory up — common pattern in this repo.
            env_local = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                                     "..", "..", ".env.local")
            if os.path.exists(env_local):
                with open(env_local) as f:
                    for line in f:
                        line = line.strip()
                        if line.startswith("ANTHROPIC_API_KEY="):
                            key = line.split("=", 1)[1].strip().strip('"')
                            break
        if not key:
            raise LLMUnavailable("claude", "ANTHROPIC_API_KEY not set")
        self._client = anthropic.Anthropic(api_key=key)

    def summarize(self, prompt: str, *, max_tokens: int = 400) -> str:
        if self.refuse_after is not None and self.calls >= self.refuse_after:
            raise BudgetExceeded("claude", self.calls)
        self._ensure_client()
        msg = self._client.messages.create(
            model=self.model,
            max_tokens=max_tokens,
            system=(
                "You are the narrator of an agent-based simulation. "
                "Given the stream of events (and possibly lower-level "
                "narrator summaries), produce a vivid but accurate "
                "summary that captures factions, conflicts, ongoing "
                "deals, and emergent dynamics. Stay grounded in what "
                "actually happened; do not invent agents or events. "
                "Match the requested length."
            ),
            messages=[{"role": "user", "content": prompt}],
        )
        self.calls += 1
        # Concatenate all text blocks.
        parts = []
        for block in (msg.content or []):
            text = getattr(block, "text", None)
            if text:
                parts.append(text)
        return "".join(parts).strip()


# --- Exceptions ---


class BudgetExceeded(RuntimeError):
    def __init__(self, name: str, calls: int):
        super().__init__(f"{name} budget exceeded after {calls} calls")
        self.name = name
        self.calls = calls


class LLMUnavailable(RuntimeError):
    def __init__(self, name: str, reason: str):
        super().__init__(f"{name} unavailable: {reason}")
        self.name = name
        self.reason = reason
