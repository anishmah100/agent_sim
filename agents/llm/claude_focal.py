"""Claude-driven focal agent (P7 cross-model showcase).

Mirrors QwenFocalAgent's observe→decide→act loop but uses the Anthropic
API instead of local grammar-constrained Qwen. Claude doesn't support
GBNF, so we ask for strict JSON and reuse the same defensive parsing
(_salvage_partial) + to_action mapper as the Qwen path — so the two
brains are directly comparable on the SAME substrate, prompt, and
action space.

Budget: each decision is one Anthropic call. Keep agent count + run
length small; the experiment harness caps via --wall-seconds.
"""
from __future__ import annotations

import asyncio
import json
import logging
import os
import time
from dataclasses import dataclass, field
from typing import Optional

from agent_sim_sdk import Agent, AgentCredentials, ActionBatch

from .actions import to_action
from .prompt import build_prompt
from .qwen_focal import _salvage_partial  # reuse truncated-JSON recovery

log = logging.getLogger("agents.llm.claude_focal")


def _load_anthropic_key() -> Optional[str]:
    key = os.environ.get("ANTHROPIC_API_KEY")
    if key:
        return key
    # .env.local at repo root (two dirs up from agents/llm/).
    env_local = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                             "..", "..", ".env.local")
    if os.path.exists(env_local):
        with open(env_local) as f:
            for line in f:
                line = line.strip()
                if line.startswith("ANTHROPIC_API_KEY="):
                    return line.split("=", 1)[1].strip().strip('"')
    return None


_SYSTEM = (
    "You control ONE character in a multi-agent social simulation. "
    "Decide this turn's actions from the menu in the user message. "
    "Respond with ONLY a single JSON object, no prose, no markdown fences: "
    '{"reasoning":"<one short sentence, max 25 words>","actions":[<1-3 action objects>]}. '
    "Each action object uses exactly the verb + fields shown in the menu. "
    "targets are entity_ids (e.g. \"spawn_7\"), never display names."
)


@dataclass
class ClaudeFocalConfig:
    model: str = "claude-haiku-4-5-20251001"
    max_tokens: int = 500
    temperature: float = 0.7
    timeout_s: float = 60.0
    max_cycles: Optional[int] = None


@dataclass
class ClaudeFocalAgent:
    creds: AgentCredentials
    persona: str = "You are a person trying to survive and prosper in a harsh town."
    goal: str = "Gather gold and food. Stay alive. Form useful relationships."
    cfg: ClaudeFocalConfig = field(default_factory=ClaudeFocalConfig)

    _stopped: bool = False
    _last_results: list[str] = field(default_factory=list)
    _intent: str = ""
    cycles: int = 0
    accepted: int = 0
    rejected: int = 0
    entity_id: Optional[str] = None
    _client: object = field(default=None, init=False, repr=False)

    def stop(self) -> None:
        self._stopped = True

    def _ensure_client(self):
        if self._client is None:
            import anthropic
            key = _load_anthropic_key()
            if not key:
                raise RuntimeError("ANTHROPIC_API_KEY not set (env or .env.local)")
            self._client = anthropic.Anthropic(api_key=key)
        return self._client

    def _call_llm(self, prompt: str) -> dict:
        client = self._ensure_client()
        msg = client.messages.create(
            model=self.cfg.model,
            max_tokens=self.cfg.max_tokens,
            temperature=self.cfg.temperature,
            system=_SYSTEM,
            messages=[{"role": "user", "content": prompt}],
        )
        # Concatenate text blocks.
        text = "".join(getattr(b, "text", "") for b in msg.content).strip()
        # Strip accidental markdown fences.
        if text.startswith("```"):
            text = text.strip("`")
            if text.startswith("json"):
                text = text[4:]
            text = text.strip()
        try:
            return json.loads(text)
        except json.JSONDecodeError:
            return _salvage_partial(text)

    async def run(self) -> None:
        async with Agent(self.creds) as agent:
            async for obs in agent.observations():
                if self._stopped:
                    return
                if self.entity_id is None:
                    self.entity_id = obs.self.entity_id
                _hp = (obs.self.extras or {}).get("hp")
                try:
                    if _hp is not None and int(_hp) <= 0:
                        log.info("[%s] dead — stopping", self.creds.agent_id)
                        return
                except (TypeError, ValueError):
                    pass
                if (self.cfg.max_cycles is not None
                        and self.cycles >= self.cfg.max_cycles):
                    return
                self.cycles += 1
                prompt = build_prompt(obs, self.persona, self.goal,
                                      self._last_results, intent=self._intent)
                t0 = time.monotonic()
                try:
                    decision = await asyncio.to_thread(self._call_llm, prompt)
                except Exception as e:
                    log.warning("[%s] Claude call failed: %s", self.creds.agent_id, e)
                    continue
                dt_ms = int((time.monotonic() - t0) * 1000)
                reasoning = str(decision.get("reasoning", ""))[:200]
                raw_actions = decision.get("actions") or []
                actions = [a for a in (to_action(d) for d in raw_actions) if a]
                if actions:
                    plan = []
                    for a in actions:
                        if a.verb == "move" and getattr(a, "target", None):
                            plan.append(f"move toward {tuple(a.target)}")
                        else:
                            plan.append(a.verb)
                    self._intent = "; ".join(plan)[:200]
                if not actions:
                    log.info("[%s] cycle %d (%dms): no valid actions from %r",
                             self.creds.agent_id, self.cycles, dt_ms, raw_actions)
                    continue
                log.info("[%s] cycle %d (%dms): %s | %s",
                         self.creds.agent_id, self.cycles, dt_ms,
                         [a.verb for a in actions], reasoning)
                try:
                    await agent.note(reasoning, tag="claude",
                                     slots={"goal": self.goal,
                                            "plan": " ".join(a.verb for a in actions)})
                except Exception:
                    pass
                try:
                    results = await agent.act_batch(
                        ActionBatch(actions=actions, reasoning=reasoning),
                        wait_for_acks=True, timeout=5.0)
                except Exception as e:
                    log.warning("[%s] act_batch failed: %s", self.creds.agent_id, e)
                    continue
                self._last_results = []
                for act, res in zip(actions, results):
                    if res is None:
                        continue
                    if res.accepted:
                        self.accepted += 1
                        self._last_results.append(f"{act.verb}: accepted")
                    else:
                        self.rejected += 1
                        self._last_results.append(
                            f"{act.verb}: {res.reason or res.reason_code or 'rejected'}")
