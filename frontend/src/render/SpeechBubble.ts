// SpeechBubble — floating Pokemon-style speech bubbles above entities
// that speak/shout. Subscribes to AudibleEvent broadcasts on the
// viewer WS; renders bubbles in the fxAbove layer with a 2.5 s
// fade-in/hold/fade-out lifecycle. Stacks vertically when an entity
// chains messages.

import { Container, Graphics, Text } from "pixi.js";
import { TILE_SIZE_PX } from "./tiles";
import type { EntityState } from "./Entity";
import type { AudibleEvent } from "../net/ws";

interface ActiveBubble {
  eventId: string;
  fromEntity: string;
  text: string;
  kind: "speech" | "shout" | "whisper" | "sound";
  // Wallclock ms at spawn.
  spawnedAt: number;
  // The Pixi container that holds bubble graphics + text.
  container: Container;
}

const LIFETIME_MS = 4000;        // bubble visible for 4 s (fade in 200 ms, hold 3600 ms, fade out 200 ms)
// Bubble copy limits. The full text always reaches the historian +
// inspector; the bubble is a glance-friendly heads-up. Past
// ~60 chars on a single line, agents' speech gets unreadable on the
// minimap-zoom level. Wrap inside the bubble for medium length, hard-
// truncate with "…" past MAX so the bubble never spans the screen.
const MAX_BUBBLE_CHARS = 60;
const BUBBLE_WRAP_PX  = 220;     // wrap width (text px BEFORE scale.set(0.4))
const FADE_IN_MS = 200;
const FADE_OUT_MS = 400;
const STACK_SPACING_PX = 14;     // vertical gap between stacked bubbles

export class SpeechBubbleLayer {
  readonly container: Container;
  private active: ActiveBubble[] = [];
  private seen = new Set<string>();   // dedupe across snapshots

  constructor() {
    this.container = new Container();
    this.container.label = "speech_bubbles";
    this.container.sortableChildren = true;
  }

  /** Ingest a batch of audible events from the viewer WS. New events
   *  spawn bubbles; old ones are ignored if already seen. */
  ingest(events: AudibleEvent[]): void {
    for (const ev of events) {
      if (this.seen.has(ev.event_id)) continue;
      if (ev.kind === "sound") continue;     // sounds don't get bubbles
      if (!ev.text) continue;
      this.seen.add(ev.event_id);
      this.spawn(ev);
    }
    // Periodically trim the seen set so it doesn't grow forever. The
    // engine's audible ring is 256, so keeping 1024 here covers ~16 s
    // of overlap easily.
    if (this.seen.size > 1024) {
      const arr = Array.from(this.seen);
      this.seen = new Set(arr.slice(arr.length - 512));
    }
  }

  /** Per-frame: position bubbles above their speakers + tick fades. */
  tick(entitiesById: Map<string, EntityState>): void {
    const now = performance.now();
    // Group active bubbles by speaker for vertical stacking.
    const bySpeaker = new Map<string, ActiveBubble[]>();
    for (const b of this.active) {
      const arr = bySpeaker.get(b.fromEntity) ?? [];
      arr.push(b);
      bySpeaker.set(b.fromEntity, arr);
    }

    const stillAlive: ActiveBubble[] = [];
    for (const [speakerId, bubbles] of bySpeaker) {
      const speaker = entitiesById.get(speakerId);
      if (!speaker) {
        // Speaker vanished — let bubbles continue but hide.
        for (const b of bubbles) {
          b.container.visible = false;
          if (now - b.spawnedAt < LIFETIME_MS) stillAlive.push(b);
          else b.container.destroy({ children: true });
        }
        continue;
      }
      // Sort by spawnedAt so the newest is on top.
      bubbles.sort((a, b) => a.spawnedAt - b.spawnedAt);

      const baseX = speaker.pos[0] * TILE_SIZE_PX + TILE_SIZE_PX / 2;
      // Anchor above the character head (~24 px tall char), with stack
      // index pushing older messages upward.
      const baseY = speaker.pos[1] * TILE_SIZE_PX - 12;

      bubbles.forEach((b, i) => {
        const age = now - b.spawnedAt;
        if (age > LIFETIME_MS) {
          b.container.destroy({ children: true });
          return;
        }
        // Alpha curve: fade-in → hold → fade-out.
        let alpha = 1;
        if (age < FADE_IN_MS) {
          alpha = age / FADE_IN_MS;
        } else if (age > LIFETIME_MS - FADE_OUT_MS) {
          alpha = (LIFETIME_MS - age) / FADE_OUT_MS;
        }
        b.container.alpha = alpha;
        b.container.visible = true;
        b.container.x = baseX;
        const stackIdx = bubbles.length - 1 - i;   // newer = lower (closer to head)
        b.container.y = baseY - stackIdx * STACK_SPACING_PX;
        b.container.zIndex = speaker.pos[1] * 1000 + 500 + i;
        stillAlive.push(b);
      });
    }
    this.active = stillAlive;
  }

