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
import { connectViewer, type ViewerClient } from "../net/ws";
import type { TileMapData } from "../render/Tilemap";
import type { EntityState } from "../render/Entity";
import { Inspector } from "./Inspector";
import { WorldRulebook } from "./WorldRulebook";
import { Leaderboards } from "./Leaderboards";
import { HUD } from "./HUD";

export function App() {
  const [worldInfo, setWorldInfo] = createSignal<WorldInfo | null>(null);
  const [connError, setConnError] = createSignal<string | null>(null);
  const [worldLoadError, setWorldLoadError] = createSignal<string | null>(null);
  const [wsState, setWsState] = createSignal<"connecting" | "open" | "closed">("connecting");
  const [liveTick, setLiveTick] = createSignal<number | null>(null);
  const [entityCount, setEntityCount] = createSignal(0);
  const [selectedId, setSelectedId] = createSignal<string | null>(null);
  const [selectedSnapshot, setSelectedSnapshot] = createSignal<EntityState | null>(null);
  const [rulebookOpen, setRulebookOpen] = createSignal(false);
  const [leaderboardsOpen, setLeaderboardsOpen] = createSignal(false);
  const [hudOpen, setHudOpen] = createSignal(true);
  let canvasContainer!: HTMLDivElement;
  let pixiHandle: PixiHandle | null = null;
  let viewer: ViewerClient | null = null;

  const closeInspector = () => {
    setSelectedId(null);
    setSelectedSnapshot(null);
    pixiHandle?.setSelectedEntity(null);
  };

  onMount(async () => {
    fetchWorldInfo()
      .then(setWorldInfo)
      .catch((e) => setConnError((e as Error).message));

    pixiHandle = await mountPixiApp(canvasContainer);

    // Dev escape hatch: expose the pixi handle on globalThis so tests
    // and browser devtools can read entity state, drive the viewport,
    // and trigger selection without going through pointer events. The
    // app NEVER reads from globalThis itself — this is one-way.
    (globalThis as unknown as { __pixiHandle?: typeof pixiHandle }).__pixiHandle = pixiHandle;

    try {
      const mapData = (await fetchWorldMap("dev_test")) as TileMapData;
      pixiHandle.loadWorld(mapData);
    } catch (e) {
      setWorldLoadError((e as Error).message);
    }

    // Click → inspect.
    pixiHandle.onClick((ev) => {
      if (ev.entity) {
        setSelectedId(ev.entity.entity_id);
        setSelectedSnapshot(ev.entity);
        pixiHandle?.setSelectedEntity(ev.entity.entity_id);
      } else {
        closeInspector();
      }
    });

    // ESC closes the inspector.
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeInspector();
    };
    window.addEventListener("keydown", onKey);
    onCleanup(() => window.removeEventListener("keydown", onKey));

    // Live viewer stream. Snapshots overwrite the entity layer; tile
    // layer is static and was already loaded from the JSON above.
    viewer = connectViewer({
      onConnState: setWsState,
      onSnapshot: (snap) => {
        setLiveTick(snap.tick);
        setEntityCount(snap.entities.length);
        pixiHandle?.setEntities(snap.entities);
        // Keep the inspector's data live for the selected entity.
        const sid = selectedId();
        if (sid !== null) {
          const found = snap.entities.find((e) => e.entity_id === sid);
          if (found) setSelectedSnapshot(found);
        }
      },
    });
  });

  onCleanup(() => {
    viewer?.close();
    viewer = null;
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
            ? `engine=${worldInfo()!.scenario}`
            : connError()
              ? <span style={{ color: "#e43b44" }}>engine offline (ok for solo render)</span>
              : "connecting to engine…"}
        </span>
        <span
          style={{
            "font-size": "11px",
            padding: "2px 8px",
            "border-radius": "3px",
            background: wsState() === "open" ? "#265c42" : wsState() === "connecting" ? "#733e39" : "#3a4466",
            color: "#ead4aa",
          }}
        >
          ws: {wsState()} {liveTick() !== null ? ` · live tick ${liveTick()}` : ""}
        </span>
        {worldLoadError() && (
          <span style={{ color: "#e43b44" }}>world load failed: {worldLoadError()}</span>
        )}
        <span style={{ "margin-left": "auto", display: "flex", gap: "8px" }}>
          <button
            type="button"
            onClick={() => setHudOpen(!hudOpen())}
            style={{
              padding: "4px 10px",
              background: hudOpen() ? "#feae34" : "#3a4466",
              color: hudOpen() ? "#181425" : "#ead4aa",
              border: "1px solid #5a6988",
              "border-radius": "3px",
              cursor: "pointer",
              "font-size": "12px",
            }}
          >
            hud
          </button>
          <button
            type="button"
            onClick={() => setLeaderboardsOpen(true)}
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
            leaderboards
          </button>
          <button
            type="button"
            onClick={() => setRulebookOpen(true)}
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
            rulebook
          </button>
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

      <Inspector entity={selectedSnapshot()} onClose={closeInspector} />

      {hudOpen() && (
        <HUD
          dayPhase="day"
          weather="clear"
          worldDims={worldInfo()?.world_dims ?? [0, 0]}
          entityCount={entityCount()}
          selected={selectedSnapshot()}
        />
      )}

      {rulebookOpen() && <WorldRulebook onClose={() => setRulebookOpen(false)} />}

      {leaderboardsOpen() && (
        <div
          style={{
            position: "fixed",
            top: "60px",
            right: "16px",
            width: "320px",
            "max-height": "70vh",
            background: "rgba(24, 20, 37, 0.96)",
            border: "1px solid #3a4466",
            "border-radius": "6px",
            padding: "12px 14px",
            color: "#ead4aa",
            "font-size": "13px",
            "z-index": "50",
            overflow: "auto",
          }}
        >
          <div style={{ display: "flex", "align-items": "center", gap: "8px", "margin-bottom": "8px" }}>
            <strong style={{ color: "#feae34" }}>Leaderboards</strong>
            <button
              type="button"
              onClick={() => setLeaderboardsOpen(false)}
              style={{
                "margin-left": "auto",
                padding: "2px 8px",
                background: "#3a4466",
                color: "#ead4aa",
                border: "1px solid #5a6988",
                "border-radius": "3px",
                cursor: "pointer",
                "font-size": "11px",
              }}
            >
              close
            </button>
          </div>
          <Leaderboards />
        </div>
      )}

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
