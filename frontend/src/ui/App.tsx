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

import { onMount, onCleanup, createSignal, createEffect } from "solid-js";
import { mountPixiApp, type PixiHandle } from "../render/PixiApp";
import { ENGINE_URL, fetchMentalState, fetchWorldInfo, fetchWorldMap, type MentalStateResponse, type WorldInfo } from "../net/api";
import type { MentalState } from "./Inspector";
import { connectViewer, type ViewerClient } from "../net/ws";
import type { TileMapData } from "../render/Tilemap";
import type { EntityState } from "../render/Entity";
import { Inspector } from "./Inspector";
import { InfoPanel } from "./InfoPanel";
import { AgentHoverCard, type AgentHoverInfo } from "./AgentHoverCard";
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

/** Shape of one row in /api/v1/agents. The full response carries
 *  more (persona_name, bio, last_verb, …) but the hover card + the
 *  inspector badge only need the fields below. Kept local to App
 *  — AgentsPicker has its own typing for the richer surface. */
interface AgentsRow {
  entity_id: string;
  is_llm?: boolean;
  archetype?: string;
  display_name?: string;
  persona_name?: string;
}

/** Cached /api/v1/agents lookup. The hover card fires a fetch ONCE
 *  per hover-enter and reuses the result for 2 seconds — keeps the
 *  engine from getting hammered if the user sweeps the pointer over
 *  a row of NPCs. */
