// Leaderboards — top N agents by various metrics. Fetches from
// `/api/v1/leaderboards`. Refreshed every N seconds.

import { For, Show, createResource } from "solid-js";

const ENGINE_URL = import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

type Board = "richest" | "kills" | "buildings" | "contracts";

interface Row {
  entity_id: string;
  label: string;
  metric: number;
}

async function fetchBoard(b: Board): Promise<Row[]> {
  const r = await fetch(`${ENGINE_URL}/api/v1/leaderboards?board=${b}`);
  if (!r.ok) return [];
  return await r.json();
}

export function Leaderboards() {
  const [richest] = createResource(() => "richest" as Board, fetchBoard);
  const [kills] = createResource(() => "kills" as Board, fetchBoard);
  const [buildings] = createResource(() => "buildings" as Board, fetchBoard);
  const [contracts] = createResource(() => "contracts" as Board, fetchBoard);
  return (
    <div class="leaderboards">
      <Section title="Richest" rows={richest()} unit="g" />
      <Section title="Most kills" rows={kills()} unit="" />
      <Section title="Most buildings owned" rows={buildings()} unit="" />
      <Section title="Most contracts completed" rows={contracts()} unit="" />
    </div>
  );
}

function Section(props: { title: string; rows: Row[] | undefined; unit: string }) {
  return (
    <div class="board">
      <h4>{props.title}</h4>
      <Show when={props.rows && props.rows.length > 0} fallback={<p class="empty">no data</p>}>
        <ol>
          <For each={props.rows!.slice(0, 10)}>
            {(r) => (
              <li>
                <span class="who">{r.label}</span>
                <span class="num">{r.metric}{props.unit}</span>
              </li>
            )}
          </For>
        </ol>
      </Show>
    </div>
  );
}
