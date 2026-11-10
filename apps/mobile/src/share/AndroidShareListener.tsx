// On Android, sharing to Scrapbook/01 opens the app (expo-share-intent). This
// sends the shared text to the /share screen. iOS uses its own extension, and
// web has nothing to listen for.
import { router } from 'expo-router';
import { ShareIntentProvider, useShareIntentContext } from 'expo-share-intent';
import { useEffect, type ReactNode } from 'react';
import { Platform } from 'react-native';

export function AndroidShareListener({ children }: { children: ReactNode }) {
  if (Platform.OS !== 'android') return <>{children}</>;
  return (
    <ShareIntentProvider>
      <Redirector />
      {children}
    </ShareIntentProvider>
  );
}

function Redirector() {
  const { hasShareIntent, shareIntent, resetShareIntent } = useShareIntentContext();

  useEffect(() => {
    if (!hasShareIntent) return;
    const text = shareIntent.webUrl ?? shareIntent.text;
    if (text) router.push({ pathname: '/share', params: { text } });
    resetShareIntent();
  }, [hasShareIntent, shareIntent, resetShareIntent]);

  return null;
}
