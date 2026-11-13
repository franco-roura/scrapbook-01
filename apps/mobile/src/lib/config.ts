// Values come from EXPO_PUBLIC_* env vars (see .env.example). Expo inlines
// them at build time, including in the iOS share extension's bundle.
export const config = {
  apiUrl: (process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080').replace(/\/$/, ''),
  // WorkOS client id (client_...), from the dashboard's API Keys page.
  workosClientId: process.env.EXPO_PUBLIC_WORKOS_CLIENT_ID ?? '',
  // The WorkOS API, or your custom auth domain.
  workosApiUrl: (process.env.EXPO_PUBLIC_WORKOS_API_URL ?? 'https://api.workos.com').replace(/\/$/, ''),
  // Must match the app group expo-share-extension adds to both iOS targets
  // ("group." + ios.bundleIdentifier). Keychain items in this group are
  // readable by the app and its share extension.
  iosAppGroup: 'group.com.scrapbook01.app',
};

export const isConfigured = Boolean(config.workosClientId);
