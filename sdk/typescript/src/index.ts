// agent_sim SDK — TypeScript client.

import WebSocket from "isomorphic-ws";
import { Action, Observation, VisionMode } from "./models";

export * from "./models";

export interface AgentCredentials {
  agent_id: string;
  agent_secret: string;
  ws_url: string;
}

export interface RegisterOpts {
  user_token: string;
  persona: Record<string, unknown>;
  vision_mode?: VisionMode;
  cadence_ms?: number;
}

export async function registerAgent(
  server: string,
  opts: RegisterOpts,
): Promise<AgentCredentials> {
  const r = await fetch(`${server}/api/v1/agent/register`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      user_token: opts.user_token,
      persona_blob: opts.persona,
      vision_mode: opts.vision_mode ?? "structured",
      cadence_ms: opts.cadence_ms ?? 1000,
    }),
  });
  if (!r.ok) throw new Error(`register failed: ${r.status} ${await r.text()}`);
  return await r.json() as AgentCredentials;
}

export class Agent {
  private ws: WebSocket | null = null;
  private obsListeners: Array<(o: Observation) => void> = [];

  constructor(private creds: AgentCredentials) {}

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(this.creds.ws_url);
      this.ws = ws;
      ws.onopen = () => {
        ws.send(JSON.stringify({ auth: this.creds.agent_secret }));
        resolve();
      };
      ws.onerror = (e: unknown) => reject(e);
      ws.onmessage = (ev: { data: unknown }) => this.handleMessage(ev.data);
      ws.onclose = () => { this.ws = null; };
    });
  }

  close(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  onObservation(handler: (o: Observation) => void): () => void {
    this.obsListeners.push(handler);
    return () => {
      const i = this.obsListeners.indexOf(handler);
      if (i >= 0) this.obsListeners.splice(i, 1);
    };
  }

  async act(action: Action): Promise<void> {
    if (!this.ws) throw new Error("not connected");
    const payload = {
      type: "action",
      action_id: cryptoRandomId(),
      ...action,
    };
    this.ws.send(JSON.stringify(payload));
  }

  async setCadence(intervalMs: number): Promise<void> {
    if (!this.ws) throw new Error("not connected");
    this.ws.send(JSON.stringify({ type: "set_cadence", interval_ms: intervalMs }));
  }

  private handleMessage(raw: unknown): void {
    let text: string;
    if (typeof raw === "string") text = raw;
    else if (raw instanceof Uint8Array) text = new TextDecoder().decode(raw);
    else if ((raw as { toString(): string }).toString) text = (raw as { toString(): string }).toString();
    else return;
    let msg: { type?: string; view_image?: { data?: string } };
    try { msg = JSON.parse(text); } catch { return; }
    if (msg.type !== "observation") return;
    if (msg.view_image?.data && typeof msg.view_image.data === "string") {
      // base64 → Uint8Array for downstream consumers
      const bin = atob(msg.view_image.data);
      const bytes = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
      (msg.view_image as unknown as { data: Uint8Array }).data = bytes;
    }
    const result = Observation.safeParse(msg);
    if (!result.success) return;
    for (const h of this.obsListeners) h(result.data);
  }
}

function cryptoRandomId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2);
}
