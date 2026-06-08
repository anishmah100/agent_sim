"""Eldoria — declarative ruleset (v1).

Loaded by the engine via bundle.toml's [rules] section.

This file is the source-of-truth for world tunings, items, and novel
verbs. Phase SUB-5 will refactor combat/money/etc systems to read
tunings from here rather than from Go constants — for now, declaring
them here makes the values discoverable and lets future runs override
without code changes.

Starlark is hermetic by design: no I/O, no time, no random. Anything
the engine needs from this file is set via register_tuning / register_item
/ register_verb.
"""

# ---- Tunings ----
# Vitality (D4 calibration)
# At 60 Hz, 30 in-game min = 108_000 ticks. hunger_per_tick = 1/108_000
# (~9.26e-6) so hunger reaches 1.0 in exactly 30 in-game min. Damage
# threshold at 0.7 → starvation starts at ~21 min. damage_interval=324
# (5.4 sec @ 60Hz) × rate=1 means 1 HP per 5.4 sec; from threshold to
# corpse = 100 HP × 5.4 sec = 9 in-game min. Total = ~30 min from full
# agent to corpse. Per user (D22): treat as starting numbers; tune by
# observed substrate-validation dynamics.
register_tuning("hunger_per_tick",                0.00001)
register_tuning("hunger_damage_above",            0.7)
register_tuning("hunger_damage_rate",             1)
register_tuning("hunger_damage_interval_ticks",   324)
register_tuning("max_hp",                         100)

# Combat
register_tuning("attack_damage",       20)     # lethal enough that fights resolve visibly (~5 hits to a kill)
register_tuning("defend_damage_mul",   0.5)     # damage taken while defending
register_tuning("heal_amount",         5)

# Economy
register_tuning("starting_gold",       25)
register_tuning("work_payment",        3)
register_tuning("pay_max_range_tiles", 3)       # pay + give: short social range, not strict adjacency

# Social
register_tuning("whisper_radius",      2)
register_tuning("speak_radius",        8)
register_tuning("shout_radius",        30)
register_tuning("shout_muffle_radius", 20)      # outside r=20, content garbled

# World dynamism — periodic item respawn (P4 user feedback: food
# was rare + items were statically fixed). Every respawn_interval
# ticks, one item is spawned at a random walkable tile within
# respawn_radius of (respawn_hub_x, respawn_hub_y), unless the
# total item count exceeds respawn_cap. Default hub matches the
# experiment hub from D5 (Crossroads market area).
# Respawn must track the CLUSTERED SPAWN hub, not the old market. With
# hub=(772,894)/radius=200 the economy replenished at the market while
# agents clustered at (764,864) and starved — and across a 200-tile disc
# a single respawn rarely landed near anyone. run7 showed one greedy
# collector (Bram, 25->245 gold) stripping the local cluster faster than
# it refilled, leaving every other agent at the starting 25 with nothing
# to do (no trades, no contracts). Aligning the respawn hub to the spawn
# hub + a tight radius + a faster interval keeps a live economy under the
# agents so scarcity drives interaction instead of just emptying out.
register_tuning("respawn_interval_ticks", 180)    # 3 in-game sec @ 60Hz
register_tuning("respawn_cap",            300)
# respawn_batch: items dropped PER interval. One-at-a-time can't keep a
# populated hub stocked — a crowd of foragers drains the local supply far
# faster than it refills and the play area starves, leaving agents milling
# with nothing to do after the opening grab. A batch keeps the town square
# continuously supplied so foraging / trading / and predation around the
# crowd sustain instead of dying out. ~14/5s feeds ~20 agents.
register_tuning("respawn_batch",          12)
# radius 40 let items — and the agents chasing them — disperse past each
# other's vision (radius 12), so runs were a coin-flip between a clustered
# negotiating group and everyone hoarding solo in opposite corners (run13:
# 6 pays vs run14: 0 contracts, same config). Match the spawn radius (14)
# so loot stays in the cluster and agents remain in mutual vision —
# proximity is the precondition for the propose/trade/pay loop to fire.
register_tuning("respawn_radius",         16)
register_tuning("respawn_hub_x",          764)    # == spawn_hub_x
register_tuning("respawn_hub_y",          864)    # == spawn_hub_y

