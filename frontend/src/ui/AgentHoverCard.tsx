// AgentHoverCard — peek-only floating card next to the cursor when
// the user hovers an agent sprite. The Inspector remains the
// click-to-open deep view; this card is a 200ms-skim affordance:
// name, archetype, LLM/rule badge, HP bar, gold.
//
// Positioning: absolute on the page, offset slightly down-right of
// the cursor so the card never sits exactly under the pointer (which
// would re-trigger pointerout). The parent (App.tsx) is responsible
// for unmounting on hover-exit; the component itself doesn't track
// mouse state.

import { Show } from "solid-js";

export interface AgentHoverInfo {
  entity_id: string;
  display_name?: string;
  archetype: string;
  is_llm: boolean;
  hp: number;
  max_hp: number;
  gold: number;
}

export function AgentHoverCard(props: {
  info: AgentHoverInfo | null;
  at: { x: number; y: number } | null;
}) {
  return (
    <Show when={props.info && props.at}>
      {(_) => {
        const info = () => props.info!;
        const at = () => props.at!;
        const hpPct = () => {
          const m = info().max_hp;
          if (m <= 0) return 0;
          return Math.max(0, Math.min(100, Math.round((info().hp / m) * 100)));
        };
        const hpColor = () =>
          hpPct() >= 70 ? "#34d399"
          : hpPct() >= 30 ? "#fbbf24"
          : "#f87171";
        return (
          <div
            data-testid="agent-hover-card"
            style={{
              position: "fixed",
              // Offset 14px down-right so the card sits next to the
              // pointer, not under it. Clamp near the right edge so
              // the 180px card never spills off-screen.
              left: `${Math.min(at().x + 14, window.innerWidth - 196)}px`,
              top: `${Math.min(at().y + 14, window.innerHeight - 120)}px`,
              width: "180px",
              background: "rgba(24, 20, 37, 0.96)",
              border: "1px solid #5a6988",
              "border-radius": "4px",
              padding: "8px 10px",
              color: "#ead4aa",
              "font-size": "12px",
              "z-index": "70",
              "box-shadow": "0 4px 14px rgba(0,0,0,0.55)",
              "pointer-events": "none",
            }}
          >
            <div
              style={{
                display: "flex",
                "align-items": "baseline",
                gap: "6px",
                "margin-bottom": "4px",
              }}
            >
              <strong
                style={{
                  color: "#fee761",
                  "font-size": "12px",
                  flex: "1",
                  overflow: "hidden",
                  "text-overflow": "ellipsis",
                  "white-space": "nowrap",
                }}
                title={info().display_name ?? info().entity_id}
              >
                {info().display_name ?? info().entity_id}
              </strong>
              {info().is_llm
                ? <Badge kind="llm" />
                : <Badge kind="rule" />}
            </div>
            <div
              style={{
                "font-size": "10px",
                color: "#8b9bb4",
                "font-family": "ui-monospace, monospace",
                "margin-bottom": "6px",
                overflow: "hidden",
                "text-overflow": "ellipsis",
                "white-space": "nowrap",
              }}
              title={info().entity_id}
            >
              {info().entity_id} · {info().archetype}
            </div>
            <div style={{ "margin-bottom": "4px" }}>
              <div
                style={{
                  display: "flex",
                  "justify-content": "space-between",
                  "font-size": "10px",
                  color: "#8b9bb4",
                  "margin-bottom": "2px",
                }}
              >
                <span>HP</span>
                <span style={{ color: hpColor(), "font-family": "ui-monospace, monospace" }}>
                  {info().hp}/{info().max_hp || "?"}
                </span>
              </div>
              <div
                style={{
                  width: "100%",
                  height: "6px",
                  background: "#1f1d2e",
                  border: "1px solid #3a4466",
                  "border-radius": "3px",
                  overflow: "hidden",
                }}
              >
                <div
                  style={{
                    width: `${hpPct()}%`,
                    height: "100%",
                    background: hpColor(),
                    transition: "width 120ms linear",
                  }}
                />
              </div>
            </div>
            <div
              style={{
                display: "flex",
                "justify-content": "space-between",
                "font-size": "11px",
                "font-family": "ui-monospace, monospace",
              }}
            >
              <span style={{ color: "#8b9bb4" }}>gold</span>
              <span style={{ color: "#facc15" }}>{info().gold} g</span>
            </div>
          </div>
        );
      }}
    </Show>
  );
}

/** Small pill — "LLM" cyan or "rule" gray. Exported because the
 *  Inspector header uses the same visual treatment (task 6.5). */
export function Badge(props: { kind: "llm" | "rule" }) {
  const isLlm = props.kind === "llm";
  return (
    <span
      data-testid={isLlm ? "llm-badge" : "rule-badge"}
      style={{
        display: "inline-block",
        padding: "1px 6px",
        "border-radius": "8px",
        "font-size": "10px",
        "font-weight": "600",
        "font-family": "ui-monospace, monospace",
        background: isLlm ? "rgba(34, 211, 238, 0.18)" : "rgba(139, 155, 180, 0.18)",
        color: isLlm ? "#22d3ee" : "#8b9bb4",
        border: `1px solid ${isLlm ? "#22d3ee" : "#5a6988"}`,
        "line-height": "1.2",
        "letter-spacing": "0.5px",
      }}
    >
      {isLlm ? "LLM" : "rule"}
    </span>
  );
}
