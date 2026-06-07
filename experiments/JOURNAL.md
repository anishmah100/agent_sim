# Experiment journal — cross-world learnings

Hand-curated. The auto-loop adds run entries to INDEX.md and
WORLD_JOURNAL.md but never edits this file.

## 2026-06-06 — P7 social-emergence debugging arc (runs 6–11)

The headline: getting from "LLM agents do nothing interesting" to a live
social economy was a chain of *substrate* bugs, not model limitations.
Each looked like a different surface symptom; all were plumbing.

**The bug that masked everything (runs 1–6): WS keepalive death.**
LLM brains take 3–8s/decision (local Qwen). The SDK read loop did
`await inbox.put(obs)` on a 64-slot queue; a slow consumer backed it up,
the reader blocked, the websockets ping path stalled, and the connection
died with `1011 keepalive ping timeout` ~2 min in. The body was then
orphan-cleaned and the agent froze at its starting 25 gold. Fast
rule-based bots never backed up, so they collected fine — the asymmetry
made it look like "LLMs can't navigate." Fix: drop-oldest on a full
inbox + coalesce observations() to the freshest. Result: run7 Bram
25→245 gold. (sdk/python/agent_sim_sdk/client.py)

**Economy depletion (run7→8).** Respawn hub defaulted to the old market
(772,894)/radius 200, not the clustered spawn (764,864). Replenishment
never reached agents; one greedy collector stripped the cluster and
everyone else idled at 25 with nothing to trade over. Fix: align
respawn to spawn hub, radius→40, interval 1800→600. A live economy UNDER
the agents is a precondition for interaction.

**Social intent without completion (run8–9).** With a live economy,
agents *tried* to socialize: 8 proposals, 26 speak, reasoning like
"moving toward spawn_41 to propose an alliance." But 0 accepted, 0 pays.
Two causes:
1. The prompt's action menu CLAIMED propose_task needs the target
   adjacent. The engine handlePropose/handleAccept have NO range check.
   So agents burned whole lifetimes chasing a moving target to "finalize"
   a deal that needed no proximity, never converging. Fix: correct the
   menu (propose/accept = any range; surface a pending offer as a top
   PRIORITY with the exact accept_task call). → run10: 0→4 accepted.
2. pay (money.go) + give (inventory.go) hardcoded Chebyshev>1, so
   honoring a deal hit the same adjacency wall. Fix: honor
   pay_max_range_tiles (now 3); attack/trade stay strict-adjacent.

**Method that worked:** every "feature broken" symptom was localized by
diffing layers — /api/v1/debug/vision (engine truth) vs the live WS obs,
and reading the agents' own reasoning traces (they literally narrate the
failure: "last move failed due to no path", "5 tiles… 6 tiles away").
Trust the reasoning logs over the metrics; the metrics only said 0.

**Still open after run10:** contracts accepted but 0 completed/0
transfers (run11 tests the pay/give range fix); LLM `move` occasionally
emits an entity-id target (salvage-path artifact, rejected harmlessly);
manipulator baseline shows false "VANISHED" in the harness poll (body is
alive — wrong entity_id), pure reporting noise.


**Update — run13 closes the loop.** With the fulfillment nudge + pay/give
3-tile range: 5 contracts accepted, **6 pays**, 1 item transfer between
Qwen agents (Gini 0.25). Full chain works: collect → propose → accept →
honor-by-pay (or defect). Scorer caveat: it labels honored contracts
"broken" because agents pay but never emit complete_task (not in grammar)
— betrayal metric overcounts; fix next.

**Update — Claude cross-model showcase (run19, post bug-fix campaign).**
First Claude-focal run on the fixed substrate (`--brain claude`,
claude-haiku-4.5, same prompt/action-space as Qwen). vs Qwen: ~3×
decision throughput (1.2–2.5s/decision), far more social initiative
(28 contracts proposed vs ~6, 21 speak), cleaner completion (2c/0broken),
strategic use of `eat` to manage inventory. 0 crashes/races. Bottleneck
shifted from "agents can't act" (Qwen substrate bugs) to "agents propose
but rarely ACCEPT each other / complete transfers" — a reciprocity/prompt
research problem, not a substrate bug. Est cost ~$0.60 (haiku). The
substrate is now solid enough that model quality is the variable.
