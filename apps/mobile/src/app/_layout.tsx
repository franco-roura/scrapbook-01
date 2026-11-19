import '../../global.css';

import { Manrope_400Regular, Manrope_600SemiBold, Manrope_700Bold } from '@expo-google-fonts/manrope';
import { Michroma_400Regular } from '@expo-google-fonts/michroma';
import { SpaceMono_400Regular, SpaceMono_700Bold } from '@expo-google-fonts/space-mono';
import * as Sentry from '@sentry/react-native';
import { useFonts } from 'expo-font';
import { Stack, type ErrorBoundaryProps } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { Platform, View } from 'react-native';

import { AndroidShareListener } from '../share/AndroidShareListener';
import { AuthProvider } from '../lib/auth';
import { PushRegistrar } from '../lib/push';
import { initSentry } from '../lib/sentry';
import { HeaderActions } from '../ui/HeaderActions';
import { Button, colors, Panel, Text } from '../ui/module01';

// Before the first render, so the first screen gets a navigation span. A
// no-op without EXPO_PUBLIC_SENTRY_DSN. expoRouterIntegration (in initSentry)
// picks up Expo Router's navigation container itself.
initSentry({ surface: Platform.OS === 'web' ? 'web' : 'app' });

function RootLayout() {
  const [fontsLoaded] = useFonts({
    Manrope_400Regular,
    Manrope_600SemiBold,
    Manrope_700Bold,
    Michroma_400Regular,
    SpaceMono_400Regular,
    SpaceMono_700Bold,
  });
  if (!fontsLoaded) return null;

  return (
    <AuthProvider>
      <AndroidShareListener>
        <StatusBar style="dark" />
        <PushRegistrar />
        <Stack
          screenOptions={{
            headerShadowVisible: false,
            headerStyle: { backgroundColor: colors.canvas },
            headerTitleStyle: { fontFamily: 'Michroma_400Regular', fontSize: 15, color: colors.ink },
            headerTintColor: colors.signalInk,
            contentStyle: { backgroundColor: colors.canvas },
          }}
        >
          <Stack.Screen
            name="index"
            options={{
              title: 'Scrapbook/01',
              headerRight: () => <HeaderActions />,
            }}
          />
          <Stack.Screen name="sign-in" options={{ title: 'Entrar' }} />
          <Stack.Screen name="item/[id]" options={{ title: 'Guardado' }} />
          <Stack.Screen name="graph" options={{ title: 'Mapa' }} />
          <Stack.Screen name="credits" options={{ title: 'Créditos' }} />
          <Stack.Screen name="connect-ai" options={{ title: 'Conectar IA' }} />
          <Stack.Screen name="auth/callback" options={{ headerShown: false }} />
          <Stack.Screen name="share" options={{ presentation: 'modal', headerShown: false }} />
        </Stack>
      </AndroidShareListener>
    </AuthProvider>
  );
}

export default Sentry.wrap(RootLayout);

// Expo Router renders this when a screen throws while rendering. The wrapper
// reports the error (when Sentry is on) before showing it. Fonts may not be
// loaded yet, so it uses system fonts.
function RouteErrorFallback({ retry }: ErrorBoundaryProps) {
  return (
    <View className="flex-1 justify-center bg-canvas p-6">
      <Panel plain className="gap-3">
        <Text variant="title" plain>
          Algo salió mal.
        </Text>
        <Text variant="bodySm" plain>
          No pudimos mostrar esta pantalla. Probá de nuevo; si sigue pasando, cerrá la app y volvé a abrirla.
        </Text>
      </Panel>
      <Button plain className="mt-6" onPress={() => void retry()}>
        Probar de nuevo
      </Button>
    </View>
  );
}

export const ErrorBoundary = Sentry.wrapExpoRouterErrorBoundary(RouteErrorFallback);
