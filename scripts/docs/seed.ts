// Fills a demo library for the docs screenshots: a made-up person's saves
// from Instagram, TikTok, YouTube and X, with notes and categories.
//
// Needs the demo API from `bun run docs:demo` (dev tokens, fake AI).
// Usage: bun scripts/docs/seed.ts [apiUrl]
import { devToken } from './token';

const API = (process.argv[2] ?? 'http://localhost:18080').replace(/\/$/, '');
type Item = { id: string; classifying: boolean };

const headers = { Authorization: `Bearer ${devToken()}`, 'Content-Type': 'application/json' };

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${API}${path}`, { method, headers, body: body ? JSON.stringify(body) : undefined });
  if (!res.ok) throw new Error(`${method} ${path}: ${res.status} ${await res.text()}`);
  return (await res.json()) as T;
}

// Notes share words within a category, so the fake embeddings link them.
const library: Record<string, [url: string, note: string][]> = {
  'Weeknight dinners': [
    ['https://www.instagram.com/reel/C9demo01/', 'One-pan lemon chicken with orzo, 25 minute weeknight dinner'],
    ['https://www.tiktok.com/@cook/video/7400000000000000001', 'Crispy chickpea curry, easy weeknight dinner with coconut milk'],
    ['https://www.youtube.com/watch?v=dinner00001', 'Sheet-pan gnocchi with roasted vegetables, quick weeknight dinner'],
    ['https://www.instagram.com/reel/C9demo02/', 'Miso salmon rice bowls, 20 minute dinner for two'],
  ],
  'Japan trip': [
    ['https://www.youtube.com/watch?v=japantrip01', 'Kyoto in 3 days: temples, Nishiki market and where to stay in Japan'],
    ['https://www.instagram.com/p/C9japan01/', 'Hidden ramen bar in Tokyo, Shinjuku, for the Japan trip'],
    ['https://x.com/travel/status/1800000000000000001', 'JR pass vs single tickets for a two week Japan trip'],
    ['https://www.tiktok.com/@trips/video/7400000000000000002', 'Onsen etiquette before the Japan trip, what to bring'],
  ],
  'Parenting': [
    ['https://www.tiktok.com/@parents/video/7400000000000000003', 'Bedtime routine that helps toddlers sleep, calm parenting tips'],
    ['https://www.instagram.com/reel/C9kids001/', '5 dad-daughter traditions to try this year, parenting ideas'],
    ['https://www.youtube.com/watch?v=kids0000001', 'Rainy day activities for toddlers, parenting ideas at home'],
  ],
  'Home workouts': [
    ['https://www.youtube.com/watch?v=workout0001', '20 minute full body home workout, no equipment'],
    ['https://www.instagram.com/reel/C9fit0001/', 'Mobility routine for desk workers, 10 minute home workout'],
    ['https://www.tiktok.com/@fit/video/7400000000000000004', 'Kettlebell home workout for beginners, full body'],
  ],
  'Design inspiration': [
    ['https://x.com/design/status/1800000000000000002', 'Beautiful dashboard design with a monochrome palette and orange accents'],
    ['https://www.instagram.com/p/C9design1/', 'Industrial product design: molded plastic devices and HUD labels'],
    ['https://www.youtube.com/watch?v=design00001', 'Typography for dashboard design: monospace labels and tight grids'],
  ],
  'Books to read': [
    ['https://www.tiktok.com/@books/video/7400000000000000005', 'Three sci-fi books to read if you loved Project Hail Mary'],
    ['https://x.com/reads/status/1800000000000000003', 'Thread of the best nonfiction books to read on habits and focus'],
  ],
};
const inbox = [
  'https://www.instagram.com/reel/C9inbox01/',
  'https://www.youtube.com/watch?v=inbox000001',
  'https://www.tiktok.com/@misc/video/7400000000000000006',
];


// Oldest first: the inbox, then one item per category filed by hand (so the
// category exists), then everything else, sorted automatically.
for (const url of inbox) await call('POST', '/v1/items', { url });
const rest: [string, string][] = [];
for (const [name, [[url, note], ...others]] of Object.entries(library)) {
  const cat = await call<{ id: string }>('POST', '/v1/categories', { name });
  const item = await call<Item>('POST', '/v1/items', { url });
  await call('PATCH', `/v1/items/${item.id}`, { categoryId: cat.id });
  await call('PATCH', `/v1/items/${item.id}`, { description: note });
  rest.push(...others);
}
await waitForSorting();
// Interleave categories, the way saves really arrive.
rest.sort((a, b) => a[0].length - b[0].length || a[1].localeCompare(b[1]));
for (const [url, note] of rest) {
  await call('POST', '/v1/items', { url, description: note });
  await new Promise((r) => setTimeout(r, 300));
}

await waitForSorting();

// Waits until the worker has filed, embedded and linked everything.
async function waitForSorting() {
  for (let i = 0; i < 60; i++) {
    const { items } = await call<{ items: Item[] }>('GET', '/v1/items?limit=100');
    if (!items.some((it) => it.classifying)) return;
    await new Promise((r) => setTimeout(r, 1000));
  }
}
const graph = await call<{ nodes: unknown[]; links: unknown[] }>('GET', '/v1/graph');
console.log(`seeded: ${graph.nodes.length} nodes, ${graph.links.length} links`);
