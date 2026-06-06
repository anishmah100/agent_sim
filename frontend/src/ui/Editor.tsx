// Editor — dev-mode panel overlaid on the Pixi viewport. Toggled with
// Cmd+E / Ctrl+E. Real-time: every paint / decoration drop mutates the
// LIVE engine world and persists to a sidecar overlay so a restart
// re-applies the same edits.
//
// Three palettes:
//   1. Tile      — repaint the ground (grass / stone / dirt / etc).
//   2. Building  — drop a cottage, blacksmith, granary, watchtower,
//                  market stall. Updates walkability + door registration.
//   3. Item      — drop a coin, gem, weapon, food. Walkable, single-tile.
//
// Clicking the canvas while the editor is open dispatches to whichever
// category is active. The Solid layer wires the canvas click to the
// appropriate POST.

import { For, Show, onCleanup, onMount } from "solid-js";
import { BUILDING_PALETTE, ITEM_PALETTE, type EditorCategory, type PaletteEntry } from "./EditorPalettes";

const ENGINE_URL =
  import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

export type EditorTool = "select" | "paint" | "erase";

export interface EditorProps {
  open: boolean;
  onToggle: (open: boolean) => void;
  tilesLegend: Record<string, string> | null;
  tool: EditorTool;
  onToolChange: (t: EditorTool) => void;
  selectedGlyph: string | null;
  onSelectedGlyphChange: (g: string | null) => void;
  category: EditorCategory;
  onCategoryChange: (c: EditorCategory) => void;
  selectedDeco: PaletteEntry | null;
  onSelectedDecoChange: (e: PaletteEntry | null) => void;
}

