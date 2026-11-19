import '../../global.css';

import { Manrope_400Regular, Manrope_600SemiBold, Manrope_700Bold } from '@expo-google-fonts/manrope';
import { Michroma_400Regular } from '@expo-google-fonts/michroma';
import { SpaceMono_400Regular, SpaceMono_700Bold } from '@expo-google-fonts/space-mono';
import { useFonts } from 'expo-font';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

import { AndroidShareListener } from '../share/AndroidShareListener';
import { AuthProvider } from '../lib/auth';
import { PushRegistrar } from '../lib/push';
import { HeaderActions } from '../ui/HeaderActions';
import { colors } from '../ui/module01';

export default function RootLayout() {
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
