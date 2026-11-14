// Push notifications: "Saved to <category>" after the server files an item.
// Registers this phone's Expo push token with the API while signed in, and
// opens the item when a notification is tapped.
import Constants from 'expo-constants';
import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';
import { AndroidImportance } from 'expo-notifications';
import { router } from 'expo-router';
import { useEffect } from 'react';
import { Platform } from 'react-native';

import { useAuth } from './auth';
import { api } from './client';
import { signOutSession } from './session';

const supported = Platform.OS !== 'web';

if (supported) {
  Notifications.setNotificationHandler({
    handleNotification: async () => ({
      shouldShowBanner: true,
      shouldShowList: true,
      shouldPlaySound: false,
      shouldSetBadge: false,
    }),
  });
}

let registeredToken: string | null = null;

async function register() {
  if (!Device.isDevice) return; // simulators can't receive pushes
  const projectId = Constants.expoConfig?.extra?.eas?.projectId ?? Constants.easConfig?.projectId;
  if (!projectId) {
    console.warn('Push: no EAS projectId in app config; run `eas init` to enable notifications.');
    return;
  }
  if (Platform.OS === 'android') {
    await Notifications.setNotificationChannelAsync('default', {
      name: 'Guardados',
      importance: AndroidImportance.DEFAULT,
    });
  }
  let { status } = await Notifications.getPermissionsAsync();
  if (status !== 'granted') status = (await Notifications.requestPermissionsAsync()).status;
  if (status !== 'granted') return;

  const { data: token } = await Notifications.getExpoPushTokenAsync({ projectId });
  await api.registerDevice(token, Platform.OS === 'ios' ? 'ios' : 'android');
  registeredToken = token;
}

/** Signs out, first telling the API to stop notifying this phone. */
export async function signOut() {
  if (registeredToken) {
    await api.unregisterDevice(registeredToken).catch(() => undefined);
    registeredToken = null;
  }
  await signOutSession();
}

/** Mount once inside AuthProvider. Renders nothing on web, which has no push. */
export function PushRegistrar() {
  return supported ? <NativePushRegistrar /> : null;
}

function NativePushRegistrar() {
  const { session } = useAuth();
  const userId = session?.user.id;

  useEffect(() => {
    if (!userId) return;
    register().catch((e) => console.warn('Push registration failed', e));
  }, [userId]);

  const response = Notifications.useLastNotificationResponse();
  useEffect(() => {
    const itemId = response?.notification.request.content.data?.itemId;
    if (typeof itemId === 'string') router.push({ pathname: '/item/[id]', params: { id: itemId } });
  }, [response]);

  return null;
}
