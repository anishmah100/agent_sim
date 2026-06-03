// Entry point. Bootstraps the Solid app and mounts it on #app.
//
// Architectural notes:
// - DOM and Canvas are STRICTLY separated. Solid owns the DOM overlay
//   (chrome, panels, feed). PixiJS owns the canvas (world rendering).
//   No widget straddles both. See docs/ANTI_MESS_PLAN.md §5.
// - The canvas layer opts OUT of HMR. Any change forces a full reload.
//   This prevents leaked PixiJS resources / event listeners — the
//   problem that ate hours of round 1. The DOM layer keeps HMR.

import { render } from "solid-js/web";
import { App } from "./ui/App";

const root = document.getElementById("app");
if (!root) throw new Error("missing #app root");
render(() => <App />, root);
