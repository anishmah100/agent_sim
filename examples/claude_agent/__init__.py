"""examples.claude_agent — reference Claude-driven 4-layer agent.

See README.md for a tour. Live API calls are feature-flagged off until
the Anthropic key + --enable-claude lands.
"""

from .harness import Harness, LLMClient
from .state import AgentRegisterEntry, BrainState, GoalStackEntry, Persona
from .stub_llm import StubLLM

__all__ = [
    "Harness", "LLMClient",
    "AgentRegisterEntry", "BrainState", "GoalStackEntry", "Persona",
    "StubLLM",
]
