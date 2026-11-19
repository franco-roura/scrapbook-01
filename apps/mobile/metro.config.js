const { getSentryExpoConfig } = require('@sentry/react-native/metro');
const { withShareExtension } = require('expo-share-extension/metro');
const { withUniwindConfig } = require('uniwind/metro');

// getSentryExpoConfig is Expo's default config plus Sentry's debug ids for
// source maps; it does nothing at runtime when the DSN is empty.
// Uniwind must be the outermost wrapper.
module.exports = withUniwindConfig(withShareExtension(getSentryExpoConfig(__dirname)), {
  cssEntryFile: './global.css',
  dtsFile: './src/uniwind-types.d.ts',
});
