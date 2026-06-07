// FxLayer — one-shot combat/economy visual effects, RPG-readable.
//
// Chunk 1 (frontend-only, no engine change): driven by the audible
// events the engine already broadcasts (sword_clang on attack,
// death_scream on death, coin_clink on pay) plus damage numbers fed
// from EntityLayer's hp-drop detection. Expanding death-scream ripple
// (#238), attack impact spark, floating "-N" damage numbers, and a
// coin sparkle on payment.
//
// All FX live in world (viewport) space; positions are tile coords
// converted to pixels.
import { Container, Graphics, Text, TextStyle } from "pixi.js";
import { TILE_SIZE_PX } from "./tiles";
import type { AudibleEvent } from "../net/ws";

interface ActiveFx {
  obj: Container | Graphics | Text;
  start: number;
  dur: number;
  kind: "ripple" | "spark" | "float" | "coin";
  // ripple
  cx?: number;
  cy?: number;
  maxR?: number;
  color?: number;
  // float
  baseY?: number;
}

const DAMAGE_STYLE = new TextStyle({
  fontFamily: "ui-monospace, monospace",
  fontSize: 11,
  fontWeight: "bold",
  fill: "#ff5555",
  stroke: { color: "#1a0000", width: 3 },
});
const GOLD_STYLE = new TextStyle({
  fontFamily: "ui-monospace, monospace",
  fontSize: 10,
  fontWeight: "bold",
  fill: "#fee761",
  stroke: { color: "#2a1f00", width: 3 },
});
// Chunk 2 economy/contract floaters (definitions were lost when the
// session crashed mid-edit; the ingest() cases below reference them).
const ITEM_STYLE = new TextStyle({
  fontFamily: "ui-monospace, monospace",
  fontSize: 10,
  fontWeight: "bold",
  fill: "#8ad0ff",
  stroke: { color: "#06212e", width: 3 },
});
const DEAL_STYLE = new TextStyle({
  fontFamily: "ui-monospace, monospace",
  fontSize: 10,
  fontWeight: "bold",
  fill: "#e8c860",
  stroke: { color: "#2a1f00", width: 3 },
});
const ACCEPT_STYLE = new TextStyle({
  fontFamily: "ui-monospace, monospace",
  fontSize: 10,
  fontWeight: "bold",
  fill: "#5ee89a",
  stroke: { color: "#06251a", width: 3 },
});
const REJECT_STYLE = new TextStyle({
  fontFamily: "ui-monospace, monospace",
  fontSize: 10,
  fontWeight: "bold",
  fill: "#ff8a8a",
  stroke: { color: "#2a0000", width: 3 },
});

function tilePx(t: [number, number]): { x: number; y: number } {
  return { x: t[0] * TILE_SIZE_PX + TILE_SIZE_PX / 2,
           y: t[1] * TILE_SIZE_PX + TILE_SIZE_PX / 2 };
}

export class FxLayer {
  readonly container: Container;
  private active: ActiveFx[] = [];
  private seen = new Set<string>();

  constructor() {
    this.container = new Container();
    this.container.label = "fx_layer";
  }

