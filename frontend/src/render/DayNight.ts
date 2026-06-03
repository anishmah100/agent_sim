// Day / night cycle.
//
// Drives a ColorMatrixFilter on the viewport so the world warms at
// dawn, brightens for midday, cools toward dusk, then dims into night.
// The cycle length is configurable; one in-game day defaults to four
// real minutes so the visual difference is obvious during a demo.
//
// Time of day is computed locally from a wall clock — the engine
// doesn't (yet) authoritatively track world time. When we add a
// `tick_of_day` field to the wire payload, we'll just consume that.

import { Container, ColorMatrixFilter } from "pixi.js";

const DAY_LENGTH_MS = 900_000;   // 15 real minutes per in-game day (was 4 min — too fast)

interface Tint {
  r: number;     // multiplier
  g: number;
  b: number;
  bright: number;  // additive offset
}

// Hand-picked keyframes around the cycle. t ranges 0..1.
//   0.00 dawn         soft warm
//   0.10 sunrise      warm gold
//   0.25 morning      bright neutral
//   0.50 noon         clean white
//   0.70 afternoon    slight gold
//   0.80 dusk         orange
//   0.90 twilight     blue-purple
//   1.00 night        deep blue dim
// Night kept moonlit-blue but NEVER pitch-black; the goal is "atmospheric
// shift", not "lights off". Deep night was -0.30 brightness before, which
// crushed visibility. Cap at -0.14.
const KEYFRAMES: Array<{ t: number; tint: Tint }> = [
  { t: 0.00, tint: { r: 0.78, g: 0.78, b: 0.95, bright: -0.10 } },  // pre-dawn
  { t: 0.08, tint: { r: 0.95, g: 0.82, b: 0.70, bright: -0.02 } },  // dawn
  { t: 0.20, tint: { r: 1.05, g: 1.02, b: 0.95, bright:  0.02 } },  // morning
  { t: 0.50, tint: { r: 1.00, g: 1.00, b: 1.00, bright:  0.00 } },  // noon
  { t: 0.70, tint: { r: 1.05, g: 0.95, b: 0.82, bright:  0.00 } },  // afternoon
  { t: 0.82, tint: { r: 1.08, g: 0.78, b: 0.62, bright: -0.03 } },  // sunset
  { t: 0.90, tint: { r: 0.82, g: 0.75, b: 0.95, bright: -0.10 } },  // twilight
  { t: 1.00, tint: { r: 0.78, g: 0.80, b: 1.00, bright: -0.14 } },  // moonlit night
];

function lerp(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

function tintAt(t: number): Tint {
  t = ((t % 1) + 1) % 1;
  for (let i = 0; i < KEYFRAMES.length - 1; i++) {
    const a = KEYFRAMES[i];
    const b = KEYFRAMES[i + 1];
    if (t >= a.t && t <= b.t) {
      const k = (t - a.t) / (b.t - a.t);
      return {
        r: lerp(a.tint.r, b.tint.r, k),
        g: lerp(a.tint.g, b.tint.g, k),
        b: lerp(a.tint.b, b.tint.b, k),
        bright: lerp(a.tint.bright, b.tint.bright, k),
      };
    }
  }
  return KEYFRAMES[KEYFRAMES.length - 1].tint;
}

export class DayNight {
  private filter = new ColorMatrixFilter();
  private startMs = performance.now();

  constructor(private target: Container) {
    this.target.filters = [this.filter];
  }

  /** Returns time-of-day 0..1 — 0 = dawn, 0.5 = noon, 1 = next dawn. */
  timeOfDay(): number {
    return ((performance.now() - this.startMs) / DAY_LENGTH_MS) % 1;
  }

  /** Skip to a normalized time. Useful for screenshots. */
  setTime(t: number): void {
    this.startMs = performance.now() - t * DAY_LENGTH_MS;
  }

  tick(): void {
    const tod = this.timeOfDay();
    const tint = tintAt(tod);
    // ColorMatrix multiplies each channel + adds an offset (in 0..1).
    // Reset + apply per-frame so we don't compound.
    this.filter.reset();
    const m = this.filter.matrix;
    m[0]  = tint.r;
    m[6]  = tint.g;
    m[12] = tint.b;
    m[4]  = tint.bright;
    m[9]  = tint.bright;
    m[14] = tint.bright;
    this.filter.matrix = m;
  }
}

