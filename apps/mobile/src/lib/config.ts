// Values come from EXPO_PUBLIC_* env vars (see .env.example). Expo inlines
// them at build time, including in the iOS share extension's bundle.
export const config = {
  apiUrl: (process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080').replace(/\/$/, ''),
  supabaseUrl: process.env.EXPO_PUBLIC_SUPABASE_URL ?? '',
  supabaseKey: process.env.EXPO_PUBLIC_SUPABASE_KEY ?? '',
  // Must match the app group expo-share-extension adds to both iOS targets
  // ("group." + ios.bundleIdentifier). Keychain items in this group are
  // readable by the app and its share extension.
  iosAppGroup: 'group.com.scrapbook01.app',
};

export const isConfigured = Boolean(config.supabaseUrl && config.supabaseKey);
