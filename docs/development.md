[← README](../README.md) · [Features](features.md) · [Getting started](getting-started.md) · [Sorting](sorting.md) · [AI assistants](ai-assistants.md) · [Self-hosting](self-hosting.md) · [Architecture](architecture.md) · [Observability](observability.md) · **Development**

# Development

How the repository is laid out, how to run the checks, and how the docs' screenshots and diagrams are
made.

**On this page:** [Layout](#layout) · [Run it](#run-it) · [Checks](#checks) ·
[Database changes](#database-changes) · [Docs screenshots and diagrams](#docs-screenshots-and-diagrams)

## Layout

| Path | What's there |
| --- | --- |
| [`apps/mobile`](../apps/mobile) | The Expo app (iOS, Android, web), the share sheet, and the graph. [README](../apps/mobile/README.md) |
| [`server`](../server) | The Go API, MCP server, sorting worker, migrations and CLIs. [README](../server/README.md) |
| [`apps/website`](../apps/website) | The landing page and waitlist: Vite+, React and Module/01. [README](../apps/website/README.md) |
| [`sst.config.ts`](../sst.config.ts) | The AWS deployment |
| [`scripts/docs`](../scripts/docs) | The demo, screenshot and diagram scripts |
| [`docs`](.) | These pages, and their images |

## Run it

[Getting started](getting-started.md) runs everything locally. In short, from `server/`:
`docker compose up -d --wait`, `go run ./cmd/fakeai` and `go run ./cmd/api`; from `apps/mobile/`:
`bun run web`.

For local work without a WorkOS account, set `DEV_JWT_SECRET` in `server/.env`: the API then also
accepts HS256 tokens with issuer `scrapbook01-dev` signed with it. `scripts/docs/token.ts` makes one.
Never set it in production.

## Checks

**Go** (from `server/`):

```bash
go vet ./...
go test ./...
```

The integration tests reset their database's schema, so they use their own (`scrapbook_test`), never
the one `go run ./cmd/api` uses. Create it once, then run them:

```bash
docker compose exec db psql -U scrapbook -c 'CREATE DATABASE scrapbook_test'
TEST_DATABASE_URL='postgres://scrapbook:scrapbook@localhost:54329/scrapbook_test?sslmode=disable' \
  go test -tags integration -p 1 ./internal/db/ ./internal/classify/ ./internal/mcpserver/ ./internal/credits/
```

**App** (from `apps/mobile/`):

```bash
bun run lint && bun run typecheck && bun test
```

**Deployment** (repository root, needs AWS credentials): `bun run diff`.

## Database changes

1. Add a migration in `server/db/migrations` (goose). Keep it additive: deploys migrate before the new
   code is live, so the old code briefly runs on the new schema.
2. Write the queries in `server/db/queries`.
3. Regenerate the Go code with sqlc. It needs cgo, so it runs in Docker (from `server/`):
   ```bash
   MSYS_NO_PATHCONV=1 docker run --rm -v "$(pwd -W)":/src -w /src/db sqlc/sqlc:1.30.0 generate
   ```
   On macOS or Linux, use `"$(pwd)"` and drop `MSYS_NO_PATHCONV=1`. Never edit `server/internal/db`
   by hand.

## Docs screenshots and diagrams

The screenshots come from a made-up library, captured at phone size with Playwright, so they can be
redone whenever the app changes. From the repository root, with the Docker Postgres running:

```bash
bun install
bun x playwright install chromium   # once
bun run docs:demo                   # demo API, fake AI and web app; leave it running
```

Then, in another terminal:

```bash
bun run docs:seed          # fills the demo library
bun run docs:screenshots   # writes docs/screenshots/*.png
bun run docs:diagram       # writes docs/brand/architecture.svg
bun run docs:render        # renders the icon, map, hero and social preview to PNG
```

`docs:demo` uses its own database (`scrapbook_demo`, reset each run) and ports 18080, 8788 and 8090, so
it doesn't disturb a local setup. The SVGs import Google Fonts, which GitHub doesn't load inside SVG
images; that's why the docs embed the rendered PNGs. Edit the map in
[`scripts/docs/architecture.ts`](../scripts/docs/architecture.ts) rather than the SVG.

GitHub's link preview image is `docs/brand/social.png`. It isn't picked up from the repository:
upload it under the repository's **Settings → General → Social preview**.

---

[← Architecture](architecture.md) · [Back to the README](../README.md)
