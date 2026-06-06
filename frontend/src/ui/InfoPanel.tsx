// InfoPanel — side drawer that shows what the user just clicked on the
// world (a building, stall, well, item, FX, or interior prop). Mirrors
// the Inspector visual language but is read-only: title, thumbnail,
// kind, description, and a stat block. Enterable buildings get an
// "Enter" button that triggers the interior view via onEnter.
//
// Opens whenever `info` is non-null. Closes via × or ESC.

import { For, Show } from "solid-js";
import type { SpriteInfo } from "./SpriteInfo";

export interface InfoPanelProps {
  info: SpriteInfo | null;
  /** Tile coords of the clicked decoration — shown under the kind line. */
  at: { x: number; y: number } | null;
  /** "world" decoration on the overworld vs "interior" prop. Drives
   *  the "Location" line copy. */
  source: "world" | "interior";
  onClose: () => void;
  /** Called when the user clicks "Enter" on an enterable building. */
  onEnter?: () => void;
}

export function InfoPanel(props: InfoPanelProps) {
  return (
    <Show when={props.info}>
      {(infoAccessor) => {
        const info = infoAccessor();
        return (
          <div
            role="dialog"
            aria-label="object info"
            data-testid="info-panel"
            style={{
              position: "absolute",
              top: "56px",
              right: "16px",
              width: "320px",
              "max-height": "calc(100vh - 88px)",
              overflow: "auto",
              background: "rgba(24, 20, 37, 0.95)",
              border: "1px solid #5a6988",
              "border-radius": "4px",
              padding: "12px 14px",
              color: "#ead4aa",
              "font-size": "13px",
              "z-index": "20",
              "box-shadow": "0 4px 18px rgba(0,0,0,0.45)",
            }}
          >
            <div
              style={{
                display: "flex",
                "justify-content": "space-between",
                "align-items": "center",
                "margin-bottom": "10px",
                "padding-bottom": "8px",
                "border-bottom": "1px solid #3a4466",
              }}
            >
              <strong style={{ color: "#fee761", "font-size": "14px" }}>
                {info.title}
              </strong>
              <button
                type="button"
                onClick={() => props.onClose()}
                aria-label="close info panel"
                style={btnStyle()}
              >
                ×
              </button>
            </div>

            <div style={{ display: "flex", gap: "12px", "margin-bottom": "10px" }}>
              <Show when={info.thumbUrl}>
                <div
                  style={{
                    flex: "0 0 64px",
                    width: "64px",
                    height: "64px",
                    background: "rgba(0,0,0,0.35)",
                    border: "1px solid #3a4466",
                    "border-radius": "3px",
                    display: "flex",
                    "align-items": "center",
                    "justify-content": "center",
                    overflow: "hidden",
                  }}
                >
                  <img
                    src={info.thumbUrl!}
                    alt={info.title}
                    style={{
                      "max-width": "100%",
                      "max-height": "100%",
                      "image-rendering": "pixelated",
                    }}
                  />
                </div>
              </Show>
              <div style={{ flex: "1", display: "grid", "row-gap": "3px" }}>
                <div style={{ color: "#8b9bb4", "font-size": "12px" }}>{info.kind}</div>
                <Show when={props.at}>
                  {(atAcc) => (
                    <div style={{ color: "#5a6988", "font-size": "11px" }}>
                      {props.source === "interior" ? "Inside · " : "Tile · "}
                      ({atAcc().x}, {atAcc().y})
                    </div>
                  )}
                </Show>
                <div style={{ color: "#5a6988", "font-size": "11px" }}>
                  {info.sprite}
                </div>
              </div>
            </div>

            <div
              style={{
                "font-family": "ui-monospace, monospace",
                "font-size": "12px",
                "line-height": "1.5",
                "margin-bottom": info.stats.length > 0 ? "10px" : "0",
                color: "#ead4aa",
              }}
            >
              {info.description}
            </div>

            <Show when={info.stats.length > 0}>
              <div
                style={{
                  display: "grid",
                  "row-gap": "4px",
                  "padding-top": "10px",
                  "border-top": "1px solid #3a4466",
                }}
              >
                <For each={info.stats}>
                  {(s) => (
                    <div style={{ display: "flex", gap: "8px" }}>
                      <span style={{ color: "#8b9bb4", "min-width": "82px", "font-size": "12px" }}>
                        {s.label}
                      </span>
                      <span style={{
                        color: "#fee761",
                        "font-family": "ui-monospace, monospace",
                        "font-size": "12px",
                      }}>
                        {s.value}
                      </span>
                    </div>
                  )}
                </For>
              </div>
            </Show>

            <Show when={info.enterable && props.onEnter}>
              <button
                type="button"
                onClick={() => props.onEnter?.()}
                data-testid="info-enter-button"
                style={{
                  display: "block",
                  width: "100%",
                  "margin-top": "12px",
                  padding: "6px 10px",
                  background: "#feae34",
                  color: "#1f2238",
                  border: "1px solid #feae34",
                  "border-radius": "3px",
                  cursor: "pointer",
                  "font-weight": "700",
                  "font-size": "12px",
                }}
              >
                Enter →
              </button>
            </Show>
          </div>
        );
      }}
    </Show>
  );
}

function btnStyle() {
  return {
    background: "transparent",
    color: "#ead4aa",
    border: "1px solid #5a6988",
    "border-radius": "3px",
    padding: "2px 8px",
    "font-size": "13px",
    cursor: "pointer",
  };
}
