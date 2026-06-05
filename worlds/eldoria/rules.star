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
# Vitality
register_tuning("hunger_per_tick",     0.0008)  # ~5min from 0 to 1
register_tuning("hunger_damage_above", 0.9)     # above this, lose hp each tick
register_tuning("hunger_damage_rate",  1)       # hp lost per tick when starving
register_tuning("max_hp",              100)

# Combat
register_tuning("attack_damage",       10)
register_tuning("defend_damage_mul",   0.5)     # damage taken while defending
register_tuning("heal_amount",         5)

# Economy
register_tuning("starting_gold",       25)
register_tuning("work_payment",        3)
register_tuning("pay_max_range_tiles", 1)       # adjacency only

# Social
register_tuning("whisper_radius",      2)
register_tuning("speak_radius",        8)
register_tuning("shout_radius",        30)
register_tuning("shout_muffle_radius", 20)      # outside r=20, content garbled

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
