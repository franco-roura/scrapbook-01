// The share sheet as a screen: Android share target, and "Add link" in the
// app. `text` is what was shared; without it the sheet asks for a link.
import { router, useLocalSearchParams } from 'expo-router';
import { SafeAreaView } from 'react-native-safe-area-context';

import { useAuth } from '../lib/auth';
import { api } from '../lib/client';
import { ShareSheet } from '../share/ShareSheet';
import { colors } from '../ui/module01';

export default function ShareScreen() {
  const { text } = useLocalSearchParams<{ text?: string }>();
  const { session } = useAuth();
  const close = () => (router.canGoBack() ? router.back() : router.replace('/'));

  return (
    <SafeAreaView style={{ flex: 1, backgroundColor: colors.canvas }}>
      <ShareSheet
        api={api}
        sharedText={text ?? null}
        signedIn={Boolean(session)}
        onSaved={close}
        onCancel={close}
        onSignIn={() => router.replace('/sign-in')}
      />
    </SafeAreaView>
  );
}
