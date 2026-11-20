# Changelog

## 0.3.0

### Minor Changes

- [#13](https://github.com/franco-roura/scrapbook-01/pull/13) [`0d17a34`](https://github.com/franco-roura/scrapbook-01/commit/0d17a34b1508688881c92f2a77444654b354f6ff) Thanks [@franco-roura](https://github.com/franco-roura)! - Add optional Sentry error and performance reporting for the API, MCP server, worker, app, share sheet and landing page. It stays off until you set the new `SentryDsnServer`, `SentryDsnMobile` and `SentryDsnWebsite` secrets; see docs/observability.md.

## 0.2.0

### Minor Changes

- [#10](https://github.com/franco-roura/scrapbook-01/pull/10) [`a588f40`](https://github.com/franco-roura/scrapbook-01/commit/a588f409c4ea240d5771886cbc8ae013e08e4e20) Thanks [@franco-roura](https://github.com/franco-roura)! - Add a Conectar IA screen with a button per assistant: Claude opens its connector dialog already filled in, ChatGPT and other AIs expand the steps and copy the server link.

### Patch Changes

- [#11](https://github.com/franco-roura/scrapbook-01/pull/11) [`3246a02`](https://github.com/franco-roura/scrapbook-01/commit/3246a020e644ce9abc6492626dd77029b984f337) Thanks [@franco-roura](https://github.com/franco-roura)! - Fix the iOS share sheet opening empty: the share extension now starts React Native through Expo's factory so Expo modules load, draws its content inside the visible sheet, no longer opens the keyboard on its own, and shows a spinner while loading and an error message instead of a blank panel.

## 0.1.1

### Patch Changes

- [#8](https://github.com/franco-roura/scrapbook-01/pull/8) [`75460a9`](https://github.com/franco-roura/scrapbook-01/commit/75460a90d4f5666c58f7fd356d9b9bed6648f85b) Thanks [@franco-roura](https://github.com/franco-roura)! - Dependency updates: pgx 5.13, goose 3.30.1, keyfunc 3.9.2 and the Anthropic Go SDK 2.10 in the API; the latest Expo SDK 58 patches and Module/01 1.1.0 in the app; Tailwind 4.3.5, lucide-react 1.73 and Vite+ 1.6 on the landing page.

## 0.1.0

The first public release. Early: the API, the MCP server, sorting, credits and the web app are tested
and run locally; the iOS share extension, push notifications and a first AWS deploy are not yet
proven on real devices and accounts.

### Features

- **Save from any app.** Share a reel, TikTok, video or link to Scrapbook/01 and say in one line what
  it is. Undescribed saves wait in an inbox. iOS share extension and Android share intent.
- **Sorting.** Jev picks one of your categories from your description; Claude Haiku names a new one
  only when nothing fits. A notification says where each save went, and you can move it.
- **Graph.** Every save is embedded once and linked to its closest neighbors with pgvector. A map of
  the whole library, and each item's connections one or two hops out.
- **Link previews.** The title and picture of each saved link, fetched once from public addresses only
  and re-encoded before they're stored.
- **MCP server.** Connect Claude, ChatGPT or any MCP client to search the library, open items and
  their neighbors, and save new links.
- **Prepaid credits.** Every AI call is metered and charged to the user's credits.
- **Sign-in** with WorkOS AuthKit (PKCE, no secret in the app).
- **Landing page** with a WorkOS waitlist.
- **Self-hosting on AWS** with SST: two Go Lambdas, API Gateway, CloudFront and Neon Postgres.
- **The app and website in Uruguayan Spanish.**
