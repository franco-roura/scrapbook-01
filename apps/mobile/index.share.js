// Entry for the iOS share extension (a separate, memory-limited bundle).
// Keep imports here small: no router, no graph, no fonts.
import { AppRegistry } from 'react-native';

import './global.css';
import { initSentry } from './src/lib/sentry';
import ShareExtensionRoot from './src/share/ShareExtensionRoot';

// Errors and timing for the extension; a no-op without EXPO_PUBLIC_SENTRY_DSN.
initSentry({ surface: 'share-extension' });

// The first argument must be "shareExtension" (expo-share-extension).
AppRegistry.registerComponent('shareExtension', () => ShareExtensionRoot);
