// Engine register flow. Calls /api/v1/agent/register; the response
// gives the user the credentials they paste into their agent backend
// (via env vars or the SDK constructor).

const ENGINE_URL = import.meta.env.VITE_ENGINE_URL ?? "http://127.0.0.1:8080";

export interface RegisterRequest {
  user_token: string;
  persona_blob: Record<string, unknown>;
  vision_mode?: "structured" | "image" | "both";
  cadence_ms?: number;
  bind_entity?: string;
}

export interface RegisterResponse {
  agent_id: string;
  agent_secret: string;
  ws_url: string;
  entity_id: string;
}

export async function registerAgent(req: RegisterRequest): Promise<RegisterResponse> {
  const r = await fetch(`${ENGINE_URL}/api/v1/agent/register`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(req),
  });
  if (!r.ok) throw new Error(`register failed: ${r.status} ${await r.text()}`);
  return await r.json();
}
