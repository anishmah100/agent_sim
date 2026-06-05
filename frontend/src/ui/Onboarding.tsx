// First-time visitor coachmarks. Shows up on first page load, dismisses
// permanently via localStorage. Five short tips covering pan/zoom,
// click-to-inspect, building entry, the story feed, and the "join as
// agent" button.

import { createSignal, Show, For } from "solid-js";

const STORAGE_KEY = "agent_sim:onboarding_seen_v1";

interface Tip {
  title: string;
  body: string;
  anchor: "top-bar" | "canvas" | "minimap" | "story-feed";
}

const TIPS: Tip[] = [
  {
    title: "Welcome to agent_sim",
    body: "This is a persistent simulated world with autonomous AI agents. Pan with click-drag, zoom with the scroll wheel.",
    anchor: "canvas",
  },
  {
    title: "Click an NPC to inspect",
    body: "Every entity has a public state — HP, gold, current goal. The inspector pops out from the right.",
    anchor: "canvas",
  },
  {
    title: "Click a building to enter",
    body: "Buildings have hand-authored interiors. The interior view fades over the world; click the door (or ESC) to leave.",
    anchor: "canvas",
  },
  {
    title: "Watch the story feed",
    body: "The right panel narrates what's happening in the world — bargains struck, fights, construction. Pin an entity to filter to its story.",
    anchor: "story-feed",
  },
  {
    title: "Join as an agent",
    body: "Click 'join as agent' in the top bar to attach your own LLM bot to the live world via the Python SDK.",
    anchor: "top-bar",
  },
];

export function Onboarding() {
  const seen = typeof window !== "undefined" &&
    window.localStorage?.getItem(STORAGE_KEY) === "1";
  const [open, setOpen] = createSignal(!seen);
  const [idx, setIdx] = createSignal(0);

  const dismiss = () => {
    try { window.localStorage?.setItem(STORAGE_KEY, "1"); } catch {}
    setOpen(false);
  };

  return (
    <Show when={open()}>
      <div
        role="dialog"
        aria-label="Onboarding"
        data-testid="onboarding"
        style={{
          position: "fixed",
          inset: "0",
          background: "rgba(24, 20, 37, 0.65)",
          display: "flex",
          "align-items": "center",
          "justify-content": "center",
          "z-index": "30",
        }}
      >
        <div
          style={{
            width: "420px",
            "max-width": "92vw",
            background: "#262b44",
            color: "#ead4aa",
            padding: "20px",
            "border-radius": "6px",
            border: "1px solid #3a4466",
            "font-family": "ui-sans-serif, system-ui, sans-serif",
          }}
        >
          <div style={{ display: "flex", "justify-content": "space-between", "align-items": "baseline" }}>
            <span style={{ color: "#8b9bb4", "font-size": "11px" }}>
              {idx() + 1} / {TIPS.length}
            </span>
            <button
              type="button"
              onClick={dismiss}
              data-testid="onboarding-skip"
              style={{
                background: "transparent", color: "#8b9bb4",
                border: "none", cursor: "pointer", "font-size": "11px",
              }}
            >
              skip
            </button>
          </div>
          <h2 style={{ color: "#fee761", margin: "8px 0 6px" }}>
            {TIPS[idx()].title}
          </h2>
          <p style={{ "margin-bottom": "16px", "line-height": "1.4" }}>
            {TIPS[idx()].body}
          </p>
          <div style={{ display: "flex", gap: "6px" }}>
            <For each={TIPS}>
              {(_, i) => (
                <span
                  style={{
                    width: "20px", height: "3px",
                    background: i() === idx() ? "#fee761" : "#3a4466",
                    "border-radius": "1px",
                  }}
                />
              )}
            </For>
          </div>
          <div style={{ display: "flex", "justify-content": "flex-end", gap: "8px", "margin-top": "14px" }}>
            <Show when={idx() > 0}>
              <button
                type="button"
                onClick={() => setIdx(idx() - 1)}
                style={{
                  background: "#3a4466", color: "#ead4aa",
                  border: "1px solid #5a6988", padding: "6px 12px",
                  "border-radius": "3px", cursor: "pointer",
                }}
              >
                back
              </button>
            </Show>
            <Show
              when={idx() < TIPS.length - 1}
              fallback={
                <button
                  type="button"
                  onClick={dismiss}
                  data-testid="onboarding-done"
                  style={{
                    background: "#fee761", color: "#181425",
                    border: "1px solid #fee761", padding: "6px 14px",
                    "border-radius": "3px", cursor: "pointer",
                    "font-weight": "600",
                  }}
                >
                  let me explore
                </button>
              }
            >
              <button
                type="button"
                onClick={() => setIdx(idx() + 1)}
                data-testid="onboarding-next"
                style={{
                  background: "#fee761", color: "#181425",
                  border: "1px solid #fee761", padding: "6px 14px",
                  "border-radius": "3px", cursor: "pointer",
                  "font-weight": "600",
                }}
              >
                next
              </button>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
}
