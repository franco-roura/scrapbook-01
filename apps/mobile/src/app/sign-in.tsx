// Email one-time code sign-in. A code (not a magic link) keeps it inside the
// app with no deep-link setup.
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { KeyboardAvoidingView, Platform, ScrollView, View } from 'react-native';

import { isConfigured } from '../lib/config';
import { supabase } from '../lib/supabase';
import { Button, Field, HudLabel, Panel, Text } from '../ui/module01';

// Only same-app paths, so a crafted link can't send people elsewhere.
const safeNext = (next?: string) => (next && next.startsWith('/') && !next.startsWith('//') ? next : '/');

export default function SignIn() {
  const { next } = useLocalSearchParams<{ next?: string }>();
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [step, setStep] = useState<'email' | 'code'>('email');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function sendCode() {
    setBusy(true);
    setError(null);
    const { error } = await supabase.auth.signInWithOtp({ email: email.trim() });
    setBusy(false);
    if (error) return setError(error.message);
    setStep('code');
  }

  async function verify() {
    setBusy(true);
    setError(null);
    const { error } = await supabase.auth.verifyOtp({ email: email.trim(), token: code.trim(), type: 'email' });
    setBusy(false);
    if (error) return setError(error.message);
    router.replace(safeNext(next) as never);
  }

  return (
    <KeyboardAvoidingView className="flex-1 bg-canvas" behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView contentContainerClassName="p-6 gap-6" keyboardShouldPersistTaps="handled">
        <HudLabel code="A-01">Account</HudLabel>
        <Text variant="display">Sign in</Text>

        {!isConfigured ? (
          <Text variant="bodySm" className="text-danger">
            Supabase isn’t configured. Set EXPO_PUBLIC_SUPABASE_URL and EXPO_PUBLIC_SUPABASE_KEY.
          </Text>
        ) : null}

        <Panel serial="A-01 · EMAIL CODE" className="gap-4">
          {step === 'email' ? (
            <>
              <Field
                label="Email"
                value={email}
                onChangeText={setEmail}
                placeholder="you@example.com"
                autoCapitalize="none"
                autoComplete="email"
                keyboardType="email-address"
                autoFocus
                onSubmitEditing={sendCode}
                error={error}
              />
              <Button onPress={sendCode} loading={busy} disabled={!email.includes('@')}>
                Send code
              </Button>
            </>
          ) : (
            <>
              <Text variant="bodySm">We sent a code to {email.trim()}.</Text>
              <Field
                label="Code"
                value={code}
                onChangeText={setCode}
                placeholder="123456"
                keyboardType="number-pad"
                autoComplete="one-time-code"
                autoFocus
                onSubmitEditing={verify}
                error={error}
              />
              <Button onPress={verify} loading={busy} disabled={code.trim().length < 6}>
                Sign in
              </Button>
              <View className="items-center">
                <Button variant="ghost" onPress={() => setStep('email')}>
                  Use another email
                </Button>
              </View>
            </>
          )}
        </Panel>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}
