// Entry for the iOS share extension (a separate, memory-limited bundle).
// Keep imports here small: no router, no graph, no fonts.
import { AppRegistry } from 'react-native';

import './global.css';
import ShareExtensionRoot from './src/share/ShareExtensionRoot';

// The first argument must be "shareExtension" (expo-share-extension).
AppRegistry.registerComponent('shareExtension', () => ShareExtensionRoot);
