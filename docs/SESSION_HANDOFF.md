# Session Handoff — 2026-06-10 (resume point)

## What this session decided + shipped

1. **Strategic verdict: CONTINUE, finding-first** (full evidence:
   `~/Desktop/info_about_me/agent_sim_go_nogo_research_2026-06-10.md`).
   Killed: canonical-benchmark / VC-virality / acquihire framings.
2. **Cross-brain pilot study** — the finding exists: models have distinct
   social-strategy signatures (Claude private whisper-pacts vs Qwen public
   broadcast; rule control zero). `docs/research/BRAIN_STUDY_PILOT_2026-06-10.md`.
   Driver: `tools/experiments/run_brain_study.py` (fresh engine per run,
   ~$2/Claude run on Haiku).
3. **Substrate hardening** — `docs/AUDIT_FINDINGS_2026-06-10.md`. Headline
   fix: inventory id-trap that made give/drop/eat fail for any agent using
   the id it SAW (87/87 give failures in the pilot). Direct-path chain suite:
   `tools/audit/paths_e2e.py` (engine on :8090; 11 pass / 4 skip / 0 fail).

## For Robert (taste calls)

- **Dead verbs**: eldoria has NO tree/rock/building entities, so
  chop/mine/forage/claim/lock/unlock + construction can never succeed while
  the rulebook still advertises them to agents (Sela the homesteader's goal
  is impossible). Spawn real entities (richer emergence) or strip the verbs
  (honest, simple)? See AUDIT_FINDINGS_2026-06-10 §Design call.
- **Push**: master is ahead of origin (study + hardening commits). Push was
  denied without explicit authorization — say the word or `git push origin master`.

## Next batch (in order)

1. Resume engine-core audit (13 subsystems) after token reset:
   `Workflow({scriptPath: <session>/workflows/scripts/substrate-full-audit-wf_78151fe8-48c.js, resumeFromRunId: "wf_78151fe8-48c"})`;
   worktree `~/projects/agent_sim_audit` kept alive for it.
2. Fix the 3 confirmed FSM mediums in one batch (scavenger RACING timeout,
   survivor FLEEING/DESPERATE priority + 5-tile radius, manipulator
   threat-list mark) — unit tests exist in agents/baselines/tests/.
3. Scale the study: ~10 runs/arm, +Sonnet arm, one knob experiment
   (whisper range or scarcity), then the public writeup.

## Standing constraints (do not drift)

- Pseudonymous repo (anishmah100) — never real name/email in commits/docs.
- .env.local stays gitignored; local Qwen :8782 for runs, hosted Claude for
  studies/showcases only (budget-tracked in .runlog/anthropic_spend.jsonl).
- Kill processes by PORT with exact PIDs (stale-engine trap, Exit-144 trap).
