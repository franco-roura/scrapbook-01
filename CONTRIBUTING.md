# Contributing

Thanks for helping out. Bug reports, fixes and ideas are all welcome. Everyone taking part follows
the [code of conduct](CODE_OF_CONDUCT.md).

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
- **Website:** run `bun run check && bun run test && bun run build` in `apps/website/`.
- If you changed a `.sql` file, regenerate the sqlc code and commit it.
- Add a changeset (see [Versions and changesets](#versions-and-changesets)).
- If you changed what the screens look like, refresh the screenshots (`bun run docs:demo`, then
  `docs:seed` and `docs:screenshots`). If you changed `sst.config.ts`, update the map in
  `scripts/docs/architecture.ts` and run `docs:diagram` and `docs:render`.
- Keep each pull request to one change, and say what changed and why.

CI runs all of these on every pull request.

## Versions and changesets

The API, the app and the website ship together under one version, tagged `vX.Y.Z`. The bump says what
someone running their own copy has to do to upgrade:

| Bump | Before 1.0 | From 1.0 on |
| --- | --- | --- |
| **Major** | (not used) | Upgrading needs manual steps: a new secret or environment variable, a manual migration, a breaking API or deploy change |
| **Minor** | Upgrading needs manual steps, as above | New features; a plain redeploy is enough |
| **Patch** | Everything else: features and fixes | Fixes |

Every pull request adds a changeset, a small Markdown file in `.changeset/` that becomes its line in
the [changelog](CHANGELOG.md). From the repository root:

```bash
bun install
bun x changeset          # pick the bump and write one sentence for the release notes
bun x changeset --empty  # when nothing user-visible changed (docs, CI, refactors)
```

Write the sentence for users ("Saves from Threads now show their picture"), not for reviewers. When
changesets reach `main`, a "Version packages" pull request collects them; merging it tags the release.

## Adding an MCP tool

The MCP server is the easiest place to extend Scrapbook/01: each tool is one function.

1. In [`server/internal/mcpserver/mcpserver.go`](server/internal/mcpserver/mcpserver.go), add input and
   output structs. `jsonschema` tags become the descriptions the AI client sees.
2. Write the handler, `func (s *Server) myTool(ctx, req, in MyInput) (*mcp.CallToolResult, MyOutput, error)`.
   Start with `user, err := userFrom(req)` and filter every query by that user.
3. Register it in `addTools` with a one-sentence description that says when to use it, and
   `readOnly` annotations unless it writes.
4. If it needs a new query, add it in `server/db/queries` and regenerate sqlc.
5. Cover it in `TestTools` in `mcpserver_integration_test.go`, including another user's items not
   showing up.
6. List it in [docs/ai-assistants.md](docs/ai-assistants.md), and add a minor changeset.

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
  colors. Copy in the app and website is Uruguayan Spanish, in sentence case; code, comments and
  docs are English.
- **Tests, fixtures and screenshots use made-up data.** Never real people's saves.

## Bugs and security issues

Open an issue for bugs, with the steps to reproduce them. For anything security-related, follow
[SECURITY.md](SECURITY.md) instead of opening an issue.