export function Editor(props: EditorProps) {
  const tool = () => props.tool;
  const setTool = (t: EditorTool) => props.onToolChange(t);
  const selectedGlyph = () => props.selectedGlyph;
  const setSelectedGlyph = (g: string | null) => props.onSelectedGlyphChange(g);
  const category = () => props.category;
  const setCategory = (c: EditorCategory) => props.onCategoryChange(c);
  const selectedDeco = () => props.selectedDeco;
  const setSelectedDeco = (e: PaletteEntry | null) => props.onSelectedDecoChange(e);

  onMount(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "e") {
        e.preventDefault();
        props.onToggle(!props.open);
      }
      if (e.key === "Escape" && props.open) {
        props.onToggle(false);
      }
    };
    window.addEventListener("keydown", onKey);
    onCleanup(() => window.removeEventListener("keydown", onKey));
  });

  return (
    <Show when={props.open}>
      <div
        style={{
          position: "absolute",
          top: "48px",
          right: "0",
          bottom: "0",
          width: "300px",
          background: "rgba(24, 20, 37, 0.95)",
          "border-left": "1px solid #3a4466",
          color: "#ead4aa",
          padding: "12px",
          display: "flex",
          "flex-direction": "column",
          gap: "10px",
          "z-index": "9",
          "font-size": "13px",
          "font-family": "system-ui, sans-serif",
          "overflow-y": "auto",
        }}
        data-testid="editor-panel"
      >
        <div style={{ display: "flex", "align-items": "center" }}>
          <strong style={{ color: "#feae34", flex: "1" }}>World editor</strong>
          <button
            type="button"
            onClick={() => props.onToggle(false)}
            style={btnStyle()}
            title="Close editor (Esc)"
          >
            ×
          </button>
        </div>

        <div style={{ opacity: "0.7", "font-size": "11px", "line-height": "1.4" }}>
          Real-time. Every edit hits the live engine so agents in the
          world see the change immediately. Cmd+E toggles.
        </div>

        {/* Category tabs */}
        <div style={{ display: "flex", gap: "4px" }}>
          <For each={["tile", "building", "item"] as EditorCategory[]}>
            {(c) => (
              <button
                type="button"
                onClick={() => setCategory(c)}
                style={categoryBtnStyle(category() === c)}
                data-testid={`category-${c}`}
              >
                {c}
              </button>
            )}
          </For>
        </div>

        {/* Tile-mode body */}
        <Show when={category() === "tile"}>
          <div>
            <div style={{ opacity: "0.7", "margin-bottom": "4px" }}>Tool</div>
            <div style={{ display: "flex", gap: "4px" }}>
              <For each={["paint", "erase"] as EditorTool[]}>
                {(t) => (
                  <button
                    type="button"
                    onClick={() => setTool(t)}
                    style={toolBtnStyle(tool() === t)}
                    data-testid={`tool-${t}`}
                  >
                    {t}
                  </button>
                )}
              </For>
            </div>
          </div>
          <Show when={props.tilesLegend} fallback={<NoLegend />}>
            {(legend) => (
              <div>
                <div style={{ opacity: "0.7", "margin-bottom": "4px" }}>
                  Ground tile
                </div>
                <div
                  style={{
                    display: "grid",
                    "grid-template-columns": "repeat(auto-fill, minmax(72px, 1fr))",
                    gap: "4px",
                  }}
                >
                  <For each={Object.entries(legend())}>
                    {([glyph, kind]) => (
                      <button
                        type="button"
                        onClick={() => setSelectedGlyph(glyph)}
                        style={paletteBtnStyle(selectedGlyph() === glyph)}
                        title={kind}
                        data-testid={`palette-${glyph}`}
                      >
                        <span style={{
                          "font-family": "monospace",
                          "font-size": "16px",
                          "font-weight": "bold",
                        }}>
                          {glyph}
                        </span>
                        <br />
                        <span style={{ opacity: "0.6", "font-size": "11px" }}>{kind}</span>
                      </button>
                    )}
                  </For>
                </div>
              </div>
            )}
          </Show>
          <div style={{ opacity: "0.5", "font-size": "11px" }}>
            Click the world to paint. Selected: <code>{selectedGlyph() ?? "(none)"}</code>
          </div>
        </Show>

        {/* Decoration-mode body — buildings + items share the same UI */}
        <Show when={category() !== "tile"}>
          <div>
            <div style={{ opacity: "0.7", "margin-bottom": "4px" }}>Mode</div>
            <div style={{ display: "flex", gap: "4px" }}>
              <For each={["paint", "erase"] as EditorTool[]}>
                {(t) => (
                  <button
                    type="button"
                    onClick={() => setTool(t)}
                    style={toolBtnStyle(tool() === t)}
                    data-testid={`tool-${t}`}
                    title={t === "paint" ? "Click to place" : "Click to remove"}
                  >
                    {t === "paint" ? "place" : "remove"}
                  </button>
                )}
              </For>
            </div>
          </div>
          <Show when={tool() === "paint"}>
            <PalettePicker
              entries={category() === "building" ? BUILDING_PALETTE : ITEM_PALETTE}
              selected={selectedDeco()}
              onSelect={setSelectedDeco}
              label={category() === "building" ? "Buildings" : "Items"}
            />
            <div style={{ opacity: "0.5", "font-size": "11px" }}>
              Click the world to drop. Selected:{" "}
              <code>{selectedDeco()?.label ?? "(none)"}</code>
            </div>
          </Show>
          <Show when={tool() === "erase"}>
            <div style={{ opacity: "0.6", "font-size": "12px", "line-height": "1.4" }}>
              Click a placed {category()} on the world to remove it.
              The topmost match at that tile gets deleted.
            </div>
          </Show>
        </Show>
      </div>
    </Show>
  );
}

