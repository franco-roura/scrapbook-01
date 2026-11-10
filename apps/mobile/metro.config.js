const { getDefaultConfig } = require('expo/metro-config');
const { withShareExtension } = require('expo-share-extension/metro');
const { withUniwindConfig } = require('uniwind/metro');

// Uniwind must be the outermost wrapper.
module.exports = withUniwindConfig(withShareExtension(getDefaultConfig(__dirname)), {
  cssEntryFile: './global.css',
  dtsFile: './src/uniwind-types.d.ts',
});
