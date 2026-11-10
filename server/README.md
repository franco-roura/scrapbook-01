# Scrapbook/01 API (Go)

The Go API behind Scrapbook/01. Signed-in users save shared links with an optional "What is it about?" description (Phase 1). A background job then files each described item into a category and links it to similar items (Phase 3). The graph view and MCP come in later phases.

## How items get a category

After a save or a description edit, a [River](https://riverqueue.com) job (stored in the same Postgres) runs `internal/classify`:

1. **Embed** the description (one small embedding) and store it with pgvector.
2. **Jev** (TypeSafe, `POST /v1/systemone`) picks among the user's categories, each described by a few recent items. It must reach `JEV_THRESHOLD` confidence, and it can answer "None of these".
3. Only if nothing fits, **Claude Haiku** (`claude-haiku-5-5`, low effort) names a new category in the description's language. If that name's embedding is very close to an existing category's (≥ 0.85), the existing one is used instead.
4. **Graph links**: the 5 nearest items by cosine similarity (≥ 0.45) become `edges`.
5. A push notification ("Saved to *Parenting*") goes to the user's phones through Expo.

Re-saving a link without a new description, or a category the user picked themselves, costs nothing: `described_at` only moves when the text changes, and `category_source = 'user'` is never overwritten. Corrections are stored in `category_feedback` for tuning. A sweep every 5 minutes re-queues anything missed. Each missing key turns its step off (with a warning) instead of failing the boot.

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
| `GET` | `/v1/categories` | Categories with item counts |
| `POST` | `/v1/categories` | `{"name": "..."}`. Returns the existing category if the name is taken (ignoring case) |
| `PUT` | `/v1/devices/{expoPushToken}` | `{"platform": "ios" \| "android"}`. Registers a phone for notifications |
| `DELETE` | `/v1/devices/{expoPushToken}` | On sign-out |

Items include `category`, `categorySource` (`auto` or `user`) and `classifying` (true while a new description waits for its category). `GET /v1/items` also takes `categoryId`.

Errors look like `{"error": {"code": "invalid_url", "message": "..."}}`.

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
  go test -tags integration -p 1 ./internal/db/ ./internal/classify/
```

The integration tests reset the schema, so point them at a disposable database.
