"""LLM-driven focal agents.

The focal agents are the SUBJECTS of an emergence experiment — the
LLMs whose social behaviour we're studying. They connect via the same
SDK as the rule-based baselines but choose actions by prompting a
language model (local Qwen by default).
"""

from .qwen_focal import QwenFocalAgent, FocalConfig

__all__ = ["QwenFocalAgent", "FocalConfig"]
