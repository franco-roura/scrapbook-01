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

// Notes share words within a category, so the fake embeddings link them. In
// Uruguayan Spanish, like the app.
const library: Record<string, [url: string, note: string][]> = {
  'Cenas de semana': [
    ['https://www.instagram.com/reel/C9demo01/', 'Pollo al limón con orzo en una sola sartén, cena rápida de semana'],
    ['https://www.tiktok.com/@cook/video/7400000000000000001', 'Curry de garbanzos crocante con leche de coco, cena rápida de semana'],
    ['https://www.youtube.com/watch?v=dinner00001', 'Ñoquis al horno con verduras asadas, cena rápida de semana'],
    ['https://www.instagram.com/reel/C9demo02/', 'Bowls de salmón y arroz con miso, cena de semana para dos'],
  ],
  'Viaje a Japón': [
    ['https://www.youtube.com/watch?v=japantrip01', 'Kioto en 3 días: templos, mercado Nishiki y dónde dormir en el viaje a Japón'],
    ['https://www.instagram.com/p/C9japan01/', 'Bar de ramen escondido en Shinjuku, Tokio, para el viaje a Japón'],
    ['https://x.com/travel/status/1800000000000000001', 'JR Pass o pasajes sueltos para dos semanas de viaje en Japón'],
    ['https://www.tiktok.com/@trips/video/7400000000000000002', 'Etiqueta del onsen antes del viaje a Japón: qué llevar'],
  ],
  'Crianza': [
    ['https://www.tiktok.com/@parents/video/7400000000000000003', 'Que los gurises duerman mejor: tips de crianza para la noche'],
    ['https://www.instagram.com/reel/C9kids001/', '5 tradiciones para hacer con tu hija, ideas de crianza'],
    ['https://www.youtube.com/watch?v=kids0000001', 'Actividades para días de lluvia con gurises, ideas de crianza'],
  ],
  'Rutinas en casa': [
    ['https://www.youtube.com/watch?v=workout0001', 'Rutina de cuerpo completo en casa, 20 minutos y sin equipo'],
    ['https://www.instagram.com/reel/C9fit0001/', 'Movilidad para los que trabajan sentados, rutina en casa'],
    ['https://www.tiktok.com/@fit/video/7400000000000000004', 'Rutina con kettlebell en casa para principiantes, cuerpo completo'],
  ],
  'Inspiración de diseño': [
    ['https://x.com/design/status/1800000000000000002', 'Dashboard en paleta monocromática con detalles naranja, inspiración de diseño'],
    ['https://www.instagram.com/p/C9design1/', 'Diseño industrial: aparatos de plástico moldeado y etiquetas HUD'],
    ['https://www.youtube.com/watch?v=design00001', 'Tipografía para el diseño de un dashboard: etiquetas monoespaciadas'],
  ],
  'Libros para leer': [
    ['https://www.tiktok.com/@books/video/7400000000000000005', 'Tres libros de ciencia ficción para leer si te gustó Proyecto Hail Mary'],
    ['https://x.com/reads/status/1800000000000000003', 'Lista de libros de no ficción sobre hábitos y concentración'],
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
