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
import { Badge } from "./AgentHoverCard";

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
  // D14 — recommended slots. Each is the latest non-empty value
  // from the agent's MentalNote ring buffer.
  plan?: string | null;
  beliefs?: string | null;
  emotion?: string | null;
}

// D19 — per-pair social interaction counters surfaced from the
// engine's social ledger. Used by the Relationships tab.
export interface SocialCounts {
  trade: number;
  whisper: number;
  pay: number;
  attack: number;
  contract: number;
}

// Live vitals snapshot from the engine. Surfaced to the inspector's
// Inventory tab + the identity header (gold / hp pills).
export interface VitalsSnapshot {
  hp:       number;
  max_hp:   number;
  hunger:   number;
  gold:     number;
  inventory: Array<{ id: string; kind: string; count: number }>;
  equipped: Record<string, string>;
  inside_building?: string;
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
  // D19 — per-pair counters keyed by the *other* agent's entity_id.
  peers?: Record<string, SocialCounts>;
  vitals?: VitalsSnapshot;
  // D17 task 6.5 — surfaced via the /api/v1/agents lookup that
  // App.tsx runs alongside fetchMentalState. Drives the LLM/rule
  // badge in the inspector header. Undefined when the entity is
  // not a registered agent (e.g. background NPCs).
  is_llm?: boolean;
}

type Tab = "speech" | "mind" | "trace" | "relationships" | "inventory";

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
          width: "440px",
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
          <div style={{ display: "flex", "align-items": "center", gap: "8px", "min-width": "0", flex: "1" }}>
            <strong style={{
              color: "#fee761", "font-size": "14px",
              overflow: "hidden", "text-overflow": "ellipsis", "white-space": "nowrap",
            }}>
              {props.entity?.display_name ?? props.entity?.entity_id ?? "(unknown)"}
            </strong>
            <Show when={props.mentalState?.is_llm !== undefined}>
              <Badge kind={props.mentalState!.is_llm ? "llm" : "rule"} />
            </Show>
          </div>
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

        {/* Live vitals pills — surfaced above the tabs so they are
            always visible regardless of which tab is open. */}
        <Show when={props.mentalState?.vitals}>
          <VitalsPills v={props.mentalState!.vitals!} />
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
          <TabBtn current={tab()} value="inventory"
                  onClick={() => setTab("inventory")}
                  label="Inventory" enabled />
          <TabBtn current={tab()} value="mind" onClick={() => setTab("mind")}
                  label="Mind" enabled={mindVisible()}
                  disabledHint="share_planner=false" />
          <TabBtn current={tab()} value="relationships"
                  onClick={() => setTab("relationships")}
                  label="Relationships" enabled />
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
          <SpeechTab lines={props.mentalState?.dialogue ?? []} selfId={props.entity?.entity_id ?? ""} />
        </Show>
        <Show when={tab() === "mind" && mindVisible()}>
          <MindTab mind={props.mentalState!.mind} />
        </Show>
        <Show when={tab() === "trace" && traceVisible()}>
          <TraceTab traces={props.mentalState!.traces} />
        </Show>
        <Show when={tab() === "relationships"}>
          <RelationshipsTab
            peers={props.mentalState?.peers ?? {}}
            selfId={props.entity?.entity_id ?? ""}
          />
        </Show>
        <Show when={tab() === "inventory"}>
          <InventoryTab vitals={props.mentalState?.vitals} />
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
        padding: "4px 8px",
        cursor: p.enabled ? "pointer" : "not-allowed",
        "font-size": "12px",
        flex: "1",
        "min-width": "0",
        "white-space": "nowrap",
        "text-overflow": "ellipsis",
        overflow: "hidden",
      }}
    >
      {p.label}
    </button>
  );
}

