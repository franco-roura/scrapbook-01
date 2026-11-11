// Prepaid AI credits: what's left, what it buys, and where it went.
import { useEffect, useState } from 'react';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { ApiError, type Credits } from '../lib/api';
import { api } from '../lib/client';
import { Button, Chip, HudLabel, Panel, Text, colors } from '../ui/module01';

const usd = (n: number) => {
  const abs = Math.abs(n);
  const digits = abs >= 1 ? 2 : abs >= 0.01 ? 3 : 5;
  return `${n < 0 ? '−' : ''}$${abs.toFixed(digits)}`;
};

const kindLabel: Record<Credits['recent'][number]['kind'], string> = {
  starter: 'Welcome credits',
  grant: 'Credits added',
  purchase: 'Pack bought',
  usage: 'Used',
};

const stepLabel: Record<string, string> = {
  classify: 'Sorting a save',
  search: 'Assistant search',
};

export default function CreditsScreen() {
  const [credits, setCredits] = useState<Credits | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getCredits()
      .then(setCredits)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Something went wrong.'));
  }, []);

  if (!credits) {
    return (
      <View className="flex-1 items-center justify-center bg-canvas p-6">
        {error ? <Text className="text-danger">{error}</Text> : <ActivityIndicator color={colors.signalInk} />}
      </View>
    );
  }

  const share = Math.round(credits.modelShare * 100);
  const empty = credits.balanceUsd <= 0;

  return (
    <ScrollView className="bg-canvas" contentContainerClassName="p-6 gap-6">
      <HudLabel code="K-01">Credits</HudLabel>

      <Panel serial="K-01 · MODEL BUDGET" className="gap-2">
        <Text variant="display">{usd(credits.balanceUsd)}</Text>
        <Text variant="bodySm">
          {empty
            ? 'You’re out of credits. New saves are kept, but won’t be sorted or linked until you add more.'
            : `About ${credits.savesLeft.toLocaleString()} more saves sorted and linked.`}
        </Text>
      </Panel>

      <Panel notch="sm" className="gap-3">
        <Text variant="title">How credits work</Text>
        <Text variant="bodySm">
          {`Packs are prepaid. ${share}% of what you pay goes to the AI models that sort your saves; ${100 - share}% keeps Scrapbook/01 running.`}
        </Text>
        <Text variant="bodySm">
          Your own assistant (Claude, ChatGPT) runs on your plan when it reads your library, so it costs you no credits. Only the quick search lookup does.
        </Text>
        <Button disabled>Buy credits · coming soon</Button>
      </Panel>

      <View className="gap-2">
        <HudLabel code="K-02">Recent activity</HudLabel>
        {credits.recent.length === 0 ? <Text variant="bodySm">Nothing yet.</Text> : null}
        {credits.recent.map((e, i) => (
          <View key={i} className="flex-row items-center justify-between rounded-md bg-shell px-4 py-3">
            <View className="gap-1">
              <Text variant="body">{e.kind === 'usage' ? (stepLabel[e.step ?? ''] ?? 'Used') : kindLabel[e.kind]}</Text>
              <Text variant="hudSm">{new Date(e.createdAt).toLocaleString()}</Text>
            </View>
            <Chip tone={e.amountUsd > 0 ? 'signal' : 'shell'}>{usd(e.amountUsd)}</Chip>
          </View>
        ))}
      </View>
    </ScrollView>
  );
}
