// RelationshipOverlay — "Society Pulse": persistent lines between agents
// colored by their dominant relationship, so coalitions and feuds read
// at a glance without clicking anyone.
//
//   red   = hostile (attacks dominate)
//   gold  = bound by contract
//   green = friendly (pay/trade/whisper)
//
// Line opacity/width scale with total interaction count. Data comes from
// GET /api/v1/social (the engine social ledger); positions come from the
// live entity render positions. Toggleable; drawn beneath entities.
import { Container, Graphics } from "pixi.js";

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
  private g: Graphics;
  private edges: SocialEdge[] = [];
  private getPos: PosLookup;
  enabled = true;

  constructor(getPos: PosLookup) {
    this.container = new Container();
    this.container.label = "relationship_overlay";
    this.g = new Graphics();
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
    this.g.clear();
    for (const e of this.edges) {
      const pa = this.getPos(e.a);
      const pb = this.getPos(e.b);
      if (!pa || !pb) continue; // an endpoint isn't on-screen / alive
      const total = e.trade + e.whisper + e.pay + e.attack + e.contract;
      if (total <= 0) continue;
      const hostile = e.attack > (e.pay + e.trade + e.contract);
      const color = hostile ? 0xff4d4d : e.contract > 0 ? 0xe8c860 : 0x5ee89a;
      // opacity + width grow with interaction volume (capped).
      const intensity = Math.min(1, total / 8);
      const alpha = 0.18 + 0.5 * intensity;
      const width = 0.6 + 2.2 * intensity;
      this.g.moveTo(pa.x, pa.y).lineTo(pb.x, pb.y)
        .stroke({ color, width, alpha });
    }
  }

  destroy(): void {
    this.container.destroy({ children: true });
  }
}
