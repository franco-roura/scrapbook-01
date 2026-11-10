// One saved item: open the link, describe it, or delete it.
import { router, useLocalSearchParams } from 'expo-router';
import * as WebBrowser from 'expo-web-browser';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Alert, KeyboardAvoidingView, Platform, ScrollView, View } from 'react-native';

import { ApiError, type Item } from '../../lib/api';
import { api } from '../../lib/client';
import { platformLabel } from '../../lib/links';
import { DESCRIPTION_MAX } from '../../share/ShareSheet';
import { Button, Chip, Field, HudLabel, Panel, Text, colors } from '../../ui/module01';

export default function ItemScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const [item, setItem] = useState<Item | null>(null);
  const [description, setDescription] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    api
      .getItem(id)
      .then((it) => {
        setItem(it);
        setDescription(it.description ?? '');
      })
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Something went wrong.'));
  }, [id]);

  async function save() {
    if (!item) return;
    setSaving(true);
    try {
      const updated = await api.describeItem(item.id, description);
      setItem(updated);
      setDescription(updated.description ?? '');
      setError(null);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Something went wrong.');
    } finally {
      setSaving(false);
    }
  }

  async function remove() {
    if (!item) return;
    const doDelete = async () => {
      try {
        await api.deleteItem(item.id);
        router.back();
      } catch (e) {
        setError(e instanceof ApiError ? e.message : 'Something went wrong.');
      }
    };
    if (Platform.OS === 'web') return void (confirm('Delete this item?') && doDelete());
    Alert.alert('Delete this item?', "This can't be undone.", [
      { text: 'Cancel', style: 'cancel' },
      { text: 'Delete', style: 'destructive', onPress: doDelete },
    ]);
  }

  if (!item) {
    return (
      <View className="flex-1 items-center justify-center bg-canvas p-6">
        {error ? <Text className="text-danger">{error}</Text> : <ActivityIndicator color={colors.signalInk} />}
      </View>
    );
  }

  const changed = description.trim() !== (item.description ?? '');

  return (
    <KeyboardAvoidingView className="flex-1 bg-canvas" behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerClassName="p-6 gap-6" keyboardShouldPersistTaps="handled">
        <HudLabel code="I-01">{platformLabel(item.url)}</HudLabel>

        <Panel serial={`SAVED · ${new Date(item.createdAt).toLocaleDateString()}`} className="gap-4">
          <View className="flex-row gap-2">
            <Chip>{platformLabel(item.url)}</Chip>
            {item.category ? <Chip tone="signal">{item.category.name}</Chip> : null}
          </View>
          <Text variant="bodySm" selectable>
            {item.url}
          </Text>
          <Button variant="secondary" onPress={() => WebBrowser.openBrowserAsync(item.url)}>
            Open link
          </Button>
        </Panel>

        <Panel notch="sm" className="gap-4">
          <Field
            label="What is it about?"
            value={description}
            onChangeText={setDescription}
            placeholder="5 dad-daughter traditions to try"
            multiline
            maxLength={DESCRIPTION_MAX}
            error={error}
          />
          <Button onPress={save} loading={saving} disabled={!changed}>
            Save description
          </Button>
        </Panel>

        <Button variant="destructive" onPress={remove}>
          Delete
        </Button>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
