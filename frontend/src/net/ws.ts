// WebSocket client for the viewer stream.
//
// Connects to /ws/viewer, decodes JSON messages, dispatches snapshot
// updates to a listener (the PixiApp handle). v0 is JSON-only; binary
// (FlatBuffers) lands in milestone 5+.
//
// Auto-reconnects with exponential backoff so a engine restart during
// dev doesn't force a page reload.

import type { EntityState } from "../render/Entity";

const ENGINE_WS_URL =
  import.meta.env.VITE_ENGINE_WS_URL ?? "ws://127.0.0.1:8080/ws/viewer";

export interface WorldSnapshot {
  tick: number;
  map_id: string;
  entities: EntityState[];
}

export interface AudibleEvent {
  event_id: string;
  kind: "speech" | "shout" | "whisper" | "sound";
  from_entity: string;
  from_pos: [number, number];
  text?: string;
  sound_kind?: string;
  tick: number;
}

export type WorldSnapshotListener = (snap: WorldSnapshot) => void;
export type AudibleListener = (events: AudibleEvent[]) => void;
export type ConnStateListener = (state: "connecting" | "open" | "closed") => void;

export interface ViewerClient {
  close(): void;
}

interface ServerEnvelope {
  type: "world_snapshot";
  snapshot?: WorldSnapshot;
  audible?: AudibleEvent[];
}

export function connectViewer(opts: {
  onSnapshot: WorldSnapshotListener;
  onAudible?: AudibleListener;
  onConnState?: ConnStateListener;
}): ViewerClient {
  const { onSnapshot, onAudible, onConnState } = opts;
  let ws: WebSocket | null = null;
  let reconnectAttempt = 0;
  let closed = false;
  let reconnectTimer: number | null = null;

  const setState = (s: "connecting" | "open" | "closed") => {
    onConnState?.(s);
  };

  const open = (): void => {
    if (closed) return;
    setState("connecting");
    ws = new WebSocket(ENGINE_WS_URL);
    ws.addEventListener("open", () => {
      reconnectAttempt = 0;
      setState("open");
    });
    ws.addEventListener("message", (ev) => {
      // v0 is JSON only — string frames. Binary lands later.
      if (typeof ev.data !== "string") return;
      try {
        const env = JSON.parse(ev.data) as ServerEnvelope;
        if (env.type === "world_snapshot" && env.snapshot) {
          onSnapshot(env.snapshot);
          if (env.audible && env.audible.length && onAudible) {
            onAudible(env.audible);
          }
        }
      } catch (e) {
        console.warn("ws parse:", e);
      }
    });
    ws.addEventListener("close", () => {
      setState("closed");
      if (closed) return;
      // Exp backoff capped at 5s. Page-load -> first connect is at 0
      // delay; subsequent after disconnects are throttled.
      const delay = Math.min(5000, 250 * 2 ** reconnectAttempt);
      reconnectAttempt += 1;
      reconnectTimer = window.setTimeout(open, delay);
    });
    ws.addEventListener("error", () => {
      // Errors trigger close; nothing else to do here.
    });
  };

  open();

  return {
    close() {
      closed = true;
      if (reconnectTimer !== null) window.clearTimeout(reconnectTimer);
      ws?.close();
    },
  };
}
