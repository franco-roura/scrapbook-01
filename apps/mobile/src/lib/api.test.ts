import { describe, expect, test } from 'bun:test';

import { ApiError, createApi } from './api';

type Call = { url: string; init: RequestInit };

function fakeFetch(status: number, body: unknown, calls: Call[]) {
  return (async (url: string, init: RequestInit) => {
    calls.push({ url, init });
    return new Response(status === 204 ? null : JSON.stringify(body), { status });
  }) as unknown as typeof fetch;
}

describe('createApi', () => {
  test('sends the bearer token and trims the description', async () => {
    const calls: Call[] = [];
    const api = createApi({
      baseUrl: 'https://api.test',
      getToken: async () => 'tok',
      fetch: fakeFetch(201, { id: '1' }, calls),
    });
    await api.saveItem('Look https://x.com/a', '  a thread  ');
    expect(calls[0].url).toBe('https://api.test/v1/items');
    expect((calls[0].init.headers as Record<string, string>).Authorization).toBe('Bearer tok');
    expect(JSON.parse(calls[0].init.body as string)).toEqual({ url: 'Look https://x.com/a', description: 'a thread' });
  });

  test('omits a blank description and sends null to clear one', async () => {
    const calls: Call[] = [];
    const api = createApi({ baseUrl: 'https://api.test', getToken: async () => 't', fetch: fakeFetch(200, {}, calls) });
    await api.saveItem('https://x.com/a', '   ');
    await api.describeItem('abc', ' ');
    expect(JSON.parse(calls[0].init.body as string)).toEqual({ url: 'https://x.com/a' });
    expect(JSON.parse(calls[1].init.body as string)).toEqual({ description: null });
    expect(calls[1].init.method).toBe('PATCH');
  });

  test('builds list query strings', async () => {
    const calls: Call[] = [];
    const api = createApi({ baseUrl: 'https://api.test', getToken: async () => 't', fetch: fakeFetch(200, { items: [], nextCursor: null }, calls) });
    await api.listItems();
    await api.listItems({ undescribed: true, cursor: 'abc', limit: 20 });
    expect(calls[0].url).toBe('https://api.test/v1/items');
    expect(calls[1].url).toBe('https://api.test/v1/items?limit=20&cursor=abc&filter=undescribed');
  });

  test('turns server errors into ApiError', async () => {
    const api = createApi({
      baseUrl: 'https://api.test',
      getToken: async () => 't',
      fetch: fakeFetch(422, { error: { code: 'invalid_url', message: "That doesn't look like a link." } }, []),
    });
    const err = await api.saveItem('nope').catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(422);
    expect(err.code).toBe('invalid_url');
  });

  test('fails fast without a token or network', async () => {
    const noToken = createApi({ baseUrl: 'https://api.test', getToken: async () => null, fetch: fakeFetch(200, {}, []) });
    expect((await noToken.listItems().catch((e) => e)).status).toBe(401);

    const offline = createApi({
      baseUrl: 'https://api.test',
      getToken: async () => 't',
      fetch: (async () => {
        throw new TypeError('Network request failed');
      }) as unknown as typeof fetch,
    });
    expect((await offline.listItems().catch((e) => e)).code).toBe('network');
  });

  test('handles 204 on delete', async () => {
    const api = createApi({ baseUrl: 'https://api.test', getToken: async () => 't', fetch: fakeFetch(204, null, []) });
    expect(await api.deleteItem('x')).toBeUndefined();
  });
});
