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

/** Fetch the world tilemap JSON. The engine serves it from /worlds/<name>.json
 *  alongside the WS endpoint, so we get correct CORS + the same origin
 *  for both. Falls back to a same-origin path if VITE_ENGINE_URL is empty
 *  (useful for static-only dev with no engine running). */
export async function fetchWorldMap(name = "dev_test"): Promise<unknown> {
  // Try engine first; fall back to Vite static.
  for (const base of [ENGINE_URL, ""]) {
    try {
      const r = await fetch(`${base}/worlds/${name}.json`);
      if (r.ok) return r.json();
    } catch {
      // try next base
    }
  }
  throw new Error(`world map not found at ${ENGINE_URL}/worlds/${name}.json or static`);
}
