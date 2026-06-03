# Art prompts

One prompt per asset. Paste the block between the `---` markers into ChatGPT chat (image gen enabled), download the result, save to `art/raw/<name>.png`.

After all are in, run intake per-asset:

```bash
python art/intake.py <name>
```

Realistic expectation: DALL-E will ignore dim specs and add unwanted shading tones to most images. Intake handles common artifacts but each sheet often needs a small per-asset tweak (custom snap palette, custom magenta threshold). Treat it as a normal post-processing pipeline, not a magic gate.

## Character prompts (1024×480 spec, ChatGPT will output whatever ratio it picks)

- `trainer_red.md` — done (the test case)
- `lyra_blue.md` — female adventurer
- `old_sage.md` — robed mentor figure
- `merchant_baker.md` — friendly shopkeeper
- `guard_iron.md` — town guard with sword
- `child_kid.md` — small child villager
- `cloaked_wanderer.md` — mysterious hooded figure

## Environment prompts

- `tileset_overworld.md` — ground + path + water + cliffs (autotile-friendly grid)
- `tileset_vegetation.md` — trees, bushes, flowers, tall grass
- `tileset_buildings.md` — house, tavern, market stall, fence, well, sign, lamp
- `tileset_interior.md` — wood floor, carpet, bed, table, chair, barrel, chest

## Item prompts

- `items_master.md` — 64 item sprites on one sheet (food, tools, weapons, armor, resources, gold, misc)
