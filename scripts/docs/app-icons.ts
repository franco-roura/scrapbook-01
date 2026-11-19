// Renders the mobile app's icons from the brand icon (docs/brand/icon.svg):
// - icon.png: iOS, a full-bleed square (iOS rounds the corners itself)
// - android-icon-{background,foreground,monochrome}.png: adaptive icon layers.
//   The launcher masks them to a circle or squircle, so the device is scaled
//   into the 66/108 safe zone and the HUD microprint is left out.
// - splash-icon.png: the device alone, on transparent
// - favicon.png: the rounded tile, for the web build
// Usage: bun scripts/docs/app-icons.ts
import { chromium } from '@playwright/test';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const svg = readFileSync('docs/brand/icon.svg', 'utf8');
const defs = svg.slice(svg.indexOf('<defs>'), svg.indexOf('</defs>') + '</defs>'.length);
const device = svg.slice(svg.indexOf('<!-- Device body'), svg.lastIndexOf('</svg>'));
const shell = 'M268 220 H732 L816 304 V744 Q816 820 740 820 H284 Q208 820 208 744 V296 Q208 220 284 220 Z';

// The device spans 208..816 x 220..820; its corners sit ~400 from the center,
// and the safe zone's radius is 1024 * 33/108 = 313.
const fit = (scale: number, body: string) =>
  `<g transform="translate(512 512) scale(${scale}) translate(-512 -520)">${body}</g>`;
const doc = (body: string) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">${defs}${body}</svg>`;

const out = 'apps/mobile/assets';
const icons: { file: string; size: number; svg: string }[] = [
  { file: 'icon.png', size: 1024, svg: svg.replace('<rect width="1024" height="1024" rx="224"', '<rect width="1024" height="1024"') },
  { file: 'android-icon-background.png', size: 1024, svg: doc('<rect width="1024" height="1024" fill="url(#tile)" />') },
  { file: 'android-icon-foreground.png', size: 1024, svg: doc(fit(0.74, device)) },
  {
    file: 'android-icon-monochrome.png',
    size: 1024,
    // One color, read as alpha: the shell with the screen, nameplate and keys cut out.
    svg: doc(
      fit(
        0.74,
        `<mask id="cut"><path d="${shell}" fill="#fff" />` +
          '<rect x="248" y="300" width="528" height="268" rx="36" fill="#000" />' +
          '<rect x="248" y="600" width="336" height="172" rx="28" fill="#000" />' +
          '<rect x="616" y="600" width="160" height="72" rx="36" fill="#000" />' +
          '<rect x="616" y="700" width="160" height="72" rx="36" fill="#000" /></mask>' +
          '<rect width="1024" height="1024" fill="#000" mask="url(#cut)" />',
      ),
    ),
  },
  { file: 'splash-icon.png', size: 1024, svg: doc(fit(0.9, device)) },
  { file: 'favicon.png', size: 48, svg },
];

const browser = await chromium.launch();
for (const icon of icons) {
  const page = await browser.newPage({ viewport: { width: 1024, height: 1024 }, deviceScaleFactor: icon.size / 1024 });
  await page.setContent(`<style>html,body{margin:0;background:transparent}svg{display:block}</style>${icon.svg}`);
  await page.evaluate(async () => {
    await Promise.all(["30px Michroma", "11px 'Space Mono'", "bold 11px 'Space Mono'"].map((f) => document.fonts.load(f)));
    await document.fonts.ready;
  });
  await page.waitForTimeout(300);
  await page.screenshot({ path: resolve(out, icon.file), omitBackground: true, clip: { x: 0, y: 0, width: 1024, height: 1024 } });
  await page.close();
  console.log(`${out}/${icon.file} (${icon.size}px)`);
}
await browser.close();
