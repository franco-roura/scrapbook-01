# Scrapbook/01: rules for contributors and coding agents

Scrapbook/01 saves links shared from any app, sorts them into categories and links similar saves into
a graph. This file is the short version of [CONTRIBUTING.md](CONTRIBUTING.md) and
[docs/development.md](docs/development.md); read those for the details.

## Layout

- `server/`: Go API, MCP server, sorting worker, migrations (goose), queries (sqlc) and CLIs.
- `apps/mobile/`: Expo app for iOS, Android and web, including the share sheet and the graph.
  [apps/mobile/AGENTS.md](apps/mobile/AGENTS.md) has the Expo rules.
- `apps/website/`: the landing page and waitlist (Vite+, React, Module/01).
- `sst.config.ts`: the AWS deployment. `scripts/docs/`: demo data, screenshots and diagrams.

## Checks before you call a change done

- `server/`: `go vet ./...` and `go test ./...`; the integration tests when you touch queries,
  sorting, credits or MCP (see docs/development.md).
- `apps/mobile/`: `bun run lint && bun run typecheck && bun test`.
- `apps/website/`: `bun run check && bun run test && bun run build`.
- Changed a `.sql` file: regenerate sqlc and commit `server/internal/db`. Never edit it by hand.
- Add a changeset (`bun x changeset`, or `bun x changeset --empty` when nothing user-visible changed).

## Ground rules

- No scraping. Sorting reads the description the user wrote.
- Every query filters by the user from the token, never from the request.
- The API stays Lambda-safe: stateless, configured from environment variables, no goroutines that
  outlive a request.
- Migrations are additive.
- Meter every AI call (`ai.RecordUsage`) and prefer the smallest model that works.
- UI uses Module/01 tokens. The app and website copy is Uruguayan Spanish; code, comments and docs
  are English.
- Tests, fixtures, docs and screenshots use made-up data only. Never real people's saves, names,
  emails or links.
- Commit messages say what changed in the imperative ("Add …", "Fix …"), one change per commit.