function SpeechTab(props: { lines: DialogueLine[]; selfId: string }) {
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
                {line.speaker !== "self" && line.speaker !== props.selfId
                  ? <> · from {line.speaker}</> : null}
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
  // D14 slots: render only the ones that carry text.
  const plan      = () => props.mind.plan      && props.mind.plan.length      > 0 ? props.mind.plan      : null;
  const beliefs   = () => props.mind.beliefs   && props.mind.beliefs.length   > 0 ? props.mind.beliefs   : null;
  const emotion   = () => props.mind.emotion   && props.mind.emotion.length   > 0 ? props.mind.emotion   : null;
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
      <Show when={plan()}>
        <div>
          <div style={{ color: "#8b9bb4", "margin-bottom": "2px" }}>Plan</div>
          <div style={{ "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
            {plan()}
          </div>
        </div>
      </Show>
      <Show when={beliefs()}>
        <div>
          <div style={{ color: "#8b9bb4", "margin-bottom": "2px" }}>Beliefs</div>
          <div style={{ "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
            {beliefs()}
          </div>
        </div>
      </Show>
      <Show when={emotion()}>
        <div>
          <div style={{ color: "#8b9bb4", "margin-bottom": "2px" }}>Emotion</div>
          <div style={{ "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
            {emotion()}
          </div>
        </div>
      </Show>
      <div>
        <div style={{ color: "#8b9bb4", "margin-bottom": "2px" }}>Last reflection</div>
        <div style={{ "font-family": "ui-monospace, monospace", "font-size": "12px" }}>
          {reflection()}
        </div>
      </div>
    </div>
  );
}

// D19 — Relationships tab. Renders the per-pair interaction counters
// surfaced by /api/v1/agent/<id>/mental_state.peers. Sorts peers by
// total interaction count desc so the strongest ties show first.
function RelationshipsTab(props: {
  peers: Record<string, SocialCounts>;
  selfId: string;
}) {
  const rows = createMemo(() => {
    const entries = Object.entries(props.peers).map(([peer, c]) => {
      const total = c.trade + c.whisper + c.pay + c.attack + c.contract;
      return { peer, c, total };
    });
    entries.sort((a, b) => b.total - a.total);
    return entries;
  });
  return (
    <Show
      when={rows().length > 0}
      fallback={
        <div style={emptyStyle()}>
          No social interactions logged yet.
          <div style={{ "font-size": "10px", "margin-top": "6px",
                        color: "#5a6988" }}>
            Counters bump when this agent trades, whispers, pays, attacks,
            or proposes / completes a contract with another agent.
          </div>
        </div>
      }
    >
      <div data-testid="relationships-tab"
           style={{ display: "grid", "row-gap": "8px" }}>
        <div style={{ display: "grid",
                      "grid-template-columns": "1fr repeat(5, 36px)",
                      gap: "4px", "font-size": "10px",
                      color: "#8b9bb4",
                      "border-bottom": "1px solid #3a4466",
                      "padding-bottom": "4px" }}>
          <span>peer</span>
          <span style={{ "text-align": "right", color: relColor("trade") }}>trd</span>
          <span style={{ "text-align": "right", color: relColor("whisper") }}>wsp</span>
          <span style={{ "text-align": "right", color: relColor("pay") }}>pay</span>
          <span style={{ "text-align": "right", color: relColor("attack") }}>atk</span>
          <span style={{ "text-align": "right", color: relColor("contract") }}>ctr</span>
        </div>
        <For each={rows()}>
          {(row) => (
            <div style={{ display: "grid",
                          "grid-template-columns": "1fr repeat(5, 36px)",
                          gap: "4px",
                          "font-family": "ui-monospace, monospace",
                          "font-size": "12px",
                          "align-items": "center" }}>
              <span title={row.peer}
                    style={{ overflow: "hidden",
                             "text-overflow": "ellipsis",
                             "white-space": "nowrap" }}>
                {row.peer}
              </span>
              <Cell n={row.c.trade}    color={relColor("trade")} />
              <Cell n={row.c.whisper}  color={relColor("whisper")} />
              <Cell n={row.c.pay}      color={relColor("pay")} />
              <Cell n={row.c.attack}   color={relColor("attack")} />
              <Cell n={row.c.contract} color={relColor("contract")} />
            </div>
          )}
        </For>
      </div>
    </Show>
  );
}

// VitalsPills — compact identity-bar showing hp / hunger / gold and
// equipped slot. Always visible above the tab body. Flexible by
// design: the inventory list lives in the InventoryTab; this strip
// only carries the high-frequency scalars + a single equipped
// indicator. Adding a new stat = add one Pill row, no schema work.
function VitalsPills(p: { v: VitalsSnapshot }) {
  const hpPct = () =>
    p.v.max_hp > 0 ? Math.max(0, Math.min(100, Math.round((p.v.hp / p.v.max_hp) * 100))) : 0;
  const hpColor = () =>
    hpPct() >= 70 ? "#34d399" : hpPct() >= 30 ? "#fbbf24" : "#f87171";
  const hungerLabel = () => {
    const h = p.v.hunger || 0;
    if (h < 0.3) return "sated";
    if (h < 0.7) return "peckish";
    return "starving";
  };
  const hungerColor = () =>
    (p.v.hunger || 0) < 0.3 ? "#34d399"
      : (p.v.hunger || 0) < 0.7 ? "#fbbf24"
      : "#f87171";
  const equipped = () => Object.entries(p.v.equipped || {})
    .filter(([, v]) => v && v.length > 0);
  return (
    <div data-testid="vitals-pills"
         style={{ display: "flex", "flex-wrap": "wrap", gap: "6px",
                  "margin-bottom": "10px",
                  "padding-bottom": "8px",
                  "border-bottom": "1px solid #3a4466" }}>
      <Pill label="HP" value={`${p.v.hp}/${p.v.max_hp || "?"}`} color={hpColor()} />
      <Pill label="hunger" value={hungerLabel()} color={hungerColor()} />
      <Pill label="gold" value={`${p.v.gold} g`} color="#facc15" />
      <For each={equipped()}>
        {([slot, item]) => (
          <Pill label={slot} value={kindFromItemId(item)} color="#a78bfa" />
        )}
      </For>
    </div>
  );
}

function Pill(p: { label: string; value: string; color: string }) {
  return (
    <span style={{ display: "inline-flex", "align-items": "center",
                    gap: "5px", padding: "2px 8px",
                    background: "rgba(255,255,255,0.04)",
                    border: `1px solid ${p.color}`,
                    "border-radius": "10px",
                    "font-size": "11px",
                    "font-family": "ui-monospace, monospace" }}>
      <span style={{ color: "#8b9bb4" }}>{p.label}</span>
      <span style={{ color: p.color }}>{p.value}</span>
    </span>
  );
}

function kindFromItemId(id: string): string {
  // "item:sword_short#42" -> "sword_short". Identity for already-bare kinds.
  let k = id || "";
  if (k.startsWith("item:")) k = k.slice(5);
  const hash = k.indexOf("#");
  return hash >= 0 ? k.slice(0, hash) : k;
}

function InventoryTab(props: { vitals?: VitalsSnapshot }) {
  const items = createMemo(() => props.vitals?.inventory ?? []);
  const equipped = createMemo(() =>
    Object.entries(props.vitals?.equipped ?? {})
      .filter(([, v]) => v && v.length > 0));
  return (
    <div data-testid="inventory-tab"
         style={{ display: "grid", "row-gap": "10px" }}>
      <Show when={equipped().length > 0}>
        <div>
          <div style={{ color: "#8b9bb4", "font-size": "10px",
                        "margin-bottom": "4px" }}>Equipped</div>
          <For each={equipped()}>
            {([slot, item]) => (
              <div style={{ display: "flex", "justify-content": "space-between",
                            "font-family": "ui-monospace, monospace",
                            "font-size": "12px",
                            padding: "3px 0",
                            "border-bottom": "1px dashed #3a4466" }}>
                <span style={{ color: "#a78bfa" }}>{slot}</span>
                <span>{kindFromItemId(item)}</span>
              </div>
            )}
          </For>
        </div>
      </Show>
      <div>
        <div style={{ color: "#8b9bb4", "font-size": "10px",
                      "margin-bottom": "4px" }}>
          Carried — {items().length} item kind{items().length === 1 ? "" : "s"}
          <span style={{ color: "#5a6988" }}>
            {" "}(of 10 slots)
          </span>
        </div>
        <Show when={items().length > 0}
              fallback={
                <div style={emptyStyle()}>
                  Inventory is empty.
                  <div style={{ "font-size": "10px",
                                "margin-top": "6px",
                                color: "#5a6988" }}>
                    Coins + gems auto-convert to gold on pickup —
                    they never show up here.
                  </div>
                </div>
              }>
          <div style={{ display: "grid", "row-gap": "4px" }}>
            <For each={items()}>
              {(it) => (
                <div style={{ display: "flex",
                              "justify-content": "space-between",
                              "align-items": "center",
                              "font-family": "ui-monospace, monospace",
                              "font-size": "12px",
                              padding: "2px 0" }}>
                  <span>{it.kind}</span>
                  <span style={{ color: "#feae34" }}>×{it.count}</span>
                </div>
              )}
            </For>
          </div>
        </Show>
      </div>
    </div>
  );
}

function Cell(p: { n: number; color: string }) {
  return (
    <span style={{ "text-align": "right",
                   color: p.n > 0 ? p.color : "#3a4466" }}>
      {p.n}
    </span>
  );
}

function relColor(kind: keyof SocialCounts): string {
  switch (kind) {
    case "trade":    return "#facc15";  // gold
    case "whisper":  return "#a78bfa";  // purple
    case "pay":      return "#34d399";  // green
    case "attack":   return "#f87171";  // red
    case "contract": return "#60a5fa";  // blue
  }
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
