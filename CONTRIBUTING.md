# Contributing

Thanks for helping out. Bug reports, fixes and ideas are all welcome.

## Getting started

Follow [Getting started](docs/getting-started.md). You need Go, Bun and Docker, and a free WorkOS
sandbox for sign-in. To work without one, set `DEV_JWT_SECRET` in `server/.env` (development only) and
make a token with `scripts/docs/token.ts`. A fake AI server (`go run ./cmd/fakeai`) stands in for
every AI provider, so you don't need any keys.

[Development](docs/development.md) covers the layout, the checks and database changes.

## Before you open a pull request

- **Go:** run `go vet ./...` and `go test ./...` in `server/`, plus the integration tests (they need
  Docker; see [Checks](docs/development.md#checks)).
- **App:** run `bun run lint && bun run typecheck && bun test` in `apps/mobile/`.
- If you changed a `.sql` file, regenerate the sqlc code and commit it.
- If you changed what the screens look like, refresh the screenshots (`bun run docs:demo`, then
  `docs:seed` and `docs:screenshots`). If you changed `sst.config.ts`, update the map in
  `scripts/docs/architecture.ts` and run `docs:diagram` and `docs:render`.
- Keep each pull request to one change, and say what changed and why.

## Ground rules

- **No scraping.** Sorting works from the description the user wrote. Don't fetch post contents from
  social networks.
- **Keep AI cheap.** Prefer the smallest model that can answer, call it once per change of input, and
  meter every call (`ai.RecordUsage`) so it's charged to the user's credits.
- **Every query filters by the signed-in user**, taken from the token, never from the request.
- **The API stays Lambda-safe:** stateless, configured only from environment variables, with
  background work behind the `EnqueueClassify` interface. Don't start goroutines that outlive a
  request.
- **Migrations are additive**, since deploys migrate before the new code is live.
- **The UI is styled through Module/01 tokens** (`global.css`, `src/ui/module01.tsx`), not ad hoc
  colors. Copy is English, in sentence case.
- **Tests, fixtures and screenshots use made-up data.** Never real people's saves.

## Bugs and security issues

Open an issue for bugs, with the steps to reproduce them. For anything security-related, follow
[SECURITY.md](SECURITY.md) instead of opening an issue.
