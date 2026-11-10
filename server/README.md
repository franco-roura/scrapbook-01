# Scrapbook/01 API (Go)

Phase 1 of the app plan: a Go HTTP API where signed-in users save shared links, with an optional "What is it about?" description. Classification, embeddings, the graph and MCP come in later phases.

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
| `PATCH` | `/v1/items/{id}` | `{"description": "..."}`, or `null` to move it back to the inbox |
| `DELETE` | `/v1/items/{id}` | |
| `GET` | `/v1/categories` | Categories with item counts. Empty until Phase 3 |

Errors look like `{"error": {"code": "invalid_url", "message": "..."}}`.

## Layout

- `cmd/api`: entry point (config, migrations, HTTP server, graceful shutdown).
- `db/migrations`: goose SQL migrations, embedded into the binary.
- `db/queries`: SQL for sqlc. Generated Go code is in `internal/db` (don't edit it).
- `internal/api`: handlers. `internal/auth`: token checks. `internal/links`: URL cleanup and platform detection.

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
  go test -tags integration ./internal/db/
```

The integration tests reset the schema, so point them at a disposable database.
