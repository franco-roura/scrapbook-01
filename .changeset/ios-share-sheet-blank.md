---
"scrapbook01": patch
---

Fix the iOS share sheet opening empty: the share extension now starts React Native through Expo's factory so Expo modules load, draws its content inside the visible sheet, no longer opens the keyboard on its own, and shows a spinner while loading and an error message instead of a blank panel.
