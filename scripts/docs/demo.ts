// Starts a throwaway demo of Scrapbook/01 for the docs screenshots:
//   - a fake AI server on :8788 (server/cmd/fakeai),
//   - the API on :18080 with its own database (scrapbook_demo, reset each run),
//     accepting dev tokens instead of WorkOS,
//   - the web app, exported and served on :8090.
// It doesn't touch the database `go run ./cmd/api` uses. Needs the Docker
// Postgres from server/docker-compose.yml. Stop it with Ctrl+C.
//
// Usage: bun run docs:demo   (then, in another terminal: bun run docs:seed && bun run docs:screenshots)
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

import { DEMO_SECRET } from './token';

const root = resolve(import.meta.dir, '../..');
const server = join(root, 'server');
const work = mkdtempSync(join(tmpdir(), 'scrapbook-demo-'));
const exe = process.platform === 'win32' ? '.exe' : '';

async function run(cmd: string[], cwd: string, env: Record<string, string> = {}) {
  const p = Bun.spawn(cmd, { cwd, env: { ...process.env, ...env }, stdout: 'inherit', stderr: 'inherit' });
  if ((await p.exited) !== 0) throw new Error(`failed: ${cmd.join(' ')}`);
}

console.log('resetting the scrapbook_demo database…');
await run(
  ['docker', 'compose', 'exec', '-T', 'db', 'psql', '-U', 'scrapbook', '-d', 'scrapbook', '-q',
    '-c', 'DROP DATABASE IF EXISTS scrapbook_demo', '-c', 'CREATE DATABASE scrapbook_demo'],
  server,
);

console.log('building the API and the fake AI…');
await run(['go', 'build', '-o', join(work, `api${exe}`), './cmd/api'], server);
await run(['go', 'build', '-o', join(work, `fakeai${exe}`), './cmd/fakeai'], server);

console.log('exporting the web app…');
await run(['bun', 'x', 'expo', 'export', '--platform', 'web', '--output-dir', join(work, 'web')], join(root, 'apps/mobile'), {
  EXPO_PUBLIC_API_URL: 'http://localhost:18080',
  EXPO_PUBLIC_WORKOS_CLIENT_ID: 'client_docs_demo',
});

const fake = 'http://127.0.0.1:8788';
// Run from the temp dir, so neither binary reads server/.env.
const procs = [
  Bun.spawn([join(work, `fakeai${exe}`)], { cwd: work, env: { ...process.env, FAKEAI_ADDR: '127.0.0.1:8788' }, stdout: 'inherit', stderr: 'inherit' }),
  Bun.spawn([join(work, `api${exe}`)], {
    cwd: work,
    stdout: 'ignore',
    stderr: 'inherit',
    env: {
      ...process.env,
      ADDR: ':18080',
      DATABASE_URL: 'postgres://scrapbook:scrapbook@localhost:54329/scrapbook_demo?sslmode=disable',
      DEV_JWT_SECRET: DEMO_SECRET,
      PUBLIC_URL: 'http://localhost:18080',
      CORS_ORIGINS: 'http://localhost:8090',
      TYPESAFE_API_KEY: 'dev', TYPESAFE_BASE_URL: fake,
      EMBEDDINGS_API_KEY: 'dev', EMBEDDINGS_BASE_URL: fake,
      ANTHROPIC_API_KEY: 'dev', ANTHROPIC_BASE_URL: fake,
      STARTER_CREDITS_USD: '0.25',
      GRAPH_MIN_SIMILARITY: '0.2',
    },
  }),
  Bun.spawn(['bun', join(import.meta.dir, 'serve.ts'), join(work, 'web'), '8090'], { stdout: 'inherit', stderr: 'inherit' }),
];

process.on('SIGINT', () => {
  for (const p of procs) p.kill();
  process.exit(0);
});
console.log('demo: API http://localhost:18080 · web http://localhost:8090 · Ctrl+C to stop');
await Promise.race(procs.map((p) => p.exited));
for (const p of procs) p.kill();
