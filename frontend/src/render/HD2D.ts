// HD-2D filter stack — Octopath-tier visual polish layered over the
// pixel-art world. Applied to the viewport so the interior overlay
// (which sits on the stage) stays neutral.
//
// Composition (each applied after the previous):
//   1. Saturation boost — small Endesga punch.
//   2. Bloom — soft glow on bright pixels (forge windows, lanterns).
//
// Tilt-shift was tried and removed: pixi-filters' TiltShiftFilter's
// default focus band lands at the top of the canvas rather than
// centered, and reconfiguring it on viewport resize is brittle.
// Chromatic aberration was also dropped — even at 1 px it visibly
// smears small pixel art at low zoom.
//
// Day/night ColorMatrixFilter is separate and runs in DayNight.ts
// (drives ambient cycle), pre-composed below this stack so the HD-2D
// effects modulate the day-tinted world rather than overwriting it.

import { ColorMatrixFilter, Container } from "pixi.js";
import { AdvancedBloomFilter } from "pixi-filters";

const PRESET = {
  // Bloom: threshold high enough that midtone grass+stone don't bloom;
  // forge windows / lit lanterns are intentionally pushed above this.
  bloomThreshold: 0.65,
  bloomScale: 0.4,
  bloomBlur: 5,
  // Saturation boost.
  saturation: 0.08,
};

export class HD2DStack {
  private bloom: AdvancedBloomFilter;
  private saturation: ColorMatrixFilter;

  constructor(target: Container) {
    this.bloom = new AdvancedBloomFilter({
      threshold: PRESET.bloomThreshold,
      bloomScale: PRESET.bloomScale,
      blur: PRESET.bloomBlur,
      quality: 4,
    });
    this.saturation = new ColorMatrixFilter();
    this.saturation.saturate(PRESET.saturation, false);
    // Chain on the target. Day/night ColorMatrix (from DayNight.ts) is
    // already on target.filters[0]; we append the HD-2D effects after.
    const existing = (target.filters as any) ?? [];
    target.filters = [...existing, this.saturation, this.bloom];
  }

  /** Toggle the entire HD-2D stack — useful for measuring its frame
   *  cost or for users with low-end GPUs. */
  setEnabled(enabled: boolean): void {
    for (const f of [this.saturation, this.bloom]) {
      (f as any).enabled = enabled;
    }
  }
}
