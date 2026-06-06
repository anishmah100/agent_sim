# Social emergence — literature snapshot

Output from the deep-research workflow (2026-06-06, 108 sub-agents, 26
sources, 25 verified claims, 20 confirmed / 5 killed). Frozen for
citation. Subsequent design decisions reference this file.

## TL;DR

The literature offers strong primitives but no canonical large-scale
emergent-AI benchmark. Existing work clusters at two extremes:

- **Small qualitative sandboxes** — Smallville/Generative Agents (25
  agents, Park 2023), CRSEC (10 agents, 2024). Demonstrate emergent
  behaviors but evaluate via human raters or LLM-as-judge.
- **Large targeted-phenomenon platforms** — AgentSociety (10k+ agents,
  2025), Concordia Contest (25 agents × 5 substrates, 2024). Prioritize
  specific social-science questions over open-ended emergence.

**The gap agent_sim could occupy:** a reproducible, mid-scale (~100-agent)
open-world benchmark with tunable mechanism-design knobs, held-out
evaluation scenarios, and non-circular emergence metrics.

## Key findings (verified)

### 1. The mid-scale open-world gap is real

Smallville reference impl ships 3-agent and 25-agent base sims only
([Park 2023](https://github.com/joonspk-research/generative_agents)).
CRSEC ran 10 agents ([CRSEC 2024](https://arxiv.org/pdf/2403.08251)).
AgentSociety targets specific phenomena (polarization, UBI, hurricanes)
not open-ended emergence ([AgentSociety 2025](https://arxiv.org/abs/2502.08691)).
Concordia uses 25 LLM agents across 5 mixed-motive substrates
([Concordia 2024](https://arxiv.org/html/2512.03318v1)). Melting Pot
uses RL-trained background populations, not LLM agents
([Leibo 2021](https://arxiv.org/abs/2107.06857)).

The ~100-agent persistent-world tile-RPG niche with rich action space
(trade, whisper, attack, propose_task) is **not occupied**.

### 2. Concordia Contest = strongest methodology template

Five mixed-motive substrates: Reality Show, Pub Coordination, Haggling,
Labor Collective Action, State Formation. Scoring uses Elo + min-max
normalized [0,1]. Key findings ([Concordia 2024](https://arxiv.org/html/2512.03318v1)):

- Only 5 of 25 evaluated LLM agents performed significantly better than
  a rational baseline.
- Average normalized score: 0.426 ± 0.005 SE — "considerable room for
  progress."
- "Veil of ignorance" design (zero-shot held-out scenarios) successfully
  prevented co-design exploitation.
- **Multiple participants reported markedly higher dev-phase scores than
  eval-phase scores.** Direct evidence of overfitting risk in LLM agent
  benchmarks. This is the strongest extant argument for holding out the
  eval scenarios from agent authors.

### 3. Nowak's five mechanisms = tunable design knobs

[Nowak 2006, Science 314:1560-1563](https://www.science.org/doi/abs/10.1126/science.1133755)
identifies five cooperation mechanisms. Each has an analytic rule:

- **Kin selection**: r > c/b (relatedness > cost/benefit)
- **Direct reciprocity**: w > c/b (repeat-encounter probability > c/b)
- **Indirect reciprocity**: q > c/b (reputation-reach probability > c/b)
- **Network reciprocity**: spatial/structured-population mechanism (the
  simple b/c > k rule was *refuted* in our verification — see caveats)
- **Group selection**: between-group competition mechanism

Tunable knobs for agent_sim:
- **w** = respawn rate / encounter probability (longer lives + small
  population → higher w → cooperation favored)
- **q** = gossip channel reach / persistence
- **Spatial locality** = tile-map structure (graph reciprocity is the
  mechanism, not merely a setting)

### 4. Gossip topology is non-obvious

[GODS 2025](https://arxiv.org/abs/2511.20248) — locally-constrained
gossip *promotes defection*, not cooperation. Contradicts the standard
intuition. Implication: gossip-channel design (whisper vs broadcast,
range, decay) is first-class. Wrong topology produces opposite emergence.

*Caveat: single Nov 2025 arXiv preprint, not peer-reviewed. Treat as
hypothesis to test, not as established rule.*

### 5. Evaluation crisis in LLM-ABM is documented

[Larooij & Törnberg 2025, AIR](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC12627210/):

- Surveyed 35 LLM-based ABM studies.
- **22 of 35** use subjective assessment as primary validation.
- **15 of 35** rely SOLELY on subjective methods.
- LLM-as-judge has "obvious circularity" — using an LLM to evaluate
  another LLM's outputs is methodologically problematic.
- Subjective methods show "weak coupling" between stated purposes and
  actual targets.

For agent_sim: emergence metrics must be quantitative (cluster
coefficients, defection rates, wealth Gini, gossip half-life, etc.).
LLM-judge stays as a *supplementary* signal at most.

### 6. CRSEC's modular decomposition for norm emergence

[CRSEC 2024](https://arxiv.org/pdf/2403.08251) decomposes norm
emergence into four modules:

1. **Creation & Representation** — where norms originate, how stored
2. **Spreading** — how they propagate via communication / observation
3. **Evaluation** — how synthesized over time
4. **Compliance** — how incorporated into planning

Implication for substrate/author boundary:
- **Substrate exposes**: Spreading (observation + communication
  channels) + Compliance (action hooks)
- **Agent author handles**: Creation & Representation + Evaluation

### 7. Melting Pot's frozen-background pattern

[Leibo 2021](https://arxiv.org/abs/2107.06857) — "one agent's behavior
constitutes (part of) another agent's environment." Scenarios =
substrate + frozen RL background population. Focal agents evaluated
zero-shot against unseen combinations.

For agent_sim: maintain a library of frozen/scripted background bots
whose behaviors define the test environment. New agents evaluated
against held-out combinations.

## Top 5 design recommendations (literature-converging)

1. **Held-out evaluation scenarios behind a veil of ignorance.**
   Concordia demonstrated this works AND exposed overfitting risk.
   ([Concordia 2024](https://arxiv.org/html/2512.03318v1))

2. **Tunable mechanism-design knobs** (w, q, spatial locality, group
   structure). Nowak's rules give falsifiable predictions about which
   knob settings induce cooperation vs. defection.
   ([Nowak 2006](https://www.science.org/doi/abs/10.1126/science.1133755))

3. **Quantitative non-circular metrics.** Avoid LLM-as-judge for
   emergence scoring. Use cluster coefficients, Gini, half-lives,
   honor-rates.
   ([Larooij 2025](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC12627210/))

4. **Frozen-bot background populations** for zero-shot generalization
   tests.
   ([Melting Pot 2021](https://arxiv.org/abs/2107.06857))

5. **Modular substrate-vs-author decomposition** (CRSEC-style).
   Substrate provides Spreading + Compliance hooks; author owns
   Representation + Evaluation.
   ([CRSEC 2024](https://arxiv.org/pdf/2403.08251))

## Caveats (claims that did NOT survive verification, or that need
context)

- **Memory architecture for ~100-agent × weeks-of-time is empirically
  open.** Generative Agents' memory stream as the canonical primitive
  was *refuted* in verification. Voyager skill library and MemGPT
  claims did not survive. We cannot cite a settled answer here — this
  is design space to explore, not best-practice to copy.
- **Network reciprocity's b/c > k rule was refuted** as a direct
  prescription for tile-map locality. The mechanism is real; the
  simple analytic rule doesn't translate cleanly to LLM-agent worlds
  with mixed strategy spaces.
- **GODS gossip-promotes-defection** rests on a single Nov 2025
  preprint. Counterintuitive direction; warrants independent
  replication before treating as design principle.
- **AgentSociety "largest" framing is time-sensitive.** A 1B-agent
  "Light Society" exists ([arxiv 2506.12078](https://arxiv.org/abs/2506.12078)).
  Scale alone isn't differentiation.
- **Nowak's rules are necessary not sufficient.** They assume specific
  strategy spaces (TFT-family, image-scoring). Under noise, open-ended
  strategy spaces, or alternative reputation norms (Ohtsuki-Iwasa
  leading eight), thresholds shift.
- **Concordia's 25-agent sample is small-N.** The 0.426 average and
  5-of-25 result is a snapshot, not a population claim.

## Open questions surfaced (need follow-up work)

1. What memory architecture actually scales to weeks of in-game time
   for ~100 agents? Direct comparison among Generative Agents memory
   stream, Voyager skill library, MemGPT, retrieval-over-traces is
   absent at this scale.
2. What quantitative non-LLM-judge emergence metrics have been
   validated in practice? The validation crisis is documented but
   positive prescription is sparse.
3. How do mechanism-design knobs (w, q, locality) interact when
   multiple are tuned simultaneously? Nowak's rules are derived for
   isolated mechanisms.
4. What is the right substrate-vs-agent-author boundary for skills,
   plans, and long-term memory? CRSEC answers for norms but not for
   action repertoires.

## Source list (26 fetched, primary unless noted)

- [Generative Agents (Park 2023)](https://github.com/joonspk-research/generative_agents)
- [Melting Pot (Leibo 2021)](https://arxiv.org/abs/2107.06857)
- [Concordia Contest 2024](https://arxiv.org/html/2512.03318v1)
- [AgentSociety 2025](https://arxiv.org/abs/2502.08691)
- [CRSEC 2024](https://arxiv.org/pdf/2403.08251)
- [SOTOPIA topic page](https://www.emergentmind.com/topics/sotopia-interactive-social-evaluation-benchmark) (secondary)
- [Nowak 2006 PDF](https://people.bu.edu/msoren/Nowak.pdf)
- [Nowak 2006 Science](https://www.science.org/doi/abs/10.1126/science.1133755)
- [Sugarscape / Growing Artificial Societies](https://mitpress.mit.edu/9780262550253/growing-artificial-societies/) (secondary)
- [Axelrod tournaments docs](https://axelrod.readthedocs.io/en/fix-documentation/reference/description.html) (secondary)
- [GODS gossip model 2025](https://arxiv.org/abs/2511.20248)
- [Larooij & Törnberg 2025 (validation crisis)](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC12627210/)
- [LLM-as-judge limitations](https://arxiv.org/pdf/2404.16698)
- [Evaluation methods survey](https://arxiv.org/pdf/2505.20411)
- [Reproducibility pitfalls](https://arxiv.org/pdf/2410.03492)
- [Benchmark contamination](https://arxiv.org/pdf/2412.05579)
- [LLM evaluation 2025 review](https://www.goodeyelabs.com/insights/llm-evaluation-2025-review) (blog)
- [Generative Agents arxiv](https://ar5iv.labs.arxiv.org/html/2304.03442)
- [MemGPT 2023](https://arxiv.org/pdf/2310.08560)
- [Voyager 2023](https://arxiv.org/abs/2305.16291)
- [Memory architectures survey 2026a](https://arxiv.org/html/2604.11978v1)
- [Memory architectures survey 2026b](https://arxiv.org/html/2605.12493v1)
- [Recent emergence work 2024a](https://arxiv.org/abs/2411.00114)
- [Recent emergence work 2024b](https://arxiv.org/html/2411.11581v4)
- [Recent emergence work 2025a](https://arxiv.org/html/2503.01935v1)
- [Sci Adv emergence paper 2025](https://www.science.org/doi/10.1126/sciadv.adu9368)

Workflow stats: 5 search angles → 26 sources fetched → 126 claims
extracted → 25 adversarially verified → 20 confirmed / 5 killed → 8
final findings after synthesis.
