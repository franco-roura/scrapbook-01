// Root of the iOS share extension bundle (index.share.js). It shares the
// keychain session with the app, so no sign-in happens here.
import { close, openHostApp, type InitialProps } from 'expo-share-extension';
import { useEffect, useState } from 'react';

import { api } from '../lib/client';
import { supabase } from '../lib/supabase';
import { ShareSheet } from './ShareSheet';

export default function ShareExtensionRoot({ url, text }: InitialProps) {
  const [signedIn, setSignedIn] = useState<boolean | null>(null);

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => setSignedIn(Boolean(data.session)));
  }, []);

  if (signedIn === null) return null;

  return (
    <ShareSheet
      api={api}
      sharedText={url ?? text ?? null}
      signedIn={signedIn}
      plain
      onSaved={() => close()}
      onCancel={() => close()}
      onSignIn={() => openHostApp('sign-in')}
    />
  );
}
