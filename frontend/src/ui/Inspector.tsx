// Inspector side panel — Phase AGENT-A7 mental-state drawer.
//
// 3 tabs:
//   Speech — last N public lines the viewer could have witnessed
//            (always visible)
//   Mind   — current top-goal + last reflection note. Visible only
//            when this agent has share_planner=true on registration
//            (engine endpoint will gate; placeholder data for now)
//   Trace  — last few reasoning traces. Visible only when BOTH
//            capture_reasoning AND share_reasoning are true
//
// Opens on entity click. Closes via × or ESC. DOM-only — no canvas
// mixing per docs/ANTI_MESS_PLAN.md §5.

import { For, Show, createMemo, createSignal } from "solid-js";
import type { EntityState } from "../render/Entity";

export interface DialogueLine {
  tick: number;
  speaker: string;
  channel: "speech" | "shout" | "whisper" | "sound";
  text: string;
}

export interface MindSnapshot {
  share_planner: boolean;
  top_goal: string | null;
  last_reflection: string | null;
  goal_stack_size: number;
}

export interface TraceLine {
  tick: number;
  action_id: string;
  verb: string;
  reasoning: string;
}

export interface MentalState {
  dialogue: DialogueLine[];
  mind: MindSnapshot;
  traces: TraceLine[];
  capture_reasoning_enabled: boolean;
}

type Tab = "speech" | "mind" | "trace";

