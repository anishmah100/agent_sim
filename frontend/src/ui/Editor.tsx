// Editor — dev-mode panel overlaid on the Pixi viewport. Toggled with
// Cmd+E / Ctrl+E. Matches the existing FrontendV3 visual style (dark
// palette, single-tile chrome, inline styles).
//
// Scope (Phase WORLD-3): scaffolds the tile palette + tool selector +
// "Save" button. Live tile-paint that mutates the Pixi tilemap and
// roundtrips to the engine is Phase WORLD-4 — this commit ships the UI
// surface so the user can see the editor and confirm the layout before
// the heavy plumbing lands.

import { For, Show, createSignal, onCleanup, onMount } from "solid-js";

export type EditorTool = "select" | "paint" | "erase";

export interface EditorProps {
  open: boolean;
  onToggle: (open: boolean) => void;
  tilesLegend: Record<string, string> | null;
  tool: EditorTool;
  onToolChange: (t: EditorTool) => void;
  selectedGlyph: string | null;
  onSelectedGlyphChange: (g: string | null) => void;
  onSave?: () => void;
}

export function Editor(props: EditorProps) {
  // Tool + selected glyph are controlled by the parent so the canvas
  // click handler can read them. Old local state is gone.
  const tool = () => props.tool;
  const setTool = (t: EditorTool) => props.onToolChange(t);
  const selectedGlyph = () => props.selectedGlyph;
  const setSelectedGlyph = (g: string | null) => props.onSelectedGlyphChange(g);
  // Cached so the Show below still has a stable value.
  void createSignal;

  onMount(() => {
    const onKey = (e: KeyboardEvent) => {
      // Cmd+E (mac) / Ctrl+E (others) toggle.
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "e") {
        e.preventDefault();
        props.onToggle(!props.open);
      }
      // ESC closes.
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
          width: "260px",
          background: "rgba(24, 20, 37, 0.95)",
          "border-left": "1px solid #3a4466",
          color: "#ead4aa",
          padding: "12px",
          display: "flex",
          "flex-direction": "column",
          gap: "12px",
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

        <div style={{ opacity: "0.7", "font-size": "11px" }}>
          Pick a tile glyph, then click anywhere on the world to paint.
          Cmd+E toggles. Erase paints grass.
        </div>

        {/* Tool selector */}
        <div>
          <div style={{ opacity: "0.7", "margin-bottom": "4px" }}>Tool</div>
          <div style={{ display: "flex", gap: "4px" }}>
            <For each={["select", "paint", "erase"] as EditorTool[]}>
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

        {/* Tile palette — from the world's tiles_legend */}
        <Show when={props.tilesLegend} fallback={<NoLegend />}>
          {(legend) => (
            <div>
              <div style={{ opacity: "0.7", "margin-bottom": "4px" }}>
                Tile palette ({Object.keys(legend()).length} glyphs)
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
                      <span
                        style={{
                          "font-family": "monospace",
                          "font-size": "16px",
                          "font-weight": "bold",
                        }}
                      >
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

        {/* Save */}
        <div style={{ "margin-top": "auto" }}>
          <button
            type="button"
            onClick={() => props.onSave?.()}
            style={primaryBtnStyle()}
            disabled={!selectedGlyph() || tool() !== "paint"}
            data-testid="save-btn"
          >
            Save back to YAML
          </button>
          <div style={{ opacity: "0.5", "font-size": "11px", "margin-top": "4px" }}>
            Selected glyph: <code>{selectedGlyph() ?? "(none)"}</code> ·
            tool: <code>{tool()}</code>
          </div>
        </div>
      </div>
    </Show>
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

function primaryBtnStyle() {
  return {
    background: "#feae34",
    color: "#1f2238",
    border: "none",
    "border-radius": "3px",
    padding: "8px 14px",
    cursor: "pointer",
    "font-weight": "bold",
    "font-size": "13px",
    width: "100%",
  };
}
