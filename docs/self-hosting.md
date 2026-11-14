# Deploying Scrapbook/01 to AWS

The whole stack is defined in [`sst.config.ts`](sst.config.ts) with [SST](https://sst.dev). At hobby scale it costs about $0 a month. Local development doesn't use any of it: `go run ./cmd/api` in `server/` still runs everything in one process.

## What gets deployed

| Piece | Where | Cost at hobby scale |
|---|---|---|
| API + MCP (`server/cmd/lambda-api`) | Lambda (Go, arm64) behind an API Gateway HTTP API | Lambda's always-free tier (1M requests, 400k GB-s a month). HTTP API: $1 per million requests |
| Classification (`server/cmd/lambda-worker`) | Lambda, invoked asynchronously by the API for each save. Lambda retries failures twice | Lambda free tier |
| Hourly sweep (missed items, credit top-ups) | EventBridge Scheduler → the worker | Free (14M a month) |
| Web app (Expo web build of `apps/mobile`) | S3 + CloudFront | CloudFront's always-free tier (1 TB, 10M requests) |
| Landing page (`site/index.html`) | S3 + CloudFront | Same |
| Postgres + pgvector | [Neon](https://neon.com) free tier, outside AWS | $0: 100 compute-hours a month, 1 GB storage per project, sleeps after 5 minutes idle |
| Logs | CloudWatch, kept 1 week | 5 GB a month free |

Nothing runs (or bills) while idle: there's no VPC, NAT gateway, load balancer or database instance.

**What would start costing money:**
- Neon past 100 compute-hours a month. Anything that queries it more than about every 5 minutes, all day, keeps it awake (that's why the sweep is hourly, not every minute).
- Adding a VPC (a NAT gateway is about $32 a month).
- Real traffic: about $1 per million API requests, plus Lambda time past the free tier.

## One-time setup

1. **AWS credentials.** Create an IAM user (or SSO profile) that can deploy, and configure it locally (`aws configure`, or `AWS_PROFILE`). SST uses the standard AWS credential chain.
2. **Neon.** Create a free project in **AWS us-east-1**, the same region as the Lambdas. On the dashboard's Connect panel, copy two connection strings:
   - the **pooled** one (host contains `-pooler`), for the Lambdas;
   - the **direct** one, for migrations (they need a session, which the pooler doesn't give).

   pgvector needs no setup: the first migration runs `CREATE EXTENSION vector`.
3. **Install** (repo root):
   ```bash
   bun install
   ```
4. **Secrets** for the `prod` stage. The three required ones:
   ```bash
   bun sst secret set DatabaseUrl "postgres://...-pooler.../neondb?sslmode=require" --stage prod
   bun sst secret set WorkosClientId client_... --stage prod
   bun sst secret set WorkosAuthkitDomain https://<yours>.authkit.app --stage prod
   ```
   Optional ones (leave unset to turn that step off): `TypesafeApiKey`, `EmbeddingsApiKey`, `AnthropicApiKey`, `ExpoAccessToken`.
5. **Deploy.** Migrations run first, against the direct URL:
   ```bash
   MIGRATE_DATABASE_URL="postgres://...direct.../neondb?sslmode=require" bun run deploy
   ```
   The first deploy takes a few minutes, mostly CloudFront. It prints `api`, `web` and `landing` URLs.
6. **WorkOS**, using those URLs. In Sandbox while testing, in Production when going live:
   - Redirects: add `<web>/auth/callback`.
   - Sessions → CORS: add `<web>`.
   - Connect → Resource Indicators: add `<api>/mcp` as the default.
   - Connect → Configuration: Client ID Metadata Documents and dynamic client registration on.

## Every later deploy

```bash
MIGRATE_DATABASE_URL="postgres://...direct..." bun run deploy
```

`bun run diff` previews the changes. A `dev` stage (`bun sst deploy --stage dev`, with its own secrets) is a full separate copy; remove it with `bun sst remove --stage dev`. The `prod` stage is protected from `sst remove` and keeps its resources if removed from the config.

## Checking it works

1. `curl <api>/healthz` returns `{"status":"ok"}` (the first call after a quiet spell is slower: Lambda and Neon both wake up).
2. Open `<web>`, sign in, save a link with a description. It gets a category within seconds. The worker's logs are in CloudWatch, under the log group of the `Worker` function.
3. `<api>/.well-known/oauth-protected-resource/mcp` names your AuthKit domain.
4. In Claude: Settings → Connectors → Add custom connector → `<api>/mcp`.

## How it maps to the local setup

| | Local (`go run ./cmd/api`) | AWS |
|---|---|---|
| Entry point | `server/cmd/api` | `server/cmd/lambda-api`, `server/cmd/lambda-worker` |
| Shared wiring | `server/internal/app` | same |
| Background jobs | River, in Postgres, in the same process | Async Lambda invoke + hourly sweep |
| Migrations | on start | `server/cmd/migrate`, before each deploy |
| Settings | `server/.env` | SST secrets, passed as environment variables |
