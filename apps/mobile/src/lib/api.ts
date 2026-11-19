import type { GraphData } from '../graph/types';

// Client for the Go API in /server. Kept free of React Native imports so it
// can be unit-tested with `bun test`.

export type Category = { id: string; name: string };

export type Item = {
  id: string;
  url: string;
  platform: string;
  description: string | null;
  category: Category | null;
  /** "auto" while Scrapbook/01 picks the category, "user" once you chose one. */
  categorySource: 'auto' | 'user';
  /** True while a new description is waiting for its category. */
  classifying: boolean;
  /** Waiting only because the credits ran out; sorted after a top-up. */
  waitingForCredits: boolean;
  /** The link's own preview (Open Graph / oEmbed), for display only. All
   * null until fetched, and for links that publish none (or hide behind a
   * login wall, like Instagram). */
  previewTitle: string | null;
  previewDescription: string | null;
  previewSite: string | null;
  /** Our own copy of the preview image (platform image links expire). */
  previewImage: string | null;
  createdAt: string;
  updatedAt: string;
};

export type ItemPage = { items: Item[]; nextCursor: string | null };

export type Credits = {
  /** Model budget left, in dollars. */
  balanceUsd: number;
  /** Rough number of saves the balance can still sort. */
  savesLeft: number;
  /** Share of a pack's price that becomes model budget (0.8). */
  modelShare: number;
  recent: { amountUsd: number; kind: 'starter' | 'grant' | 'purchase' | 'usage'; step?: string; itemId?: string; createdAt: string }[];
};

export type CategorySummary = Category & { itemCount: number; createdAt: string };

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
    if (!token) throw new ApiError(401, 'unauthorized', 'Volvé a entrar.');

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
      throw new ApiError(0, 'network', 'No llegamos a Scrapbook/01. Revisá tu conexión.');
    }

    if (res.status === 204) return undefined as T;
    const data = await res.json().catch(() => null);
    if (!res.ok) {
      const err = data?.error;
      throw new ApiError(res.status, err?.code ?? 'unknown', err?.message ?? 'Algo salió mal.');
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

    /** Moves an item to a category you chose (null: no category). */
    setItemCategory: (id: string, categoryId: string | null) =>
      request<Item>('PATCH', `/v1/items/${encodeURIComponent(id)}`, { categoryId }),

    listCategories: () => request<{ categories: CategorySummary[] }>('GET', '/v1/categories').then((r) => r.categories),

    /** Creates a category, or returns the existing one with that name. */
    createCategory: (name: string) => request<Category>('POST', '/v1/categories', { name: name.trim() }),

    registerDevice: (token: string, platform: 'ios' | 'android') =>
      request<void>('PUT', `/v1/devices/${encodeURIComponent(token)}`, { platform }),

    getCredits: () => request<Credits>('GET', '/v1/credits'),

    /** Every item, category hub and link, for the graph view. */
    getGraph: () => request<GraphData>('GET', '/v1/graph'),

    /** The local graph around one item, 1 or 2 hops out. */
    getNeighbors: (id: string, depth: 1 | 2) =>
      request<GraphData>('GET', `/v1/items/${encodeURIComponent(id)}/neighbors?depth=${depth}`),

    unregisterDevice: (token: string) => request<void>('DELETE', `/v1/devices/${encodeURIComponent(token)}`),
  };
}

export type Api = ReturnType<typeof createApi>;
