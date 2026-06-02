# ANTI-MESS PLAN

How we avoid the polish debt that killed round 1.

## Why round 1 ended up looking hacky

Honest postmortem from `province_sim`:

1. **No locked visual reference.** Each batch of AI gen was a fresh micro-style. Nothing forced consistency across tiles, chars, buildings, UI.
2. **No palette discipline.** Different generation runs returned different palettes. Tiles didn't composite.
3. **No autotile system.** We wrote regex hacks for edges; corners never blended right.
4. **Animation was ad-hoc.** Single-frame sprites; manual position bumps in code.
5. **Day/night was a `Graphics` rectangle overlay** that wasn't aligned to the viewport. Looked broken in every state.
6. **No quality gate.** "Looks fine" was the bar; we accepted broken output and tried to patch downstream.
7. **God-object scene classes.** `WorldScene` did rendering + input + animation + UI binding + scenario logic. Touching anything broke something else.
8. **DOM and canvas widgets mixed.** Drama feed in DOM, speech bubbles in canvas, label rendering in both. No consistent rules.
9. **HMR-survivable scenes leaked listeners.** Multiple iterations later, the second click on a character stopped firing because old + new pointer listeners both ran.
10. **"Trust me it works" handoffs.** Code changes claimed "done" without verification by screenshot.

This file lists the specific countermeasures.

## §1 — Inherit, don't author

Every rendering primitive we use comes from a battle-tested library. We write game logic and glue; we do not write engines or widgets.

| What we will NOT write | What we use instead |
|---|---|
| Tile renderer | `@pixi/tilemap` |
| Camera (pan/zoom/follow) | `pixi-viewport` |
| LDtk → PixiJS loader | official `ldtk-ts` |
| Tile autotile rules | LDtk's built-in visual rules |
| Sprite animation | PixiJS `AnimatedSprite` (built-in) |
| UI widgets (buttons, panels, dialogs) | Kobalte |
| Auth flow | Auth.js |
| State diff serialization | FlatBuffers |
| Build tool / HMR | Vite (with canvas-layer HMR opt-out) |

If we find ourselves about to write a custom version of any of the above, we stop and check whether the existing tool already handles our case.

## §2 — Locked style guide before any art

Before ANY sprite is generated for the world, we lock:

- A **32-color palette** (e.g. Endesga 32, AAP-64, or a custom 32 from Lospec). Every pixel in every asset must come from this palette.
- **Tile size**: 16×16 px, hard.
- **Character sprite dims**: 16×24 px (body fills the 16×16 footprint with the head extending up). Feet pixel y = 23. Anchor point bottom-center.
- **Animation specs**: walk = 4 frames per direction × 4 directions. Frame 0 = idle pose. Frames 1–3 = step cycle. Attack = 4 frames. Hit = 2 frames. Death = 4 frames.
- **Layout in spritesheets**: each character is one 64×96 (4 cols × 4 rows) walk sheet + one 64×24 attack strip + one 32×24 hit strip + one 64×24 death strip.

All locked in `art/style.json` with a validation script that enforces it on every input image.

## §3 — Generate one perfect asset before scaling

Before mass-generating sprites:

1. Generate ONE grass tile, ONE oak tree, ONE character. With ChatGPT, using the frozen prompt template.
2. Run them through the validation pipeline.
3. Composite them in a test scene.
4. **Side-by-side against a HeartGold reference screenshot.**
5. If the bar is met, lock as the reference and scale up generation.
6. If the bar is NOT met:
   - Tweak the prompt template once.
   - Or switch to a commercial tileset for the base (Sprout Lands $20, Tiny packs CC0) and reserve AI gen for fills.
   - We do NOT iterate broken output trying to fix it post-hoc.

## §4 — UI design before UI code

For every screen / panel / overlay:

1. Wireframe it (excalidraw, low-fi sketch, or ASCII mock). Layout fixed.
2. Identify which Kobalte primitive each element maps to.
3. Write the component.
4. Screenshot. Compare to wireframe.
5. If it drifted, it's a redesign — not a CSS patch. Revisit the wireframe.

No widget gets built without the wireframe step.

## §5 — Zero mixing of canvas + DOM widgets

The rule:

- **Canvas owns the world.** Tiles, sprites, particles, character labels floating above heads, speech bubbles attached to characters, selection rings.
- **DOM owns the chrome.** Top bar, minimap, inspector panel, drama feed (text scroller), story feed, leaderboards, all modals.

