import { createHash } from 'node:crypto';

import { beforeEach, describe, expect, mock, test } from 'bun:test';

const init = mock(() => undefined);
const setUser = mock(() => undefined);
const fakeConfig = { apiUrl: 'https://api.test', sentryDsn: '', sentryTracesSampleRate: 1 };

mock.module('@sentry/react-native', () => ({
  init,
  setUser,
  expoRouterIntegration: () => ({ name: 'ExpoRouter' }),
}));
mock.module('expo-crypto', () => ({ CryptoDigestAlgorithm: { SHA1: 'SHA-1' }, digest: async () => new ArrayBuffer(20) }));
mock.module('./config', () => ({ config: fakeConfig }));

const { initSentry, scrubBreadcrumb, scrubEvent, setSentryUser, userIdFor } = await import('./sentry');

const sha1 = async (data: Uint8Array) => {
  const b = createHash('sha1').update(data).digest();
  return b.buffer.slice(b.byteOffset, b.byteOffset + b.byteLength) as ArrayBuffer;
};

describe('initSentry', () => {
  beforeEach(() => {
    init.mockClear();
    setUser.mockClear();
  });

  test('does nothing without a DSN', async () => {
    fakeConfig.sentryDsn = '';
    expect(initSentry({ surface: 'app' })).toBe(false);
    expect(initSentry({ surface: 'share-extension' })).toBe(false);
    await setSentryUser('user_01JTESTEXAMPLE000000000000');
    expect(init).not.toHaveBeenCalled();
    expect(setUser).not.toHaveBeenCalled();
  });

  test('starts once with a DSN, without PII', () => {
    fakeConfig.sentryDsn = 'https://key@o0.ingest.sentry.io/0';
    expect(initSentry({ surface: 'web' })).toBe(true);
    expect(initSentry({ surface: 'web' })).toBe(true);
    expect(init).toHaveBeenCalledTimes(1);
    const opts = (init.mock.calls[0] as unknown[])[0] as Record<string, unknown>;
    expect(opts.sendDefaultPii).toBe(false);
    expect(opts.initialScope).toEqual({ tags: { surface: 'web' } });
    const [target] = opts.tracePropagationTargets as RegExp[];
    expect(target.test('https://api.test/v1/items')).toBe(true);
    expect(target.test('https://api.workos.com/user_management/authenticate')).toBe(false);
    expect(target.test('https://api.test.evil.com/')).toBe(false);
  });
});

describe('userIdFor', () => {
  test('matches the server for WorkOS ids', async () => {
    // From uuid.NewSHA1(namespace, sub) in server/internal/auth.
    expect(await userIdFor('user_01JTESTEXAMPLE000000000000', sha1)).toBe('d0833793-d442-5053-bd8d-2f1d35eca13b');
  });

  test('keeps uuid subjects', async () => {
    expect(await userIdFor('0B7A9C2E-1111-4222-8333-444455556666', sha1)).toBe('0b7a9c2e-1111-4222-8333-444455556666');
  });
});

describe('scrubbing', () => {
  test('drops bodies, query strings, auth headers and user details', () => {
    const event = scrubEvent({
      request: {
        url: 'https://api.test/v1/items?q=secret',
        data: '{"description":"private"}',
        query_string: 'q=secret',
        headers: { Authorization: 'Bearer tok', Accept: 'application/json' },
      },
      user: { id: 'abc', email: 'someone@example.com', ip_address: '10.0.0.1' },
      breadcrumbs: [
        { category: 'console', message: 'saving private' },
        { category: 'fetch', data: { url: 'https://api.test/v1/items?cursor=x', method: 'GET', body: 'x' } },
      ],
    });
    expect(event.request).toEqual({ url: 'https://api.test/v1/items', headers: { Accept: 'application/json' } });
    expect(event.user).toEqual({ id: 'abc' });
    expect(event.breadcrumbs).toEqual([{ category: 'fetch', data: { url: 'https://api.test/v1/items', method: 'GET' } }]);
  });

  test('strips query strings from breadcrumb messages', () => {
    expect(scrubBreadcrumb({ message: 'GET https://x.test/a?token=1 failed' })?.message).toBe('GET https://x.test/a failed');
  });
});
