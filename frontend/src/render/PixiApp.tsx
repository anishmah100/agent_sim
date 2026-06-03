// PixiJS world canvas host.
//
// Responsibilities:
// - Create the PixiJS Application
// - Mount it into the container element passed by App.tsx
// - Set up the world container, viewport (pan/zoom/follow), tile layer
// - Expose a small handle for the Solid layer to control camera +
//   inspect state
//
// HARD RULE: nothing Solid-reactive lives here. The Solid layer
// communicates with PixiJS via the returned handle's methods, never by
// shared signals. Keeps reactivity isolated and the canvas immune to
// VDOM-style re-renders.
//
// Round 1 lesson baked in: don't mix HMR with PixiJS scenes — the
// listeners leak. The Pixi Application is created once per page load
// (full-reload on source change). Vite HMR for THIS file is opted out
// at the bottom.

import { Application, Container, Graphics, Text } from "pixi.js";

export interface PixiHandle {
  app: Application;
  destroy(): void;
}

export async function mountPixiApp(host: HTMLElement): Promise<PixiHandle> {
  const app = new Application();
  await app.init({
    background: "#193c3e",                  // dusky placeholder, palette-aligned
    resizeTo: host,
    antialias: false,                       // pixel art — never AA
    roundPixels: true,                      // crisp pixel rendering
    resolution: window.devicePixelRatio,
    autoDensity: true,
    powerPreference: "high-performance",
  });

  host.appendChild(app.canvas);

  // World container. Tiles, sprites, particles all live under here.
  // The viewport (pan/zoom) will wrap this container in Milestone 1.
  const world = new Container();
  world.label = "world";
  app.stage.addChild(world);

  // Placeholder content so the canvas isn't blank during scaffolding.
  // Removed when the LDtk tilemap renderer lands (Milestone 1).
  const placeholderTile = new Graphics()
    .rect(0, 0, 48, 48)
    .fill(0x63c74d);                        // Endesga grass green
  placeholderTile.x = -24;
  placeholderTile.y = -24;
  world.addChild(placeholderTile);

  const placeholderLabel = new Text({
    text: "agent_sim · canvas mounted\nawaiting tilemap + entities (Milestone 1)",
    style: {
      fontFamily: "ui-sans-serif, system-ui, sans-serif",
      fontSize: 16,
      fill: 0xead4aa,
      align: "center",
      lineHeight: 22,
    },
  });
  placeholderLabel.anchor.set(0.5);
  app.stage.addChild(placeholderLabel);

  const center = () => {
    placeholderLabel.x = app.screen.width / 2;
    placeholderLabel.y = app.screen.height / 2;
    world.x = app.screen.width / 2;
    world.y = app.screen.height / 2 - 60;
  };
  center();
  app.renderer.on("resize", center);

  return {
    app,
    destroy: () => {
      app.destroy(true, { children: true, texture: true });
    },
  };
}

// Opt this file's owners out of HMR — see header comment.
if (import.meta.hot) {
  import.meta.hot.invalidate();
}
