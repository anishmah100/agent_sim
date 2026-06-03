// Particle / FX layer.
//
// Lightweight Pixi-Graphics-based emitters for the gameplay FX:
//   - footstepDust(pos)    — small puff under a walking entity
//   - attackHit(pos)       — short red flash for combat damage
//   - waterSparkle(pos)    — gentle blue twinkle on water tiles
//   - leafFall(pos)        — slow drift under a forest tree
//
// Particle pool keeps allocation flat — recycled Graphics objects.

import { Container, Graphics, type Application } from "pixi.js";
import { TILE_SIZE_PX } from "./tiles";

interface Particle {
  gfx: Graphics;
  vx: number;
  vy: number;
  ttlMs: number;
  ageMs: number;
  fadeFrom: number;
}

export class ParticleLayer {
  readonly container: Container;
  private pool: Particle[] = [];
  private alive: Particle[] = [];

  constructor(app: Application) {
    this.container = new Container();
    this.container.label = "particles";
    app.ticker.add((delta) => this.tick(delta.deltaMS));
  }

  footstepDust(worldX: number, worldY: number): void {
    for (let i = 0; i < 3; i++) {
      const p = this.acquire();
      p.gfx.circle(0, 0, 1).fill({ color: 0xaa996b, alpha: 0.6 });
      p.gfx.x = worldX;
      p.gfx.y = worldY + TILE_SIZE_PX - 2;
      p.vx = (Math.random() - 0.5) * 0.4;
      p.vy = -0.3 - Math.random() * 0.3;
      p.ttlMs = 380;
      p.fadeFrom = 0.6;
      p.ageMs = 0;
    }
  }

  attackHit(worldX: number, worldY: number): void {
    const p = this.acquire();
    p.gfx.circle(0, 0, 6).fill({ color: 0xff4444, alpha: 0.85 });
    p.gfx.x = worldX + TILE_SIZE_PX / 2;
    p.gfx.y = worldY + TILE_SIZE_PX / 2;
    p.vx = 0;
    p.vy = 0;
    p.ttlMs = 160;
    p.fadeFrom = 0.85;
    p.ageMs = 0;
  }

  waterSparkle(worldX: number, worldY: number): void {
    const p = this.acquire();
    p.gfx.rect(-1, -1, 2, 2).fill({ color: 0xffffff, alpha: 0.9 });
    p.gfx.x = worldX + Math.random() * TILE_SIZE_PX;
    p.gfx.y = worldY + Math.random() * TILE_SIZE_PX;
    p.vx = 0;
    p.vy = 0;
    p.ttlMs = 700;
    p.fadeFrom = 0.9;
    p.ageMs = 0;
  }

  leafFall(worldX: number, worldY: number): void {
    const p = this.acquire();
    const color = Math.random() < 0.5 ? 0x6c8b3a : 0xb88b3a;
    p.gfx.rect(-1, -1, 2, 2).fill({ color, alpha: 0.8 });
    p.gfx.x = worldX + Math.random() * TILE_SIZE_PX;
    p.gfx.y = worldY - 6;
    p.vx = (Math.random() - 0.5) * 0.5;
    p.vy = 0.3 + Math.random() * 0.3;
    p.ttlMs = 2200;
    p.fadeFrom = 0.8;
    p.ageMs = 0;
  }

  destroy(): void {
    for (const p of this.alive) p.gfx.destroy();
    for (const p of this.pool) p.gfx.destroy();
    this.container.destroy();
  }

  private acquire(): Particle {
    let p = this.pool.pop();
    if (!p) {
      const g = new Graphics();
      this.container.addChild(g);
      p = { gfx: g, vx: 0, vy: 0, ttlMs: 0, ageMs: 0, fadeFrom: 1 };
    } else {
      p.gfx.clear();
      p.gfx.alpha = 1;
      p.gfx.visible = true;
    }
    this.alive.push(p);
    return p;
  }

  private tick(deltaMs: number): void {
    const remaining: Particle[] = [];
    for (const p of this.alive) {
      p.ageMs += deltaMs;
      p.gfx.x += p.vx * deltaMs / 16;
      p.gfx.y += p.vy * deltaMs / 16;
      const t = p.ageMs / p.ttlMs;
      p.gfx.alpha = p.fadeFrom * (1 - t);
      if (p.ageMs >= p.ttlMs) {
        p.gfx.visible = false;
        this.pool.push(p);
      } else {
        remaining.push(p);
      }
    }
    this.alive = remaining;
  }
}