export function Inspector(props: {
  entity: EntityState | null;
  mentalState?: MentalState;
  onClose: () => void;
}) {
  const isOpen = createMemo(() => props.entity !== null);
  const [tab, setTab] = createSignal<Tab>("speech");

  // Original gates required share_planner=true (never wired —
  // hardcoded false in the engine handler comment) and traces non-
  // empty. Both made the Mind + Trace tabs invisible in practice.
  // Now visible whenever the engine has -capture-reasoning on; the
  // tabs render their own "no data yet" inside if there's nothing.
  const mindVisible = createMemo(() =>
    !!props.mentalState?.capture_reasoning_enabled ||
    !!props.mentalState?.mind?.last_reflection,
  );
  const traceVisible = createMemo(() =>
    !!props.mentalState?.capture_reasoning_enabled,
  );

  return (
    <Show when={isOpen()}>
      <div
        role="dialog"
        aria-label="entity inspector"
        data-testid="inspector"
        style={{
          position: "absolute",
          top: "56px",
          right: "16px",
          width: "360px",
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
        {/* Header */}
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
            style={btnStyle()}
          >
            ×
          </button>
        </div>

        {/* Identity block */}
        <Show when={props.entity}>
          {(e) => (
            <div style={{ display: "grid", "row-gap": "4px", "margin-bottom": "10px" }}>
              <Field label="entity_id">{e().entity_id}</Field>
              <Field label="archetype">{e().archetype}</Field>
              <Field label="pos">
                ({e().pos[0].toFixed(2)}, {e().pos[1].toFixed(2)})
              </Field>
              <Field label="facing">{e().facing}</Field>
            </div>
          )}
        </Show>

        {/* Tab bar */}
        <div
          style={{
            display: "flex",
            gap: "4px",
            "border-bottom": "1px solid #3a4466",
            "margin-bottom": "8px",
            "padding-bottom": "4px",
          }}
        >
          <TabBtn current={tab()} value="speech" onClick={() => setTab("speech")}
                  label="Speech" enabled />
          <TabBtn current={tab()} value="mind" onClick={() => setTab("mind")}
                  label="Mind" enabled={mindVisible()}
                  disabledHint="share_planner=false" />
          <TabBtn current={tab()} value="trace" onClick={() => setTab("trace")}
                  label="Trace" enabled={traceVisible()}
                  disabledHint={
                    props.mentalState?.capture_reasoning_enabled
                      ? "share_reasoning=false"
                      : "capture_reasoning=off"
                  } />
        </div>

        {/* Tab body */}
        <Show when={tab() === "speech"}>
          <SpeechTab lines={props.mentalState?.dialogue ?? []} />
        </Show>
        <Show when={tab() === "mind" && mindVisible()}>
          <MindTab mind={props.mentalState!.mind} />
        </Show>
        <Show when={tab() === "trace" && traceVisible()}>
          <TraceTab traces={props.mentalState!.traces} />
        </Show>
      </div>
    </Show>
  );
}

function TabBtn(p: {
  current: Tab;
  value: Tab;
  label: string;
  enabled: boolean;
  disabledHint?: string;
  onClick: () => void;
}) {
  // Bug fix: evaluating `const active = p.current === p.value` once at
  // mount means the active style never updates when the user clicks
  // another tab — `Speech` stayed yellow forever. Solid's reactivity
  // only fires inside JSX expressions / function-valued props, so the
  // style object's values now read p.current inline.
  const active = () => p.current === p.value;
  return (
    <button
      type="button"
      data-testid={`tab-${p.value}`}
      onClick={() => p.enabled && p.onClick()}
      disabled={!p.enabled}
      title={p.enabled ? p.label : `${p.label} (gated: ${p.disabledHint ?? ""})`}
      style={{
        background: active() ? "#feae34" : "transparent",
        color: active() ? "#1f2238" : p.enabled ? "#ead4aa" : "#5a6988",
        border: "1px solid " + (active() ? "#feae34" : "#3a4466"),
        "border-radius": "3px",
        padding: "4px 10px",
        cursor: p.enabled ? "pointer" : "not-allowed",
        "font-size": "12px",
        flex: "1",
      }}
    >
      {p.label}
    </button>
  );
}

function SpeechTab(props: { lines: DialogueLine[] }) {
  return (
    <Show
      when={props.lines.length > 0}
      fallback={
        <div style={emptyStyle()}>
          No recent dialogue from this agent.
        </div>
      }
    >
      <div data-testid="speech-tab" style={{ display: "grid", "row-gap": "6px" }}>
        <For each={props.lines.slice(-20)}>
          {(line) => (
            <div style={{ display: "grid", "row-gap": "2px" }}>
              <div style={{ "font-size": "11px", color: "#8b9bb4" }}>
                t={line.tick} · <span style={{ color: channelColor(line.channel) }}>
                  {line.channel}
                </span>
                {line.speaker !== "self" ? <> · from {line.speaker}</> : null}
              </div>
              <div style={{ "font-family": "ui-monospace, monospace" }}>
                {line.text}
              </div>
            </div>
          )}
        </For>
      </div>
    </Show>
  );
}

function MindTab(props: { mind: MindSnapshot }) {
  // The engine returns "" (empty string) for unset fields, not null —
  // so `value ?? fallback` evaluates to "" and renders nothing.
  // Bug fix: explicit empty-string check.
  const goal = () => props.mind.top_goal && props.mind.top_goal.length > 0
    ? props.mind.top_goal
    : "(no published goal — share_planner not wired)";
  const reflection = () => props.mind.last_reflection && props.mind.last_reflection.length > 0
    ? props.mind.last_reflection
    : "(no reflection yet — agent hasn't reflected, or share_reasoning=false)";
  return (
    <div data-testid="mind-tab" style={{ display: "grid", "row-gap": "10px" }}>
      <div>
        <div style={{ color: "#8b9bb4", "margin-bottom": "2px" }}>Top goal</div>
        <div style={{ "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
          {goal()}
        </div>
        <div style={{ color: "#5a6988", "font-size": "11px", "margin-top": "2px" }}>
          {props.mind.goal_stack_size} goal(s) in stack
        </div>
      </div>
      <div>
        <div style={{ color: "#8b9bb4", "margin-bottom": "2px" }}>Last reflection</div>
        <div style={{ "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
          {reflection()}
        </div>
      </div>
    </div>
  );
}

function TraceTab(props: { traces: TraceLine[] }) {
  return (
    <Show
      when={props.traces.length > 0}
      fallback={
        <div style={emptyStyle()}>
          No reasoning traces. Either this agent is heuristic (no LLM
          to reason) or the agent connected with share_reasoning=false.
        </div>
      }
    >
      <div data-testid="trace-tab" style={{ display: "grid", "row-gap": "8px" }}>
        <For each={props.traces.slice(-10)}>
          {(t) => (
            <div>
              <div style={{ "font-size": "11px", color: "#8b9bb4" }}>
                t={t.tick} · verb=<code>{t.verb}</code>
              </div>
              <div style={{ "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
                {t.reasoning}
              </div>
            </div>
          )}
        </For>
      </div>
    </Show>
  );
}

function channelColor(c: DialogueLine["channel"]) {
  switch (c) {
    case "whisper": return "#a78bfa";
    case "shout":   return "#f87171";
    case "sound":   return "#5a6988";
    default:        return "#ead4aa";
  }
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

function emptyStyle() {
  return {
    color: "#5a6988",
    "font-size": "12px",
    "text-align": "center" as const,
    padding: "20px 0",
  };
}
