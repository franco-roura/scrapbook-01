// Optional Sentry. With no VITE_SENTRY_DSN (self-hosting, local dev) nothing here loads the SDK:
// the SDK is only reached through the dynamic import of `./sentry-sdk` in `initSentry`, so it ends
// up in its own chunk that the page never fetches. Every helper below is a no-op until that import finishes.
//
// Privacy: no user info, cookies, headers, bodies or query strings; the waitlist email is never
// attached to anything. Replay and profiling are off (their integrations aren't added).

import type * as SentryModule from "./sentry-sdk";
import type { Breadcrumb, BrowserOptions, ErrorEvent } from "@sentry/react";

type Sentry = typeof SentryModule;
type StreamedSpanJSON = Parameters<NonNullable<BrowserOptions["beforeSendSpan"]>>[0];

let sentry: Sentry | null = null;

export type SentryEnv = {
  VITE_SENTRY_DSN?: string;
  VITE_SENTRY_TRACES_SAMPLE_RATE?: string;
  VITE_API_URL?: string;
  MODE?: string;
};

/** Loads and starts Sentry when a DSN is configured. Resolves to whether it did. */
export async function initSentry(
  env: SentryEnv = import.meta.env,
  load: () => Promise<Sentry> = () => import("./sentry-sdk"),
): Promise<boolean> {
  const dsn = env.VITE_SENTRY_DSN?.trim();
  if (!dsn) return false;
  const Sentry = await load();
  const apiUrl = (env.VITE_API_URL ?? "").replace(/\/$/, "");
  Sentry.init({
    dsn,
    environment: env.MODE,
    integrations: [Sentry.browserTracingIntegration()],
    tracesSampleRate: sampleRate(env.VITE_SENTRY_TRACES_SAMPLE_RATE),
    // Only our API gets trace headers; an empty string would match every URL.
    tracePropagationTargets: apiUrl ? [apiUrl] : [],
    // v11 replaced `sendDefaultPii` with per-category switches. Collect none of it.
    dataCollection: {
      userInfo: false,
      cookies: false,
      httpHeaders: false,
      httpBodies: [],
      urlQueryParams: false,
      stackFrameVariables: false,
    },
    beforeSend: scrubEvent,
    beforeBreadcrumb: scrubBreadcrumb,
    beforeSendSpan: scrubSpan,
  });
  sentry = Sentry;
  return true;
}

/** Reports an unexpected error. No-op without Sentry. */
export function captureError(error: unknown): void {
  sentry?.captureException(error);
}

/** Runs `fn` inside a span. Without Sentry it just runs `fn`. */
export function withSpan<T>(name: string, op: string, fn: () => Promise<T>): Promise<T> {
  if (!sentry) return fn();
  return sentry.startSpan({ name, op }, fn);
}

export function sampleRate(raw: string | undefined): number {
  if (raw === undefined || raw.trim() === "") return 1;
  const n = Number(raw);
  return Number.isFinite(n) && n >= 0 && n <= 1 ? n : 1;
}

/** Drops the query string and fragment from a URL-ish string. */
export function stripQuery(url: string): string {
  return url.replace(/[?#].*$/, "");
}

export function scrubEvent(event: ErrorEvent): ErrorEvent {
  delete event.user;
  if (event.request) {
    delete event.request.data;
    delete event.request.cookies;
    delete event.request.headers;
    delete event.request.query_string;
    if (event.request.url) event.request.url = stripQuery(event.request.url);
  }
  return event;
}

export function scrubBreadcrumb(crumb: Breadcrumb): Breadcrumb {
  const data = crumb.data;
  if (data) {
    for (const key of ["url", "from", "to"]) {
      if (typeof data[key] === "string") data[key] = stripQuery(data[key]);
    }
    delete data.body;
    delete data.request_body;
    delete data.response_body;
  }
  if (crumb.category === "navigation" && crumb.message) crumb.message = stripQuery(crumb.message);
  return crumb;
}

const queryAttributes = new Set(["url.query", "http.query", "http.fragment", "url.fragment"]);

export function scrubSpan(span: StreamedSpanJSON): StreamedSpanJSON {
  span.name = stripUrlsIn(span.name);
  for (const key of Object.keys(span.attributes)) {
    if (queryAttributes.has(key) || key.startsWith("user.")) {
      delete span.attributes[key];
      continue;
    }
    const value = span.attributes[key];
    if (typeof value === "string" && /^(https?:)?\/\//.test(value)) {
      span.attributes[key] = stripQuery(value);
    }
  }
  return span;
}

/** Strips query strings from any URLs inside a span name like "POST https://api/x?y=1". */
function stripUrlsIn(text: string): string {
  return text.replace(/(\S+?)[?#]\S*/g, "$1");
}

/** For tests: forget the loaded SDK. */
export function resetSentryForTests(): void {
  sentry = null;
}
