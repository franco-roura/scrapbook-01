---
"scrapbook01": patch
---

Fix the iOS share sheet opening empty: the share extension now starts React Native through Expo's factory, so Expo modules load, and it shows a spinner while loading and an error message instead of a blank panel.
