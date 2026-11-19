// Copies the root package.json version (which Changesets bumps) to the apps,
// so the API, the app and the website always ship under one version.
// Run by `bun run release:version`.
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const root = join(import.meta.dir, '..', '..');
const read = (path: string) => JSON.parse(readFileSync(join(root, path), 'utf8'));
const write = (path: string, data: unknown) => writeFileSync(join(root, path), `${JSON.stringify(data, null, 2)}\n`);

const { version } = read('package.json');
if (!/^\d+\.\d+\.\d+$/.test(version)) throw new Error(`unexpected root version: ${version}`);

for (const path of ['apps/mobile/package.json', 'apps/website/package.json']) {
  const pkg = read(path);
  pkg.version = version;
  write(path, pkg);
}

// The store version users see. EAS manages the build number separately.
const app = read('apps/mobile/app.json');
app.expo.version = version;
write('apps/mobile/app.json', app);

console.log(`apps synced to ${version}`);
