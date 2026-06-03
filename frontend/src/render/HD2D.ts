// HD-2D filter stack — Octopath-tier visual polish layered over the
// pixel-art world. Applied to the viewport so the interior overlay
// (which sits on the stage) stays neutral.
//
// Composition order (each applied after the previous):
//   1. Bloom — soft glow on bright pixels (forge windows, lanterns)
//   2. Color matrix — subtle saturation boost + warm tint
//   3. Vignette — radial darkening from corners (~12%)
//   4. Tilt-shift — vertical Gaussian blur masked above + below the
//      central band, reads as "miniature" depth-of-field
//
// Day/night ColorMatrixFilter is separate and runs in DayNight.ts
// (drives ambient cycle), pre-composed below this stack so the HD-2D
// effects modulate the day-tinted world rather than overwriting it.

import { ColorMatrixFilter, Container } from "pixi.js";
import {
  AdvancedBloomFilter,
  TiltShiftFilter,
  RGBSplitFilter,
} from "pixi-filters";

const PRESET = {
  // Bloom: threshold high enough that midtone grass+stone don't bloom;
  // forge windows / lit lanterns are intentionally pushed above this.
  bloomThreshold: 0.65,
  bloomScale: 0.5,
  bloomBlur: 6,
  // Saturation boost.
  saturation: 0.10,
  // Tilt-shift band: middle 40% of screen sharp, top/bottom 30% blurred.
  tiltShiftBlur: 12,
  tiltShiftGradient: 220,
};

export class HD2DStack {
  private bloom: AdvancedBloomFilter;
  private tilt: TiltShiftFilter;
  private saturation: ColorMatrixFilter;
  private chroma: RGBSplitFilter;

  constructor(target: Container) {
    this.bloom = new AdvancedBloomFilter({
      threshold: PRESET.bloomThreshold,
      bloomScale: PRESET.bloomScale,
      blur: PRESET.bloomBlur,
      quality: 4,
    });
    this.saturation = new ColorMatrixFilter();
    this.saturation.saturate(PRESET.saturation, false);
    // Very subtle chromatic aberration — pushes the world out of
    // "flat tile" territory into "glassy lens". Single-pixel split.
    this.chroma = new RGBSplitFilter({
      red: { x: -1, y: 0 },
      blue: { x: 1, y: 0 },
    });
    this.tilt = new TiltShiftFilter({
      blur: PRESET.tiltShiftBlur,
      gradientBlur: PRESET.tiltShiftGradient,
    });
    // Chain on the target. Day/night ColorMatrix (from DayNight.ts) is
    // already on target.filters[0]; we append the HD-2D effects after.
    const existing = (target.filters as any) ?? [];
    target.filters = [
      ...existing,
      this.saturation,
      this.bloom,
      this.tilt,
      this.chroma,
    ];
  }

  /** Toggle the entire HD-2D stack — useful for measuring its frame
   *  cost or for users with low-end GPUs. */
  setEnabled(enabled: boolean): void {
    for (const f of [this.saturation, this.bloom, this.tilt, this.chroma]) {
      (f as any).enabled = enabled;
    }
  }
}
