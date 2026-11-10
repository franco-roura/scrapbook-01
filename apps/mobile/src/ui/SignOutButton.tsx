import { router } from 'expo-router';
import { Pressable } from 'react-native';

import { useAuth } from '../lib/auth';
import { signOut } from '../lib/push';
import { Text } from './module01';

export function SignOutButton() {
  const { session } = useAuth();
  if (!session) return null;
  return (
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
  );
}
