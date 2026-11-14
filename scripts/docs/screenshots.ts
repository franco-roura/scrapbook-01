// Captures docs/screenshots from the demo library, at phone size.
//
// Needs the demo API (port 18080) seeded with scripts/docs/seed.ts, and the
// web export served on port 8090 (scripts/docs/serve.ts). See
// docs/development.md#docs-screenshots-and-diagrams.
// Usage: bun scripts/docs/screenshots.ts [webUrl]
import { chromium, type Page } from '@playwright/test';
import { mkdirSync } from 'node:fs';

import { devToken } from './token';

const WEB = (process.argv[2] ?? 'http://localhost:8090').replace(/\/$/, '');
const API = process.env.DEMO_API_URL ?? 'http://localhost:18080';
const OUT = 'docs/screenshots';
mkdirSync(OUT, { recursive: true });

const token = devToken();
const session = {
  accessToken: token,
  refreshToken: 'docs',
  expiresAt: Date.now() + 5 * 3600_000,
  user: { id: 'user_docs_demo', email: 'demo@example.com' },
};

const browser = await chromium.launch();
const context = await browser.newContext({
  viewport: { width: 390, height: 844 },
  deviceScaleFactor: 2,
  colorScheme: 'light',
});

async function open(path: string, signedIn = true, viewport?: { width: number; height: number }): Promise<Page> {
  const page = await context.newPage();
  if (viewport) await page.setViewportSize(viewport);
  await page.addInitScript(
    ([s, on]) => {
      if (on) localStorage.setItem('scrapbook01_session', s);
      else localStorage.removeItem('scrapbook01_session');
    },
    [JSON.stringify(session), signedIn] as const,
  );
  await page.goto(`${WEB}${path}`);
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
  return page;
}

async function shot(page: Page, name: string, settleMs = 600) {
  await page.waitForTimeout(settleMs);
  await page.screenshot({ path: `${OUT}/${name}.png` });
  console.log(`${OUT}/${name}.png`);
  await page.close();
}

type Item = { id: string; description: string | null };
const { items } = (await (await fetch(`${API}/v1/items?limit=100`, { headers: { Authorization: `Bearer ${token}` } })).json()) as {
  items: Item[];
};
const featured = items.find((it) => it.description?.startsWith('Bar de ramen')) ?? items[0];

await shot(await open('/'), 'library');
{
  const page = await open('/');
  await page.getByText('Sin describir', { exact: true }).click();
  await shot(page, 'inbox');
}
await shot(await open(`/item/${featured.id}`), 'item', 1500);
{
  const page = await open(`/item/${featured.id}`);
  await page.waitForTimeout(1500);
  // React Native Web scrolls an inner element, not the page.
  await page.evaluate(() => {
    for (const el of document.querySelectorAll<HTMLElement>('*')) {
      if (el.scrollHeight > el.clientHeight + 20 && getComputedStyle(el).overflowY !== 'visible') {
        el.scrollTop = el.scrollHeight;
      }
    }
  });
  await shot(page, 'item-connections', 5000);
}
await shot(await open('/graph'), 'graph', 5000);
{
  // The whole library, wide, for the README.
  const page = await open('/graph', true, { width: 1280, height: 760 });
  await shot(page, 'graph-wide', 7000);
}
{
  const page = await open(`/share?text=${encodeURIComponent('https://www.instagram.com/reel/C9demo99/')}`);
  await page.keyboard.type('Pasta e ceci, una cena calentita de semana', { delay: 10 });
  await shot(page, 'share');
}
await shot(await open('/credits'), 'credits');
await shot(await open('/sign-in', false), 'sign-in');

await browser.close();
