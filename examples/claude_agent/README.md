# Claude harness — reference agent

A reference 4-layer brain (persona / reflective / tactical / reflex)
driving an agent_sim character via the Anthropic API.

**Status (June 2026):** code lands feature-flagged — the harness skeleton
is committed, but actual Anthropic API calls only fire when both:

- `ANTHROPIC_API_KEY` is set in the env, AND
- `main.py` is invoked with `--enable-claude` (defaults to off)

Without the flag the harness uses a deterministic stub LLM so unit
tests + smoke runs are reproducible. When you're ready to burn tokens,
flip the switch.

## Layout

```
examples/claude_agent/
  README.md          — this file
  main.py            — entry point; loops observation → batch
  harness.py         — Harness class + 4 layer methods
  state.py           — BrainState (persona, GoalStack, AgentRegister, notes)
  prompts.py         — prompt templates (tactical + reflective)
  stub_llm.py        — deterministic stub for tests + no-API mode
```

The persona / reflective / tactical / reflex methods on Harness mirror
the architecture plan exactly (see `docs/AGENT_ARCHITECTURE_PLAN.md`).
Each is a small Python function; the LLM client is injected so
swapping a stub for the real Anthropic client is a one-line change.

## Why feature-flag the API

Per the locked decisions (see `PROGRESS.md` row 1) the user is holding
off on the API key until the Qwen path is verified to produce
interesting behavior end-to-end. The harness ships so it can be
inspected, reviewed, and tested against the stub.

## Reading the code

Start at `main.py`. It:

1. Calls `register_and_connect` with `share_reasoning=True` so the
   engine captures the tactical brain's per-action `reasoning`.
2. Constructs a `Harness` with the stub LLM.
3. Drives the observation loop: each obs goes into `harness.tactical`,
   which returns an `ActionBatch`.

The reflective layer runs on a staggered timer; the persona layer runs
once at startup.
