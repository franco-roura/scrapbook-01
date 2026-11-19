// Optional Sentry reporting for the app, the iOS share extension and web.
// With no EXPO_PUBLIC_SENTRY_DSN, initSentry does nothing and no event, span
// or session ever leaves the device.
//
// Privacy: the only user field sent is the internal user uuid (the same one
// the server uses). Request bodies, query strings, cookies and auth headers
// are dropped, so saved links and their descriptions stay on the device.
import * as Sentry from '@sentry/react-native';
import { CryptoDigestAlgorithm, digest } from 'expo-crypto';

import { config } from './config';

export type Surface = 'app' | 'share-extension' | 'web';

let enabled = false;
let navigation: ReturnType<typeof Sentry.reactNavigationIntegration> | null = null;

/** Starts Sentry once per process. Returns whether it is on. */
export function initSentry({ surface }: { surface: Surface }): boolean {
  if (!config.sentryDsn) return false;
  if (enabled) return true;
  // One transaction per screen. expoRouterIntegration reuses this one and adds
  // templated route names, but it only waits 5 s for the navigation
  // container; the root layout renders nothing until fonts load, so
  // registerNavigation (from the layout) makes sure it's attached.
  navigation =
    surface === 'share-extension'
      ? null
      : Sentry.reactNavigationIntegration({ enableTimeToInitialDisplay: surface === 'app' });
  Sentry.init({
    dsn: config.sentryDsn,
    debug: config.sentryDebug,
    environment: typeof __DEV__ !== 'undefined' && __DEV__ ? 'development' : 'production',
    sendDefaultPii: false,
    tracesSampleRate: config.sentryTracesSampleRate,
    enableAutoSessionTracking: true,
    // No Session Replay and no profiling: neither integration is added and
    // no sample rate is set for them.
    profilesSampleRate: 0,
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 0,
    // Trace headers go to our API only, never to WorkOS or other hosts.
    tracePropagationTargets: [new RegExp(`^${escapeRegExp(config.apiUrl)}(/|$)`)],
    integrations:
      // The share extension has no router: one screen, manual spans.
      navigation ? [navigation, Sentry.expoRouterIntegration()] : [],
    enableNativeFramesTracking: surface === 'app',
    initialScope: { tags: { surface } },
    beforeSend: (event) => scrubEvent(event),
    beforeSendTransaction: (event) => scrubEvent(event),
    beforeBreadcrumb: (breadcrumb) => scrubBreadcrumb(breadcrumb),
  });
  enabled = true;
  return true;
}

/** Attaches screen tracking to Expo Router's navigation container. */
export function registerNavigation(ref: unknown): void {
  navigation?.registerNavigationContainer(ref);
}

// --- user -------------------------------------------------------------------

let currentSub: string | null = null;

/** Tags later events with the user's internal uuid. Takes the WorkOS user id ("sub"). */
export async function setSentryUser(sub: string): Promise<void> {
  if (!enabled || sub === currentSub) return;
  currentSub = sub;
  const id = await userIdFor(sub);
  // A sign-out or another user may have come in while hashing.
  if (currentSub === sub) Sentry.setUser({ id });
}

export function clearSentryUser(): void {
  if (!enabled) return;
  currentSub = null;
  Sentry.setUser(null);
}

// Mirrors UserIDFor in server/internal/auth/auth.go: a uuid subject is used
// as is, anything else becomes a name-based (v5, SHA-1) uuid.
const NAMESPACE = '6f1c2b8e-3d7a-4f0e-9b65-2a4c8d1e7f30';
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export async function userIdFor(
  sub: string,
  sha1: (data: Uint8Array<ArrayBuffer>) => Promise<ArrayBuffer> = (data) => digest(CryptoDigestAlgorithm.SHA1, data),
): Promise<string> {
  if (UUID_RE.test(sub)) return sub.toLowerCase();
  const ns = NAMESPACE.replace(/-/g, '');
  const name = new TextEncoder().encode(sub);
  const data = new Uint8Array(16 + name.length);
  for (let i = 0; i < 16; i++) data[i] = parseInt(ns.slice(i * 2, i * 2 + 2), 16);
  data.set(name, 16);
  const b = new Uint8Array(await sha1(data)).slice(0, 16);
  b[6] = (b[6] & 0x0f) | 0x50; // version 5
  b[8] = (b[8] & 0x3f) | 0x80; // RFC 4122 variant
  const hex = Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

// --- scrubbing --------------------------------------------------------------

/** Drops the query string and fragment, which can carry the shared link or tokens. */
export function stripQuery(url: string): string {
  const cut = url.search(/[?#]/);
  return cut === -1 ? url : url.slice(0, cut);
}

const SECRET_HEADER = /^(authorization|cookie|set-cookie|x-api-key)$/i;

type Scrubbable = {
  request?: {
    url?: string;
    data?: unknown;
    query_string?: unknown;
    cookies?: unknown;
    headers?: Record<string, string>;
  };
  user?: { id?: string | number } & Record<string, unknown>;
  breadcrumbs?: Sentry.Breadcrumb[];
  spans?: { description?: string; data?: Record<string, unknown> }[];
};

export function scrubEvent<E extends Scrubbable>(event: E): E {
  const req = event.request;
  if (req) {
    delete req.data;
    delete req.query_string;
    delete req.cookies;
    if (req.url) req.url = stripQuery(req.url);
    if (req.headers) {
      for (const k of Object.keys(req.headers)) if (SECRET_HEADER.test(k)) delete req.headers[k];
    }
  }
  // Only the internal id, even if something set more (email, ip, username).
  if (event.user) event.user = event.user.id === undefined ? {} : { id: event.user.id };
  if (event.breadcrumbs) {
    event.breadcrumbs = event.breadcrumbs.map(scrubBreadcrumb).filter((b): b is Sentry.Breadcrumb => b !== null);
  }
  for (const span of event.spans ?? []) {
    if (span.description) span.description = scrubUrlsIn(span.description);
    if (span.data) scrubData(span.data);
  }
  return event;
}

export function scrubBreadcrumb(breadcrumb: Sentry.Breadcrumb): Sentry.Breadcrumb | null {
  // Console output can contain whatever was being logged, descriptions included.
  if (breadcrumb.category === 'console') return null;
  const b = { ...breadcrumb, data: breadcrumb.data ? { ...breadcrumb.data } : undefined };
  if (b.data) scrubData(b.data);
  if (b.message) b.message = scrubUrlsIn(b.message);
  return b;
}

const URL_KEYS = ['url', 'http.url', 'url.full', 'from', 'to'];
const BODY_KEYS = ['body', 'request_body', 'response_body', 'requestBody', 'responseBody', 'http.query', 'url.query'];

function scrubData(data: Record<string, unknown>) {
  for (const k of BODY_KEYS) delete data[k];
  for (const k of URL_KEYS) if (typeof data[k] === 'string') data[k] = stripQuery(data[k] as string);
}

function scrubUrlsIn(text: string): string {
  return text.replace(/https?:\/\/[^\s?#]*[?#]\S*/g, (u) => stripQuery(u));
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
