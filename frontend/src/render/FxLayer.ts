// FxLayer — one-shot combat/economy/social visual effects, RPG-readable.
//
// Driven by the audible events the engine already broadcasts (sword_clang
// on attack, death_scream on death, coin_clink on pay, item/contract
// sounds, hunger_pang, building enter/exit) plus floating damage numbers
// fed from EntityLayer's hp-drop detection.
//
// Design goals (game-feel pass):
//  - Damage numbers POP: spawn with an overshoot punch, arc upward, ease
//    out and fade. Bigger hits read bigger/bolder. Combat = red, hunger =
//    amber, so a viewer instantly tells "starving" from "being attacked".
//  - Anti-clutter: when many numbers fire on one tile at once we fan them
//    out horizontally + stagger vertically so they don't stack illegibly.
//  - Economy floaters use a consistent icon+color language; gold feels
//    rewarding (sparkle + warm ring).
//  - Rings taper and fade; sparks burst then dissolve. Everything cleans
//    itself up.
//
// All FX live in world (viewport) space; positions are tile coords
// converted to pixels. Graphics are pooled where cheap and always
// destroyed when their lifetime ends.
import { Container, Graphics, Text, TextStyle } from "pixi.js";
import { TILE_SIZE_PX } from "./tiles";
import type { AudibleEvent } from "../net/ws";

// ── palette ────────────────────────────────────────────────────────────
// Endesga-ish: dark outlines, warm yellows, reds, greens.
const C_DMG = 0xff4d4d;       // combat red
const C_DMG_BIG = 0xff2730;   // heavier combat red for big hits
const C_GOLD = 0xfee761;      // reward yellow
const C_ITEM = 0x8ad0ff;      // gift/trade blue
const C_DUST = 0xcdbb9a;      // building dust

type FxKind = "ripple" | "spark" | "float" | "slash";

interface ActiveFx {
  obj: Container | Graphics | Text;
  start: number;
  dur: number;
  kind: FxKind;
  // ripple
  cx?: number;
  cy?: number;
  maxR?: number;
  r0?: number;
  color?: number;
  width?: number;
  // float
  baseX?: number;
  baseY?: number;
  driftX?: number;          // horizontal fan to de-clutter stacked numbers
  rise?: number;            // total upward travel in px
  pop?: boolean;            // overshoot scale punch on spawn (damage numbers)
  // slash
  ang?: number;
}

function makeStyle(fill: string, size: number, stroke: string): TextStyle {
  return new TextStyle({
    fontFamily: "ui-monospace, monospace",
    fontSize: size,
    fontWeight: "bold",
    fill,
    stroke: { color: stroke, width: 3 },
    // soft drop shadow gives the floaters depth over busy tiles.
    dropShadow: { color: "#000000", alpha: 0.45, blur: 0, distance: 1, angle: Math.PI / 2 },
  });
}

const GOLD_STYLE = makeStyle("#fee761", 11, "#2a1f00");
const ITEM_STYLE = makeStyle("#8ad0ff", 10, "#06212e");
const DEAL_STYLE = makeStyle("#fee761", 10, "#2a1f00");
const ACCEPT_STYLE = makeStyle("#5ee89a", 10, "#06251a");
const REJECT_STYLE = makeStyle("#ff8a8a", 10, "#2a0000");
const HUNGER_STYLE = makeStyle("#feae34", 10, "#2a1500");

// Damage-number styles are picked by magnitude so a big hit literally
// reads bigger + bolder. Cached so we don't allocate a TextStyle per hit.
const DMG_STYLE_SM = makeStyle("#ff5d5d", 11, "#1a0000");
const DMG_STYLE_MD = makeStyle("#ff4242", 13, "#1a0000");
const DMG_STYLE_LG = makeStyle("#ff2730", 16, "#240000");

function tilePx(t: [number, number]): { x: number; y: number } {
  return { x: t[0] * TILE_SIZE_PX + TILE_SIZE_PX / 2,
           y: t[1] * TILE_SIZE_PX + TILE_SIZE_PX / 2 };
}

