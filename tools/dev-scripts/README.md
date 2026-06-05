# Dev scripts

Ad-hoc tools for development + debugging. NOT run by CI; live separately
from `frontend/tests/` (which holds the real integration suite). Move
graduates here when a script earns its place as a useful diagnostic.

Each script assumes engine on `127.0.0.1:8080` and vite dev server on
`127.0.0.1:5173`. Run them from the repo root:

```
cd ~/projects/agent_sim
node tools/dev-scripts/<name>.mjs
```

## Scripts

- **`bench_drag.mjs`** — pan benchmark: simulates a 4-second diagonal
  camera sweep and reports per-frame timings (mean, p99, max). Used to
  validate the chunked-tilemap perf claim (~21 ms/frame on Eldoria).
- **`bench_pan_blank.mjs`** — pan benchmark with tilemap +
  decoration containers hidden — gives a baseline for what the
  viewport drag alone costs vs the rendering on top.
- **`screenshot_eldoria.mjs`** — wide-zoom + per-region screenshots of
  the default world. Used to spot stretched sprites / in-water settlements.
- **`screenshot_audit.mjs`** — comprehensive sweep: wide + 12-grid +
  every named town + every enterable interior. The "is the world
  visually shipping?" gate.
- **`screenshot_interiors.mjs`** — interior view per template
  (cottage / blacksmith / town hall / granary). Compares against the
  pre-migration baseline.
- **`screenshot_form.mjs`** — captures the persona / join-as-agent form.
- **`screenshot_full.mjs`** — single full-page screenshot for the
  share-moment UI.
- **`smoke_artcatalog.mjs`** — verifies the ArtCatalog actually loads
  + the world renders with zero console errors. Used as a quick guard
  during the catalog migration.
