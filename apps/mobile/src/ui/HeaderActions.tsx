import { router } from 'expo-router';
import { Platform, Pressable, View } from 'react-native';

import { useAuth } from '../lib/auth';
import { signOut } from '../lib/push';
import { Text } from './module01';

/** The library header's right side: Credits and Sign out. */
export function HeaderActions() {
  const { session } = useAuth();
  if (!session) return null;
  return (
    // The web header has no right inset of its own; match the title's.
    <View className={`flex-row items-center gap-4 ${Platform.OS === 'web' ? 'pr-4' : ''}`}>
      <Pressable accessibilityRole="button" hitSlop={8} onPress={() => router.push('/credits')}>
        <Text variant="label" className="text-signal-ink">
          Credits
        </Text>
      </Pressable>
      <Pressable
        accessibilityRole="button"
        hitSlop={8}
        onPress={async () => {
          await signOut();
          router.replace('/sign-in');
        }}
      >
        <Text variant="label" className="text-signal-ink">
          Sign out
        </Text>
      </Pressable>
    </View>
  );
}
