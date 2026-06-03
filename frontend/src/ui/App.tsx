// Root Solid component.
//
// Layout (per docs/ANTI_MESS_PLAN.md §5 + the v1 wireframe):
//
//   ┌──────────────────────────────────────────────────────────────┐
//   │  TopBar   (world clock · tick · my-agent pill · snap button) │
//   ├──────────────────────────────────────────────────────────────┤
//   │                                                              │
//   │                  WorldCanvas (PixiJS)                        │
//   │                                                              │
//   │  ┌─────────┐                          ┌────────────────────┐ │
//   │  │ Minimap │                          │   Drama feed       │ │
//   │  └─────────┘                          └────────────────────┘ │
//   └──────────────────────────────────────────────────────────────┘

import { onMount, onCleanup, createSignal } from "solid-js";
import { mountPixiApp, type PixiHandle } from "../render/PixiApp";
import { fetchWorldInfo, fetchWorldMap, type WorldInfo } from "../net/api";
import type { TileMapData } from "../render/Tilemap";

export function App() {
  const [worldInfo, setWorldInfo] = createSignal<WorldInfo | null>(null);
  const [connError, setConnError] = createSignal<string | null>(null);
  const [worldLoadError, setWorldLoadError] = createSignal<string | null>(null);
  let canvasContainer!: HTMLDivElement;
  let pixiHandle: PixiHandle | null = null;

  onMount(async () => {
    // Engine info is best-effort — frontend is useful for solo dev
    // even if the engine isn't running yet (we just render the world
    // file directly).
    fetchWorldInfo()
      .then(setWorldInfo)
      .catch((e) => setConnError((e as Error).message));

    pixiHandle = await mountPixiApp(canvasContainer);

    try {
      const mapData = (await fetchWorldMap("dev_test")) as TileMapData;
      pixiHandle.loadWorld(mapData);
    } catch (e) {
      setWorldLoadError((e as Error).message);
    }
  });

  onCleanup(() => {
    pixiHandle?.destroy();
    pixiHandle = null;
  });

  const fitToWorld = () => pixiHandle?.fitToWorld();

  return (
    <div
      style={{
        position: "relative",
        width: "100%",
        height: "100%",
      }}
    >
      <div
        ref={canvasContainer}
        style={{
          position: "absolute",
          inset: "0",
          "z-index": "0",
        }}
      />

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
            ? `engine=${worldInfo()!.scenario} · tick=${worldInfo()!.tick}`
            : connError()
              ? <span style={{ color: "#e43b44" }}>engine offline (ok for solo render)</span>
              : "connecting to engine…"}
        </span>
        {worldLoadError() && (
          <span style={{ color: "#e43b44" }}>world load failed: {worldLoadError()}</span>
        )}
        <span style={{ "margin-left": "auto", display: "flex", gap: "8px" }}>
          <button
            type="button"
            onClick={fitToWorld}
            style={{
              padding: "4px 10px",
              background: "#3a4466",
              color: "#ead4aa",
              border: "1px solid #5a6988",
              "border-radius": "3px",
              cursor: "pointer",
              "font-size": "12px",
            }}
          >
            fit to world
          </button>
        </span>
      </div>

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
