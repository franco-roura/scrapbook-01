# Changelog

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
