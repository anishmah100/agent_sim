// Root Solid component.
//
// Layout (per docs/ANTI_MESS_PLAN.md §5 + the v1 wireframe):
//
//   ┌──────────────────────────────────────────────────────────────┐
//   │  TopBar   (world clock · tick · my-agent pill · snap button) │
//   ├──────────────────────────────────────────────────────────────┤
//   │                                                              │
//   │                                                              │
//   │                  WorldCanvas (PixiJS)                        │
//   │                                                              │
//   │  ┌─────────┐                          ┌────────────────────┐ │
//   │  │ Minimap │                          │   Drama feed       │ │
//   │  └─────────┘                          └────────────────────┘ │
//   └──────────────────────────────────────────────────────────────┘
//
// Each panel is a Solid component (DOM, themed with Kobalte primitives).
// WorldCanvas hosts a PixiJS application, isolated from Solid reactivity.

import { onMount, onCleanup, createSignal } from "solid-js";
import { mountPixiApp, type PixiHandle } from "../render/PixiApp";
import { fetchWorldInfo, type WorldInfo } from "../net/api";

export function App() {
  const [worldInfo, setWorldInfo] = createSignal<WorldInfo | null>(null);
  const [connError, setConnError] = createSignal<string | null>(null);
  let canvasContainer!: HTMLDivElement;
  let pixiHandle: PixiHandle | null = null;

  onMount(async () => {
    try {
      setWorldInfo(await fetchWorldInfo());
    } catch (e) {
      setConnError((e as Error).message);
    }
    pixiHandle = await mountPixiApp(canvasContainer);
  });

  onCleanup(() => {
    pixiHandle?.destroy();
  });

  return (
    <div
      style={{
        position: "relative",
        width: "100%",
        height: "100%",
      }}
    >
      {/* world canvas behind everything */}
      <div
        ref={canvasContainer}
        style={{
          position: "absolute",
          inset: "0",
          "z-index": "0",
        }}
      />

      {/* top bar */}
      <div
        style={{
          position: "absolute",
          top: "0",
          left: "0",
          right: "0",
          padding: "8px 16px",
          background: "rgba(24, 20, 37, 0.85)",
          "border-bottom": "1px solid #3a4466",
          display: "flex",
          gap: "16px",
          "align-items": "center",
          "z-index": "10",
          color: "#ead4aa",
          "font-size": "13px",
        }}
      >
        <strong style={{ color: "#feae34" }}>agent_sim</strong>
        <span style={{ opacity: "0.65" }}>
          {worldInfo()
            ? `scenario=${worldInfo()!.scenario} · world=${worldInfo()!.world} · tick=${worldInfo()!.tick}`
            : connError()
              ? <span style={{ color: "#e43b44" }}>engine offline: {connError()}</span>
              : "connecting…"}
        </span>
      </div>

      {/* minimap placeholder */}
      <div
        style={{
          position: "absolute",
          bottom: "16px",
          left: "16px",
          width: "200px",
          height: "150px",
          background: "rgba(24, 20, 37, 0.85)",
          border: "1px solid #3a4466",
          "border-radius": "4px",
          padding: "8px",
          color: "#8b9bb4",
          "font-size": "12px",
          "z-index": "10",
        }}
      >
        minimap (Milestone 9)
      </div>

      {/* drama feed placeholder */}
      <div
        style={{
          position: "absolute",
          bottom: "16px",
          right: "16px",
          width: "280px",
          height: "240px",
          background: "rgba(24, 20, 37, 0.85)",
          border: "1px solid #3a4466",
          "border-radius": "4px",
          padding: "8px",
          color: "#8b9bb4",
          "font-size": "12px",
          "z-index": "10",
        }}
      >
        drama feed (Milestone 6)
      </div>
    </div>
  );
}
