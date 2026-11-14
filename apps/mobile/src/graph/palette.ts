// Colors for the graph, read from Module/01's published tokens so the canvas
// matches the rest of the app. Token values may reference other tokens
// ("{graphite-700}"); those are resolved here.
import tokens from '@franco_roura/module-01/tokens.json';

type Token = { name: string; value: string };

const byName = new Map<string, string>(
  (tokens as { color: { tokens: Token[] } }).color.tokens.map((t) => [t.name, t.value]),
);

function resolve(name: string, depth = 0): string {
  const v = byName.get(name);
  if (!v) return '#000000';
  const ref = /^\{(.+)\}$/.exec(v);
  return ref && depth < 5 ? resolve(ref[1], depth + 1) : v;
}

export const palette = {
  canvas: resolve('canvas'),
  shell: resolve('shell'),
  recess: resolve('shell-recess'),
  seam: resolve('seam'),
  ink: resolve('ink'),
  inkSoft: resolve('ink-soft'),
  hudMark: resolve('hud-mark'),
  signal: resolve('signal'),
  signalInk: resolve('signal-ink'),
  signalTint: resolve('signal-tint'),
};

// Module/01 reserves cat-red … cat-slate for categories and labels.
const categoryColors = [
  'cat-orange', 'cat-teal', 'cat-indigo', 'cat-amber', 'cat-pink', 'cat-green', 'cat-sky', 'cat-violet',
  'cat-red', 'cat-cyan', 'cat-lime', 'cat-rose', 'cat-blue', 'cat-yellow', 'cat-brown', 'cat-slate',
].map((n) => resolve(n));

function slotOf(id: string): number {
  let h = 0;
  for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
  return h % categoryColors.length;
}

/** A stable color per category id. */
export function categoryColor(id: string): string {
  return categoryColors[slotOf(id)];
}

/**
 * Colors for the categories on screen together. Each keeps its own color
 * unless two collide; then the later id (in sorted order) takes the next
 * free one, so up to 16 categories all look different.
 */
export function categoryColorMap(ids: string[]): Map<string, string> {
  const used = new Set<number>();
  const out = new Map<string, string>();
  for (const id of [...new Set(ids)].sort()) {
    let slot = slotOf(id);
    for (let i = 0; used.has(slot) && i < categoryColors.length; i++) slot = (slot + 1) % categoryColors.length;
    used.add(slot);
    out.set(id, categoryColors[slot]);
  }
  return out;
}
