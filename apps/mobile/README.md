# Scrapbook/01 app (Expo)

Phase 2 of the app plan: the React Native app and the "What is it about?" share sheet. It talks to the Go API in `../../server`.

- **iOS:** sharing to Scrapbook/01 opens a small sheet on top of the other app (`expo-share-extension`, entry `index.share.js`). The sheet is signed in because the Supabase session is kept in the keychain under the app group `group.com.scrapbook01.app`, which the app and the extension share.
- **Android:** sharing opens the app on the same sheet (`expo-share-intent`, `src/share/AndroidShareListener.tsx` → `src/app/share.tsx`).
- **In the app:** library (newest first), the "Undescribed" inbox, item screen (describe, open, delete, change category), "Add link", email-code sign-in and sign-out.
- **Categories (Phase 3):** the server sorts each described item. Rows show "Sorting…" until it's done, and the item screen lets you move it (which stops automatic sorting for that item).
- **Graph (Phase 4):** "Graph" on the library opens the whole library as a force-directed graph: category hubs in Module/01's category colors, items around them, similar items linked. Tap a node to focus it (rings, crosshair, neighbors highlighted, the rest dimmed). Each item screen shows its connections 1 or 2 hops out; tapping a neighbor swaps the screen to it while the graph keeps its layout. Drawn with `react-force-graph-2d` in an Expo DOM component (`src/graph/GraphCanvas.tsx`), which runs in a web view on phones, with colors from `@franco_roura/module-01`'s `tokens.json`.
- **Connecting an assistant (Phase 5):** `src/app/oauth/consent.tsx` is the page Supabase's OAuth server sends people to when they add Scrapbook/01 to Claude or ChatGPT. It needs the web build hosted at the Supabase Site URL. Signed-out visitors go through sign-in and come back.
- **Notifications:** after sign-in the app registers its Expo push token, and the server sends "Saved to *category*". Tapping it opens the item. This needs an EAS project id (`bun x eas-cli init`) and a development build.

The UI follows Module/01. Its web components can't run in React Native, so `src/ui/module01.tsx` rebuilds the few parts used here (Panel with a notched corner, HudLabel, Chip, stadium Button, sunken Field), and `global.css` copies its tokens for Uniwind (Tailwind v4 for React Native).

## Run

```bash
cp .env.example .env.local   # API URL + Supabase URL and publishable key
bun install
bun run web                  # quickest check, in a browser
bun run android              # needs Android Studio / an emulator or device
```

The share sheet needs a development build, not Expo Go: `bun x eas-cli build --profile development`. iOS builds need macOS or EAS. For the web build, set `CORS_ORIGINS=http://localhost:8081` on the API.

## Checks

```bash
bun run lint && bun run typecheck && bun test
```

## Notes

- `expo-share-extension` lists Expo SDK 54 as its newest tested version and is marked "untested on the New Architecture" by expo-doctor. We're on SDK 58, so test the iOS extension on a device early.
- The iOS extension uses system fonts (it can't load fonts at runtime). The app loads Manrope, Michroma and Space Mono.
- Bundle ids (`com.scrapbook01.app`) and the app group are placeholders until you register real ones with Apple and Google.