  /** Route audible sound events to one-shot FX (dedup by event_id). */
  ingest(events: AudibleEvent[]): void {
    for (const ev of events) {
      if (ev.kind !== "sound") continue;
      if (this.seen.has(ev.event_id)) continue;
      this.seen.add(ev.event_id);
      const p = tilePx(ev.from_pos);
      switch (ev.sound_kind) {
        case "death_scream":
          // wide, slow ring — also signals who *heard* the death (#238).
          this.ripple(p.x, p.y, 36 * TILE_SIZE_PX / 16, 0xff3b3b, 1500);
          break;
        case "sword_clang":
          this.spark(p.x, p.y, 0xffe08a);
          this.ripple(p.x, p.y, 1.6 * TILE_SIZE_PX, 0xffd166, 320);
          break;
        case "coin_clink":
          this.float(ev.from_pos, "+gold", GOLD_STYLE);
          this.spark(p.x, p.y, 0xfee761);
          break;
        case "item_give":
          this.float(ev.from_pos, "gift", ITEM_STYLE);
          this.spark(p.x, p.y, 0x8ad0ff);
          break;
        case "item_trade":
          this.float(ev.from_pos, "trade", ITEM_STYLE);
          this.spark(p.x, p.y, 0x8ad0ff);
          break;
        case "contract_propose":
          this.float(ev.from_pos, "deal?", DEAL_STYLE);
          this.ripple(p.x, p.y, 1.4 * TILE_SIZE_PX, 0xc9a227, 360);
          break;
        case "contract_accept":
          this.float(ev.from_pos, "deal ✓", ACCEPT_STYLE);
          this.ripple(p.x, p.y, 2.0 * TILE_SIZE_PX, 0x3ad17a, 500);
          break;
        case "contract_complete":
          this.float(ev.from_pos, "honored ✓", ACCEPT_STYLE);
          break;
        case "contract_reject":
          this.float(ev.from_pos, "declined", REJECT_STYLE);
          break;
      }
    }
    if (this.seen.size > 1024) {
      this.seen = new Set(Array.from(this.seen).slice(-512));
    }
  }

  /** Floating damage number (called from EntityLayer on hp drop). */
  damage(tile: [number, number], amount: number): void {
    this.float(tile, `-${amount}`, DAMAGE_STYLE);
  }

  /** Floating gold gain. */
  gold(tile: [number, number], amount: number): void {
    this.float(tile, `+${amount}g`, GOLD_STYLE);
  }

  private float(tile: [number, number], text: string, style: TextStyle): void {
    const p = tilePx(tile);
    const t = new Text({ text, style });
    t.anchor.set(0.5, 1);
    t.x = p.x;
    t.y = p.y - TILE_SIZE_PX * 0.8;
    this.container.addChild(t);
    this.active.push({ obj: t, start: performance.now(), dur: 900,
                       kind: "float", baseY: t.y });
  }

  private ripple(cx: number, cy: number, maxR: number, color: number, dur: number): void {
    const g = new Graphics();
    g.x = cx; g.y = cy;
    this.container.addChild(g);
    this.active.push({ obj: g, start: performance.now(), dur, kind: "ripple",
                       cx, cy, maxR, color });
  }

  private spark(cx: number, cy: number, color: number): void {
    const g = new Graphics();
    g.x = cx; g.y = cy;
    // small burst of lines
    for (let i = 0; i < 6; i++) {
      const a = (i / 6) * Math.PI * 2;
      g.moveTo(0, 0).lineTo(Math.cos(a) * 5, Math.sin(a) * 5);
    }
    g.stroke({ color, width: 1.5, alpha: 1 });
    this.container.addChild(g);
    this.active.push({ obj: g, start: performance.now(), dur: 260, kind: "spark" });
  }

  tick(_dtMs: number): void {
    const now = performance.now();
    for (let i = this.active.length - 1; i >= 0; i--) {
      const fx = this.active[i];
      const t01 = Math.min(1, (now - fx.start) / fx.dur);
      if (fx.kind === "ripple") {
        const g = fx.obj as Graphics;
        g.clear();
        g.circle(0, 0, (fx.maxR ?? 16) * t01)
          .stroke({ color: fx.color ?? 0xffffff, width: 2, alpha: 1 - t01 });
      } else if (fx.kind === "float") {
        fx.obj.y = (fx.baseY ?? fx.obj.y) - t01 * TILE_SIZE_PX * 0.9;
        fx.obj.alpha = 1 - t01;
      } else { // spark / coin
        fx.obj.alpha = 1 - t01;
        fx.obj.scale.set(1 + t01);
      }
      if (t01 >= 1) {
        fx.obj.destroy();
        this.active.splice(i, 1);
      }
    }
  }

  destroy(): void {
    for (const fx of this.active) fx.obj.destroy();
    this.active = [];
    this.container.destroy({ children: true });
  }
}