No widget straddles both. If a thing needs to feel attached to a world entity (a name label above a head), it lives in canvas. If a thing is a list / scrollable / has text input / has buttons, it lives in DOM.

## §6 — Visual regression in CI from day 1

The day the first tile renders, we set up:

- A deterministic test world (`worlds/fixtures/regression.ldtk`).
- A seeded entity layout.
- Playwright takes screenshots at: full-zoom, mid-zoom, close-zoom, day, night.
- CI diffs against committed reference PNGs.
- A pixel diff > N (configurable) fails the build.

The reference PNGs are updated only when we INTEND to change something visually. A surprise diff = a regression to investigate, not approve.

## §7 — Iteration discipline

Hard rules I commit to:

- **After every substantive UI change**, run the dev server, take a screenshot via CDP, look at it BEFORE saying "done".
- **If a UI element takes >3 attempts to look right, STOP.** Open the wireframe, identify what drifted, redesign the widget rather than adding more CSS.
- **No "trust me" handoffs.** If I say a change is done, it must include a screenshot or test result.
- **Commit after every checkpoint.** History reflects what passed the bar, not what tried to.

## §8 — HMR + scene-leak prevention

In `province_sim` we discovered that Phaser scenes survive Vite HMR, so each save accumulated duplicate pointer listeners. We had clicks fire 4× and selection silently fail.

Countermeasures for round 2:

- **Canvas layer opts out of HMR.** The PixiJS application is a top-level singleton; any change to its source forces a full reload. Slightly worse iteration speed; eliminates leaked listeners by construction.
- **DOM layer keeps HMR.** Solid's HMR is clean (no surviving state).
- **All event listeners use a centralized registry** so we can verify counts in tests.

## §9 — Click handling that works on real mice

Round 1 spent a full hour debugging that mouse jitter (10–20px during a relaxed click) was getting classified as drag and silently swallowing every character-select. The fix:

- **At pointerup, decide click-vs-drag from `|up - down|` Manhattan distance.** Not from cumulative pointermove delta. Mouse wobble during the click no longer matters.
- **Don't start the camera pan until the pointer moves >18px from down.** Tiny jitter doesn't trigger phantom pans.
- **Hit-test entities against the rendered (interpolated) position, not the server target position.** Server target races ahead of where the sprite actually is.
- **Hit radius: ~1.3 tiles** (Manhattan distance squared < 1.7). Forgiving enough for moving targets, tight enough not to grab neighbors.

This is in the engine spec because it's a known landmine.

## §10 — Day/night is a real shader, not an overlay

Round 1 used a `Graphics.fillRect` over the camera viewport. It misaligned at every zoom level.

Round 2: a **PixiJS `ColorMatrixFilter`** applied to the world container. It rides the camera transform inherently. Dusk = orange-shifted matrix; night = blue-shifted with reduced brightness. The same filter trick handles weather (rainy = desaturated cool matrix) when we add it.

## §11 — When to buy / commission instead of iterate

Hard rule: **if 3 generation attempts for an asset can't pass the quality gate, we either buy a commercial asset or commission a pixel artist** ($5–20 on Fiverr or Twitter) rather than iterating broken output.

We treat AI generation as a means, not the goal. The goal is the world looking like HeartGold.

## §12 — The "does this look like HeartGold?" gate at every milestone

Every milestone ends with:

1. Screenshot of our output.
2. Reference screenshot from HeartGold or our locked anchor tileset.
3. Honest side-by-side comparison. I describe the visible gap.
4. If the gap is large, we don't proceed. We address it.

Polish accrues at the milestone level, not "we'll get to it before launch."

## §13 — Architectural rules to prevent God-objects

- **No file > 500 lines** without explicit justification.
- **Scene / page / route files are coordinators**; they wire components, they don't implement them.
- **Engine state mutations go through typed methods on the world object**, not direct field writes. Easier to test, easier to log, easier to swap.
- **Rendering and input are separate subsystems** with explicit interfaces. Pointer events → InputBus → handlers. World state → RenderQueue → PixiJS draw calls. No subsystem touches another's internals.

## §14 — Trust ladder for "is this done?"

In ascending order of confidence:

1. "Code compiles." (Minimal.)
2. "Unit tests pass." (Necessary, insufficient.)
3. "Integration test confirms end-to-end behavior."
4. "I took a screenshot and it matches the wireframe."
5. "Visual regression CI passes."
6. "I ran it manually with the dev server, clicked through every flow, took screenshots, compared to references."

A milestone is not done unless it hits level 6.
