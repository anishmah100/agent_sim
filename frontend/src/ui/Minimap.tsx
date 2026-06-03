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

  const draw = () => {
    if (!canvas) return;
    const [w, h] = size();
    const [tw, th] = props.worldDims;
    const ctx = canvas.getContext("2d");
    if (!ctx || tw <= 0 || th <= 0) return;
    canvas.width = w;
    canvas.height = h;

    // Fit world into canvas preserving aspect.
    const sx = w / tw;
    const sy = h / th;

    // Tiles.
    if (props.tiles && props.tiles.length === th) {
      for (let y = 0; y < th; y++) {
        const row = props.tiles[y];
        for (let x = 0; x < tw; x++) {
          const ch = row[x] ?? ".";
          ctx.fillStyle = TILE_COLOR[ch] ?? "#262b44";
          ctx.fillRect(Math.floor(x * sx), Math.floor(y * sy),
                       Math.ceil(sx), Math.ceil(sy));
        }
      }
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
