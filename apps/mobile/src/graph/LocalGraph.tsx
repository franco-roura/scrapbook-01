// The item screen's graph: the item and what it links to, 1 or 2 hops out.
// Tapping a neighbor makes it the screen's item; the graph stays mounted, so
// nodes keep their places and the camera glides to the new focus.
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, View } from 'react-native';

import { api } from '../lib/client';
import { Panel, Text, colors } from '../ui/module01';
import GraphCanvas from './GraphCanvas';
import type { GraphData } from './types';

export function LocalGraph({ itemId, onOpen }: { itemId: string; onOpen: (id: string) => void }) {
  const [depth, setDepth] = useState<1 | 2>(1);
  const [graph, setGraph] = useState<GraphData | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let live = true;
    api
      .getNeighbors(itemId, depth)
      .then((g) => live && (setGraph(g), setFailed(false)))
      .catch(() => live && setFailed(true));
    return () => {
      live = false;
    };
  }, [itemId, depth]);

  const neighbors = graph ? graph.nodes.filter((n) => n.kind === 'item').length - 1 : 0;

  return (
    <Panel notch="sm" serial={`N-01 · ${neighbors} LINKED`} className="gap-3 p-0 pt-6">
      <View className="flex-row items-center justify-between px-6">
        <Text variant="label">Connections</Text>
        <View className="flex-row gap-1 rounded-full bg-shell-recess p-1">
          {([1, 2] as const).map((d) => (
            <Pressable
              key={d}
              accessibilityRole="button"
              accessibilityState={{ selected: depth === d }}
              onPress={() => setDepth(d)}
              className={depth === d ? 'rounded-full bg-shell px-3 py-1' : 'rounded-full px-3 py-1'}
            >
              <Text variant="hudSm" className={depth === d ? 'text-ink' : undefined}>
                {d === 1 ? '1 hop' : '2 hops'}
              </Text>
            </Pressable>
          ))}
        </View>
      </View>

      <View className="overflow-hidden rounded-b-lg" style={{ height: 300 }}>
        {graph ? (
          <GraphCanvas
            graph={graph}
            focusId={itemId}
            onSelect={async (id, kind) => {
              if (kind === 'item' && id !== itemId) onOpen(id);
            }}
            dom={{ style: { height: 300 }, scrollEnabled: false }}
          />
        ) : (
          <View className="flex-1 items-center justify-center">
            {failed ? <Text variant="bodySm">Couldn’t load connections.</Text> : <ActivityIndicator color={colors.signalInk} />}
          </View>
        )}
      </View>
      {graph && neighbors === 0 ? (
        <Text variant="bodySm" className="px-6 pb-4">
          No similar items yet. Links appear once items have descriptions.
        </Text>
      ) : null}
    </Panel>
  );
}
