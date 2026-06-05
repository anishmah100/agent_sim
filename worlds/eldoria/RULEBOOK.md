# Eldoria — Rulebook

1500x1500 fantasy continent with 6 regions and 6 towns.

*Auto-generated from `worlds/eldoria/{bundle.toml, rules.star}` + the engine manifest. Do not edit by hand.*

---

## 1. Overview
- Scenario: `fantasy_town`
- Schema version: 1

## 2. Time
- Tick rate: 60 Hz

## 3. Map
- Dimensions: 1500 × 1500 tiles
- Map id: `eldoria`

## 4. Stats
| Key | Kind | Range | Default | Meaning |
|---|---|---|---|---|
| `gold` | int | 0–1e+06 | 25 | Currency. Earned via work_for_pay or trade; spent via pay. |
| `hp` | int | 0–100 | 100 | Hit points. 0 = dead. Restored by `heal` verb. |
| `hunger` | float | 0–1 | 0 | 0 = sated, 1 = starving. Grows at hunger_per_tick; above hunger_damage_above drains hp. |

## 5. Items
| ID | Kind | Props |
|---|---|---|
| `apple` | food | satiety=0.25, weight=0.2 |
| `coin_pouch` | currency_container | gold=10 |
| `iron_sword` | weapon | damage=15, two_handed=false, weight=3.5 |
| `loaf_bread` | food | satiety=0.5, weight=0.3 |
| `small_chest` | container | capacity=10, lockable=true |
| `village_sign` | readable | text=Welcome to Eldoria. |
| `wooden_shield` | armor | defense=5, weight=2 |

## 6. Verbs
| Name | Category | System | Description |
|---|---|---|---|
| `accept_task` | common | `verbalquests` | Accept a proposed contract addressed to you. |
| `advance_construction` | common | `construction` | Advance an owned adjacent blueprint by one step; consumes one advance_materials batch. Completes the blueprint when progress reaches 100. |
| `attack` | common | `combat` | Damage an adjacent target. |
| `chop` | common | `resources` | Chop an adjacent tree. Yields wood item IDs; depletes after N hits. |
| `claim_ownership` | common | `property` | Take ownership of an unowned adjacent building. |
| `complete_task` | common | `verbalquests` | Mark an accepted contract as complete (from the proposer's PoV — no engine verification). |
| `defend` | common | `combat` | Raise guard; halves the next incoming damage. |
| `demolish` | common | `construction` | Remove an owned blueprint OR building. |
| `drop` | common | `inventory` | Drop an item from inventory. |
| `enter` | common | `property` | Step inside an adjacent building. |
| `equip` | common | `inventory` | Wear / wield an inventory item. |
| `exit` | common | `property` | Leave the current building. |
| `give` | common | `inventory` | Give an inventory item to an adjacent target. |
| `heal` | common | `combat` | Restore HP on self or adjacent target. |
| `lock` | common | `property` | Lock an owned building. |
| `loot` | common | `loot` | Strip gold and inventory from an adjacent dead entity. |
| `mine` | common | `resources` | Mine an adjacent rock. Yields stone item IDs; depletes after N hits. |
| `pay` | common | `money` | Transfer gold to an adjacent entity. |
| `pickup` | common | `inventory` | Pick up an adjacent item. |
| `place_blueprint` | common | `construction` | Place a blueprint at an adjacent walkable tile. Pays the initial-materials cost up front. |
| `propose_task` | common | `verbalquests` | Propose a verbal contract to a known entity. Records the contract on both parties' extras.contracts. |
| `reject_task` | common | `verbalquests` | Reject a proposed contract addressed to you. |
| `trade` | common | `trade` | Give an item to an adjacent target in exchange for gold (target pays). |
| `transfer_ownership` | common | `property` | Give ownership of an owned building to another entity. |
| `unlock` | common | `property` | Unlock an owned building. |
| `work_for_pay` | common | `money` | Perform labor (stub: just credits gold). Real version will validate a work-site. |
| `read` | novel | `rules.star` |  |

## 7. NPC Archetypes
- `blueprint` — Construction-in-progress entity. Owned by the builder, becomes a 'building' when progress hits 100.
- `blueprint:cottage` — Small one-room dwelling.
- `blueprint:shed` — Storage hut. Cheap, fast.
- `building` — A first-class structure entity. Owned via claim_ownership; constructed via the construction system; enterable as an interior tile space.
- `rock` — Mineable resource node. Yields stone item entities.
- `tree` — Choppable resource node. Yields wood item entities.

## 8. Tunings
| Name | Value |
|---|---|
| `attack_damage` | 10 |
| `defend_damage_mul` | 0.5 |
| `heal_amount` | 5 |
| `hunger_damage_above` | 0.9 |
| `hunger_damage_rate` | 1 |
| `hunger_per_tick` | 0.0008 |
| `max_hp` | 100 |
| `pay_max_range_tiles` | 1 |
| `shout_muffle_radius` | 20 |
| `shout_radius` | 30 |
| `speak_radius` | 8 |
| `starting_gold` | 25 |
| `whisper_radius` | 2 |
| `work_payment` | 3 |

## 9. Quirks
- novel verb: read
