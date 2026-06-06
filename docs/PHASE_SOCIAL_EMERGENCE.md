# Phase: Social Emergence

Living design doc for the phase that decides whether the substrate we
built is actually worth anything. North star: ~10 agents on screen in
real time exhibiting attack, hidden communication, promises, broken
promises, quests, collaboration, scheming, backstabbing, manipulation,
coalitions, contract enforcement — and a UI that makes this legible
at a glance.

This doc is **append-only as decisions land**. Every turn of the
conversation that produces a decision gets a section appended + a
git push, so a wifi/laptop crash doesn't lose context. Resume from
this file alone.

---

## Decision log

(Decisions land here as we make them. Format: short title, the
choice, and the *why* so future-us understands the tradeoff.)

_None yet — discussion in progress._

---

## Open design questions

(Updated as we identify them. The agreed-upon answer migrates up to
the decision log; the question itself stays here struck through.)

- Mental-state representation: agent-architecture-agnostic raw-text
  channel vs. structured schema? (User leans raw text.)
- Inventory visibility: opaque (infer from behavior) vs. partial
  (see equipped only) vs. transparent? (User leans opaque.)
- Item universe minimum viable set: food / money / weapons. What
  else? (Open.)
- Hierarchical historian layers: individual / group / society /
  kingdom / world. Where does each live, how is it rolled up?
- Movement smoothing when batch actions arrive: tweening on the
  frontend vs. accepted-as-is? (Likely defer.)
- Engine-side incentive structure: HP-death loss, hunger, money
  goal. What are the exact tunings?
- Mix of rule-based vs. LLM agents: target ratio, which archetypes?
- Anthropic vs. local-Qwen split for the iteration budget.

---

## Testing discipline for this phase

(Lessons from prior failures + the plan to avoid repeating.)

_To be filled in this conversation turn._

---

## Reference: current agent observation + action model

(Source-of-truth snapshot from the codebase audit. Updated only when
the model itself changes, not as design conversation evolves.)

_To be filled in this conversation turn._
