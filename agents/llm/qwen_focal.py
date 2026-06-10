"""Qwen-driven focal agent (P7.1).

A single-layer observe → decide → act loop. Each cycle:
  1. Render the observation into a compact prompt (prompt.build_prompt)
  2. Call local Qwen with the full-verb GBNF grammar (always-valid JSON)
  3. Map the chosen actions to SDK Action objects
  4. Submit them as an ActionBatch + emit the reasoning as a mental note

This is the SUBJECT of an emergence experiment — the model whose
social behaviour we study. Unlike the rule-based baselines it has no
hard-coded FSM; it decides freely from the action menu.

Design choices:
  - Grammar-constrained decoding (reasoning-budget=0 per the local
    Qwen reference) → output is always schema-valid, no defensive
    JSON parsing of model slop.
  - The LLM call runs in a thread (httpx sync client) so the asyncio
    obs loop isn't blocked; a slow Qwen tick just delays THIS agent.
  - A lightweight goal string persists across cycles and is updated
    by the model itself via an optional "goal" field in its output
    (falls back to the persona's default goal).
"""
from __future__ import annotations

import asyncio
import json
import logging
import time
from dataclasses import dataclass, field
from typing import Optional

import httpx

from agent_sim_sdk import Agent, AgentCredentials, ActionBatch

from .actions import to_action
from .grammar import FOCAL_GRAMMAR
from .motor_loop import MotorLoop
from .prompt import build_prompt

log = logging.getLogger("agents.llm.qwen_focal")


def _salvage_partial(text: str) -> dict:
    """Best-effort recovery of complete action objects from a
    truncated grammar output. Scans for whole {...} objects inside the
    actions array via brace-matching; ignores any trailing partial.
    Returns {"reasoning": "...", "actions": [<complete objects>]}."""
    import re
    actions: list[dict] = []
    # Find the start of the actions array.
    idx = text.find('"actions"')
    if idx >= 0:
        arr = text[text.find("[", idx) + 1:] if "[" in text[idx:] else ""
        depth = 0
        start = -1
        for i, ch in enumerate(arr):
            if ch == "{":
                if depth == 0:
                    start = i
                depth += 1
            elif ch == "}":
                depth -= 1
                if depth == 0 and start >= 0:
                    try:
                        actions.append(json.loads(arr[start:i + 1]))
                    except json.JSONDecodeError:
                        pass
                    start = -1
    rm = re.search(r'"reasoning"\s*:\s*"([^"]*)', text)
    reasoning = rm.group(1) if rm else "(truncated)"
    if actions:
        log.info("salvaged %d action(s) from truncated output", len(actions))
    return {"reasoning": reasoning, "actions": actions}


@dataclass
class FocalConfig:
    base_url: str = "http://127.0.0.1:8782/v1"
    model: str = "qwen3.6-27b"
    timeout_s: float = 60.0
    temperature: float = 0.7
    # 400 was too tight: a long reasoning sentence + 1-3 action objects
    # can hit the cap mid-string, and the grammar can't rescue a
    # cut-off output (JSON parse fails with "Unterminated string").
    # 700 gives headroom; the grammar still stops as soon as the JSON
    # closes, so well-behaved short outputs cost no extra latency.
    max_tokens: int = 700
    # How many decision cycles before giving up (None = until stopped).
    max_cycles: Optional[int] = None


