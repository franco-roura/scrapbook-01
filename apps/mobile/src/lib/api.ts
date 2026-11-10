// Client for the Go API in /server. Kept free of React Native imports so it
// can be unit-tested with `bun test`.

export type Category = { id: string; name: string };

export type Item = {
  id: string;
  url: string;
  platform: string;
  description: string | null;
  category: Category | null;
  previewTitle: string | null;
  previewImage: string | null;
  createdAt: string;
  updatedAt: string;
};

export type ItemPage = { items: Item[]; nextCursor: string | null };

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type Options = {
  baseUrl: string;
  getToken: () => Promise<string | null>;
  fetch?: typeof fetch;
};

export function createApi({ baseUrl, getToken, fetch: f = fetch }: Options) {
  async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const token = await getToken();
    if (!token) throw new ApiError(401, 'unauthorized', 'Sign in again.');

    let res: Response;
    try {
      res = await f(`${baseUrl}${path}`, {
        method,
        headers: {
          Authorization: `Bearer ${token}`,
          ...(body === undefined ? {} : { 'Content-Type': 'application/json' }),
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw new ApiError(0, 'network', "Can't reach Scrapbook/01. Check your connection.");
    }

    if (res.status === 204) return undefined as T;
    const data = await res.json().catch(() => null);
    if (!res.ok) {
      const err = data?.error;
      throw new ApiError(res.status, err?.code ?? 'unknown', err?.message ?? 'Something went wrong.');
    }
    return data as T;
  }

  return {
    /** Saves a link. `url` may be raw share-sheet text containing a link. */
    saveItem: (url: string, description?: string) =>
      request<Item>('POST', '/v1/items', { url, description: description?.trim() || undefined }),

    listItems: (opts: { cursor?: string | null; undescribed?: boolean; limit?: number } = {}) => {
      const q = new URLSearchParams();
      if (opts.limit) q.set('limit', String(opts.limit));
      if (opts.cursor) q.set('cursor', opts.cursor);
      if (opts.undescribed) q.set('filter', 'undescribed');
      const qs = q.toString();
      return request<ItemPage>('GET', `/v1/items${qs ? `?${qs}` : ''}`);
    },

    getItem: (id: string) => request<Item>('GET', `/v1/items/${encodeURIComponent(id)}`),

    /** Sets the description; an empty string moves the item back to the inbox. */
    describeItem: (id: string, description: string) =>
      request<Item>('PATCH', `/v1/items/${encodeURIComponent(id)}`, {
        description: description.trim() || null,
      }),

    deleteItem: (id: string) => request<void>('DELETE', `/v1/items/${encodeURIComponent(id)}`),
  };
}

export type Api = ReturnType<typeof createApi>;
