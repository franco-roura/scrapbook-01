// AuthKit sends the browser back here after sign-in. On web this page runs in
// the sign-in popup and hands the result to the app; on phones the browser
// sheet catches the redirect first, so this only shows if a link opens it.
import { Redirect } from 'expo-router';
import * as WebBrowser from 'expo-web-browser';
import { ActivityIndicator, View } from 'react-native';

import { colors } from '../../ui/module01';

const handled = WebBrowser.maybeCompleteAuthSession().type === 'success';

export default function AuthCallback() {
  if (!handled) return <Redirect href="/" />;
  return (
    <View className="flex-1 items-center justify-center bg-canvas">
      <ActivityIndicator color={colors.signalInk} />
    </View>
  );
}
