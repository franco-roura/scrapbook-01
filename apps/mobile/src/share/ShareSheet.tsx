// The "What is it about?" sheet. The same component runs inside the iOS
// share extension and as the Android share screen, so it takes everything
// platform-specific (closing, opening the app) as props.
import { useEffect, useRef, useState, type ComponentRef } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, TextInput, View } from 'react-native';

import { ApiError, type Api, type Item } from '../lib/api';
import { extractUrl, platformLabel } from '../lib/links';
import { Button, Chip, Field, HudLabel, Panel, Text } from '../ui/module01';

export type ShareSheetProps = {
  api: Api;
  /** What the other app shared: a URL, or text that contains one. */
  sharedText: string | null;
  signedIn: boolean;
  onSaved: (item: Item) => void;
  onCancel: () => void;
  onSignIn: () => void;
  /** In the iOS extension: system fonts and no font scaling. */
  plain?: boolean;
};

type State = { kind: 'editing' } | { kind: 'saving' } | { kind: 'saved'; item: Item } | { kind: 'error'; message: string };

export const DESCRIPTION_MAX = 2000;

export function ShareSheet({ api, sharedText, signedIn, onSaved, onCancel, onSignIn, plain }: ShareSheetProps) {
  const url = extractUrl(sharedText);
  const [manualUrl, setManualUrl] = useState('');
  const [description, setDescription] = useState('');
  const [state, setState] = useState<State>({ kind: 'editing' });
  const descriptionRef = useRef<ComponentRef<typeof TextInput>>(null);
  const savedTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  useEffect(() => () => clearTimeout(savedTimer.current), []);

  const link = url ?? extractUrl(manualUrl);
  const textProps = { plain };

  async function save() {
    if (!link) return;
    setState({ kind: 'saving' });
    try {
      // Send the full shared text when we have it: the server extracts and
      // cleans the link the same way for every caller.
      const item = await api.saveItem(url ? (sharedText as string) : manualUrl, description);
      setState({ kind: 'saved', item });
      savedTimer.current = setTimeout(() => onSaved(item), 700);
    } catch (e) {
      setState({ kind: 'error', message: e instanceof ApiError ? e.message : 'Algo salió mal.' });
    }
  }

  if (!signedIn) {
    return (
      <Frame>
        <HudLabel code="S-01" plain={plain}>
          Guardar en Scrapbook/01
        </HudLabel>
        <Panel plain={plain} className="mt-4 gap-3">
          <Text variant="title" {...textProps}>
            Primero entrá a tu cuenta
          </Text>
          <Text variant="bodySm" {...textProps}>
            Abrí Scrapbook/01 y entrá a tu cuenta. Después vas a poder guardar desde cualquier app.
          </Text>
        </Panel>
        <View className="mt-6 gap-3">
          <Button plain={plain} onPress={onSignIn}>
            Abrir Scrapbook/01
          </Button>
          <Button plain={plain} variant="secondary" onPress={onCancel}>
            Cancelar
          </Button>
        </View>
      </Frame>
    );
  }

  const busy = state.kind === 'saving' || state.kind === 'saved';

  return (
    <Frame>
      <HudLabel code="S-01" plain={plain}>
        Guardar en Scrapbook/01
      </HudLabel>

      <Panel plain={plain} serial="P-01 · NUEVO" className="mt-4 gap-4">
        {url ? (
          <View className="gap-2">
            <Chip plain={plain}>{platformLabel(url)}</Chip>
            <Text variant="bodySm" numberOfLines={2} {...textProps}>
              {url}
            </Text>
          </View>
        ) : (
          <Field
            label="Link"
            plain={plain}
            value={manualUrl}
            onChangeText={setManualUrl}
            placeholder="https://"
            autoCapitalize="none"
            autoCorrect={false}
            keyboardType="url"
            autoFocus={!plain}
            returnKeyType="next"
            onSubmitEditing={() => descriptionRef.current?.focus()}
            error={manualUrl && !link ? 'Pegá el link completo, que empiece con https://' : null}
          />
        )}

        <Field
          ref={descriptionRef}
          label="¿De qué se trata?"
          plain={plain}
          value={description}
          onChangeText={setDescription}
          placeholder="5 tradiciones para hacer con tu hija"
          multiline
          maxLength={DESCRIPTION_MAX}
          // The iOS extension's sheet is short: an open keyboard would cover
          // the buttons before the person has seen them.
          autoFocus={Boolean(url) && !plain}
          editable={!busy}
          accessibilityHint="Opcional. Describilo ahora o después desde Sin describir."
        />
        <Text variant="hudSm" {...textProps}>
          Opcional · podés describirlo después
        </Text>
      </Panel>

      {state.kind === 'error' ? (
        <Text variant="bodySm" className="mt-4 text-danger" accessibilityLiveRegion="polite" {...textProps}>
          {state.message}
        </Text>
      ) : null}

      <View className="mt-6 gap-3">
        <Button plain={plain} onPress={save} disabled={!link || busy} loading={state.kind === 'saving'}>
          {state.kind === 'saved' ? 'Guardado' : state.kind === 'error' ? 'Probar de nuevo' : 'Guardar'}
        </Button>
        <Button plain={plain} variant="secondary" onPress={onCancel} disabled={busy}>
          Cancelar
        </Button>
      </View>
    </Frame>
  );
}

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <KeyboardAvoidingView className="flex-1 bg-canvas" behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerClassName="p-6" keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag">
        {children}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
