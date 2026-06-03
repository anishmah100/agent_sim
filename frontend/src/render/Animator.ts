// Animator — small state machine that drives the per-character visual
// effects we can't get from the existing AnimatedSprite frames alone:
//
//   - Idle bob: a slow 1-pixel up/down sine on the sprite when the
//     entity is idle. Adds life so static NPCs don't feel like cardboard.
//   - Death tween: fade alpha 1→0 + slight rotation when an entity's
//     HP drops to 0. The entity stays in the world for ~2 seconds in
//     a "fallen" state before the scenario removes it.
//   - Hit flash: 100ms red tint when the entity takes damage.
//
// Attaches to an entity's sprite + reads the entity state on tick.

import { Container, Sprite, type ColorMatrix } from "pixi.js";
import { ColorMatrixFilter } from "pixi.js";

export interface AnimatorTarget {
  container: Container;
  sprite: Sprite | { tint: number };
  /** Engine-driven HP — when this goes to 0, death tween fires. */
  hp?: number;
  /** Stable identifier for one-shot reaction tracking. */
  id: string;
}

interface State {
  hpLast: number;
  hitFlashUntilMs: number;
  deathTweenStartMs: number; // 0 if not dying
  baseY: number;             // recorded once for idle bob
  filter: ColorMatrixFilter | null;
}

export class Animator {
  private states = new Map<string, State>();

  attach(t: AnimatorTarget): void {
    this.states.set(t.id, {
      hpLast: t.hp ?? -1,
      hitFlashUntilMs: 0,
      deathTweenStartMs: 0,
      baseY: t.container.y,
      filter: null,
    });
  }

  detach(id: string): void {
    this.states.delete(id);
  }

  /** Called per frame from the entity layer ticker. */
  tick(targets: AnimatorTarget[], nowMs: number, isMoving: (id: string) => boolean): void {
    for (const t of targets) {
      let s = this.states.get(t.id);
      if (!s) { this.attach(t); s = this.states.get(t.id)!; }

      // Idle bob — only when not moving and not dying.
      if (!isMoving(t.id) && s.deathTweenStartMs === 0) {
        const bob = Math.sin(nowMs / 380) * 0.5;
        t.container.y = s.baseY + bob;
      }

      // Hit detection on HP drop.
      if (t.hp !== undefined && t.hp !== s.hpLast) {
        if (s.hpLast > 0 && t.hp < s.hpLast) {
          s.hitFlashUntilMs = nowMs + 120;
        }
        if (t.hp <= 0 && s.deathTweenStartMs === 0) {
          s.deathTweenStartMs = nowMs;
        }
        s.hpLast = t.hp;
      }

      // Hit flash — apply red color matrix.
      if (s.hitFlashUntilMs > nowMs) {
        if (!s.filter) {
          s.filter = new ColorMatrixFilter();
          (t.container as Container).filters = [s.filter];
        }
        s.filter.reset();
        const m = s.filter.matrix;
        m[0] = 2.2; m[6] = 0.8; m[12] = 0.6;
        s.filter.matrix = m as unknown as ColorMatrix;
      } else if (s.filter && s.deathTweenStartMs === 0) {
        (t.container as Container).filters = [];
        s.filter = null;
      }

      // Death tween — 2-second fade + rotation.
      if (s.deathTweenStartMs > 0) {
        const t01 = Math.min(1, (nowMs - s.deathTweenStartMs) / 2000);
        t.container.alpha = 1 - t01;
        t.container.rotation = (Math.PI / 2) * t01;
      }
    }
  }
}
