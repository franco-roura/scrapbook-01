// Consent screen for connecting an AI assistant (Claude, ChatGPT, ...) to
// the user's library over MCP. Supabase Auth's OAuth server sends the
// browser here as <Site URL>/oauth/consent?authorization_id=..., and the
// answer sends it back to the assistant. Runs in the web build.
import { Redirect, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Platform, ScrollView, View } from 'react-native';

import { useAuth } from '../../lib/auth';
import { supabase } from '../../lib/supabase';
import { Button, Chip, HudLabel, Panel, Text, colors } from '../../ui/module01';

type Details = { clientName: string; clientUri: string; scopes: string[]; email: string };

const scopeText: Record<string, string> = {
  openid: 'Confirm who you are',
  email: 'See your email address',
  profile: 'See your basic profile',
};

function go(url: string) {
  if (Platform.OS === 'web') window.location.assign(url);
}

export default function Consent() {
  const { authorization_id: authorizationId } = useLocalSearchParams<{ authorization_id?: string }>();
  const { session, loading } = useAuth();
  const [details, setDetails] = useState<Details | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<'approve' | 'deny' | null>(null);

  useEffect(() => {
    if (!session || !authorizationId) return;
    supabase.auth.oauth
      .getAuthorizationDetails(authorizationId)
      .then(({ data, error }) => {
        if (error?.message === 'Failed to fetch') return setError('Can’t reach the sign-in service. Try again.');
        if (error || !data) return setError(error?.message ?? 'This request has expired. Start again from your assistant.');
        if (!('client' in data)) return go(data.redirect_url); // already approved earlier
        setDetails({
          clientName: data.client.name || 'An app',
          clientUri: data.client.uri,
          scopes: data.scope.split(' ').filter(Boolean),
          email: data.user.email,
        });
      })
      .catch(() => setError('Can’t reach the sign-in service. Try again.'));
  }, [session, authorizationId]);

  async function decide(approve: boolean) {
    if (!authorizationId) return;
    setBusy(approve ? 'approve' : 'deny');
    const { data, error } = await (approve
      ? supabase.auth.oauth.approveAuthorization(authorizationId)
      : supabase.auth.oauth.denyAuthorization(authorizationId)
    ).catch(() => ({ data: null, error: { message: 'Can’t reach the sign-in service. Try again.' } }));
    if (error || !data) {
      setBusy(null);
      return setError(error?.message ?? 'Something went wrong.');
    }
    go(data.redirect_url);
  }

  if (loading) return null;
  if (!authorizationId) {
    return (
      <View className="flex-1 items-center justify-center bg-canvas p-6">
        <Text>Missing request. Start again from your assistant.</Text>
      </View>
    );
  }
  if (!session) {
    const next = `/oauth/consent?authorization_id=${encodeURIComponent(authorizationId)}`;
    return <Redirect href={{ pathname: '/sign-in', params: { next } }} />;
  }

  return (
    <ScrollView className="bg-canvas" contentContainerClassName="mx-auto w-full max-w-[560px] p-6 gap-6">
      <HudLabel code="M-01">Connect an assistant</HudLabel>
      {error ? (
        <Text className="text-danger">{error}</Text>
      ) : !details ? (
        <ActivityIndicator color={colors.signalInk} />
      ) : (
        <>
          <Text variant="display">{`Let ${details.clientName} use your library?`}</Text>
          <Panel serial={`MCP · ${details.email}`} className="gap-4">
            <Text variant="body">{`${details.clientName} will be able to:`}</Text>
            <View className="gap-2">
              <Text variant="body">• Search your saved posts and links, and read your notes on them</Text>
              <Text variant="body">• See how your items are grouped and linked</Text>
              <Text variant="body">• Save new links when you ask it to</Text>
              {details.scopes.filter((s) => scopeText[s]).map((s) => (
                <Text key={s} variant="body">{`• ${scopeText[s]}`}</Text>
              ))}
            </View>
            <Text variant="bodySm">
              Its answers use your own {details.clientName} plan, not your Scrapbook/01 credits. You can disconnect it any time from its settings.
            </Text>
            {details.clientUri ? <Chip>{details.clientUri.replace(/^https?:\/\//, '')}</Chip> : null}
          </Panel>
          <View className="gap-3">
            <Button onPress={() => decide(true)} loading={busy === 'approve'} disabled={busy !== null}>
              Allow
            </Button>
            <Button variant="secondary" onPress={() => decide(false)} loading={busy === 'deny'} disabled={busy !== null}>
              Don’t allow
            </Button>
          </View>
        </>
      )}
    </ScrollView>
  );
}
