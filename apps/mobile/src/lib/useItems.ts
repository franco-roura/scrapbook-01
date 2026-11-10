import { useCallback, useEffect, useRef, useState } from 'react';

import { ApiError, type Item, type ItemPage } from './api';
import { api } from './client';

const message = (e: unknown) => (e instanceof ApiError ? e.message : 'Something went wrong.');

/** Paged item list with refresh and load-more. Remount (key) to switch lists. */
export function useItems(undescribed: boolean) {
  const [items, setItems] = useState<Item[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Only the latest request may update state; older responses are dropped.
  const latest = useRef(0);

  const fetchPage = useCallback(
    (after: string | null, apply: (page: ItemPage) => void) => {
      const id = ++latest.current;
      return api
        .listItems({ undescribed, cursor: after })
        .then((page) => {
          if (id !== latest.current) return;
          apply(page);
          setCursor(page.nextCursor);
          setError(null);
        })
        .catch((e) => {
          if (id === latest.current) setError(message(e));
        })
        .finally(() => {
          if (id !== latest.current) return;
          setLoading(false);
          setRefreshing(false);
        });
    },
    [undescribed],
  );

  useEffect(() => {
    fetchPage(null, (page) => setItems(page.items));
  }, [fetchPage]);

  const refresh = useCallback(() => {
    setRefreshing(true);
    return fetchPage(null, (page) => setItems(page.items));
  }, [fetchPage]);

  const loadMore = useCallback(() => {
    if (!cursor || loading || refreshing) return;
    return fetchPage(cursor, (page) => setItems((prev) => [...prev, ...page.items]));
  }, [cursor, loading, refreshing, fetchPage]);

  return { items, loading, refreshing, error, hasMore: cursor !== null, refresh, loadMore };
}