# D5 — clustered spawn for emergence experiments. When set, new
# agents drop into the disc of `spawn_radius` tiles around
# (spawn_hub_x, spawn_hub_y) instead of a random world tile. This
# satisfies Nowak's analytic precondition w > c/b for direct
# reciprocity: every agent encounters every other within the first
# few in-game minutes. Default hub = Crossroads market (same tile
# as the respawn hub) so food + agents cluster together. Radius
# 18 ≈ everyone in mutual vision (vision_radius=12) from frame 1.
# Setting spawn_radius to 0 disables clustering (falls back to
# random world tile, the demo behaviour).
# Hub relocated from the Crossroads market (772,894) to the open NW
# pocket (764,864). The market is dense with buildings whose walls
# BLOCK line-of-sight: a probe at the old hub saw 0 items even with
# coins 6 tiles away, so clustered agents spawned blind and never
# pursued anything (gold frozen at the starting 25 across every run).
# A debug/vision scan found (764,864) sees 6 items / 5 monetary with
# clear LOS. Radius tightened 18→14 to keep the cluster in the open
# pocket rather than spilling back into the market.
register_tuning("spawn_hub_x",   764)
register_tuning("spawn_hub_y",   864)
register_tuning("spawn_radius",  14)

# ---- Stats ----
# Per-entity stats this world tracks. The engine creates the extras
# slots at spawn (defaulting to `default`) and clamps to [min, max] on
# every system write. Phase A6 surfaces these in the auto-generated
# rulebook so agents know what they can read about themselves and others.

register_stat({
    "key":         "hp",
    "kind":        "int",
    "min":         0,
    "max":         100,
    "default":     100,
    "description": "Hit points. 0 = dead. Restored by `heal` verb.",
})

register_stat({
    "key":         "gold",
    "kind":        "int",
    "min":         0,
    "max":         1000000,
    "default":     25,
    "description": "Currency. Earned via work_for_pay or trade; spent via pay.",
})

register_stat({
    "key":         "hunger",
    "kind":        "float",
    "min":         0.0,
    "max":         1.0,
    "default":     0.0,
    "description": "0 = sated, 1 = starving. Grows at hunger_per_tick; above hunger_damage_above drains hp.",
})

# ---- Items ----
# Minimal starter set — Phase WORLD-3/SUB-6 will scatter these around
# the world via the editor + procedural pass.

register_item({
    "id":    "apple",
    "kind":  "food",
    "props": {"satiety": 0.25, "weight": 0.2},
})

register_item({
    "id":    "loaf_bread",
    "kind":  "food",
    "props": {"satiety": 0.5, "weight": 0.3},
})

register_item({
    "id":    "iron_sword",
    "kind":  "weapon",
    "props": {"damage": 15, "two_handed": False, "weight": 3.5},
})

register_item({
    "id":    "wooden_shield",
    "kind":  "armor",
    "props": {"defense": 5, "weight": 2.0},
})

register_item({
    "id":    "small_chest",
    "kind":  "container",
    "props": {"capacity": 10, "lockable": True},
})

register_item({
    "id":    "village_sign",
    "kind":  "readable",
    "props": {"text": "Welcome to Eldoria."},
})

register_item({
    "id":    "coin_pouch",
    "kind":  "currency_container",
    "props": {"gold": 10},
})

# ---- Novel verbs ----
# Phase SUB-5 wires these into the engine. For now they're declarative;
# loader stores the callables so a later phase can register them as
# real engine verbs.

def read_precond(state, actor, args):
    """Reading a sign requires being on the same tile as it."""
    # Real implementation comes in Phase SUB-5; this is a placeholder
    # that documents the SHAPE of preconds expected from rules.star.
    return True

def read_effect(state, actor, args):
    """Reading reveals the sign's text into the actor's audible buffer."""
    pass

register_verb({
    "name":    "read",
    "precond": read_precond,
    "effect":  read_effect,
})
