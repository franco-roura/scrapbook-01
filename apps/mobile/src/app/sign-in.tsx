// Sign-in happens on WorkOS AuthKit's hosted page (an emailed 6-digit code),
// opened in a browser sheet. AuthKit sends the email, so there's no mail
// server to run.
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ScrollView } from 'react-native';

import { isConfigured } from '../lib/config';
import { useSignIn } from '../lib/signIn';
import { Button, HudLabel, Panel, Text } from '../ui/module01';

// Only same-app paths, so a crafted link can't send people elsewhere.
const safeNext = (next?: string) => (next && next.startsWith('/') && !next.startsWith('//') ? next : '/');

export default function SignIn() {
  const { next } = useLocalSearchParams<{ next?: string }>();
  const { ready, start: signIn } = useSignIn();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function start() {
    setBusy(true);
    setError(null);
    try {
      if (await signIn()) router.replace(safeNext(next) as never);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'No se pudo entrar.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <ScrollView className="flex-1 bg-canvas" contentContainerClassName="p-6 gap-6">
      <HudLabel code="A-01">Cuenta</HudLabel>
      <Text variant="display">Entrar</Text>

      {!isConfigured ? (
        <Text variant="bodySm" className="text-danger">
          El ingreso no está configurado. Definí EXPO_PUBLIC_WORKOS_CLIENT_ID.
        </Text>
      ) : null}

      <Panel serial="A-01 · CÓDIGO POR EMAIL" className="gap-4">
        <Text variant="bodySm">
          Te mandamos un código de 6 dígitos por email. ¿Sos nuevo? El mismo paso crea tu cuenta.
        </Text>
        {error ? (
          <Text variant="bodySm" className="text-danger">
            {error}
          </Text>
        ) : null}
        <Button onPress={start} loading={busy} disabled={!isConfigured || !ready}>
          Seguir con email
        </Button>
      </Panel>
    </ScrollView>
  );
}