function PalettePicker(props: {
  entries: PaletteEntry[];
  selected: PaletteEntry | null;
  onSelect: (e: PaletteEntry) => void;
  label: string;
}) {
  return (
    <div>
      <div style={{ opacity: "0.7", "margin-bottom": "4px" }}>
        {props.label} ({props.entries.length})
      </div>
      <div
        style={{
          display: "grid",
          "grid-template-columns": "repeat(auto-fill, minmax(64px, 1fr))",
          gap: "4px",
        }}
      >
        <For each={props.entries}>
          {(e) => (
            <button
              type="button"
              onClick={() => props.onSelect(e)}
              title={e.label}
              data-testid={`palette-deco-${e.sprite.replace(/[:_]/g, "-")}`}
              style={{
                ...paletteBtnStyle(props.selected?.sprite === e.sprite),
                display: "flex",
                "flex-direction": "column",
                "align-items": "center",
                gap: "2px",
                padding: "4px",
              }}
            >
              <img
                src={`${ENGINE_URL}/art/manifests/`} // placeholder; real URL injected
                alt=""
                style={{ display: "none" }}
              />
              <DecoThumb sprite={e.sprite} />
              <span style={{
                "font-size": "9px",
                "line-height": "1.1",
                "text-align": "center",
                opacity: 0.85,
              }}>
                {e.label}
              </span>
            </button>
          )}
        </For>
      </div>
    </div>
  );
}

function DecoThumb(props: { sprite: string }) {
  // Resolve a thumbnail URL for the sprite. Mirrors the catalog
  // logic — buildings live at v2_<name>.png or processed/objects/...,
  // items at v2_items_master_v2/<name>.png, stalls at v2_market_stall/...
  // For an editor preview we accept a missing image (broken-image icon
  // in browser, no fault state needed).
  const [cat, name] = props.sprite.split(":");
  let url = `${ENGINE_URL}/art/processed/`;
  if (cat === "item") url += `v2_items_master_v2/${name}.png`;
  else if (cat === "bld" && name.startsWith("stall_")) url += `v2_market_stall/${name}.png`;
  else if (cat === "bld" && /^(blacksmith|town_hall|granary|watchtower|well|000|001)$/.test(name)) {
    url += name === "000" || name === "001" ? `objects/buildings/obj_${name}.png` : `v2_${name}.png`;
  } else url += `${name}.png`;
  return (
    <img
      src={url}
      alt=""
      style={{
        width: "32px",
        height: "32px",
        "object-fit": "contain",
        "image-rendering": "pixelated",
      }}
      onError={(e) => { (e.currentTarget as HTMLImageElement).style.visibility = "hidden"; }}
    />
  );
}

function NoLegend() {
  return (
    <div style={{ opacity: "0.6", "font-size": "12px" }}>
      Tile legend unavailable — world.json didn't ship one or the engine
      isn't reachable.
    </div>
  );
}

function btnStyle() {
  return {
    background: "transparent",
    color: "#ead4aa",
    border: "1px solid #3a4466",
    "border-radius": "3px",
    padding: "2px 8px",
    cursor: "pointer",
    "font-size": "13px",
  };
}

function categoryBtnStyle(active: boolean) {
  return {
    flex: "1",
    background: active ? "#feae34" : "#262b44",
    color: active ? "#1f2238" : "#ead4aa",
    border: "1px solid #3a4466",
    "border-radius": "3px",
    padding: "6px 8px",
    cursor: "pointer",
    "font-size": "12px",
    "font-weight": "700",
    "text-transform": "capitalize" as const,
  };
}

function toolBtnStyle(active: boolean) {
  return {
    flex: "1",
    background: active ? "#feae34" : "#262b44",
    color: active ? "#1f2238" : "#ead4aa",
    border: "1px solid #3a4466",
    "border-radius": "3px",
    padding: "4px 6px",
    cursor: "pointer",
    "font-size": "12px",
  };
}

function paletteBtnStyle(active: boolean) {
  return {
    background: active ? "#feae34" : "#1f2238",
    color: active ? "#1f2238" : "#ead4aa",
    border: active ? "2px solid #feae34" : "1px solid #3a4466",
    "border-radius": "3px",
    padding: "6px",
    cursor: "pointer",
    "text-align": "center" as const,
  };
}
