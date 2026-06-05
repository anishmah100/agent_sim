# Qwen harness — reference agent

A reference 4-layer brain (persona / reflective / tactical / reflex)
driving an agent_sim character via the local Qwen 3.6-27B llama-server
on port 8782.

**Why a separate harness?** Per the architecture decision, Qwen + Claude
get DIFFERENT optimized harnesses — Qwen on the local rig needs:

- **Grammar-constrained decoding** (GBNF) so its JSON output is always
  schema-valid. Reasoning-budget=0 mode is fast but loose without a grammar.
- **Flatter prompts** — no nested instructions, one task per call.
- **Compass-mode observation** rather than absolute coords — fewer tokens.
- **Slower reflection cadence** — local-CPU/GPU inference is the
  bottleneck, so reflection runs every ~120 sim-seconds (2× Claude's
  60s) and the prompt is half the size.

## Layout

```
examples/qwen_agent/
  README.md
  main.py            — entry point; lifecycle on port 8782 LLM
  harness.py         — extends the base Harness with Qwen-tuned cadence
  qwen_llm.py        — OpenAI-compat client that hits :8782 with
                       grammar-constrained requests
  grammar/
    tactical.gbnf    — action batch schema
    reflective.gbnf  — reflection schema
    persona.gbnf     — persona schema
```

Run (assumes the llama-server is already up — see [[reference_local_llm]]
in user memory, or `tools/start_qwen.sh`):

```
python -m examples.qwen_agent.main \\
    --server http://127.0.0.1:8080 \\
    --token dev \\
    --qwen-url http://127.0.0.1:8782/v1
```

## Capacity

A 4090 sustains ~3–5 concurrent Qwen 3.6-27B Q4_K_M sequences at
1–3 second tactical-cycle latency. Capacity is measured empirically
during Phase A5 + tuned for the Eldoria live-agent roster in Wave 6.