// ease-out cubic — fast start, gentle settle. Used for rises + ring growth.
function easeOut(t: number): number { return 1 - Math.pow(1 - t, 3); }
// ease-in cubic — for fades that hold then drop off.
function easeInQuad(t: number): number { return t * t; }

export class FxLayer {
  readonly container: Container;
  private active: ActiveFx[] = [];
  private seen = new Set<string>();
  // Per-tile recent-damage clustering: remember the last few damage
  // spawns at each tile-key within a short window so we can fan them
  // out instead of stacking them on top of each other.
  private dmgCluster = new Map<string, { n: number; until: number }>();

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
          // wide, slow double-ring — also signals who *heard* the death.
          this.ripple(p.x, p.y, 36 * TILE_SIZE_PX / 16, 0xff3b3b, 1500, 2.2);
          this.ripple(p.x, p.y, 22 * TILE_SIZE_PX / 16, 0xff7070, 1100, 1.4);
          this.burst(p.x, p.y, 0xffb0b0, 10, 9);
          break;
        case "sword_clang":
          // bright impact star + a quick directional slash glint + ring.
          this.impactStar(p.x, p.y, 0xfff2c0);
          this.slash(p.x, p.y);
          this.ripple(p.x, p.y, 1.7 * TILE_SIZE_PX, 0xffd166, 300, 2.0);
          break;
        case "coin_clink":
          this.float(ev.from_pos, "+ gold", GOLD_STYLE, { rise: TILE_SIZE_PX * 1.0 });
          this.coinSparkle(p.x, p.y);
          break;
        case "item_give":
          this.float(ev.from_pos, "gift", ITEM_STYLE);
          this.burst(p.x, p.y, C_ITEM, 6, 6);
          break;
        case "item_trade":
          this.float(ev.from_pos, "trade", ITEM_STYLE);
          this.burst(p.x, p.y, C_ITEM, 6, 6);
          break;
        case "contract_propose":
          this.float(ev.from_pos, "deal?", DEAL_STYLE);
          this.ripple(p.x, p.y, 1.4 * TILE_SIZE_PX, 0xc9a227, 380, 1.6);
          break;
        case "contract_accept":
          this.float(ev.from_pos, "deal +", ACCEPT_STYLE);
          this.ripple(p.x, p.y, 2.0 * TILE_SIZE_PX, 0x3ad17a, 520, 2.0);
          break;
        case "contract_complete":
          this.float(ev.from_pos, "honored", ACCEPT_STYLE);
          this.ripple(p.x, p.y, 1.6 * TILE_SIZE_PX, 0x5ee89a, 460, 1.6);
          break;
        case "contract_reject":
          this.float(ev.from_pos, "declined", REJECT_STYLE);
          break;
        case "hunger_pang":
          // amber pang — distinct from red combat damage.
          this.float(ev.from_pos, "hungry", HUNGER_STYLE);
          break;
        case "building_enter":
          this.dustPuff(p.x, p.y);
          break;
        case "building_exit":
          this.dustPuff(p.x, p.y);
          break;
      }
    }
    if (this.seen.size > 1024) {
      this.seen = new Set(Array.from(this.seen).slice(-512));
    }
  }

  /** Floating damage number (called from EntityLayer on hp drop). Red,
   *  size scales with magnitude, fanned out to avoid clutter. */
  damage(tile: [number, number], amount: number): void {
    const style = amount >= 12 ? DMG_STYLE_LG : amount >= 5 ? DMG_STYLE_MD : DMG_STYLE_SM;
    const color = amount >= 12 ? C_DMG_BIG : C_DMG;
    const fan = this.nextFan(tile);
    this.float(tile, `-${amount}`, style, {
      rise: TILE_SIZE_PX * (1.0 + Math.min(0.6, amount / 20)),
      pop: true,
      driftX: fan,
    });
    // a small red flash ring at the victim sells the impact.
    const p = tilePx(tile);
    this.ripple(p.x, p.y - TILE_SIZE_PX * 0.3, 0.9 * TILE_SIZE_PX, color, 240, 1.6);
  }

  /** Floating gold gain. */
  gold(tile: [number, number], amount: number): void {
    const fan = this.nextFan(tile);
    this.float(tile, `+${amount}g`, GOLD_STYLE, { driftX: fan, rise: TILE_SIZE_PX });
    const p = tilePx(tile);
    this.coinSparkle(p.x, p.y);
  }

  // Track how many numbers spawned on this tile recently so we can fan
  // them out left/right instead of stacking. Returns a horizontal offset.
  private nextFan(tile: [number, number]): number {
    const key = `${tile[0]},${tile[1]}`;
    const now = performance.now();
    const c = this.dmgCluster.get(key);
    let n = 0;
    if (c && c.until > now) { n = c.n + 1; }
    this.dmgCluster.set(key, { n, until: now + 600 });
    if (n === 0) return 0;
    // alternate sides, growing outward: +6, -6, +12, -12 …
    const step = Math.ceil(n / 2) * 6;
    return (n % 2 === 1 ? step : -step);
  }

  private float(
    tile: [number, number],
    text: string,
    style: TextStyle,
    opts?: { rise?: number; pop?: boolean; driftX?: number },
  ): void {
    const p = tilePx(tile);
    const t = new Text({ text, style });
    t.anchor.set(0.5, 1);
    t.resolution = 2;                       // crisp at world zoom
    const baseX = p.x + (opts?.driftX ?? 0);
    const baseY = p.y - TILE_SIZE_PX * 0.8;
    t.x = baseX;
    t.y = baseY;
    if (opts?.pop) t.scale.set(0.4);        // punches up to 1 on spawn
    this.container.addChild(t);
    this.active.push({
      obj: t, start: performance.now(), dur: 950, kind: "float",
      baseX, baseY,
      driftX: opts?.driftX ?? 0,
      rise: opts?.rise ?? TILE_SIZE_PX * 0.9,
      pop: opts?.pop ?? false,
    });
  }

  private ripple(
    cx: number, cy: number, maxR: number, color: number, dur: number, width = 2,
  ): void {
    const g = new Graphics();
    g.x = cx; g.y = cy;
    this.container.addChild(g);
    this.active.push({
      obj: g, start: performance.now(), dur, kind: "ripple",
      cx, cy, maxR, r0: 0.2 * TILE_SIZE_PX, color, width,
    });
  }

  // Sharp asymmetric star — sells a metal-on-metal hit better than even
  // radial lines. Long horizontal/vertical glints + short diagonals.
  private impactStar(cx: number, cy: number, color: number): void {
    const g = new Graphics();
    g.x = cx; g.y = cy;
    const long = 8, short = 3.5;
    g.moveTo(-long, 0).lineTo(long, 0);
    g.moveTo(0, -long).lineTo(0, long);
    g.stroke({ color, width: 2, alpha: 1 });
    const d = short;
    g.moveTo(-d, -d).lineTo(d, d);
    g.moveTo(-d, d).lineTo(d, -d);
    g.stroke({ color, width: 1, alpha: 0.8 });
    this.container.addChild(g);
    this.active.push({ obj: g, start: performance.now(), dur: 220, kind: "spark" });
  }

  // A short curved slash glint at a random angle near the impact point —
  // a quick suggestion of a weapon arc. (Engine doesn't give us the
  // victim direction in the audible event, so we keep this generic.)
  private slash(cx: number, cy: number): void {
    const g = new Graphics();
    g.x = cx; g.y = cy;
    g.rotation = (Math.random() - 0.5) * Math.PI;
    this.container.addChild(g);
    this.active.push({ obj: g, start: performance.now(), dur: 200, kind: "slash" });
  }

  // Generic particle burst — n short lines flung outward, fade + spread.
  private burst(cx: number, cy: number, color: number, n: number, len: number): void {
    const g = new Graphics();
    g.x = cx; g.y = cy;
    for (let i = 0; i < n; i++) {
      const a = (i / n) * Math.PI * 2 + Math.random() * 0.3;
      const r = len * (0.6 + Math.random() * 0.4);
      g.moveTo(0, 0).lineTo(Math.cos(a) * r, Math.sin(a) * r);
    }
    g.stroke({ color, width: 1.5, alpha: 1 });
    this.container.addChild(g);
    this.active.push({ obj: g, start: performance.now(), dur: 280, kind: "spark" });
  }

  // Coin sparkle — a warm four-point twinkle that reads as "money".
  private coinSparkle(cx: number, cy: number): void {
    const g = new Graphics();
    g.x = cx; g.y = cy - TILE_SIZE_PX * 0.2;
    const r = 6;
    g.moveTo(-r, 0).lineTo(r, 0);
    g.moveTo(0, -r).lineTo(0, r);
    g.stroke({ color: C_GOLD, width: 1.5, alpha: 1 });
    g.circle(0, 0, 1.6).fill({ color: 0xfffbe0, alpha: 1 });
    this.container.addChild(g);
    this.active.push({ obj: g, start: performance.now(), dur: 360, kind: "spark" });
  }

  // Soft dust puff at a doorway — a low expanding earth-tone ring.
  private dustPuff(cx: number, cy: number): void {
    this.ripple(cx, cy + TILE_SIZE_PX * 0.2, 1.1 * TILE_SIZE_PX, C_DUST, 420, 2.2);
    this.burst(cx, cy + TILE_SIZE_PX * 0.2, C_DUST, 5, 5);
  }

  tick(_dtMs: number): void {
    const now = performance.now();
    for (let i = this.active.length - 1; i >= 0; i--) {
      const fx = this.active[i];
      const t01 = Math.min(1, (now - fx.start) / fx.dur);
      if (fx.kind === "ripple") {
        const g = fx.obj as Graphics;
        const e = easeOut(t01);
        const r = (fx.r0 ?? 2) + ((fx.maxR ?? 16) - (fx.r0 ?? 2)) * e;
        const w = (fx.width ?? 2) * (1 - 0.7 * t01);     // taper as it grows
        g.clear();
        g.circle(0, 0, r)
          .stroke({ color: fx.color ?? 0xffffff, width: Math.max(0.4, w), alpha: (1 - t01) * 0.9 });
      } else if (fx.kind === "float") {
        const e = easeOut(t01);
        fx.obj.x = (fx.baseX ?? fx.obj.x);
        fx.obj.y = (fx.baseY ?? fx.obj.y) - e * (fx.rise ?? TILE_SIZE_PX * 0.9);
        // hold opacity early, fall off late so the number is readable.
        fx.obj.alpha = t01 < 0.55 ? 1 : 1 - easeInQuad((t01 - 0.55) / 0.45);
        if (fx.pop) {
          // overshoot punch: scale snaps 0.4 → 1.15 in ~140ms (ease-out),
          // then eases back to 1.0 over the next ~120ms — a satisfying
          // "pop" that draws the eye to a fresh hit.
          const ms = now - fx.start;
          let s: number;
          if (ms < 140) s = 0.4 + (1.15 - 0.4) * easeOut(ms / 140);
          else s = 1.15 - 0.15 * Math.min(1, (ms - 140) / 120);
          fx.obj.scale.set(s);
        }
      } else if (fx.kind === "slash") {
        // a thin crescent that sweeps in then thins out.
        const g = fx.obj as Graphics;
        const e = easeOut(t01);
        const len = 7 + 9 * e;
        g.clear();
        g.moveTo(-len, 2).quadraticCurveTo(0, -len * 0.5, len, 2)
          .stroke({ color: 0xffffff, width: 2 * (1 - t01), alpha: (1 - t01) * 0.9 });
        fx.obj.alpha = 1;                  // alpha handled per-stroke
      } else { // spark
        fx.obj.alpha = 1 - t01;
        fx.obj.scale.set(1 + 0.9 * easeOut(t01));
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
    this.dmgCluster.clear();
    this.container.destroy({ children: true });
  }
}
