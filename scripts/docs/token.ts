// A dev token for the demo API (DEV_JWT_SECRET), for docs scripts only.
import { createHmac } from 'node:crypto';

export const DEMO_SECRET = process.env.DEV_JWT_SECRET ?? 'docs-demo-secret-docs-demo-secret-1';

export function devToken(sub = 'user_docs_demo', ttlSeconds = 6 * 3600): string {
  const b = (o: object) => Buffer.from(JSON.stringify(o)).toString('base64url');
  const now = Math.floor(Date.now() / 1000);
  const body = `${b({ alg: 'HS256', typ: 'JWT' })}.${b({ sub, iss: 'scrapbook01-dev', sid: 'session_docs', iat: now, exp: now + ttlSeconds })}`;
  return `${body}.${createHmac('sha256', DEMO_SECRET).update(body).digest('base64url')}`;
}