@dataclass
class QwenFocalAgent:
    creds: AgentCredentials
    persona: str = "You are a person trying to survive and prosper in a harsh town. You need food to avoid starving, and gold to buy what you need. Other people around you may help or harm you."
    goal: str = "Gather gold and food. Stay alive. Form useful relationships."
    cfg: FocalConfig = field(default_factory=FocalConfig)

    _stopped: bool = False
    _last_results: list[str] = field(default_factory=list)
    # Short intent memory: what the agent committed to last turn. Fed
    # back into the prompt so the model maintains a goal across cycles
    # instead of flip-flopping targets every turn and never arriving
    # anywhere (the reason LLM agents never reached coins in early runs
    # while deterministic step-toward bots collected fine).
    _intent: str = ""
    cycles: int = 0
    accepted: int = 0
    rejected: int = 0
    # Set from the first observation; lets the experiment runner map
    # this bot to its world entity without racing the /agents endpoint.
    entity_id: Optional[str] = None
    # Two-rate motor layer: reflex movement toward a standing goal between
    # the (slow) LLM deliberations. See agents/llm/motor_loop.py.
    engine_url: str = "http://127.0.0.1:8080"
    _loop: MotorLoop = field(default_factory=MotorLoop)

    def __post_init__(self) -> None:
        self._loop.engine_url = self.engine_url

    def stop(self) -> None:
        self._stopped = True

    # ---- LLM call (sync, run in a thread) ----

    def _call_llm(self, prompt: str) -> dict:
        body = {
            "model": self.cfg.model,
            "messages": [{"role": "user", "content": prompt}],
            "max_tokens": self.cfg.max_tokens,
            "temperature": self.cfg.temperature,
            "grammar": FOCAL_GRAMMAR,
        }
        url = f"{self.cfg.base_url.rstrip('/')}/chat/completions"
        with httpx.Client(timeout=self.cfg.timeout_s) as h:
            resp = h.post(url, json=body)
            resp.raise_for_status()
            data = resp.json()
        text = data["choices"][0]["message"]["content"]
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            # Rare: generation hit max_tokens mid-string so the JSON
            # never closed. Try to salvage any complete action objects
            # by closing the structure; if that fails, return an empty
            # decision (the cycle is skipped, agent retries next obs).
            return _salvage_partial(text)

    # ---- runtime ----

    async def run(self) -> None:
        """Two-rate loop: a fast reflex (motor steps toward the standing goal
        every observation) and a slow deliberation (the LLM runs in the
        background and updates the goal + fires direct verbs when it returns).
        See agents/llm/motor_loop.py."""
        ml = self._loop
        async with Agent(self.creds) as agent:
            async for obs in agent.observations():
                if self._stopped:
                    return
                if self.entity_id is None:
                    self.entity_id = obs.self.entity_id
                # MAJ-7: stop cleanly when dead instead of looping forever
                # calling the (expensive) LLM on a corpse.
                _hp = (obs.self.extras or {}).get("hp")
                try:
                    if _hp is not None and int(_hp) <= 0:
                        log.info("[%s] dead (hp=%s) — stopping agent loop",
                                 self.creds.agent_id, _hp)
                        return
                except (TypeError, ValueError):
                    pass
                if (self.cfg.max_cycles is not None
                        and self.cycles >= self.cfg.max_cycles):
                    return

                ml.ensure_motor()
                ml.observe(obs)

                # 1) Collect a finished background deliberation, if any, and
                #    act on it (set goal + fire direct verbs).
                decision = ml.take_decision(obs)
                direct_emitted = False
                if decision is not None:
                    direct_emitted = await self._handle_decision(agent, obs, decision)

                # 2) Kick off the next deliberation in the background if due.
                if ml.should_deliberate(obs):
                    self.cycles += 1
                    prompt = build_prompt(obs, self.persona, self.goal,
                                          self._last_results, intent=self._intent)
                    ml.start_deliberation(prompt, self._call_llm)

                # 3) Reflex movement toward the standing goal — unless we
                #    already fired a direct verb this tick.
                if not direct_emitted:
                    step = ml.reflex_step(obs)
                    if step is not None:
                        try:
                            await agent.act_batch(ActionBatch(actions=[step]))
                        except Exception as e:
                            log.warning("[%s] reflex step failed: %s",
                                        self.creds.agent_id, e)

    async def _handle_decision(self, agent, obs, decision: dict) -> bool:
        """Apply a finished LLM decision: update the standing goal from any
        movement verb, emit the direct verbs, record results + a mental note.
        Returns True if a direct verb was submitted this tick."""
        ml = self._loop
        reasoning = str(decision.get("reasoning", ""))[:200]
        raw_actions = decision.get("actions") or []
        direct_dicts = ml.apply_movement(raw_actions)
        actions = [a for a in (to_action(d) for d in direct_dicts) if a]
        # Intent = standing movement goal + the direct verbs chosen.
        self._intent = "; ".join([str(ml.goal)] + [a.verb for a in actions])[:200]
        log.info("[%s] cycle %d: goal=%s direct=%s | %s",
                 self.creds.agent_id, self.cycles, ml.goal,
                 [a.verb for a in actions], reasoning)
        try:
            await agent.note(reasoning, tag="llm",
                             slots={"goal": str(ml.goal),
                                    "plan": " ".join(a.verb for a in actions) or "move"})
        except Exception:
            pass
        if not actions:
            return False
        try:
            results = await agent.act_batch(
                ActionBatch(actions=actions, reasoning=reasoning),
                wait_for_acks=True, timeout=5.0)
        except Exception as e:
            log.warning("[%s] act_batch failed: %s", self.creds.agent_id, e)
            return False
        self._last_results = []
        for act, res in zip(actions, results):
            if res is None:
                continue
            if res.accepted:
                self.accepted += 1
                self._last_results.append(f"{act.verb}: accepted")
            else:
                self.rejected += 1
                reason = res.reason or res.reason_code or "rejected"
                self._last_results.append(f"{act.verb}: {reason}")
        if any(not r.endswith("accepted") for r in self._last_results):
            log.info("[%s] acks: %s", self.creds.agent_id, self._last_results)
        return True
