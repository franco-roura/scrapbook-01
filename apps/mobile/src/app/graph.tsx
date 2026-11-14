// The whole library as a graph. Tap a node to focus it; the card at the
// bottom opens items.
import { router } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import GraphCanvas from '../graph/GraphCanvas';
import type { GraphData } from '../graph/types';
import { ApiError } from '../lib/api';
import { api } from '../lib/client';
import { platformLabel } from '../lib/links';
import { Button, Chip, HudLabel, Panel, Text, colors } from '../ui/module01';

export default function GraphScreen() {
  const [graph, setGraph] = useState<GraphData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [focusId, setFocusId] = useState<string | null>(null);

  useEffect(() => {
    api
      .getGraph()
      .then(setGraph)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Something went wrong.'));
  }, []);

  const focused = useMemo(() => graph?.nodes.find((n) => n.id === focusId) ?? null, [graph, focusId]);
  const counts = useMemo(() => {
    const items = graph?.nodes.filter((n) => n.kind === 'item').length ?? 0;
    return { items, links: graph?.links.filter((l) => l.kind === 'similar').length ?? 0 };
  }, [graph]);

  if (!graph) {
    return (
      <View className="flex-1 items-center justify-center bg-canvas p-6">
        {error ? <Text className="text-danger">{error}</Text> : <ActivityIndicator color={colors.signalInk} />}
      </View>
    );
  }

  return (
    <SafeAreaView edges={['bottom']} style={{ flex: 1, backgroundColor: colors.canvas }}>
      <View className="px-6 pb-3">
        <HudLabel code="G-01">{`${counts.items} guardados · ${counts.links} conexiones`}</HudLabel>
      </View>

      {graph.nodes.length === 0 ? (
        <View className="flex-1 items-center justify-center gap-2 p-6">
          <Text variant="title">Todavía no hay nada para mapear</Text>
          <Text variant="bodySm" className="text-center">
            Guardá algunas cosas con descripción y van a aparecer acá, agrupadas y conectadas.
          </Text>
        </View>
      ) : (
        <View style={{ flex: 1 }}>
          <GraphCanvas
            graph={graph}
            focusId={focusId}
            fitOnLoad
            onSelect={async (id) => setFocusId((cur) => (cur === id ? null : id))}
            dom={{ style: { flex: 1 }, scrollEnabled: false }}
          />
        </View>
      )}

      {focused ? (
        <View className="absolute bottom-8 left-4 right-4">
          <Panel notch="sm" serial={focused.kind === 'category' ? 'CATEGORÍA' : platformLabel(focused.platform ? `https://${focused.platform}.com` : '')} className="gap-3">
            <Text variant={focused.kind === 'category' ? 'title' : 'body'} numberOfLines={2}>
              {focused.label}
            </Text>
            <View className="flex-row items-center justify-between gap-3">
              <Chip>{`${focused.degree} conexiones`}</Chip>
              {focused.kind === 'item' ? (
                <Button onPress={() => router.push({ pathname: '/item/[id]', params: { id: focused.id } })}>Abrir</Button>
              ) : null}
            </View>
          </Panel>
        </View>
      ) : null}
      {graph.truncated ? (
        <Text variant="hudSm" className="px-6 pb-2">
          Se muestran tus 2000 guardados más nuevos
        </Text>
      ) : null}
    </SafeAreaView>
  );
}
