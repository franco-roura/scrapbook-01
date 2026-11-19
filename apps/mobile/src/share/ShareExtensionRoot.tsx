// Root of the iOS share extension bundle (index.share.js). It shares the
// keychain session with the app, so no sign-in happens here.
import { close, openHostApp, type InitialProps } from 'expo-share-extension';
import { Component, useEffect, useState, type ReactNode } from 'react';
import { ActivityIndicator, View } from 'react-native';

import { api } from '../lib/client';
import { getSession } from '../lib/session';
import { Button, colors, Text } from '../ui/module01';
import { ShareSheet } from './ShareSheet';

export default function ShareExtensionRoot({ url, text }: InitialProps) {
  return (
    <ExtensionErrorBoundary>
      <Sheet url={url} text={text} />
    </ExtensionErrorBoundary>
  );
}

function Sheet({ url, text }: Pick<InitialProps, 'url' | 'text'>) {
  const [signedIn, setSignedIn] = useState<boolean | null>(null);

  useEffect(() => {
    getSession()
      .then((s) => setSignedIn(Boolean(s)))
      .catch(() => setSignedIn(false));
  }, []);

  if (signedIn === null) {
    return (
      <View className="flex-1 items-center justify-center bg-canvas">
        <ActivityIndicator color={colors.inkSoft} />
      </View>
    );
  }

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

// The extension has no red box or dev menu, so a render error would otherwise
// leave an empty sheet. Show it instead, with a way out.
class ExtensionErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;
    return (
      <View className="flex-1 gap-4 bg-canvas p-6">
        <Text variant="title" plain>
          Algo salió mal.
        </Text>
        <Text variant="bodySm" plain selectable>
          {error.message}
        </Text>
        <Button plain variant="secondary" onPress={() => close()}>
          Cerrar
        </Button>
      </View>
    );
  }
}
