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
  world_dims: [number, number];
  tick_rate: number;
  tick: number;
  uptime_s: number;
}

export const ENGINE_URL = import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

export async function fetchWorldInfo(): Promise<WorldInfo> {
  const r = await fetch(`${ENGINE_URL}/api/v1/world/info`);
  if (!r.ok) throw new Error(`world info ${r.status}`);
  return (await r.json()) as WorldInfo;
}

export interface MentalStateResponse {
  entity_id: string;
  capture_reasoning_enabled: boolean;
  dialogue: Array<{ tick: number; speaker: string; channel: string; text: string }>;
  mind: {
    share_planner: boolean;
    top_goal: string;
    last_reflection: string;
    goal_stack_size: number;
  };
  traces: Array<{ tick: number; action_id: string; verb: string; reasoning: string }>;
}

export async function fetchMentalState(entityID: string): Promise<MentalStateResponse> {
  const r = await fetch(`${ENGINE_URL}/api/v1/agent/${encodeURIComponent(entityID)}/mental_state`);
  if (!r.ok) throw new Error(`mental_state ${r.status}`);
  return (await r.json()) as MentalStateResponse;
}

/** Fetch the world tilemap JSON. The engine serves it from /worlds/<name>.json
 *  alongside the WS endpoint, so we get correct CORS + the same origin
 *  for both. Falls back to a same-origin path if VITE_ENGINE_URL is empty
 *  (useful for static-only dev with no engine running). */
export interface AffordanceManifest {
  world: string;
  scenario: string;
  schema_version: number;
  systems: SystemDeclaration[];
}

export interface SystemDeclaration {
  name: string;
  description: string;
  verbs: VerbDeclaration[];
  state_fields: StateFieldDecl[];
  sounds_emitted: SoundDecl[];
  archetypes: ArchetypeDecl[];
}

export interface VerbDeclaration {
  verb: string;
  description: string;
  params_schema: unknown;
  preconditions: string[];
  rejection_reasons: string[];
  emits_events?: string[];
  examples: { params: unknown; result: string }[];
}

export interface StateFieldDecl {
  key: string;
  type: string;
  owner: string;
  public_at_any_distance: boolean;
  public_within_distance?: number;
  meaning: string;
}

export interface SoundDecl {
  kind: string;
  description: string;
  emitted_by: string;
}

export interface ArchetypeDecl {
  archetype: string;
  description: string;
  default_extras?: unknown;
  default_verbs_used?: string[];
}

export async function fetchAffordances(): Promise<AffordanceManifest> {
  const r = await fetch(`${ENGINE_URL}/api/v1/world/affordances`);
  if (!r.ok) throw new Error(`affordances ${r.status}`);
  return (await r.json()) as AffordanceManifest;
}

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
