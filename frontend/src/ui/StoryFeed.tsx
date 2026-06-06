// StoryFeed — per-entity chronological narrative.
//
// When an entity is selected, poll /api/v1/world/history?entity=<id>
// and render a tight, human-readable list of what happened around
// them. When nothing is selected, show a global feed (last ~20 events)
// so the panel always has something to look at.
//
// We summarize the raw event payload into a short sentence per kind.
// New event kinds default to "Kind happened" — explicit summaries are
// added as we add systems.

import { createSignal, onCleanup, onMount, createMemo, For, Show } from "solid-js";

const ENGINE_URL = import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

interface HistoryEvent {
  tick: number;
  seq: number;
  kind: string;
  payload: Record<string, unknown>;
}

interface StoryFeedProps {
  entityId: string | null;
  /** When true, the editor panel is taking the right edge — shift
   *  the story feed left so they don't overlap. */
  shiftLeft?: boolean;
}

function shortId(id: unknown): string {
  if (typeof id !== "string") return "?";
  // Trim to the last meaningful segment for readability.
  return id.length > 18 ? id.slice(0, 16) + "…" : id;
}

function summarize(ev: HistoryEvent): string {
  const p = ev.payload;
  const get = (k: string): string => shortId(p[k]);
  switch (ev.kind) {
    case "ResourceHarvested":
      return `${get("Harvester")} harvested ${get("Kind")} from ${get("Target")}`;
    case "ResourceDepleted":
      return `${get("Target")} was depleted`;
    case "ItemPicked":
      return `${get("Picker")} picked up ${get("Item")}`;
    case "ItemDropped":
      return `${get("Dropper")} dropped ${get("Item")}`;
    case "ItemTransferred":
      return `${get("From")} gave ${get("Item")} to ${get("To")}`;
    case "EnteredBuilding":
      return `${get("Entity")} entered ${get("Building")}`;
    case "ExitedBuilding":
      return `${get("Entity")} left ${get("Building")}`;
    case "BuildingLocked":
      return `${get("Owner")} locked ${get("Building")}`;
    case "BuildingUnlocked":
      return `${get("Owner")} unlocked ${get("Building")}`;
    case "OwnershipChanged":
      return `${get("Building")} now owned by ${get("NewOwner")}`;
    case "GoldTransferred":
      return `${get("From")} paid ${String(p["Amount"] ?? "?")} gold to ${get("To")}`;
    case "DamageDealt":
      return `${get("Attacker")} hit ${get("Victim")} for ${String(p["Damage"] ?? "?")}`;
    case "EntityDied":
      return `${get("Victim")} died — killed by ${get("Killer")}`;
    case "TaskProposed":
      return `${get("Proposer")} → ${get("Target")}: ${String(p["Terms"] ?? "(no terms)")}`;
    case "TaskAccepted":
      return `${get("Target")} accepted task from ${get("Proposer")}`;
    case "TaskRejected":
      return `${get("Target")} rejected ${get("Proposer")}'s task`;
    case "TaskCompleted":
      return `${get("Proposer")} marked task with ${get("Target")} done`;
    case "ConstructionStarted":
      return `${get("Builder")} started ${String(p["BlueprintKind"] ?? "structure")} (${get("Blueprint")})`;
    case "ConstructionAdvanced":
      return `${get("Builder")} advanced ${get("Blueprint")} to ${String(p["NewProgress"] ?? "?")}%`;
    case "ConstructionCompleted":
      return `${get("Builder")} finished ${String(p["BlueprintKind"] ?? "structure")} → ${get("BuildingID")}`;
    case "Demolished":
      return `${get("By")} demolished ${get("Target")}`;
    default:
      return ev.kind;
  }
}

export function StoryFeed(props: StoryFeedProps) {
  const [events, setEvents] = createSignal<HistoryEvent[]>([]);
  const [err, setErr] = createSignal<string | null>(null);
  let pollHandle: number | undefined;
  let lastFetchKey = "";

  const url = createMemo(() => {
    const params = new URLSearchParams({ limit: "40" });
    if (props.entityId) params.set("entity", props.entityId);
    return `${ENGINE_URL}/api/v1/world/history?${params.toString()}`;
  });

  const fetchOnce = async () => {
    const u = url();
    // Skip duplicate polls fired while a prior one was inflight.
    if (u === lastFetchKey) return;
    lastFetchKey = u;
    try {
      const r = await fetch(u);
      if (!r.ok) throw new Error(`history ${r.status}`);
      const body = await r.json();
      const list = (body?.events ?? []) as HistoryEvent[];
      // Show newest first.
      setEvents(list.slice().reverse());
      setErr(null);
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      lastFetchKey = "";
    }
  };

  onMount(() => {
    void fetchOnce();
    pollHandle = window.setInterval(() => void fetchOnce(), 2000);
  });
  onCleanup(() => {
    if (pollHandle) window.clearInterval(pollHandle);
  });

  return (
    <div
      style={{
        position: "absolute",
        bottom: "16px",
        // Editor panel takes the right 260px when open; shift story
        // 276px left so they don't overlap. Controlled by parent via
        // shiftLeft prop.
        right: props.shiftLeft ? "276px" : "16px",
        width: "300px",
        "max-height": "260px",
        background: "rgba(24, 20, 37, 0.92)",
        border: "1px solid #3a4466",
        "border-radius": "4px",
        padding: "8px",
        color: "#ead4aa",
        "font-size": "11px",
        "z-index": "10",
        display: "flex",
        "flex-direction": "column",
      }}
    >
      <div
        style={{
          display: "flex",
          "justify-content": "space-between",
          "align-items": "baseline",
          "margin-bottom": "6px",
        }}
      >
        <strong style={{ color: "#fee761" }}>
          {props.entityId ? `story · ${shortId(props.entityId)}` : "world story"}
        </strong>
        <span style={{ color: "#8b9bb4", "font-size": "10px" }}>
          {events().length} event{events().length === 1 ? "" : "s"}
        </span>
      </div>
      <Show when={err()}>
        <div style={{ color: "#e43b44", "margin-bottom": "4px" }}>{err()}</div>
      </Show>
      <div
        style={{
          flex: 1,
          "overflow-y": "auto",
          "padding-right": "4px",
        }}
      >
        <Show
          when={events().length > 0}
          fallback={
            <div style={{ color: "#8b9bb4", "font-style": "italic" }}>
              {props.entityId
                ? "no events for this entity yet"
                : "waiting for world events…"}
            </div>
          }
        >
          <For each={events()}>
            {(ev) => (
              <div
                style={{
                  "border-left": "2px solid #3a4466",
                  "padding-left": "6px",
                  "margin-bottom": "4px",
                  "line-height": "1.35",
                }}
              >
                <span style={{ color: "#8b9bb4", "margin-right": "6px" }}>
                  t{ev.tick}
                </span>
                <span>{summarize(ev)}</span>
              </div>
            )}
          </For>
        </Show>
      </div>
    </div>
  );
}
