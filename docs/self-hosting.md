[← README](../README.md) · [Features](features.md) · [Getting started](getting-started.md) · [Sorting](sorting.md) · [AI assistants](ai-assistants.md) · **Self-hosting** · [Architecture](architecture.md) · [Observability](observability.md) · [Development](development.md)

# Host it yourself on AWS

Rather not run anything? [Join the waitlist](https://d2gh6vwoycfx90.cloudfront.net) for the hosted app
instead. This guide is for running your own copy in your AWS account.

The whole stack is defined in [`sst.config.ts`](../sst.config.ts) with [SST](https://sst.dev) and
deploys to your own AWS account with one command. At personal scale it costs about two cents a month
(see [what it costs](architecture.md#what-it-costs)).

**On this page:** [What you need](#what-you-need) · [One-time setup](#one-time-setup) ·
[Deploy](#deploy) · [After the first deploy](#after-the-first-deploy) · [Check it works](#check-it-works) ·
[Credits](#credits) · [Updates, stages and removal](#updates-stages-and-removal) ·
[Troubleshooting](#troubleshooting)

## What you need

- An **AWS account**, with credentials on your machine (`aws configure`, an SSO profile or
  `AWS_PROFILE`). SST uses the standard AWS credential chain.
- A free **[Neon](https://neon.com)** project, for Postgres.
- A **[WorkOS](https://workos.com)** environment, for sign-in. The sandbox works for trying it; use a
  production environment for real users.
- Go, Bun, and this repository.
- AI keys, all optional: [Jev](https://typesafe.ai), an OpenAI-compatible embeddings API
  ([OpenRouter](https://openrouter.ai) by default) and [Anthropic](https://console.anthropic.com).
- Optionally, a **[Sentry](https://sentry.io)** account (the free plan is enough) for errors and
  performance. See [Observability](observability.md).

## One-time setup

1. **Neon.** Create a project in **AWS sa-east-1** (São Paulo), the region the Lambdas run in. From its Connect
   panel, copy two connection strings:
   - the **pooled** one (the host contains `-pooler`): the Lambdas use it;
   - the **direct** one: migrations use it, since they need a session the pooler doesn't keep.

   pgvector needs no setup: the first migration enables it.
2. **Install** (repository root):
   ```bash
   bun install
   ```
3. **Secrets**, for the `prod` stage. Three are required:
   ```bash
   bun sst secret set DatabaseUrl "postgres://…-pooler…/neondb?sslmode=require" --stage prod
   bun sst secret set WorkosClientId client_… --stage prod
   bun sst secret set WorkosAuthkitDomain https://<yours>.authkit.app --stage prod
   ```
   The rest are optional; leaving one unset turns its step off: `TypesafeApiKey`, `EmbeddingsApiKey`,
   `AnthropicApiKey`, `ExpoAccessToken`, and `WorkosApiKey` (the WorkOS secret key, `sk_…`, which
   turns on the landing page's waitlist form; see [Waitlist](#waitlist)). The first `sst secret set` also creates SST's state storage
   in your account.
4. **Sentry (optional).** Skip this step to send nothing anywhere. Otherwise create three Sentry projects
   (Go, React Native and React), named `scrapbook01-server`, `scrapbook01-mobile` and
   `scrapbook01-website`, and set each one's DSN:
   ```bash
   bun sst secret set SentryDsnServer https://…@….ingest.sentry.io/… --stage prod
   bun sst secret set SentryDsnMobile https://…@….ingest.sentry.io/… --stage prod
   bun sst secret set SentryDsnWebsite https://…@….ingest.sentry.io/… --stage prod
   ```
   Each DSN is independent: set only the ones you want. For readable stack traces in the web builds,
   also set `SentryOrg` (your organization's slug) and `SentryAuthToken` (an organization token with
   the default scopes). If you named your projects differently, deploy with
   `SENTRY_PROJECT_MOBILE` and `SENTRY_PROJECT_WEBSITE` set to their slugs.

## Deploy

```bash
MIGRATE_DATABASE_URL="postgres://…direct…/neondb?sslmode=require" bun run deploy
```

`bun run deploy` applies the migrations first (`server/cmd/migrate`, against the direct URL), then runs
`sst deploy --stage prod`. The first deploy takes a few minutes, mostly CloudFront, and ends by printing
three URLs: `api`, `web` and `landing`.

## After the first deploy

Tell WorkOS about the new addresses:

- **Redirects:** add `<web>/auth/callback`.
- **Authentication → Sessions → CORS:** add `<web>`.
- **Connect → Resource Indicators:** add `<api>/mcp` as the default.
- **Connect → Configuration:** turn on Client ID Metadata Documents and dynamic client registration.

For the phone builds, set `EXPO_PUBLIC_API_URL=<api>` when building, and add
`scrapbook01://auth/callback` to the redirects.
With Sentry, also add `EXPO_PUBLIC_SENTRY_DSN` (the mobile project's DSN) to the EAS environment, and
`SENTRY_AUTH_TOKEN` as an EAS secret if you want readable native crash reports.

## Check it works

1. `curl <api>/healthz` returns `{"status":"ok"}`. The first call after a quiet spell is slower, since
   both the Lambda and Neon wake up.
2. Open `<web>`, sign in, and save a link with a description. It gets a category within seconds.
3. `<api>/.well-known/oauth-protected-resource/mcp` names your AuthKit domain.
4. Connect Claude to `<api>/mcp` (see [AI assistants](ai-assistants.md)).

Logs are in CloudWatch, one log group per function (`Api…` and `Worker…`), kept for a week.

## Credits

Sorting is metered against prepaid credits. New users get `STARTER_CREDITS_USD` ($0.10 by default).
Until a payment provider is wired in, grant or record purchases from your machine, against the direct
database URL:

```bash
cd server
DATABASE_URL="postgres://…direct…" go run ./cmd/credits balance  -user user_01H…
DATABASE_URL="postgres://…direct…" go run ./cmd/credits grant    -user user_01H… -usd 1 -note "friend"
DATABASE_URL="postgres://…direct…" go run ./cmd/credits purchase -user user_01H… -price 5 -pack starter -ref <payment id>
```

`-user` takes the WorkOS user id (shown in the WorkOS dashboard). A purchase adds 80% of the price as
model budget, and a payment reference only ever counts once. Hosting it just for yourself? Grant
yourself a few dollars, and you pay the AI providers directly.

## Updates, stages and removal

- **Update:** pull, then run the same deploy command. Migrations run first, so for a moment the
  old code runs on the new schema; that's why migrations here only add, never rename or drop.
- **Preview:** `bun run diff` shows what a deploy would change.
- **A second copy:** `bun sst deploy --stage dev` deploys a separate stage with its own secrets (set
  them with `--stage dev`). Remove it with `bun sst remove --stage dev`.
- **The prod stage is protected:** `sst remove` refuses it, and its resources are kept if they're ever
  dropped from the config.

## Troubleshooting

- **Sign-in fails with a CORS error in the browser:** the web address isn't on WorkOS's CORS list.
- **The API answers 401 to the app:** its log has an "auth rejected" line with the reason. Check
  `WorkosClientId` matches the app's `EXPO_PUBLIC_WORKOS_CLIENT_ID`.
- **Queries fail through Neon's pooler** with prepared-statement errors: add
  `default_query_exec_mode=simple_protocol` to the pooled `DatabaseUrl`.
- **Saves stay "Ordenando…" (sorting):** check the Worker's logs. Saves that failed are retried by the hourly sweep.

---

[← AI assistants](ai-assistants.md) · Next: [Architecture and costs →](architecture.md)

## Waitlist

Sign-up is closed in AuthKit, and the WorkOS waitlist is on (Authentication → Features). People join
from the landing page's form, which posts to the API's public `POST /v1/waitlist`; the API adds them to
the WorkOS waitlist with the `WorkosApiKey` secret, and WorkOS emails a confirmation. Approve entries in
the WorkOS dashboard (Users → Waitlist): each approval emails an invitation to create an account.
Without the secret, the form answers that the waitlist isn't open.
