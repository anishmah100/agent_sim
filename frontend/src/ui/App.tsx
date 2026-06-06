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
import { fetchMentalState, fetchWorldInfo, fetchWorldMap, type MentalStateResponse, type WorldInfo } from "../net/api";
import type { MentalState } from "./Inspector";
import { connectViewer, type ViewerClient } from "../net/ws";
import type { TileMapData } from "../render/Tilemap";
import type { EntityState } from "../render/Entity";
import { Inspector } from "./Inspector";
import { InfoPanel } from "./InfoPanel";
import { describeSprite, type SpriteInfo } from "./SpriteInfo";
import { WorldRulebook } from "./WorldRulebook";
import { Leaderboards } from "./Leaderboards";
import { HUD } from "./HUD";
import { Editor } from "./Editor";
import type { EditorCategory, PaletteEntry } from "./EditorPalettes";
import type { TileKind } from "../render/tiles";
import { Minimap } from "./Minimap";
import { StoryFeed } from "./StoryFeed";
import { AgentsPicker } from "./AgentsPicker";
import { JoinAgent } from "./JoinAgent";
import { Onboarding } from "./Onboarding";

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
  const [joinOpen, setJoinOpen] = createSignal(false);
  const [liveEntities, setLiveEntities] = createSignal<EntityState[]>([]);
  const [worldTiles, setWorldTiles] = createSignal<string[] | undefined>(undefined);
  const [editorOpen, setEditorOpen] = createSignal(false);
  const [agentsPickerOpen, setAgentsPickerOpen] = createSignal(false);
  // Editor state hoisted up so the canvas click handler can read it.
  // When paint+glyph is set and editorOpen, a click paints instead of
  // inspecting. Default tool=paint so opening the editor + clicking a
  // glyph is enough to start working.
  const [editorTool, setEditorTool] = createSignal<"select" | "paint" | "erase">("paint");
  const [editorGlyph, setEditorGlyph] = createSignal<string | null>(null);
  const [editorCategory, setEditorCategory] = createSignal<EditorCategory>("tile");
  const [editorDeco, setEditorDeco] = createSignal<PaletteEntry | null>(null);
  const [tilesLegend, setTilesLegend] = createSignal<Record<string, TileKind> | null>(null);
  const [mentalState, setMentalState] = createSignal<MentalState | null>(null);
  // InfoPanel state — populated when the user clicks any non-veg
  // decoration on the overworld OR an interior prop. Holds enough to
  // also offer an "Enter" button for enterable buildings.
  const [info, setInfo] = createSignal<SpriteInfo | null>(null);
  const [infoAt, setInfoAt] = createSignal<{ x: number; y: number } | null>(null);
  const [infoSource, setInfoSource] = createSignal<"world" | "interior">("world");

  function fetchAndSetMentalState(entityID: string) {
    fetchMentalState(entityID).then((m: MentalStateResponse) => {
      setMentalState({
        dialogue: m.dialogue.map((d) => ({
          tick: d.tick,
          speaker: d.speaker,
          channel: d.channel as "speech" | "shout" | "whisper" | "sound",
          text: d.text,
        })),
        mind: m.mind,
        traces: m.traces,
        capture_reasoning_enabled: m.capture_reasoning_enabled,
      });
    }).catch(() => setMentalState(null));
  }
  let canvasContainer!: HTMLDivElement;
  let pixiHandle: PixiHandle | null = null;
  let viewer: ViewerClient | null = null;

  const closeInspector = () => {
    setMentalState(null);
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
      // Ask the engine which world it has loaded, then fetch that JSON.
      // The frontend used to hard-code "dev_test" but now adapts to
      // whatever world start.sh booted.
      const info = await fetchWorldInfo();
      const worldName = info?.world ?? "dev_test";
      const mapData = (await fetchWorldMap(worldName)) as TileMapData;
      pixiHandle.loadWorld(mapData);
      setWorldTiles((mapData as { tiles?: string[] }).tiles);
      if (mapData.tiles_legend) setTilesLegend(mapData.tiles_legend);
    } catch (e) {
      setWorldLoadError((e as Error).message);
    }

    // Click → inspect, OR paint when the editor is open with a glyph.
    pixiHandle.onClick((ev) => {
      // Editor paint takes precedence over inspect: when the editor is
      // active and a glyph is selected, clicking the canvas paints that
      // tile instead of selecting an entity. Without this branch the
      // editor's "click to paint" surface never wired up — Phase
      // WORLD-3 shipped the panel, this branch is Phase WORLD-4's
      // missing piece.
      if (editorOpen() && editorCategory() === "tile" && editorTool() === "paint" && editorGlyph()) {
        paintTileAt(ev.tileX, ev.tileY, editorGlyph()!);
        return;
      }
      if (editorOpen() && editorCategory() === "tile" && editorTool() === "erase") {
        // Erase paints the default walkable glyph the world declares.
        const legend = tilesLegend();
        const defaultGlyph =
          (legend && legend["g"] !== undefined) ? "g"
          : Object.keys(legend ?? {}).find((k) => legend?.[k] === "grass")
          ?? Object.keys(legend ?? {})[0];
        if (defaultGlyph) paintTileAt(ev.tileX, ev.tileY, defaultGlyph);
        return;
      }
      if (editorOpen() && editorCategory() !== "tile" && editorDeco()) {
        // Decoration drop — POSTs the engine then optimistically renders.
        dropDecorationAt(ev.tileX, ev.tileY, editorDeco()!);
        return;
      }
      if (ev.entity) {
        setSelectedId(ev.entity.entity_id);
        setSelectedSnapshot(ev.entity);
        pixiHandle?.setSelectedEntity(ev.entity.entity_id);
        fetchAndSetMentalState(ev.entity.entity_id);
      } else {
        closeInspector();
      }
    });

    // dropDecorationAt — POST to /api/v1/world/edit_deco + optimistically
    // render the new sprite locally. Real-time: the engine adds the
    // decoration to the live world so agents observe it on the next
    // tick, walkability updates immediately, and a building's door
    // gets registered for entry. Persists to a sidecar overlay so
    // a restart re-applies the drop.
    async function dropDecorationAt(tileX: number, tileY: number, entry: PaletteEntry) {
      if (tileX < 0 || tileY < 0) return;
      const spec = {
        x: tileX,
        y: tileY,
        sprite: entry.sprite,
        height_tiles: entry.height_tiles,
        footprint_w: entry.footprint_w,
        footprint_h: entry.footprint_h,
        walkable: entry.walkable,
      };
      // Optimistic render first so the user gets instant feedback.
      void pixiHandle?.addDecoration(spec);
      try {
        const { ENGINE_URL } = await import("../net/api");
        const r = await fetch(
          `${ENGINE_URL}/api/v1/world/edit_deco`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(spec),
          },
        );
        if (!r.ok) {
          const body = await r.json().catch(() => ({}));
          console.warn(`deco drop rejected: ${body.reason ?? r.status}`);
        }
      } catch (e) {
        console.warn(`deco drop failed: ${(e as Error).message}`);
      }
    }

    // paintTileAt — POST to /api/v1/world/edit + optimistically repaint
    // the Pixi tilemap so the user sees the change instantly. Reverts
    // on engine reject. Pixi.setTileGlyph is exposed via the handle.
    async function paintTileAt(tileX: number, tileY: number, glyph: string) {
      if (tileX < 0 || tileY < 0) return;
      const prev = pixiHandle?.getTileGlyph?.(tileX, tileY);
      pixiHandle?.setTileGlyph?.(tileX, tileY, glyph);
      try {
        const { ENGINE_URL } = await import("../net/api");
        const r = await fetch(
          `${ENGINE_URL}/api/v1/world/edit`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ x: tileX, y: tileY, glyph }),
          },
        );
        if (!r.ok) {
          if (prev !== undefined) pixiHandle?.setTileGlyph?.(tileX, tileY, prev);
          const body = await r.json().catch(() => ({}));
          console.warn(`edit rejected: ${body.reason ?? r.status}`);
        }
      } catch (e) {
        if (prev !== undefined) pixiHandle?.setTileGlyph?.(tileX, tileY, prev);
        console.warn(`edit failed: ${(e as Error).message}`);
      }
    }

    // Hover-driven InfoPanel: enter shows it for the sprite under the
    // pointer, exit hides it. Mouse-leave-then-enter on a neighbouring
    // sprite swaps the contents naturally without an explicit close.
    // The `hoveredSprite` guard handles the race when pointerover on
    // sprite B fires before pointerout on sprite A.
    let hoveredSprite: string | null = null;
    const showInfoFor = (sprite: string, x: number, y: number, source: "world" | "interior") => {
      const desc = describeSprite(sprite);
      if (!desc) return;
      hoveredSprite = sprite;
      setInfo(desc);
      setInfoAt({ x, y });
      setInfoSource(source);
    };
    const hideInfoFor = (sprite: string) => {
      // Only hide if the most-recent hover was this sprite — otherwise
      // the user has already moved to a different decoration and the
      // panel is showing that one.
      if (hoveredSprite === sprite) {
        hoveredSprite = null;
        setInfo(null);
      }
    };
    pixiHandle.onDecorationHoverEnter((ev) => showInfoFor(ev.sprite, ev.x, ev.y, "world"));
    pixiHandle.onDecorationHoverExit((ev) => hideInfoFor(ev.sprite));
    pixiHandle.onInteriorPropHoverEnter((ev) => showInfoFor(ev.sprite, ev.x, ev.y, "interior"));
    pixiHandle.onInteriorPropHoverExit((ev) => hideInfoFor(ev.sprite));

    // ESC closes the inspector. (The info panel hides on mouse-out
    // automatically — no manual close needed.)
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeInspector();
    };
    window.addEventListener("keydown", onKey);
    onCleanup(() => window.removeEventListener("keydown", onKey));

    // Live viewer stream. Snapshots overwrite the entity layer; tile
    // layer is static and was already loaded from the JSON above.
    viewer = connectViewer({
      onConnState: setWsState,
      onAudible: (events) => {
        pixiHandle?.ingestAudible(events);
      },
      onSnapshot: (snap) => {
        setLiveTick(snap.tick);
        setEntityCount(snap.entities.length);
        setLiveEntities(snap.entities);
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
            data-testid="agents-toggle-button"
            onClick={() => setAgentsPickerOpen(!agentsPickerOpen())}
            title="Find connected LLM agents"
            style={{
              padding: "4px 10px",
              background: agentsPickerOpen() ? "#feae34" : "#3a4466",
              color: agentsPickerOpen() ? "#1f2238" : "#ead4aa",
              border: agentsPickerOpen() ? "1px solid #feae34" : "1px solid #5a6988",
              "border-radius": "3px",
              cursor: "pointer",
              "font-size": "12px",
            }}
          >
            agents
          </button>
          <button
            type="button"
            data-testid="editor-toggle-button"
            onClick={() => setEditorOpen(!editorOpen())}
            title="Toggle world editor (Cmd+E)"
            style={{
              padding: "4px 10px",
              background: editorOpen() ? "#feae34" : "#3a4466",
              color: editorOpen() ? "#1f2238" : "#ead4aa",
              border: editorOpen() ? "1px solid #feae34" : "1px solid #5a6988",
              "border-radius": "3px",
              cursor: "pointer",
              "font-size": "12px",
            }}
          >
            editor
          </button>
          <button
            type="button"
            data-testid="join-agent-button"
            onClick={() => setJoinOpen(true)}
            style={{
              padding: "4px 10px",
              background: "#fee761",
              color: "#181425",
              border: "1px solid #fee761",
              "border-radius": "3px",
              cursor: "pointer",
              "font-size": "12px",
              "font-weight": "600",
            }}
          >
            join as agent
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

      <Inspector
        entity={selectedSnapshot()}
        mentalState={mentalState() ?? undefined}
        onClose={closeInspector}
      />

      <InfoPanel
        info={info()}
        at={infoAt()}
        source={infoSource()}
      />

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

      <Editor
        open={editorOpen()}
        onToggle={setEditorOpen}
        tilesLegend={tilesLegend()}
        tool={editorTool()}
        onToolChange={setEditorTool}
        selectedGlyph={editorGlyph()}
        onSelectedGlyphChange={setEditorGlyph}
        category={editorCategory()}
        onCategoryChange={setEditorCategory}
        selectedDeco={editorDeco()}
        onSelectedDecoChange={setEditorDeco}
      />

      <AgentsPicker
        open={agentsPickerOpen()}
        onClose={() => setAgentsPickerOpen(false)}
        onPick={(a) => {
          // Center the viewport on the agent's tile, zoom in enough
          // to see them, and open the inspector so the user can
          // watch their reasoning trace live.
          pixiHandle?.centerOn(a.pos[0], a.pos[1]);
          pixiHandle?.setSelectedEntity(a.entity_id);
          setSelectedId(a.entity_id);
          // Seed the inspector with what the picker already knows so
          // it opens immediately. The snapshot loop refines this with
          // the live entity record once it arrives in the viewport.
          setSelectedSnapshot({
            entity_id: a.entity_id,
            archetype: a.archetype ?? "unknown",
            pos: a.pos,
            facing: "S",
            display_name: a.display_name ?? a.persona_name,
          });
          // The mental-state inspector populates from the historian
          // — fire the same fetch the click-to-inspect path uses.
          fetchAndSetMentalState(a.entity_id);
          setAgentsPickerOpen(false);
        }}
      />

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

      <Minimap
        worldDims={worldInfo()?.world_dims ?? [60, 40]}
        tiles={worldTiles()}
        entities={liveEntities()}
        selfId={selectedId()}
        onTileClick={(x, y) => pixiHandle?.centerOn(x, y)}
        getViewportTileRect={() => pixiHandle?.getViewportTileRect() ?? null}
      />

      <StoryFeed entityId={selectedId()} shiftLeft={editorOpen()} />

      <JoinAgent open={joinOpen()} onClose={() => setJoinOpen(false)} />
      <Onboarding />
    </div>
  );
}
