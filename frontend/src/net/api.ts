// HTTP API client for the engine's cold-path endpoints.
//
// Hot-path (observations, world deltas) goes through a separate
// WebSocket client (src/net/ws.ts, landing in Milestone 3). This file
// is just for one-shot JSON calls: world info, registration, snapshots.

export interface WorldInfo {
  name: string;
  version: string;
  scenario: string;
  world: string;
  tick_rate: number;
  tick: number;
  uptime_s: number;
}

const ENGINE_URL = import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

export async function fetchWorldInfo(): Promise<WorldInfo> {
  const r = await fetch(`${ENGINE_URL}/api/v1/world/info`);
  if (!r.ok) throw new Error(`world info ${r.status}`);
  return (await r.json()) as WorldInfo;
}

/** Fetch the world tilemap JSON. In v0 the file is served by Vite as
 *  a static asset; once the engine owns world ingestion (Milestone 3
 *  onward) this moves to /api/v1/world/render or comes through the
 *  viewer WS as a chunked stream. */
export async function fetchWorldMap(name = "dev_test"): Promise<unknown> {
  const r = await fetch(`/worlds/${name}.json`);
  if (!r.ok) throw new Error(`world map ${r.status}`);
  return r.json();
}
