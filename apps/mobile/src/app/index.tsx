// Library: everything saved, newest first, with the "Undescribed" inbox as
// a second tab.
import { Link, Redirect, router, useFocusEffect } from 'expo-router';
import { useCallback, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, Image, Pressable, RefreshControl, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import type { Item } from '../lib/api';
import { useAuth } from '../lib/auth';
import { hostOf, platformLabel } from '../lib/links';
import { useItems } from '../lib/useItems';
import { Button, Chip, HudLabel, Text, colors } from '../ui/module01';

export default function Library() {
  const { session, loading } = useAuth();
  if (loading) return null;
  if (!session) return <Redirect href="/sign-in" />;
  return <LibraryList />;
}

type TabName = 'all' | 'inbox';

function LibraryList() {
  const [tab, setTab] = useState<TabName>('all');
  // Keyed by tab so each list starts fresh instead of flashing the other one.
  return <ItemList key={tab} tab={tab} setTab={setTab} />;
}

function ItemList({ tab, setTab }: { tab: TabName; setTab: (t: TabName) => void }) {
  const list = useItems(tab === 'inbox');

  // Reload when coming back from the share sheet or an item.
  const first = useRef(true);
  const { refresh } = list;
  useFocusEffect(
    useCallback(() => {
      if (first.current) {
        first.current = false;
        return;
      }
      refresh();
    }, [refresh]),
  );

  return (
    <SafeAreaView edges={['bottom']} style={{ flex: 1, backgroundColor: colors.canvas }}>
      <FlatList
        data={list.items}
        keyExtractor={(it) => it.id}
        contentContainerClassName="px-6 pb-28 gap-3"
        ListHeaderComponent={
          <View className="gap-4 pb-2 pt-2">
            <HudLabel code="L-01">Biblioteca</HudLabel>
            <View className="flex-row items-center justify-between">
              <View className="flex-row gap-2 rounded-full bg-shell-recess p-1" accessibilityRole="tablist">
                <Tab label="Todo" active={tab === 'all'} onPress={() => setTab('all')} />
                <Tab label="Sin describir" active={tab === 'inbox'} onPress={() => setTab('inbox')} />
              </View>
              <Button variant="secondary" onPress={() => router.push('/graph')}>
                Mapa
              </Button>
            </View>
            {list.error ? <Text variant="bodySm" className="text-danger">{list.error}</Text> : null}
          </View>
        }
        renderItem={({ item, index }) => <Row item={item} index={index} />}
        ListEmptyComponent={
          list.loading ? (
            <ActivityIndicator className="mt-12" color={colors.signalInk} />
          ) : (
            <View className="mt-12 items-center gap-2">
              <Text variant="title">{tab === 'inbox' ? 'Nada para describir' : 'Todavía no guardaste nada'}</Text>
              <Text variant="bodySm" className="text-center">
                {tab === 'inbox'
                  ? 'Lo que guardes sin descripción aparece acá.'
                  : 'Compartí un post desde cualquier app a Scrapbook/01, o agregá un link.'}
              </Text>
            </View>
          )
        }
        refreshControl={<RefreshControl refreshing={list.refreshing} onRefresh={list.refresh} tintColor={colors.signalInk} />}
        onEndReached={list.loadMore}
        onEndReachedThreshold={0.4}
      />
      <View className="absolute bottom-8 left-6 right-6">
        <Button onPress={() => router.push('/share')}>Agregar link</Button>
      </View>
    </SafeAreaView>
  );
}

function Tab({ label, active, onPress }: { label: string; active: boolean; onPress: () => void }) {
  return (
    <Pressable
      accessibilityRole="tab"
      accessibilityState={{ selected: active }}
      onPress={onPress}
      className={active ? 'rounded-full bg-shell px-4 py-2' : 'rounded-full px-4 py-2'}
    >
      <Text variant="label" className={active ? 'text-ink' : 'text-ink-soft'}>
        {label}
      </Text>
    </Pressable>
  );
}

function Row({ item, index }: { item: Item; index: number }) {
  const date = new Date(item.createdAt).toLocaleDateString('es-UY', { day: 'numeric', month: 'short' });
  return (
    <Link href={{ pathname: '/item/[id]', params: { id: item.id } }} asChild>
      <Pressable className="gap-2 rounded-lg bg-shell p-4 active:bg-shell-recess">
        <View className="flex-row items-center justify-between">
          <Chip>{platformLabel(item.url)}</Chip>
          <Text variant="hudSm">
            R-{String(index + 1).padStart(2, '0')} · {date}
          </Text>
        </View>
        <View className="flex-row gap-3">
          {item.previewImage ? (
            <Image
              source={{ uri: item.previewImage }}
              resizeMode="cover"
              accessibilityIgnoresInvertColors
              className="h-16 w-16 rounded-md bg-shell-recess"
            />
          ) : null}
          <View className="flex-1 gap-1">
            {item.description ? (
              <Text variant="body" numberOfLines={3}>
                {item.description}
              </Text>
            ) : (
              <Text variant="bodySm" className="text-signal-ink">
                Sin descripción · tocá para agregarla
              </Text>
            )}
            {item.previewTitle ? (
              <Text variant="bodySm" numberOfLines={1}>
                {item.previewTitle}
              </Text>
            ) : null}
          </View>
        </View>
        <View className="flex-row items-center gap-2">
          {item.category ? (
            <Chip tone="signal">{item.category.name}</Chip>
          ) : item.waitingForCredits ? (
            <Chip>Esperando créditos</Chip>
          ) : item.classifying ? (
            <Chip>Ordenando…</Chip>
          ) : null}
          <Text variant="bodySm" numberOfLines={1} className="flex-1">
            {hostOf(item.url)}
          </Text>
        </View>
      </Pressable>
    </Link>
  );
}
