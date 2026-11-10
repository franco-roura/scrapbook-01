'use dom';

// The force-directed graph, drawn on a canvas with react-force-graph-2d. As
// an Expo DOM component it runs in a web view on iOS/Android and directly on
// web. Props must be serializable; callbacks are async and run natively.
//
// Look: Module/01 on a canvas. White shell nodes with a seam outline,
// category hubs in Module/01's category colors, a faint HUD dot grid, and
// on the focused node two signal rings, crosshair ticks and a mono label.
import { useEffect, useMemo, useRef, useState } from 'react';
import ForceGraph2D, { type ForceGraphMethods, type LinkObject, type NodeObject } from 'react-force-graph-2d';

import './fonts.css';
import { categoryColor, palette } from './palette';
import type { GraphData, GraphLink, GraphNode } from './types';

type Node = NodeObject<GraphNode & { x?: number; y?: number }>;
type Link = LinkObject<Node, GraphLink>;

export type GraphCanvasProps = {
  graph: GraphData;
  /** The selected node: centered, ringed and labeled. */
  focusId?: string | null;
  /** Zoom to show everything on first layout (global view). */
  fitOnLoad?: boolean;
  onSelect?: (id: string, kind: 'item' | 'category') => Promise<void>;
  dom?: import('expo/dom').DOMProps;
};

// Google Fonts names inside the web view; expo-font's names in the web build.
const FONT_MONO = '"Space Mono", SpaceMono_400Regular, ui-monospace, Menlo, monospace';
const FONT_DISPLAY = 'Michroma, Michroma_400Regular, "Eurostile", system-ui, sans-serif';

function radius(n: Node) {
  return n.kind === 'category' ? 6 + Math.sqrt(n.degree) * 1.8 : 3.5 + Math.sqrt(n.degree) * 1.2;
}

/** A d3-style force pulling every node toward (0, 0). */
function gravity(k: number) {
  let nodes: Node[] = [];
  const force = (alpha: number) => {
    for (const n of nodes) {
      n.vx = (n.vx ?? 0) - (n.x ?? 0) * k * alpha;
      n.vy = (n.vy ?? 0) - (n.y ?? 0) * k * alpha;
    }
  };
  force.initialize = (ns: Node[]) => {
    nodes = ns;
  };
  return force;
}

const endId = (end: Link['source']) => (typeof end === 'object' ? String((end as Node).id) : String(end));

