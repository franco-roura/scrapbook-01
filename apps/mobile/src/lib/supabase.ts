import { createClient, type SupportedStorage } from '@supabase/supabase-js';
import * as SecureStore from 'expo-secure-store';
import { Platform } from 'react-native';

import { config } from './config';

// The session lives in the iOS keychain under the shared app group, so the
// share extension (a separate process) is signed in whenever the app is.
const keychainOptions: SecureStore.SecureStoreOptions = {
  accessGroup: Platform.OS === 'ios' ? config.iosAppGroup : undefined,
  keychainAccessible: SecureStore.AFTER_FIRST_UNLOCK,
};

// SecureStore keys may only contain letters, numbers, ".", "-" and "_".
const safeKey = (key: string) => key.replace(/[^A-Za-z0-9._-]/g, '_');

const nativeStorage: SupportedStorage = {
  getItem: (key) => SecureStore.getItemAsync(safeKey(key), keychainOptions),
  setItem: (key, value) => SecureStore.setItemAsync(safeKey(key), value, keychainOptions),
  removeItem: (key) => SecureStore.deleteItemAsync(safeKey(key), keychainOptions),
};

// Web is only used for previews and tests.
const webStorage: SupportedStorage = {
  getItem: async (key) => (typeof localStorage === 'undefined' ? null : localStorage.getItem(key)),
  setItem: async (key, value) => localStorage?.setItem(key, value),
  removeItem: async (key) => localStorage?.removeItem(key),
};

export const supabase = createClient(
  config.supabaseUrl || 'https://not-configured.supabase.co',
  config.supabaseKey || 'not-configured',
  {
    auth: {
      storage: Platform.OS === 'web' ? webStorage : nativeStorage,
      autoRefreshToken: true,
      persistSession: true,
      detectSessionInUrl: false,
    },
  },
);

/** The current access token, refreshed by supabase-js when it is close to expiring. */
export async function accessToken(): Promise<string | null> {
  const { data } = await supabase.auth.getSession();
  return data.session?.access_token ?? null;
}
