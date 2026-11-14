// Opens WorkOS AuthKit's hosted sign-in. Kept apart from session.ts so the
// iOS share extension (tight memory budget) never loads the browser and
// PKCE modules.
import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { useCallback } from 'react';

import { config } from './config';
import { authenticate, save, type Session } from './session';

WebBrowser.maybeCompleteAuthSession();

export const redirectUri = AuthSession.makeRedirectUri({ scheme: 'scrapbook01', path: 'auth/callback' });

const discovery: AuthSession.DiscoveryDocument = {
  authorizationEndpoint: `${config.workosApiUrl}/user_management/authorize`,
};

/**
 * Prepares the sign-in request (PKCE) as soon as the screen mounts, so the
 * tap opens the browser right away (mobile browsers block popups opened
 * late). `start` resolves with the session, or null when the person closes
 * the sheet.
 */
export function useSignIn(): { ready: boolean; start: () => Promise<Session | null> } {
  const [request, , promptAsync] = AuthSession.useAuthRequest(
    {
      clientId: config.workosClientId,
      redirectUri,
      responseType: AuthSession.ResponseType.Code,
      usePKCE: true,
      scopes: [],
      extraParams: { provider: 'authkit' },
    },
    discovery,
  );

  const start = useCallback(async () => {
    if (!request) return null;
    const result = await promptAsync();
    if (result.type === 'error') throw new Error(result.error?.message ?? 'No se pudo entrar.');
    if (result.type !== 'success') return null;
    const session = await authenticate({
      grant_type: 'authorization_code',
      code: result.params.code,
      code_verifier: request.codeVerifier ?? '',
    });
    await save(session);
    return session;
  }, [request, promptAsync]);

  return { ready: Boolean(request), start };
}