  destroy(): void {
    for (const b of this.active) b.container.destroy({ children: true });
    this.active = [];
    this.container.destroy({ children: true });
  }

  private spawn(ev: AudibleEvent): void {
    // Truncate at MAX_BUBBLE_CHARS — anything longer gets ellipsized.
    // The full text is still in the historian + inspector; the bubble
    // is a heads-up display, not the canonical record.
    const raw = ev.text ?? "";
    const text = raw.length > MAX_BUBBLE_CHARS
      ? raw.slice(0, MAX_BUBBLE_CHARS - 1).trimEnd() + "…"
      : raw;
    const bubble = this.makeBubble(text, ev.kind);
    this.container.addChild(bubble);
    this.active.push({
      eventId: ev.event_id,
      fromEntity: ev.from_entity,
      text,
      kind: ev.kind,
      spawnedAt: performance.now(),
      container: bubble,
    });
  }

  private makeBubble(text: string, kind: AudibleEvent["kind"]): Container {
    const c = new Container();
    // Render the text high-res then scale down — same trick as Entity
    // labels: avoids small-font blurriness.
    const label = new Text({
      text,
      style: {
        fontFamily: "ui-sans-serif, system-ui, sans-serif",
        fontSize: 16,
        fontWeight: kind === "shout" ? "700" : "500",
        fill: kind === "shout" ? 0xe43b44 : 0x181425,
        align: "center",
        // Multi-line wrap so the bubble grows DOWN, not just out.
        // BUBBLE_WRAP_PX is the unscaled px width; the bubble's
        // visual width after scale.set(0.4) is BUBBLE_WRAP_PX * 0.4.
        wordWrap: true,
        wordWrapWidth: BUBBLE_WRAP_PX,
        breakWords: true,
      },
      resolution: 3,
    });
    label.scale.set(0.4);
    label.anchor.set(0.5, 1);

    // Bubble background — rounded rect sized to text + small pad.
    const padX = 3;
    const padY = 2;
    const w = label.width + padX * 2;
    const h = label.height + padY * 2;
    const bg = new Graphics();
    if (kind === "shout") {
      // Shout: jagged starburst yellow background
      bg.poly([
        -w / 2 - 2, -h - padY,
        -w / 4, -h - padY - 3,
        0, -h - padY,
        w / 4, -h - padY - 3,
        w / 2 + 2, -h - padY,
        w / 2 + 3, -h / 2,
        w / 2 + 2, 0,
        w / 4, 1,
        0, 4,                          // small "tail" downward
        -w / 4, 1,
        -w / 2 - 2, 0,
        -w / 2 - 3, -h / 2,
      ])
        .fill({ color: 0xfee761 })
        .stroke({ color: 0x733e39, width: 1 });
    } else {
      // Speech: rounded white bubble with a small triangle pointing down.
      bg.roundRect(-w / 2, -h - padY, w, h, 3)
        .fill({ color: 0xfffaef })
        .stroke({ color: 0x3e2731, width: 1 });
      // Tail.
      bg.poly([-2, -padY, 2, -padY, 0, 3])
        .fill({ color: 0xfffaef })
        .stroke({ color: 0x3e2731, width: 1 });
    }
    c.addChild(bg);
    label.x = 0;
    label.y = -padY;
    c.addChild(label);
    return c;
  }
}
