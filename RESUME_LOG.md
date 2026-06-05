# RESUME_LOG — per-phase append-only log

**This is the crash-recovery log. Every phase appends an entry here
BEFORE attempting work, and updates it on completion or interruption.
A resuming session can read this file to know exactly where the
previous session was when it stopped.**

Format per entry:
```
## [PHASE_ID] title — YYYY-MM-DD HH:MM
status: pending | in_progress | done | blocked
started: HH:MM
finished: HH:MM (or empty if not done)
commit: <sha> (or empty)
files_touched: list
in_flight_notes:
  - free-text scratchpad for what was happening if interrupted
  - what to check first on resume
blockers: (if blocked)
```

---

## SESSION-INIT — 2026-06-05
status: done
note: PROGRESS.md + RESUME_LOG.md initialized. 24-phase autonomous push authorized.
Locked decisions are in PROGRESS.md. Next phase to execute: WORLD-1.

## WORLD-1 — World bundle data refactor — 2026-06-05
status: done
finished: ~15:40
files_touched:
  - worlds/<name>/world.json (moved from worlds/<name>.json) — 4 worlds
  - worlds/eldoria/npcs.json (moved from scenarios/fantasy_town/)
  - worlds/<name>/design/*_gen.py (moved from worlds/_design/)
  - worlds/<name>/bundle.toml (new, 4 files)
  - engine/internal/world/bundle.go (new — LoadBundle / ReadBundle)
  - engine/internal/world/bundle_test.go (new — 3 tests)
  - engine/cmd/engine/main.go (new -bundle flag; legacy -world preserved)
  - engine/Dockerfile + docker-compose.yml + deploy/fly.toml (BUNDLE env)
  - start.sh + examples/spawn_emergent_cast.py + CLAUDE.md + README.md
  - go.mod / go.sum (added github.com/BurntSushi/toml v1.4.0)
  - SIDE FIX: engine/internal/scenario/fantasy_town/fantasy_town_test.go
    was hanging for 10 min on TestAttackDealsDamage because SubmitAction
    blocks on a reply channel that only resolves after Tick(). Added a
    submit() helper that does QueueAction + Tick + receive. All tests
    pass in 0.003s now.
in_flight_notes: complete; ready to start WORLD-2.

## WORLD-2 — Starlark rule engine — 2026-06-05
status: in_progress
plan:
  1. Add go.starlark.net dependency.
  2. New package engine/internal/world/rules with RuleSet type.
     - Tunings: map[string]float64 (and helpers GetFloat, GetInt, GetBool)
     - Items: map[string]ItemDef (id, kind, props)
     - Verbs: map[string]VerbDef (predicate + effect, future use)
  3. Loader rules.LoadStarlark(path) → *RuleSet.
  4. worlds/<name>/rules.star (start with minimal eldoria tunings).
  5. bundle.toml gains [rules] section with file = "rules.star".
  6. world.Bundle parses + LoadBundle wires it; World stores *RuleSet.
  7. Public API for systems to read tunings later (Phase 5/SUB-5
     refactors combat/money/etc to use these).
  8. Tests + smoke.
in_flight_notes:
  - DO NOT refactor existing systems to read from RuleSet yet — that's
    SUB-5/6 in Wave 2. WORLD-2 just makes the rules available.
  - Eldoria rules.star starts with hunger_per_tick, attack_damage,
    starting_gold — a few representative tunings for tests.
  - Test pattern: load minimal star, assert .GetFloat("hunger_per_tick")
    matches the literal in the .star file.

## WORLD-1 — original in-flight plan (historical) — 2026-06-05
status: superseded by entry above
plan:
  1. Create per-world bundle dirs: worlds/eldoria/, worlds/dev_test/,
     worlds/dev_wilderness/, worlds/soak_1000x1000/.
plan:
  1. Create per-world bundle dirs: worlds/eldoria/, worlds/dev_test/,
     worlds/dev_wilderness/, worlds/soak_1000x1000/.
  2. Move worlds/*.json → worlds/<name>/world.json
  3. Create bundle.toml per world (name, scenario_pkg, art_pack, description).
  4. Move scenarios/fantasy_town/npcs.json → worlds/eldoria/npcs.json.
     Other worlds get their own (empty for now).
  5. Move worlds/_design/*.py → worlds/<name>/design/
  6. Add world.LoadBundle(dir) loader; --bundle flag in main.go
     (backward compat: --world flag still accepts a path).
  7. Update start.sh to use --bundle.
  8. Update tests + verify smoke + commit.
in_flight_notes:
  - Scenario Go-code relocation is DEFERRED to WORLD-2 (it pairs with the
    Starlark DSL — most scenario logic becomes Starlark, residual Go shrinks).
  - Go version: 1.22 system + auto-toolchain pulls 1.25 per engine/go.mod.
  - Default world per start.sh: eldoria. Per main.go: dev_test.json.
    Inconsistency — keep eldoria as default everywhere.
