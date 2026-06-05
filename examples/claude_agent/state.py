"""BrainState — the structured spine + free-text notes for a Claude
agent. Mirrors the architecture plan's memory schema."""

from __future__ import annotations

from collections import deque
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class GoalStackEntry:
    goal: str
    why: str
    status: str = "active"  # "active" | "done" | "abandoned"


@dataclass
class AgentRegisterEntry:
    """First-order theory-of-mind slot for one other agent."""
    entity_id: str
    last_seen_pos: tuple[int, int]
    disposition: float = 0.0      # -1 (hostile) ... +1 (allied)
    beliefs: list[str] = field(default_factory=list)
    debts: list[str] = field(default_factory=list)
    theory_of_me: str = ""        # what I think THEY believe about me
    last_seen_with: list[str] = field(default_factory=list)


@dataclass
class Persona:
    name: str
    archetype: str
    bio: str
    long_term_values: list[str] = field(default_factory=list)


@dataclass
class BrainState:
    persona: Persona
    # Goal stack: small (≤5).
    goal_stack: list[GoalStackEntry] = field(default_factory=list)
    # Sparse top-K register of other agents.
    agent_register: dict[str, AgentRegisterEntry] = field(default_factory=dict)
    register_cap: int = 20
    # Per-agent learned map (what I've personally seen).
    known_tiles: dict[tuple[int, int], dict] = field(default_factory=dict)
    # Dialogue ring buffer.
    dialogue_log: deque = field(default_factory=lambda: deque(maxlen=50))
    # Free-text scratchpads.
    tactical_notes: deque = field(default_factory=lambda: deque(maxlen=20))
    reflective_notes: deque = field(default_factory=lambda: deque(maxlen=10))

    def top_goal(self) -> Optional[GoalStackEntry]:
        for entry in self.goal_stack:
            if entry.status == "active":
                return entry
        return None

    def remember_other(self, entry: AgentRegisterEntry) -> None:
        """Insert/update an other-agent entry. Caps the register at
        register_cap via LRU-by-last_seen_pos (cheap heuristic — drop
        the entry we haven't refreshed for the longest)."""
        self.agent_register[entry.entity_id] = entry
        if len(self.agent_register) > self.register_cap:
            # Keep all top-K most-recently-updated by insertion order
            # (dict preserves insertion order in CPython).
            drop = next(iter(self.agent_register))
            del self.agent_register[drop]

    def push_tactical_note(self, note: str) -> None:
        if note:
            self.tactical_notes.append(note)

    def push_reflective_note(self, note: str) -> None:
        if note:
            self.reflective_notes.append(note)
