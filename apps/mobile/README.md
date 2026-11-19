# Scrapbook/01 app (Expo)

Phase 2 of the app plan: the React Native app and the "¿De qué se trata?" (what is it about?) share sheet. It talks to the Go API in `../../server`.

The app is in Uruguayan Spanish (voseo, `es-UY` dates and numbers, dollars shown as `US$`). Strings live in the components; there is no translation layer, since it ships in one language. The server's messages that people can see (errors the app shows, the push notification) are in Spanish too; API contract errors meant for developers stay in English.

- **iOS:** sharing to Scrapbook/01 opens a small sheet on top of the other app (`expo-share-extension`, entry `index.share.js`). The sheet is signed in because the session is kept in the keychain under the app group `group.com.scrapbook01.app`, which the app and the extension share. The extension refreshes the token itself with one HTTP call (`src/lib/session.ts`) and never loads the sign-in code.
- **Android:** sharing opens the app on the same sheet (`expo-share-intent`, `src/share/AndroidShareListener.tsx` → `src/app/share.tsx`).
- **In the app:** library (newest first), the "Sin describir" inbox, item screen (describe, open, delete, change category), "Agregar link", sign-in and sign-out.
- **Sign-in:** WorkOS AuthKit's hosted page, opened in a browser sheet (`src/lib/signIn.ts`, OAuth code flow with PKCE, no secret in the app). People get a 6-digit code by email, sent by WorkOS. Tokens last a few minutes and are refreshed when the API needs one; refresh tokens are single use, so the app and share extension re-read the shared session before refreshing.
- **Categories (Phase 3):** the server sorts each described item. Rows show "Ordenando…" until it's done, and the item screen lets you move it (which stops automatic sorting for that item).
- **Graph (Phase 4):** "Mapa" on the library opens the whole library as a force-directed graph: category hubs in Module/01's category colors, items around them, similar items linked. Tap a node to focus it (rings, crosshair, neighbors highlighted, the rest dimmed). Each item screen shows its connections 1 or 2 hops out; tapping a neighbor swaps the screen to it while the graph keeps its layout. Drawn with `react-force-graph-2d` in an Expo DOM component (`src/graph/GraphCanvas.tsx`), which runs in a web view on phones, with colors from `@franco_roura/module-01`'s `tokens.json`.
- **Connecting an assistant (Phase 5):** AuthKit shows the sign-in and "Allow access?" screens when someone adds Scrapbook/01 to Claude or ChatGPT, so the app has no page for it.
- **Credits (Phase 6):** "Créditos" in the library header shows the model budget left, roughly how many saves it covers, how the 80/20 split works, and recent activity. Buying packs isn't wired to a payment provider yet (the button says "muy pronto").
- **Notifications:** after sign-in the app registers its Expo push token, and the server sends "Guardado en *category*". Tapping it opens the item. This needs an EAS project id (`bun x eas-cli init`) and a development build.

The UI follows Module/01. Its web components can't run in React Native, so `src/ui/module01.tsx` rebuilds the few parts used here (Panel with a notched corner, HudLabel, Chip, stadium Button, sunken Field), and `global.css` copies its tokens for Uniwind (Tailwind v4 for React Native).

## Run

```bash
cp .env.example .env.local   # API URL + WorkOS client id
bun install
bun run web                  # quickest check, in a browser
bun run android              # needs Android Studio / an emulator or device
```

The share sheet needs a development build, not Expo Go: `bun x eas-cli build --profile development`. iOS builds need macOS or EAS. For the web build, set `CORS_ORIGINS=http://localhost:8081` on the API, and add the web origin to WorkOS's CORS allow list (Authentication → Sessions → Configure CORS), since the browser calls WorkOS directly.

In the WorkOS dashboard (Redirects), add the redirect URIs: `scrapbook01://auth/callback` for the phone builds and `http://localhost:8081/auth/callback` (sandbox) or your web origin's `/auth/callback` for the web build.

## Install on phones (EAS)

Builds run on Expo's servers (`eas.json`), so this works from Windows, iOS included. Builds point at the deployed API; the WorkOS client id comes from an EAS environment variable.

One-time setup, from `apps/mobile`:

```bash
bun x eas-cli login
bun x eas-cli init                 # creates the EAS project, writes its id to app.json
bun x eas-cli update:configure     # adds the update URL to app.json
bun x eas-cli env:create --name EXPO_PUBLIC_WORKOS_CLIENT_ID --value client_… --environment production --visibility plaintext
```

- **Android:** `bun x eas-cli build -p android --profile preview` builds an APK and prints a link. Open it on the phone, allow installing from the browser, done. It stays installed.
- **iPhone:** needs a paid Apple Developer account. `bun x eas-cli build -p ios --profile production --auto-submit` registers the bundle ids and app group with Apple, builds, and uploads to TestFlight. Add testers in App Store Connect (TestFlight → Internal Testing) and install from the TestFlight app. A build lasts 90 days.

### iPhone without a paid account (Mac + Xcode)

A free Apple ID signs builds for 7 days, then the app stops opening until you install it again. Fine for a developer's own phone; send everyone else the Android APK or TestFlight.

1. On the Mac, `bun install` in `apps/mobile`, then create `.env.local` with the deployed values (local builds read it, not `eas.json`):
   ```
   EXPO_PUBLIC_API_URL=https://l1t92cu8hj.execute-api.sa-east-1.amazonaws.com
   EXPO_PUBLIC_WORKOS_CLIENT_ID=client_…   # production WorkOS client id
   ```
2. Xcode → Settings → Accounts: sign in with your Apple ID (this makes a free "Personal Team").
3. `bun x expo prebuild -p ios`, then `xed ios`. In Signing & Capabilities, pick the Personal Team for both targets, the app and the share extension.
4. Plug in the iPhone and turn on Settings → Privacy & Security → Developer Mode.
5. `bun x expo run:ios --device --configuration Release`. Release bundles the JavaScript, so the app runs without the Mac.
6. On the phone, trust the profile: Settings → General → VPN & Device Management.

Every 7 days, plug in and repeat step 5. Free teams have limits: push notifications aren't available (remove the capability if Xcode objects), and if App Groups are refused, the share sheet can't see the app's sign-in. If `com.scrapbook01.app` is taken, change `ios.bundleIdentifier` in `app.json` to something of your own.

**Updates:** JavaScript-only changes reach both phones without a rebuild: `bun x eas-cli update --channel production -m "what changed"`. Phones pick it up on the next launch after it downloads. Native changes (a new module, the share extension, `app.json` plugins) change the runtime fingerprint and need a new build; updates never reach a build with a different fingerprint.

## Icons

`assets/icon.png` (iOS), the three `android-icon-*.png` layers, `splash-icon.png` and `favicon.png` are rendered from the brand icon, `docs/brand/icon.svg`. After changing it, run `bun run app:icons` from the repository root. Phones only show a new icon after a new build.

## Checks

```bash
bun run lint && bun run typecheck && bun test
```

## Notes

- `expo-share-extension` lists Expo SDK 54 as its newest tested version and is marked "untested on the New Architecture" by expo-doctor. We're on SDK 58, so test the iOS extension on a device early.
- The iOS extension uses system fonts (it can't load fonts at runtime). The app loads Manrope, Michroma and Space Mono.
- Bundle ids (`com.scrapbook01.app`) and the app group are placeholders until you register real ones with Apple and Google.
