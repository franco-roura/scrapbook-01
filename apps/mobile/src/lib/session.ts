// Sign-in with WorkOS AuthKit, as a public client (no secret on the device):
// the hosted AuthKit page handles the email code, PKCE protects the code
// exchange, and the app keeps the refresh token.
//
// The session lives in the iOS keychain under the shared app group, so the
// share extension (a separate process) is signed in whenever the app is and
// can refresh the token itself with one plain HTTP call. Sign-in itself is
// in signIn.ts, which the share extension doesn't load.
import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

import { config } from './config';

export type Session = {
  accessToken: string;
  refreshToken: string;
  /** Milliseconds since the epoch. */
  expiresAt: number;
  user: { id: string; email: string };
};

const KEY = 'scrapbook01_session';

const keychainOptions: SecureStore.SecureStoreOptions = {
  accessGroup: Platform.OS === 'ios' ? config.iosAppGroup : undefined,
  keychainAccessible: SecureStore.AFTER_FIRST_UNLOCK,
};

// Web is for previews and the PWA; it keeps the session in localStorage.
const storage = {
  get: async (): Promise<string | null> =>
    Platform.OS === 'web'
      ? typeof localStorage === 'undefined'
        ? null
        : localStorage.getItem(KEY)
      : SecureStore.getItemAsync(KEY, keychainOptions),
  set: async (v: string) =>
    Platform.OS === 'web' ? localStorage.setItem(KEY, v) : SecureStore.setItemAsync(KEY, v, keychainOptions),
  remove: async () =>
    Platform.OS === 'web' ? localStorage.removeItem(KEY) : SecureStore.deleteItemAsync(KEY, keychainOptions),
};

// --- current session and listeners -----------------------------------------

let current: Session | null | undefined; // undefined: not loaded yet
const listeners = new Set<(s: Session | null) => void>();

export function onSessionChange(fn: (s: Session | null) => void): () => void {
  listeners.add(fn);
  return () => listeners.delete(fn);
}

export async function save(s: Session | null) {
  current = s;
  if (s) await storage.set(JSON.stringify(s));
  else await storage.remove();
  listeners.forEach((fn) => fn(s));
}

async function readStored(): Promise<Session | null> {
  try {
    const raw = await storage.get();
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

/** The stored session, without refreshing it. */
export async function getSession(): Promise<Session | null> {
  if (current === undefined) current = await readStored();
  return current;
}

// --- talking to WorkOS ------------------------------------------------------

type AuthResponse = {
  access_token: string;
  refresh_token: string;
  user: { id: string; email: string };
};

class AuthError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

export async function authenticate(body: Record<string, string>): Promise<Session> {
  const res = await fetch(`${config.workosApiUrl}/user_management/authenticate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify({ client_id: config.workosClientId, ...body }),
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new AuthError(json.error_description ?? json.message ?? `No se pudo entrar (${res.status}).`, res.status);
  }
  const r = json as AuthResponse;
  return {
    accessToken: r.access_token,
    refreshToken: r.refresh_token,
    expiresAt: expiryOf(r.access_token),
    user: { id: r.user.id, email: r.user.email },
  };
}

function claims(jwt: string): Record<string, unknown> {
  try {
    const part = jwt.split('.')[1].replace(/-/g, '+').replace(/_/g, '/');
    return JSON.parse(atob(part.padEnd(part.length + ((4 - (part.length % 4)) % 4), '=')));
  } catch {
    return {};
  }
}

function expiryOf(jwt: string): number {
  const exp = claims(jwt).exp;
  return typeof exp === 'number' ? exp * 1000 : Date.now() + 60_000;
}

// --- tokens -----------------------------------------------------------------

let refreshing: Promise<Session | null> | null = null;

async function refresh(s: Session): Promise<Session | null> {
  try {
    const next = await authenticate({ grant_type: 'refresh_token', refresh_token: s.refreshToken });
    await save({ ...next, user: next.user ?? s.user });
    return current ?? null;
  } catch (e) {
    // Refresh tokens are single use. The app and the share extension share
    // one session, so the other one may have just used it: take its result.
    const stored = await readStored();
    if (stored && stored.refreshToken !== s.refreshToken) {
      current = stored;
      return stored;
    }
    // Rejected for good (expired, revoked): sign out. Network trouble keeps
    // the session for the next try.
    if (e instanceof AuthError && e.status >= 400 && e.status < 500 && e.status !== 408 && e.status !== 429) {
      await save(null);
    }
    return null;
  }
}

/** A valid access token, refreshed when it's within 30 s of expiring. */
export async function accessToken(): Promise<string | null> {
  // The other process may have rotated the tokens, so re-read storage.
  const s = await readStored();
  current = s;
  if (!s) return null;
  if (s.expiresAt - Date.now() > 30_000) return s.accessToken;
  refreshing ??= refresh(s).finally(() => (refreshing = null));
  return (await refreshing)?.accessToken ?? null;
}

// --- sign out ---------------------------------------------------------------

/** Forgets the session here and ends it at WorkOS. */
export async function signOutSession(): Promise<void> {
  const s = await getSession();
  await save(null);
  const sid = s ? claims(s.accessToken).sid : undefined;
  if (typeof sid !== 'string') return;
  // Ending the WorkOS session means the next sign-in asks for an email again.
  // The response is a redirect to the sign-out page, which we don't need.
  // (Browsers only allow an opaque "no-cors" request here, which must follow
  // redirects.)
  const init: RequestInit = Platform.OS === 'web' ? { mode: 'no-cors' } : { redirect: 'manual' };
  await fetch(`${config.workosApiUrl}/user_management/sessions/logout?session_id=${encodeURIComponent(sid)}`, init).catch(
    () => undefined,
  );
}
