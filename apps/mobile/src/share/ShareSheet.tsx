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
      setState({ kind: 'error', message: e instanceof ApiError ? e.message : 'Something went wrong.' });
    }
  }

  if (!signedIn) {
    return (
      <Frame>
        <HudLabel code="S-01" plain={plain}>
          Save to Scrapbook/01
        </HudLabel>
        <Panel plain={plain} className="mt-4 gap-3">
          <Text variant="title" {...textProps}>
            Sign in first
          </Text>
          <Text variant="bodySm" {...textProps}>
            Open Scrapbook/01 and sign in. After that you can save from any app.
          </Text>
        </Panel>
        <View className="mt-6 gap-3">
          <Button plain={plain} onPress={onSignIn}>
            Open Scrapbook/01
          </Button>
          <Button plain={plain} variant="secondary" onPress={onCancel}>
            Cancel
          </Button>
        </View>
      </Frame>
    );
  }

  const busy = state.kind === 'saving' || state.kind === 'saved';

  return (
    <Frame>
      <HudLabel code="S-01" plain={plain}>
        Save to Scrapbook/01
      </HudLabel>

      <Panel plain={plain} serial="P-01 · NEW SAVE" className="mt-4 gap-4">
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
            autoFocus
            returnKeyType="next"
            onSubmitEditing={() => descriptionRef.current?.focus()}
            error={manualUrl && !link ? 'Paste a full link, starting with https://' : null}
          />
        )}

        <Field
          ref={descriptionRef}
          label="What is it about?"
          plain={plain}
          value={description}
          onChangeText={setDescription}
          placeholder="5 dad-daughter traditions to try"
          multiline
          maxLength={DESCRIPTION_MAX}
          autoFocus={Boolean(url)}
          editable={!busy}
          accessibilityHint="Optional. Describe it now or later from your inbox."
        />
        <Text variant="hudSm" {...textProps}>
          Optional · you can describe it later
        </Text>
      </Panel>

      {state.kind === 'error' ? (
        <Text variant="bodySm" className="mt-4 text-danger" accessibilityLiveRegion="polite" {...textProps}>
          {state.message}
        </Text>
      ) : null}

      <View className="mt-6 gap-3">
        <Button plain={plain} onPress={save} disabled={!link || busy} loading={state.kind === 'saving'}>
          {state.kind === 'saved' ? 'Saved' : state.kind === 'error' ? 'Try again' : 'Save'}
        </Button>
        <Button plain={plain} variant="secondary" onPress={onCancel} disabled={busy}>
          Cancel
        </Button>
      </View>
    </Frame>
  );
}

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <KeyboardAvoidingView className="flex-1 bg-canvas" behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerClassName="p-6" keyboardShouldPersistTaps="handled">
        {children}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
