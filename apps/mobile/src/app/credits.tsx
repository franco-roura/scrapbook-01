// Prepaid AI credits: what's left, what it buys, and where it went.
import { useEffect, useState } from 'react';
import { ActivityIndicator, ScrollView, View } from 'react-native';

import { ApiError, type Credits } from '../lib/api';
import { api } from '../lib/client';
import { Button, Chip, HudLabel, Panel, Text, colors } from '../ui/module01';

// Dollars, marked US$ so they aren't read as Uruguayan pesos, with Uruguay's
// decimal comma.
const usd = (n: number) => {
  const abs = Math.abs(n);
  const digits = abs >= 1 ? 2 : abs >= 0.01 ? 3 : 5;
  const amount = abs.toLocaleString('es-UY', { minimumFractionDigits: digits, maximumFractionDigits: digits });
  return `${n < 0 ? '−' : ''}US$ ${amount}`;
};

const kindLabel: Record<Credits['recent'][number]['kind'], string> = {
  starter: 'Créditos de bienvenida',
  grant: 'Créditos cargados',
  purchase: 'Pack comprado',
  usage: 'Uso',
};

const stepLabel: Record<string, string> = {
  classify: 'Ordenar un guardado',
  search: 'Búsqueda de tu IA',
};

export default function CreditsScreen() {
  const [credits, setCredits] = useState<Credits | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getCredits()
      .then(setCredits)
      .catch((e) => setError(e instanceof ApiError ? e.message : 'Algo salió mal.'));
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
      <HudLabel code="K-01">Créditos</HudLabel>

      <Panel serial="K-01 · SALDO PARA IA" className="gap-2">
        <Text variant="display">{usd(credits.balanceUsd)}</Text>
        <Text variant="bodySm">
          {empty
            ? 'Te quedaste sin créditos. Lo que guardes se conserva, pero no se ordena ni se conecta hasta que cargues más.'
            : `Te alcanza para unos ${credits.savesLeft.toLocaleString('es-UY')} guardados más.`}
        </Text>
      </Panel>

      <Panel notch="sm" className="gap-3">
        <Text variant="title">Cómo funcionan los créditos</Text>
        <Text variant="bodySm">
          {`Los packs se pagan por adelantado. El ${share}% de lo que pagás va a los modelos de IA que ordenan tus guardados; el ${100 - share}% mantiene Scrapbook/01 andando.`}
        </Text>
        <Text variant="bodySm">
          Tu propia IA (Claude, ChatGPT) corre en tu plan cuando lee tu biblioteca, así que no te gasta créditos. Solo la búsqueda rápida, que cuesta casi nada.
        </Text>
        <Button disabled>Comprar créditos · muy pronto</Button>
      </Panel>

      <View className="gap-2">
        <HudLabel code="K-02">Actividad reciente</HudLabel>
        {credits.recent.length === 0 ? <Text variant="bodySm">Nada todavía.</Text> : null}
        {credits.recent.map((e, i) => (
          <View key={i} className="flex-row items-center justify-between rounded-md bg-shell px-4 py-3">
            <View className="gap-1">
              <Text variant="body">{e.kind === 'usage' ? (stepLabel[e.step ?? ''] ?? 'Uso') : kindLabel[e.kind]}</Text>
              <Text variant="hudSm">{new Date(e.createdAt).toLocaleString('es-UY')}</Text>
            </View>
            <Chip tone={e.amountUsd > 0 ? 'signal' : 'shell'}>{usd(e.amountUsd)}</Chip>
          </View>
        ))}
      </View>
    </ScrollView>
  );
}
