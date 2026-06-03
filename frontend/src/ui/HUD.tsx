// HUD overlay — top-right clock, top-left mini-stats, bottom-left
// minimap stub. Lives outside the PixiJS viewport (Solid DOM).
//
// Inputs:
//   - tick + day_phase + weather from the world clock
//   - selected entity HP/gold/inventory from the inspector signal
//   - world dims for the minimap

import { Show } from "solid-js";
import type { EntityState } from "../render/Entity";

export interface HUDProps {
  tick: number;
  dayPhase: string;
  weather: string;
  worldDims: [number, number];
  selected: EntityState | null;
}

export function HUD(props: HUDProps) {
  return (
    <>
      <div class="hud-topright">
        <div class="clock">tick {props.tick}</div>
        <div class="phase">{props.dayPhase}</div>
        <Show when={props.weather !== "clear"}>
          <div class="weather">{props.weather}</div>
        </Show>
      </div>
      <Show when={props.selected}>
        <div class="hud-topleft">
          <h3>{props.selected!.display_name ?? props.selected!.entity_id}</h3>
          <div class="archetype">{props.selected!.archetype}</div>
          <StatLine label="HP" value={extra(props.selected, "hp")} maxValue={extra(props.selected, "max_hp")} color="#d44" />
          <StatLine label="Gold" value={extra(props.selected, "gold")} color="#dc4" />
          <Show when={(extra(props.selected, "inventory") as string[] | undefined)?.length}>
            <div class="inventory">
              <div class="title">Inventory</div>
              <ul>
                {(extra(props.selected, "inventory") as string[]).map((id) => <li>{id}</li>)}
              </ul>
            </div>
          </Show>
        </div>
      </Show>
    </>
  );
}

function StatLine(props: { label: string; value: unknown; maxValue?: unknown; color: string }) {
  const n = Number(props.value ?? 0);
  const m = Number(props.maxValue ?? n);
  const pct = m > 0 ? Math.max(0, Math.min(100, (n / m) * 100)) : 0;
  return (
    <div class="stat">
      <span class="label">{props.label}</span>
      <span class="num">{n}{props.maxValue !== undefined ? `/${m}` : ""}</span>
      <Show when={props.maxValue !== undefined}>
        <div class="bar">
          <div class="fill" style={{ width: `${pct}%`, background: props.color }} />
        </div>
      </Show>
    </div>
  );
}

function extra(e: EntityState | null, key: string): unknown {
  if (!e) return undefined;
  const x = (e as unknown as { extras?: Record<string, unknown> }).extras;
  if (!x) return undefined;
  return x[key];
}
