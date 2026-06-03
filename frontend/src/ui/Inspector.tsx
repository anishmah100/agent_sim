// Inspector side panel.
//
// Opens on entity click. Shows id / archetype / position / facing /
// extras blob. Closes via the X button or ESC. DOM-only — no canvas
// mixing per docs/ANTI_MESS_PLAN.md §5.

import { Show, createMemo } from "solid-js";
import type { EntityState } from "../render/Entity";

export function Inspector(props: {
  entity: EntityState | null;
  onClose: () => void;
}) {
  const isOpen = createMemo(() => props.entity !== null);

  return (
    <Show when={isOpen()}>
      <div
        role="dialog"
        aria-label="entity inspector"
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
            {props.entity?.display_name ?? props.entity?.entity_id ?? "(unknown)"}
          </strong>
          <button
            type="button"
            onClick={() => props.onClose()}
            aria-label="close inspector"
            style={{
              background: "transparent",
              color: "#ead4aa",
              border: "1px solid #5a6988",
              "border-radius": "3px",
              padding: "2px 8px",
              "font-size": "13px",
              cursor: "pointer",
            }}
          >
            ×
          </button>
        </div>

        <Show when={props.entity}>
          {(e) => (
            <div style={{ display: "grid", "row-gap": "4px" }}>
              <Field label="entity_id">{e().entity_id}</Field>
              <Field label="archetype">{e().archetype}</Field>
              <Field label="pos">
                ({e().pos[0].toFixed(2)}, {e().pos[1].toFixed(2)})
              </Field>
              <Field label="facing">{e().facing}</Field>
            </div>
          )}
        </Show>

        <div
          style={{
            "margin-top": "12px",
            "padding-top": "10px",
            "border-top": "1px solid #3a4466",
            "font-size": "11px",
            color: "#8b9bb4",
          }}
        >
          Full persona / vitals / recent actions land in milestone 6.
        </div>
      </div>
    </Show>
  );
}

function Field(props: { label: string; children: any }) {
  return (
    <div style={{ display: "flex", gap: "8px" }}>
      <span style={{ color: "#8b9bb4", "min-width": "82px" }}>{props.label}</span>
      <span style={{ color: "#ead4aa", "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
        {props.children}
      </span>
    </div>
  );
}
