// Minimap — a real one. Renders the world tile grid + entity dots into
// a small canvas in the bottom-left corner. Click to recenter the
// camera. Entities re-rendered every snapshot.

import { onMount, onCleanup, createSignal } from "solid-js";
import type { EntityState } from "../render/Entity";

interface MinimapProps {
  worldDims: [number, number];
  tiles?: string[];                       // optional — ASCII rows from the world JSON
  entities: EntityState[];
  selfId: string | null;
  onTileClick?: (x: number, y: number) => void;
  /** Returns the currently-visible world rectangle in TILE coordinates.
   *  Drawn as an outline over the minimap so users can see where the
   *  viewport is in the larger world. Returning null hides the box. */
  getViewportTileRect?: () => { x: number; y: number; w: number; h: number } | null;
}

const TILE_COLOR: Record<string, string> = {
  g: "#3e8948",   // grass
  d: "#b86f50",   // dirt
  p: "#e4a672",   // path
  w: "#0099db",   // water
  s: "#c0cbdc",   // stone
  W: "#3a4466",   // wall
  f: "#b86f50",   // floor_wood
  ".": "#181425", // void
};

export function Minimap(props: MinimapProps) {
  let canvas!: HTMLCanvasElement;
  const [size] = createSignal<[number, number]>([200, 150]);

  // Pre-baked tile layer. The tilemap is STATIC for the session, so we
  // draw it ONCE into an off-screen canvas and just blit it onto the
  // live canvas every refresh. Pre-bake skips drawing 2.25M fillRects
  // (1500×1500 Eldoria) 5 times a second, which was the main lag source.
  let bakedTiles: HTMLCanvasElement | null = null;
  let bakedTilesKey = "";

  const bakeTiles = (): HTMLCanvasElement | null => {
    const [w, h] = size();
    const [tw, th] = props.worldDims;
    if (tw <= 0 || th <= 0) return null;
    const tiles = props.tiles;
    const key = `${tw}x${th}-${tiles?.length ?? 0}-${w}x${h}`;
    if (bakedTiles && bakedTilesKey === key) return bakedTiles;

    const off = document.createElement("canvas");
    off.width = w;
    off.height = h;
    const ctx = off.getContext("2d");
    if (!ctx) return null;
    if (tiles && tiles.length === th) {
      const sx = w / tw;
      const sy = h / th;
      // Group adjacent same-colour spans into one fill — for biome-heavy
      // worlds that's an order-of-magnitude reduction in fillRect calls.
      for (let y = 0; y < th; y++) {
        const row = tiles[y];
        const py = Math.floor(y * sy);
        const pyN = Math.floor((y + 1) * sy);
        const phEff = Math.max(1, pyN - py);
        let runStart = 0;
        let runCh = row[0] ?? ".";
        for (let x = 1; x <= tw; x++) {
          const ch = x < tw ? (row[x] ?? ".") : "\0";
          if (ch !== runCh) {
            const x0 = Math.floor(runStart * sx);
            const x1 = Math.floor(x * sx);
            ctx.fillStyle = TILE_COLOR[runCh] ?? "#262b44";
            ctx.fillRect(x0, py, Math.max(1, x1 - x0), phEff);
            runStart = x;
            runCh = ch;
          }
        }
      }
    } else {
      ctx.fillStyle = "#262b44";
      ctx.fillRect(0, 0, w, h);
    }
    bakedTiles = off;
    bakedTilesKey = key;
    return off;
  };

  const draw = () => {
    if (!canvas) return;
    const [w, h] = size();
    const [tw, th] = props.worldDims;
    const ctx = canvas.getContext("2d");
    if (!ctx || tw <= 0 || th <= 0) return;
    canvas.width = w;
    canvas.height = h;
    const sx = w / tw;
    const sy = h / th;

    // Blit the pre-baked tile layer.
    const baked = bakeTiles();
    if (baked) {
      ctx.drawImage(baked, 0, 0);
    } else {
      ctx.fillStyle = "#262b44";
      ctx.fillRect(0, 0, w, h);
    }

    // Entity dots.
    for (const e of props.entities) {
      const cx = (e.pos[0] + 0.5) * sx;
      const cy = (e.pos[1] + 0.5) * sy;
      const isSelf = e.entity_id === props.selfId;
      ctx.fillStyle = isSelf ? "#fee761" :
                       e.archetype === "goblin" ? "#e43b44" :
                       e.archetype === "tree" ? "#265c42" :
                       e.archetype === "rock" ? "#8b9bb4" :
                       "#ead4aa";
      ctx.beginPath();
      ctx.arc(cx, cy, isSelf ? 3 : 2, 0, Math.PI * 2);
      ctx.fill();
      if (isSelf) {
        ctx.strokeStyle = "#181425";
        ctx.lineWidth = 1;
        ctx.stroke();
      }
    }

    // Viewport rectangle — drawn last so it sits over entity dots.
    if (props.getViewportTileRect) {
      const r = props.getViewportTileRect();
      if (r) {
        const x = r.x * sx;
        const y = r.y * sy;
        const wPx = r.w * sx;
        const hPx = r.h * sy;
        ctx.strokeStyle = "#fee761";
        ctx.lineWidth = 1.2;
        ctx.strokeRect(x, y, wPx, hPx);
        ctx.fillStyle = "rgba(254, 231, 97, 0.10)";
        ctx.fillRect(x, y, wPx, hPx);
      }
    }
  };

  onMount(() => {
    draw();
    const interval = window.setInterval(draw, 200);   // 5 Hz refresh
    onCleanup(() => window.clearInterval(interval));
  });

  const onClick = (e: MouseEvent) => {
    if (!props.onTileClick || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const px = (e.clientX - rect.left) / rect.width;
    const py = (e.clientY - rect.top) / rect.height;
    props.onTileClick(
      Math.floor(px * props.worldDims[0]),
      Math.floor(py * props.worldDims[1]),
    );
  };

  return (
    <div
      style={{
        position: "fixed",
        bottom: "16px",
        left: "16px",
        background: "rgba(24, 20, 37, 0.92)",
        border: "1px solid #3a4466",
        "border-radius": "4px",
        padding: "6px",
        "z-index": "8",
      }}
    >
      <canvas
        ref={canvas}
        onClick={onClick}
        style={{
          display: "block",
          width: `${size()[0]}px`,
          height: `${size()[1]}px`,
          cursor: "pointer",
          "image-rendering": "pixelated",
        }}
      />
      <div style={{ color: "#8b9bb4", "font-size": "10px", "margin-top": "4px", "text-align": "center" }}>
        click to center camera
      </div>
    </div>
  );
}
