# Scrapbook/01 API (Go)

The Go API behind Scrapbook/01. Signed-in users save shared links with an optional "What is it about?" description (Phase 1). A background job then files each described item into a category and links it to similar items (Phase 3), which the app draws as a graph (Phase 4). Users' own AI assistants can search the library over MCP (Phase 5). AI work is paid from prepaid credits (Phase 6).

## How items get a category

After a save or a description edit, a [River](https://riverqueue.com) job (stored in the same Postgres) runs `internal/classify`:

1. **Embed** the description (one small embedding) and store it with pgvector.
2. **Jev** (TypeSafe, `POST /v1/systemone`) picks among the user's categories, each described by a few recent items. It must reach `JEV_THRESHOLD` confidence, and it can answer "None of these".
3. Only if nothing fits, **Claude Haiku** (`claude-haiku-5-5`, low effort) names a new category in the description's language. If that name's embedding is very close to an existing category's (≥ 0.85), the existing one is used instead.
4. **Graph links**: the 5 nearest items by cosine similarity (≥ 0.45) become `edges`.
5. A push notification ("Saved to *Parenting*") goes to the user's phones through Expo.

Re-saving a link without a new description, or a category the user picked themselves, costs nothing: `described_at` only moves when the text changes, and `category_source = 'user'` is never overwritten. Corrections are stored in `category_feedback` for tuning. A sweep every minute re-queues anything missed, including items that waited for a credit top-up. Each missing key turns its step off (with a warning) instead of failing the boot.

## Run locally

```bash
docker compose up -d --wait          # Postgres (pgvector image) on port 54329
cp .env.example .env                 # then fill SUPABASE_URL
set -a; . ./.env; set +a
go run ./cmd/api                     # applies migrations on start
```

Tokens are Supabase Auth access tokens (`Authorization: Bearer <token>`). By default they are checked against the project's JWKS. Set `SUPABASE_JWT_SECRET` to use the legacy HS256 secret instead.

## Endpoints

All `/v1` routes need a token. Users only ever see their own items.

| Method | Path | Notes |
|---|---|---|
| `GET` | `/healthz` | No auth. Pings the database |
| `POST` | `/v1/items` | `{"url": "...", "description": "..."}`. `url` can be share-sheet text containing a link. The link is cleaned up (tracking params removed, `twitter.com` → `x.com`, `youtu.be` → `youtube.com/watch`), so saving the same post twice updates one item. Returns `201` when new, `200` when it already existed |
| `GET` | `/v1/items` | Newest first. `limit` (default 30, max 100), `cursor` (from `nextCursor`), `filter=undescribed` for the inbox |
| `GET` | `/v1/items/{id}` | |
| `PATCH` | `/v1/items/{id}` | `{"description": "..."}` (`null` moves it back to the inbox) and/or `{"categoryId": "..."}` (`null` for none). Picking a category stops automatic sorting for that item |
| `DELETE` | `/v1/items/{id}` | |
| `GET` | `/v1/graph` | Every item (newest 2000), category hub and link: `{nodes, links, truncated}`. Nodes are `item` or `category` (id `c:<uuid>`); links are `similar` (with a weight) or `category` |
| `GET` | `/v1/items/{id}/neighbors` | The local graph around one item: `depth=1` (default) or `2` hops over `similar` links, at most 60 items, plus their category hubs. The item comes first |
| `GET` | `/v1/categories` | Categories with item counts |
| `POST` | `/v1/categories` | `{"name": "..."}`. Returns the existing category if the name is taken (ignoring case) |
| `PUT` | `/v1/devices/{expoPushToken}` | `{"platform": "ios" \| "android"}`. Registers a phone for notifications |
| `DELETE` | `/v1/devices/{expoPushToken}` | On sign-out |

Items include `category`, `categorySource` (`auto` or `user`), `classifying` (true while a new description waits for its category) and `waitingForCredits` (true when it's waiting only because the user is out of credits). `GET /v1/items` also takes `categoryId`.

Errors look like `{"error": {"code": "invalid_url", "message": "..."}}`.

## Credits

Every AI call is metered: the clients record tokens and cost (list price × tokens) on a per-job meter, and the job charges the total to the user's ledger (`credit_ledger`, `credit_balances`). Amounts are micro-dollars of model cost.

- **New users** get `STARTER_CREDITS_USD` of budget the first time they're seen.
- **Packs**: `credits.Ledger.RecordPurchase(user, price, pack, paymentRef)` adds 80% of the price as budget, the split promised on the waitlist page. The payment reference is unique, so a payment notification processed twice credits once. Until a payment provider is wired in, use the CLI: `go run ./cmd/credits purchase -user <uuid> -price 5 -pack starter -ref <payment id>` (or `grant`, `balance`).
- **Out of credits**: saves are still kept, but classification waits; the sweep skips the user until a top-up, then sorts the waiting items within a minute. MCP search falls back to free keyword matching. A run can dip slightly below zero, since it's charged after it finishes.
- `GET /v1/credits` returns the balance, an estimate of saves left (from the user's average cost), and recent activity.

Typical costs at list prices: about $0.000014 to sort a save into an existing category (Jev + one embedding), about $0.0002 when Claude Haiku names a new category.

## MCP: the library in Claude, ChatGPT and other assistants

`/mcp` is a [Model Context Protocol](https://modelcontextprotocol.io) server (official Go SDK, Streamable HTTP, stateless). The assistant's reasoning runs on the user's own plan; this server only does retrieval, so a search costs one query embedding at most.

| Tool | What it does |
|---|---|
| `search_library` | Semantic search (query embedding + pgvector) over the user's items, optionally in one category. Falls back to keyword matching when embeddings are off |
| `get_item` | One item with its full note and its most similar items |
| `get_neighbors` | Items linked to an item in the graph, 1 or 2 hops out, and their categories |
| `save_item` | Saves a link with a note; it's sorted like any other save |

**Sign-in is OAuth 2.1 through Supabase Auth.** The server publishes protected-resource metadata at `/.well-known/oauth-protected-resource/mcp` naming `<SUPABASE_URL>/auth/v1` as the authorization server, answers unauthenticated calls with `401` + `WWW-Authenticate: Bearer resource_metadata=...`, and accepts the access tokens Supabase issues to MCP clients (same issuer and audience as the app's tokens). One-time setup in the Supabase dashboard:

1. Authentication → OAuth Server: turn it on, allow dynamic client registration (MCP clients register themselves), and set the authorization path to `/oauth/consent`.
2. Authentication → URL Configuration: Site URL = where the app's web build is hosted. The consent screen is `apps/mobile/src/app/oauth/consent.tsx`.
3. Use asymmetric JWT signing keys (RS256/ES256), as Supabase recommends for OAuth, and leave `SUPABASE_JWT_SECRET` empty so tokens are checked against the JWKS.
4. Set `PUBLIC_URL` to the API's public origin.

Then in Claude: Settings → Connectors → Add custom connector → `https://<api host>/mcp`.

## Run the whole flow without API keys

`cmd/fakeai` imitates Jev, the embeddings API and Claude with crude word matching, for local development only:

```bash
go run ./cmd/fakeai &
TYPESAFE_API_KEY=dev TYPESAFE_BASE_URL=http://127.0.0.1:8787 EMBEDDINGS_API_KEY=dev EMBEDDINGS_BASE_URL=http://127.0.0.1:8787 ANTHROPIC_API_KEY=dev ANTHROPIC_BASE_URL=http://127.0.0.1:8787 go run ./cmd/api
```

## Layout

- `cmd/api`: entry point (config, migrations, HTTP server, graceful shutdown).
- `db/migrations`: goose SQL migrations, embedded into the binary.
- `db/queries`: SQL for sqlc. Generated Go code is in `internal/db` (don't edit it).
- `internal/api`: handlers. `internal/auth`: token checks. `internal/links`: URL cleanup and platform detection.
- `internal/credits`: the prepaid ledger. `cmd/credits`: grant or record purchases by hand.
- `internal/graph`: builds graph and neighborhood responses. `internal/mcpserver`: the MCP tools and their OAuth wiring.
- `internal/classify`: the sorting pipeline. `internal/ai`: Jev, embeddings and Claude clients. `internal/jobs`: River setup. `internal/push`: Expo notifications.

## Regenerate queries

sqlc needs cgo for its Postgres parser, so run it through Docker:

```bash
MSYS_NO_PATHCONV=1 docker run --rm -v "$(pwd -W)":/src -w /src/db sqlc/sqlc:1.30.0 generate
```

On macOS or Linux, use `"$(pwd)"` instead of `"$(pwd -W)"` and drop `MSYS_NO_PATHCONV=1`.

## Tests

```bash
go test ./...
TEST_DATABASE_URL='postgres://scrapbook:scrapbook@localhost:54329/scrapbook?sslmode=disable' \
  go test -tags integration -p 1 ./internal/db/ ./internal/classify/ ./internal/mcpserver/ ./internal/credits/
```

The integration tests reset the schema, so point them at a disposable database.