const AGENTS_TTL_MS = 2000;

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
  // D17 task 6.2 + 6.5 — hover card + inspector badge state.
  const [hoverCard, setHoverCard] = createSignal<AgentHoverInfo | null>(null);
  const [hoverAt, setHoverAt] = createSignal<{ x: number; y: number } | null>(null);

  // Cached /api/v1/agents lookup. Refreshed when older than
  // AGENTS_TTL_MS; multiple concurrent callers share one in-flight
  // promise so a quick mouse-sweep doesn't fan out N fetches.
  let agentsCache: { fetchedAtMs: number; byId: Map<string, AgentsRow> } | null = null;
  let agentsInFlight: Promise<Map<string, AgentsRow>> | null = null;
  async function getAgentsLookup(): Promise<Map<string, AgentsRow>> {
    const now = Date.now();
    if (agentsCache && now - agentsCache.fetchedAtMs < AGENTS_TTL_MS) {
      return agentsCache.byId;
    }
    if (agentsInFlight) return agentsInFlight;
    agentsInFlight = (async () => {
      try {
        const r = await fetch(`${ENGINE_URL}/api/v1/agents`);
        if (!r.ok) return new Map<string, AgentsRow>();
        const j = await r.json() as { agents?: AgentsRow[] };
        const byId = new Map<string, AgentsRow>();
        for (const a of j.agents ?? []) byId.set(a.entity_id, a);
        agentsCache = { fetchedAtMs: Date.now(), byId };
        return byId;
      } catch {
        return new Map<string, AgentsRow>();
      } finally {
        agentsInFlight = null;
      }
    })();
    return agentsInFlight;
  }

  function fetchAndSetMentalState(entityID: string) {
    // Kick off both fetches in parallel. The agents lookup is cheap
    // (cached for 2s) and its only contribution is the is_llm flag
    // for the inspector header badge.
    const mentalP = fetchMentalState(entityID);
    const agentsP = getAgentsLookup();
    Promise.all([mentalP, agentsP]).then(([m, agents]: [MentalStateResponse, Map<string, AgentsRow>]) => {
      const row = agents.get(entityID);
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
        peers: m.peers ?? {},
        vitals: m.vitals,
        is_llm: row?.is_llm,
      });
    }).catch(() => setMentalState(null));
  }
  let canvasContainer!: HTMLDivElement;
  let pixiHandle: PixiHandle | null = null;
  let viewer: ViewerClient | null = null;
  // Tracks the sprite id under the pointer so hover-out only hides
  // the info panel when the cursor *actually* leaves THAT sprite.
  // Also reset explicitly by edits that destroy the hovered sprite
  // (Pixi doesn't fire pointerout when the target gets destroyed
  // under the cursor — the panel would otherwise stick forever).
  let hoveredSprite: string | null = null;
  const clearHoverInfo = () => {
    hoveredSprite = null;
    setInfo(null);
  };

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
      if (editorOpen() && editorCategory() !== "tile" && editorTool() === "erase") {
        removeDecorationAt(ev.tileX, ev.tileY);
        return;
      }
      if (editorOpen() && editorCategory() !== "tile" && editorTool() === "paint" && editorDeco()) {
        // Decoration drop — POSTs the engine then optimistically renders.
        dropDecorationAt(ev.tileX, ev.tileY, editorDeco()!);
        return;
      }
      if (ev.entity) {
        // D8 — items are NOT agents. Don't open the Mind/Speech/Trace
        // inspector for them; the InfoPanel (hover-driven) is the
        // right surface for "what is this thing on the ground".
        if (ev.entity.archetype === "item") {
          return;
        }
        setSelectedId(ev.entity.entity_id);
        setSelectedSnapshot(ev.entity);
        pixiHandle?.setSelectedEntity(ev.entity.entity_id);
        fetchAndSetMentalState(ev.entity.entity_id);
      } else {
        closeInspector();
      }
    });

    // removeDecorationAt — engine deletes the topmost decoration at
    // (tileX, tileY) and we mirror locally for instant feedback. Same
    // overlay path as dropDecorationAt; the engine encodes the removal
    // with op=remove so a restart still cleans up.
    async function removeDecorationAt(tileX: number, tileY: number) {
      if (tileX < 0 || tileY < 0) return;
      // Optimistic local removal.
      const removed = pixiHandle?.removeDecorationAt(tileX, tileY) ?? false;
      // The hovered sprite likely just got destroyed under the cursor
      // — Pixi won't fire pointerout on a destroyed target, so the
      // panel would stick. Clear it manually; the next hover will
      // repopulate it.
      if (removed) clearHoverInfo();
      try {
        const { ENGINE_URL } = await import("../net/api");
        const r = await fetch(
          `${ENGINE_URL}/api/v1/world/edit_deco`,
          {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ op: "remove", x: tileX, y: tileY }),
          },
        );
        if (!r.ok) {
          const body = await r.json().catch(() => ({}));
          console.warn(`deco remove rejected: ${body.reason ?? r.status}`);
        }
      } catch (e) {
        console.warn(`deco remove failed: ${(e as Error).message}`);
      }
      void removed;
    }

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
    // sprite B fires before pointerout on sprite A. hoveredSprite is
    // hoisted to component scope so editor remove can clear it when
    // the hovered sprite gets destroyed under the cursor.
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
    // D8 — items get the same hover-driven InfoPanel as decorations.
    pixiHandle.onItemHoverEnter((ev) => showInfoFor(ev.sprite, ev.pos[0], ev.pos[1], "world"));
    pixiHandle.onItemHoverExit((ev) => hideInfoFor(ev.sprite));
    pixiHandle.onDecorationHoverExit((ev) => hideInfoFor(ev.sprite));
    pixiHandle.onInteriorPropHoverEnter((ev) => showInfoFor(ev.sprite, ev.x, ev.y, "interior"));
    pixiHandle.onInteriorPropHoverExit((ev) => hideInfoFor(ev.sprite));

    // D17 task 6.2 — agent hover-card. One fetch per hover-enter,
    // result cached for 2s via getAgentsLookup(). We also tap the
    // entity's vitals via fetchMentalState if it's in our live
    // entity list, but vitals there require an extra round-trip and
    // a 200ms hover doesn't justify it — the engine's /api/v1/agents
    // doesn't carry hp/gold today, so we fall back to "0" until the
    // engine exposes vitals there. For now, the /mental_state vitals
    // path drives the hover card too: one fetch, cached briefly.
    //
    // hoveredEntity guards the swap-race in the same way hoveredSprite
    // does for decorations.
    let hoveredEntity: string | null = null;
    const vitalsCache = new Map<string, { fetchedAtMs: number; hp: number; max_hp: number; gold: number }>();
    const VITALS_TTL_MS = 2000;
    async function getVitals(entityId: string) {
      const now = Date.now();
      const c = vitalsCache.get(entityId);
      if (c && now - c.fetchedAtMs < VITALS_TTL_MS) return c;
      try {
        const m = await fetchMentalState(entityId);
        const v = {
          fetchedAtMs: Date.now(),
          hp: m.vitals?.hp ?? 0,
          max_hp: m.vitals?.max_hp ?? 0,
          gold: m.vitals?.gold ?? 0,
        };
        vitalsCache.set(entityId, v);
        return v;
      } catch {
        return { fetchedAtMs: Date.now(), hp: 0, max_hp: 0, gold: 0 };
      }
    }
    pixiHandle.onAgentHoverEnter((ev) => {
      hoveredEntity = ev.entity_id;
      setHoverAt({ x: ev.screen_x, y: ev.screen_y });
      // Fire the agents lookup + vitals lookup. Show whatever resolves
      // first; the hover card re-renders when state updates.
      void Promise.all([getAgentsLookup(), getVitals(ev.entity_id)]).then(([agents, vit]) => {
        if (hoveredEntity !== ev.entity_id) return;   // pointer already moved
        const row = agents.get(ev.entity_id);
        setHoverCard({
          entity_id: ev.entity_id,
          display_name: ev.display_name ?? row?.display_name ?? row?.persona_name,
          archetype: row?.archetype ?? ev.archetype,
          is_llm: row?.is_llm ?? false,
          // M1: a background NPC isn't in /api/v1/agents (connected agents
          // only), so it would otherwise show a misleading "rule" badge +
          // 0/0 HP. Flag it so the card renders "NPC" and hides vitals.
          is_npc: row === undefined,
          hp: vit.hp,
          max_hp: vit.max_hp,
          gold: vit.gold,
        });
      });
    });
    pixiHandle.onAgentHoverExit((ev) => {
      if (hoveredEntity === ev.entity_id) {
        hoveredEntity = null;
        setHoverCard(null);
        setHoverAt(null);
      }
    });

    // ESC closes the inspector. (The info panel hides on mouse-out
    // automatically — no manual close needed.)
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeInspector();
    };
    window.addEventListener("keydown", onKey);
    onCleanup(() => window.removeEventListener("keydown", onKey));

    // Tell PixiApp whether the editor panel is open. While it is,
    // building click → interior entry is suppressed so the click
    // belongs entirely to the editor (place / remove).
    createEffect(() => {
      pixiHandle?.setEditorActive(editorOpen());
    });

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

      <AgentHoverCard info={hoverCard()} at={hoverAt()} />

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

      <StoryFeed />

      <JoinAgent open={joinOpen()} onClose={() => setJoinOpen(false)} />
      <Onboarding />
    </div>
  );
}
