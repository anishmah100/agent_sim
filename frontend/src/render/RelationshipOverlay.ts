// RelationshipOverlay — "Society Pulse": persistent lines between agents
// colored by their dominant relationship, so coalitions and feuds read
// at a glance without clicking anyone.
//
//   red   = hostile (attacks dominate)
//   gold  = bound by contract
//   green = friendly (pay/trade/whisper)
//
// Visual design (clarity pass):
//  - Lines bow into a gentle arc instead of straight segments, so two
//    agents with mutual ties don't overlap into one thick stripe and the
//    web reads as an organic social graph.
//  - A soft dark "halo" stroke sits under each colored line so ties stay
//    legible over any tile.
//  - A slow brightness pulse travels the whole overlay (shared phase) so
//    the society feels alive without strobing per-edge.
//  - Opacity/width scale with total interaction count (capped) so a
//    single whisper is a faint thread and a blood feud is a bold artery.
//
// Data comes from GET /api/v1/social (the engine social ledger);
// positions come from the live entity render positions. Toggleable; drawn
// beneath entities.
import { Container, Graphics } from "pixi.js";

// Palette-aligned tie colors + their dark halo companions.
const C_FEUD = 0xe43b44;       // hostile red
const C_CONTRACT = 0xfee761;   // gold
const C_ALLY = 0x5ee89a;       // green
const C_HALO = 0x181425;       // dark outline under every line

export interface SocialEdge {
  a: string;
  b: string;
  trade: number;
  whisper: number;
  pay: number;
  attack: number;
  contract: number;
}

type PosLookup = (id: string) => { x: number; y: number } | null;

export class RelationshipOverlay {
  readonly container: Container;
  private halo: Graphics;        // dark under-strokes (drawn first)
  private g: Graphics;           // colored ties (drawn over halo)
  private edges: SocialEdge[] = [];
  private getPos: PosLookup;
  private phase = 0;             // shared pulse phase (radians)
  enabled = true;

  constructor(getPos: PosLookup) {
    this.container = new Container();
    this.container.label = "relationship_overlay";
    this.halo = new Graphics();
    this.g = new Graphics();
    this.container.addChild(this.halo);
    this.container.addChild(this.g);
    this.getPos = getPos;
  }

  setEdges(edges: SocialEdge[]): void {
    this.edges = edges ?? [];
  }

  setEnabled(on: boolean): void {
    this.enabled = on;
    this.container.visible = on;
  }

  /** Redraw each frame so lines track moving agents. */
  tick(): void {
    if (!this.enabled) return;
    // ~0.9s per cycle — a calm breathing pulse, not a strobe.
    this.phase = (this.phase + 0.012) % (Math.PI * 2);
    const pulse = 0.85 + 0.15 * Math.sin(this.phase);

    this.halo.clear();
    this.g.clear();
    for (const e of this.edges) {
      const pa = this.getPos(e.a);
      const pb = this.getPos(e.b);
      if (!pa || !pb) continue; // an endpoint isn't on-screen / alive
      const total = e.trade + e.whisper + e.pay + e.attack + e.contract;
      if (total <= 0) continue;

      // Semantics: feud if attacks dominate; else gold if under contract;
      // else friendly green.
      const hostile = e.attack > (e.pay + e.trade + e.contract);
      const color = hostile ? C_FEUD : e.contract > 0 ? C_CONTRACT : C_ALLY;

      const intensity = Math.min(1, total / 8);
      const alpha = (0.16 + 0.5 * intensity) * pulse;
      const width = 0.7 + 2.3 * intensity;

      // Bow the line into a gentle arc. The control point is the midpoint
      // pushed perpendicular by a fixed fraction of the segment length, so
      // longer ties bow more and reciprocal ties don't collapse together.
      const mx = (pa.x + pb.x) / 2;
      const my = (pa.y + pb.y) / 2;
      const dx = pb.x - pa.x;
      const dy = pb.y - pa.y;
      const len = Math.hypot(dx, dy) || 1;
      const bow = Math.min(18, len * 0.14);
      // perpendicular unit vector
      const px = -dy / len;
      const py = dx / len;
      const cx = mx + px * bow;
      const cy = my + py * bow;

      // Dark halo under the tie for legibility on any tile.
      this.halo.moveTo(pa.x, pa.y)
        .quadraticCurveTo(cx, cy, pb.x, pb.y)
        .stroke({ color: C_HALO, width: width + 1.4, alpha: alpha * 0.6, cap: "round" });
      // Colored tie.
      this.g.moveTo(pa.x, pa.y)
        .quadraticCurveTo(cx, cy, pb.x, pb.y)
        .stroke({ color, width, alpha, cap: "round" });

      // Strong ties get a bright node dot at each endpoint so the most
      // important relationships anchor visually.
      if (intensity > 0.5) {
        const dot = 1.0 + 1.4 * intensity;
        this.g.circle(pa.x, pa.y, dot).fill({ color, alpha: alpha * 1.1 });
        this.g.circle(pb.x, pb.y, dot).fill({ color, alpha: alpha * 1.1 });
      }
    }
  }

  destroy(): void {
    this.container.destroy({ children: true });
  }
}
