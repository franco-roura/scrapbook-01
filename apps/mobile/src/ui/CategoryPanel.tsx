// Shows an item's category and lets the user move it. Choosing a category
// here tells the server to stop picking one automatically for this item.
import { router } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, View } from 'react-native';

import { ApiError, type CategorySummary, type Item } from '../lib/api';
import { api } from '../lib/client';
import { Button, Field, Panel, Text, colors } from './module01';

export function CategoryPanel({ item, onChange }: { item: Item; onChange: (item: Item) => void }) {
  const [open, setOpen] = useState(false);
  const [categories, setCategories] = useState<CategorySummary[] | null>(null);
  const [newName, setNewName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function openPicker() {
    setOpen(true);
    setError(null);
    try {
      setCategories(await api.listCategories());
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Algo salió mal.');
    }
  }

  async function choose(categoryId: string | null) {
    setBusy(true);
    setError(null);
    try {
      onChange(await api.setItemCategory(item.id, categoryId));
      setOpen(false);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Algo salió mal.');
    } finally {
      setBusy(false);
    }
  }

  async function createAndChoose() {
    if (!newName.trim()) return;
    setBusy(true);
    setError(null);
    try {
      const cat = await api.createCategory(newName);
      setNewName('');
      onChange(await api.setItemCategory(item.id, cat.id));
      setOpen(false);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : 'Algo salió mal.');
    } finally {
      setBusy(false);
    }
  }

  const waiting = item.waitingForCredits && item.categorySource === 'auto';
  const sorting = item.classifying && item.categorySource === 'auto' && !waiting;

  return (
    <Panel notch="sm" serial={item.categorySource === 'user' ? 'C-01 · ELEGIDA POR VOS' : 'C-01 · AUTOMÁTICA'} className="gap-4">
      <View className="flex-row items-center justify-between gap-3">
        <View className="flex-1 gap-1">
          <Text variant="label">Categoría</Text>
          {waiting ? (
            <View className="gap-1">
              <Text variant="title">Esperando créditos</Text>
              <Text variant="bodySm">Está guardado. Se va a ordenar y conectar cuando cargues créditos.</Text>
              <Button variant="ghost" className="self-start px-0" onPress={() => router.push('/credits')}>
                Ver créditos
              </Button>
            </View>
          ) : sorting ? (
            <View className="flex-row items-center gap-2">
              <ActivityIndicator size="small" color={colors.signalInk} />
              <Text variant="bodySm">Ordenando…</Text>
            </View>
          ) : (
            <Text variant="title">{item.category?.name ?? 'Ninguna todavía'}</Text>
          )}
        </View>
        {!open ? (
          <Button variant="secondary" onPress={openPicker}>
            Cambiar
          </Button>
        ) : null}
      </View>

      {open ? (
        <View className="gap-4">
          {categories === null && !error ? <ActivityIndicator color={colors.signalInk} /> : null}
          {categories ? (
            <View className="flex-row flex-wrap gap-2">
              {categories.map((c) => (
                <Option key={c.id} label={c.name} active={item.category?.id === c.id} disabled={busy} onPress={() => choose(c.id)} />
              ))}
              <Option label="Sin categoría" active={!item.category} disabled={busy} onPress={() => choose(null)} />
            </View>
          ) : null}
          <Field
            label="Nueva categoría"
            value={newName}
            onChangeText={setNewName}
            placeholder="Recetas"
            maxLength={60}
            onSubmitEditing={createAndChoose}
            returnKeyType="done"
          />
          <View className="flex-row gap-3">
            <Button className="flex-1" onPress={createAndChoose} loading={busy} disabled={!newName.trim()}>
              Crear y mover
            </Button>
            <Button variant="ghost" onPress={() => setOpen(false)} disabled={busy}>
              Cancelar
            </Button>
          </View>
        </View>
      ) : null}

      {error ? (
        <Text variant="bodySm" className="text-danger">
          {error}
        </Text>
      ) : null}
    </Panel>
  );
}

function Option({ label, active, disabled, onPress }: { label: string; active: boolean; disabled: boolean; onPress: () => void }) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ selected: active, disabled }}
      disabled={disabled}
      onPress={onPress}
      className={active ? 'rounded-full bg-signal px-4 py-2' : 'rounded-full bg-shell-recess px-4 py-2 active:bg-seam'}
    >
      <Text variant="label" className="text-ink">
        {label}
      </Text>
    </Pressable>
  );
}
