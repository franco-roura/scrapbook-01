import { afterEach, describe, expect, test, vi } from "vite-plus/test";

import {
  captureError,
  initSentry,
  resetSentryForTests,
  sampleRate,
  scrubBreadcrumb,
  scrubEvent,
  scrubSpan,
  withSpan,
} from "./sentry";

afterEach(() => resetSentryForTests());

describe("without a DSN", () => {
  test("never loads the SDK", async () => {
    const load = vi.fn();
    expect(await initSentry({ VITE_SENTRY_DSN: "" }, load)).toBe(false);
    expect(await initSentry({}, load)).toBe(false);
    expect(load).not.toHaveBeenCalled();
  });

  test("captureError and withSpan are no-ops", async () => {
    expect(() => captureError(new Error("boom"))).not.toThrow();
    await expect(withSpan("waitlist.submit", "ui.action.submit", async () => 42)).resolves.toBe(42);
    await expect(
      withSpan("waitlist.submit", "ui.action.submit", async () => {
        throw new Error("nope");
      }),
    ).rejects.toThrow("nope");
  });
});

describe("with a DSN", () => {
  test("inits once with no PII, and forwards captures and spans", async () => {
    const init = vi.fn();
    const captureException = vi.fn();
    const startSpan = vi.fn((_o: unknown, fn: () => unknown) => fn());
    const fake = {
      init,
      captureException,
      startSpan,
      browserTracingIntegration: () => ({ name: "BrowserTracing" }),
    };
    const ok = await initSentry(
      {
        VITE_SENTRY_DSN: "https://key@o0.ingest.example.test/1",
        VITE_SENTRY_TRACES_SAMPLE_RATE: "0.25",
        VITE_API_URL: "https://api.example.test/",
      },
      async () => fake as never,
    );
    expect(ok).toBe(true);
    const opts = init.mock.calls[0][0];
    expect(opts.tracesSampleRate).toBe(0.25);
    expect(opts.tracePropagationTargets).toEqual(["https://api.example.test"]);
    expect(opts.dataCollection).toMatchObject({
      userInfo: false,
      cookies: false,
      httpHeaders: false,
      httpBodies: [],
      urlQueryParams: false,
    });

    captureError(new Error("boom"));
    expect(captureException).toHaveBeenCalledTimes(1);
    await expect(withSpan("waitlist.submit", "ui.action.submit", async () => 1)).resolves.toBe(1);
    expect(startSpan.mock.calls[0][0]).toEqual({ name: "waitlist.submit", op: "ui.action.submit" });
  });

  test("propagates traces nowhere when there's no API URL", async () => {
    const init = vi.fn();
    await initSentry(
      { VITE_SENTRY_DSN: "https://key@o0.ingest.example.test/1" },
      async () => ({ init, browserTracingIntegration: () => ({}) }) as never,
    );
    expect(init.mock.calls[0][0].tracePropagationTargets).toEqual([]);
  });
});

describe("scrubbing", () => {
  test("sample rate defaults to 1 and rejects junk", () => {
    expect(sampleRate(undefined)).toBe(1);
    expect(sampleRate("")).toBe(1);
    expect(sampleRate("0")).toBe(0);
    expect(sampleRate("2")).toBe(1);
    expect(sampleRate("abc")).toBe(1);
  });

  test("events lose bodies, query strings and the user", () => {
    const out = scrubEvent({
      type: undefined,
      user: { email: "ana@example.com" },
      request: {
        url: "https://scrapbook.example.test/?email=ana%40example.com#x",
        query_string: "email=ana%40example.com",
        data: { email: "ana@example.com" },
        cookies: { a: "b" },
        headers: { Referer: "https://x.example.test/?q=1" },
      },
    });
    expect(out.user).toBeUndefined();
    expect(out.request).toEqual({ url: "https://scrapbook.example.test/" });
  });

  test("breadcrumbs lose query strings and bodies", () => {
    const out = scrubBreadcrumb({
      category: "fetch",
      data: { url: "https://api.example.test/v1/waitlist?x=1", method: "POST", body: "{}" },
    });
    expect(out.data).toEqual({ url: "https://api.example.test/v1/waitlist", method: "POST" });
    const nav = scrubBreadcrumb({ category: "navigation", data: { from: "/?a=1", to: "/#b" } });
    expect(nav.data).toEqual({ from: "/", to: "/" });
  });

  test("spans lose query strings and user attributes", () => {
    const out = scrubSpan({
      trace_id: "t",
      span_id: "s",
      name: "GET https://api.example.test/v1/x?email=ana%40example.com",
      start_timestamp: 0,
      status: "ok",
      is_segment: false,
      attributes: {
        "url.full": "https://api.example.test/v1/x?email=ana%40example.com",
        "url.query": "?email=ana%40example.com",
        "user.email": "ana@example.com",
        "http.method": "GET",
      },
    });
    expect(out.name).toBe("GET https://api.example.test/v1/x");
    expect(out.attributes).toEqual({
      "url.full": "https://api.example.test/v1/x",
      "http.method": "GET",
    });
  });
});
