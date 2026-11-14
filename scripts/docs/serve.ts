// Serves an Expo web export as a single-page app (unknown paths get
// index.html), like CloudFront does in production. For docs screenshots.
// Usage: bun scripts/docs/serve.ts <dir> [port]
import { join, normalize } from 'node:path';

const [dir, port = '8090'] = process.argv.slice(2);
if (!dir) throw new Error('usage: serve.ts <dir> [port]');

Bun.serve({
  port: Number(port),
  async fetch(req) {
    const path = normalize(decodeURIComponent(new URL(req.url).pathname)).replace(/^[/\\]+/, '');
    if (path.startsWith('..')) return new Response('Not found', { status: 404 });
    const file = Bun.file(join(dir, path));
    if (path && (await file.exists())) return new Response(file);
    return new Response(Bun.file(join(dir, 'index.html')), { headers: { 'Content-Type': 'text/html' } });
  },
});
console.log(`serving ${dir} on http://localhost:${port}`);
