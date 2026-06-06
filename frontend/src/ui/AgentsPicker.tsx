// AgentsPicker — toolbar dropdown listing every LLM/SDK-connected
// agent in the world. Click one to center the camera on them and
// open the mental-state inspector.
//
// Polls /api/v1/agents every 2s — the engine builds the response from
// its agent hub + snapshot, so positions stay current as agents walk
// around. The picker is closed by default; toggled via the toolbar
// button next to "editor".

import { For, Show, createSignal, onCleanup, onMount } from "solid-js";
import { ENGINE_URL } from "../net/api";

export interface ConnectedAgent {
  agent_id: string;
  entity_id: string;
  persona_name?: string;
  bio?: string;
  archetype?: string;
  display_name?: string;
  pos: [number, number];
  ms_connected: number;
}

export interface AgentsPickerProps {
  open: boolean;
  onClose: () => void;
  /** Called when the user clicks an agent row — camera should center
   *  on the agent's tile and open the inspector for them. */
  onPick: (agent: ConnectedAgent) => void;
}

export function AgentsPicker(props: AgentsPickerProps) {
  const [agents, setAgents] = createSignal<ConnectedAgent[]>([]);
  const [err, setErr] = createSignal<string | null>(null);
  let timer: number | undefined;

  async function refresh() {
    try {
      const r = await fetch(`${ENGINE_URL}/api/v1/agents`);
      if (!r.ok) {
        setErr(`${r.status}`);
        return;
      }
      const j = await r.json();
      setAgents(j.agents ?? []);
      setErr(null);
    } catch (e) {
      setErr((e as Error).message);
    }
  }

  onMount(() => {
    refresh();
    timer = window.setInterval(refresh, 2000);
  });
  onCleanup(() => {
    if (timer !== undefined) window.clearInterval(timer);
  });

  return (
    <Show when={props.open}>
      <div
        style={{
          position: "absolute",
          top: "48px",
          right: "16px",
          width: "320px",
          "max-height": "60vh",
          background: "rgba(24, 20, 37, 0.96)",
          border: "1px solid #3a4466",
          "border-radius": "6px",
          padding: "10px 12px",
          color: "#ead4aa",
          "font-size": "12px",
          "z-index": "60",
          display: "flex",
          "flex-direction": "column",
          gap: "8px",
          overflow: "auto",
        }}
        data-testid="agents-picker"
      >
        <div style={{ display: "flex", "align-items": "baseline" }}>
          <strong style={{ color: "#feae34", flex: "1" }}>
            connected agents ({agents().length})
          </strong>
          <button
            type="button"
            onClick={props.onClose}
            style={{
              background: "transparent",
              color: "#ead4aa",
              border: "1px solid #3a4466",
              "border-radius": "3px",
              padding: "1px 6px",
              cursor: "pointer",
            }}
          >
            ×
          </button>
        </div>

        <Show when={err()}>
          <div style={{ color: "#e43b44" }}>error: {err()}</div>
        </Show>

        <Show
          when={agents().length > 0}
          fallback={
            <div style={{ opacity: "0.7" }}>
              No agents connected. Spawn one via the SDK or start a
              `python -m examples.qwen_agent.main` against this engine.
            </div>
          }
        >
          <For each={agents()}>
            {(a) => (
              <button
                type="button"
                onClick={() => props.onPick(a)}
                data-testid={`agent-row-${a.entity_id}`}
                style={{
                  display: "flex",
                  "flex-direction": "column",
                  gap: "2px",
                  background: "#262b44",
                  color: "#ead4aa",
                  border: "1px solid #3a4466",
                  "border-radius": "4px",
                  padding: "6px 8px",
                  cursor: "pointer",
                  "text-align": "left",
                  "font-family": "inherit",
                }}
              >
                <div style={{ display: "flex", "align-items": "baseline" }}>
                  <strong style={{ color: "#fee761", flex: "1" }}>
                    {a.persona_name || "(no name)"}
                  </strong>
                  <span style={{ opacity: "0.6", "font-size": "11px" }}>
                    ({a.pos[0]}, {a.pos[1]})
                  </span>
                </div>
                <div style={{ opacity: "0.7", "font-size": "11px" }}>
                  bound to <code>{a.entity_id}</code>
                  {a.archetype ? ` · ${a.archetype}` : ""}
                </div>
                <Show when={a.bio}>
                  <div
                    style={{
                      opacity: "0.55",
                      "font-size": "11px",
                      "font-style": "italic",
                    }}
                  >
                    {a.bio!.length > 90 ? a.bio!.slice(0, 90) + "…" : a.bio}
                  </div>
                </Show>
              </button>
            )}
          </For>
        </Show>
      </div>
    </Show>
  );
}