export default function GraphCanvas({ graph, focusId, fitOnLoad, onSelect }: GraphCanvasProps) {
  const fg = useRef<ForceGraphMethods<Node, Link> | undefined>(undefined);
  const box = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0 });
  const [hoverId, setHoverId] = useState<string | null>(null);
  // Positions survive data changes, so moving the focus doesn't reshuffle.
  // (A Map held in state, not a ref: it's read while building graph data.)
  const [positions] = useState(() => new Map<string, { x: number; y: number }>());
  const fitted = useRef(false);

  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(([e]) => setSize({ width: e.contentRect.width, height: e.contentRect.height }));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const data = useMemo(() => {
    const nodes: Node[] = graph.nodes.map((n) => {
      const p = positions.get(n.id);
      return p ? { ...n, x: p.x, y: p.y } : { ...n };
    });
    const links: Link[] = graph.links.map((l) => ({ ...l }));
    return { nodes, links };
  }, [graph, positions]);

  // Who's next to the highlighted node, for dimming everything else.
  const active = hoverId ?? focusId ?? null;
  const near = useMemo(() => {
    const s = new Set<string>();
    if (!active) return s;
    s.add(active);
    for (const l of graph.links) {
      if (l.source === active) s.add(l.target);
      if (l.target === active) s.add(l.source);
    }
    return s;
  }, [graph, active]);

  // Configure forces once the graph is mounted (it mounts after the first
  // size measurement) and again whenever the data changes.
  const mounted = size.width > 0;
  useEffect(() => {
    const f = fg.current;
    if (!mounted || !f) return;
    // Repulsion only between nearby nodes, so clusters stay close together.
    const charge = f.d3Force('charge') as unknown as { strength?: (n: number) => void; distanceMax?: (n: number) => void } | undefined;
    charge?.strength?.(-35);
    charge?.distanceMax?.(90);
    // A gentle pull to the middle keeps separate clusters from drifting apart.
    f.d3Force('gravity', gravity(0.1) as never);
    const link = f.d3Force('link') as unknown as { distance?: (fn: (l: Link) => number) => void } | undefined;
    link?.distance?.((l) => (l.kind === 'category' ? 28 : 18 + (1 - l.weight) * 40));
    f.d3ReheatSimulation();
  }, [data, mounted]);

  // Bring the focused node to the middle when it changes.
  useEffect(() => {
    if (!focusId) return;
    const t = setTimeout(() => {
      const n = data.nodes.find((d) => d.id === focusId);
      if (n?.x !== undefined && n.y !== undefined) {
        fg.current?.centerAt(n.x, n.y, 600);
        fg.current?.zoom(2.4, 600);
      }
    }, 250);
    return () => clearTimeout(t);
  }, [focusId, data]);

  function drawNode(n: Node, ctx: CanvasRenderingContext2D, scale: number) {
    const x = n.x ?? 0;
    const y = n.y ?? 0;
    const r = radius(n);
    const isFocus = n.id === focusId;
    const dim = active !== null && !near.has(String(n.id));
    ctx.globalAlpha = dim ? 0.25 : 1;

    const hue = n.kind === 'category' ? categoryColor(String(n.id).slice(2)) : n.categoryId ? categoryColor(n.categoryId) : palette.seam;
    ctx.beginPath();
    ctx.arc(x, y, r, 0, 2 * Math.PI);
    ctx.fillStyle = n.kind === 'category' ? hue : isFocus ? palette.signalTint : palette.shell;
    ctx.fill();
    ctx.lineWidth = (n.kind === 'category' ? 1 : 1.6) / scale;
    ctx.strokeStyle = n.kind === 'category' ? palette.ink : hue;
    ctx.stroke();

    if (isFocus) {
      // HUD: a solid and a dashed ring, plus crosshair ticks.
      ctx.strokeStyle = palette.signalInk;
      ctx.lineWidth = 1.2 / scale;
      ctx.beginPath();
      ctx.arc(x, y, r + 4, 0, 2 * Math.PI);
      ctx.stroke();
      ctx.setLineDash([3 / scale, 3 / scale]);
      ctx.beginPath();
      ctx.arc(x, y, r + 8, 0, 2 * Math.PI);
      ctx.stroke();
      ctx.setLineDash([]);
      const a = r + 10;
      const b = r + 15;
      ctx.beginPath();
      for (const [dx, dy] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
        ctx.moveTo(x + dx * a, y + dy * a);
        ctx.lineTo(x + dx * b, y + dy * b);
      }
      ctx.stroke();
    }

    // Labels: categories always; items when hovered, next to the highlighted
    // node, or (with nothing highlighted) once zoomed in.
    const showLabel =
      n.kind === 'category' || n.id === hoverId || (active !== null ? near.has(String(n.id)) : scale > 2.6);
    if (showLabel) {
      const isCat = n.kind === 'category';
      const px = (isCat ? 11 : 10) / Math.min(scale, 3);
      ctx.font = `${isCat || isFocus ? 700 : 400} ${px}px ${isCat ? FONT_DISPLAY : FONT_MONO}`;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'top';
      const max = isFocus ? 40 : 28;
      const text = !isCat && n.label.length > max ? n.label.slice(0, max - 1) + '…' : n.label;
      const ty = y + r + (isFocus ? 18 : 3);
      // A canvas-colored halo keeps labels readable over links.
      ctx.lineWidth = 3 / scale;
      ctx.strokeStyle = palette.canvas;
      ctx.strokeText(isCat ? text.toUpperCase() : text, x, ty);
      ctx.fillStyle = isFocus ? palette.signalInk : isCat ? palette.ink : palette.inkSoft;
      ctx.fillText(isCat ? text.toUpperCase() : text, x, ty);
    }
    ctx.globalAlpha = 1;
  }

  function drawGrid(ctx: CanvasRenderingContext2D, scale: number) {
    // HUD microprint: a sparse dot grid in graph space.
    const step = 40;
    const w = size.width / scale;
    const h = size.height / scale;
    const c = fg.current?.centerAt() ?? { x: 0, y: 0 };
    ctx.fillStyle = palette.seam;
    const r = 0.9 / scale;
    for (let x = Math.floor((c.x - w) / step) * step; x < c.x + w; x += step) {
      for (let y = Math.floor((c.y - h) / step) * step; y < c.y + h; y += step) {
        ctx.fillRect(x - r, y - r, r * 2, r * 2);
      }
    }
  }

  return (
    <div ref={box} style={{ position: 'absolute', inset: 0, background: palette.canvas, overflow: 'hidden', touchAction: 'none' }}>
      {size.width > 0 ? (
        <ForceGraph2D<Node, Link>
          ref={fg}
          width={size.width}
          height={size.height}
          graphData={data}
          backgroundColor={palette.canvas}
          nodeId="id"
          nodeRelSize={1}
          nodeVal={(n) => radius(n) ** 2}
          nodeCanvasObject={drawNode}
          nodeCanvasObjectMode={() => 'replace'}
          nodePointerAreaPaint={(n, color, ctx) => {
            ctx.fillStyle = color;
            ctx.beginPath();
            ctx.arc(n.x ?? 0, n.y ?? 0, radius(n) + 6, 0, 2 * Math.PI);
            ctx.fill();
          }}
          linkColor={(l) => {
            const touches = active !== null && (endId(l.source) === active || endId(l.target) === active);
            if (touches) return palette.signalInk;
            const alpha = active !== null ? 0.12 : l.kind === 'category' ? 0.35 : 0.25 + l.weight * 0.5;
            return `rgba(83, 87, 94, ${alpha.toFixed(2)})`;
          }}
          linkWidth={(l) => (l.kind === 'similar' ? 0.6 + l.weight * 1.2 : 0.6)}
          linkLineDash={(l) => (l.kind === 'category' ? [2, 2] : null)}
          onRenderFramePre={drawGrid}
          onNodeHover={(n) => setHoverId(n ? String(n.id) : null)}
          onNodeClick={(n) => onSelect?.(String(n.id), n.kind)}
          onEngineTick={() => {
            for (const n of data.nodes) if (n.x !== undefined && n.y !== undefined) positions.set(String(n.id), { x: n.x, y: n.y });
          }}
          onEngineStop={() => {
            if (fitOnLoad && !fitted.current) {
              fitted.current = true;
              fg.current?.zoomToFit(500, 30);
            }
          }}
          cooldownTicks={180}
          enableNodeDrag
        />
      ) : null}
    </div>
  );
}
