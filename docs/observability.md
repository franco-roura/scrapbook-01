[← README](../README.md) · [Features](features.md) · [Getting started](getting-started.md) · [Sorting](sorting.md) · [AI assistants](ai-assistants.md) · [Self-hosting](self-hosting.md) · [Architecture](architecture.md) · **Observability** · [Development](development.md)

# Observability

Scrapbook/01 can report errors and performance to [Sentry](https://sentry.io). It's **off unless you
set a DSN**: with none, the SDKs aren't started and nothing leaves your deployment. Each surface has
its own Sentry project and DSN, so you can turn on any of them alone:

| Surface | Sentry project | Where the DSN goes |
| --- | --- | --- |
| API, MCP server, worker (Go) | `scrapbook01-server` | `SENTRY_DSN` (SST secret `SentryDsnServer`) |
| Phone app, share sheet, web app | `scrapbook01-mobile` | `EXPO_PUBLIC_SENTRY_DSN` (SST secret `SentryDsnMobile`, and EAS for phone builds) |
| Landing page | `scrapbook01-website` | `VITE_SENTRY_DSN` (SST secret `SentryDsnWebsite`) |

Setup is in [Self-hosting](self-hosting.md#one-time-setup). Locally, put the same variables in
`server/.env` or `apps/mobile/.env.local`.

## What gets recorded

| Question | Where to look in Sentry |
| --- | --- |
| How long each endpoint takes (p50, p95, throughput) | Insights → Backend. One transaction per request, named after its route, e.g. `GET /v1/items/{id}`. |
| Which queries are slow | Insights → Queries. Every sqlc query is a `db` span named after its SQL, with the sqlc name (`GetItem`) as `db.operation.name`. |
| How long AI calls take and what they cost | `gen_ai.*` spans with model, input and output tokens, and the metered cost in micro-dollars. |
| How each MCP tool performs | Transactions named `MCP tools/call <tool>`. |
| How long sorting and previews take | `worker classify`, `worker preview` and `worker sweep` transactions. A save's classification shows up in the same trace as the request that queued it. |
| Which screens people use, and how much | Insights → Mobile → Screens: one transaction per screen visit. Release Health shows sessions, active users and crash-free rate per release. |
| Share sheet health | Events tagged `surface:share-extension`. |
| Landing page visits and load speed | The website project's page loads and Web Vitals. |
| Errors and crashes | Issues, in all three projects. |

## What never gets recorded

- **No personal data:** the only user detail is the internal user id (a UUID). No emails, names or IP
  addresses.
- **No content:** no request or response bodies, so no saved links, descriptions or category names.
- **No headers, cookies or query strings.** Request URLs aren't kept either, since some paths carry
  push tokens: the route pattern stands in.
- **No query arguments:** database spans carry the SQL with its `$1` placeholders, never the values.
- **Links' own addresses:** preview fetches are named after the site's host only.
- **No Session Replay and no profiling.**
- Trace headers are only sent from the apps to your own API, never to third parties.

## Staying on the free plan

The free Developer plan has monthly quotas for errors and spans (see
[sentry.io/pricing](https://sentry.io/pricing)). At hobby scale, tracing every request fits. If you get
busier, lower the sample rates:

- `SENTRY_TRACES_SAMPLE_RATE` (server, default `1`)
- `EXPO_PUBLIC_SENTRY_TRACES_SAMPLE_RATE` (apps, default `1`)
- `VITE_SENTRY_TRACES_SAMPLE_RATE` (landing page, default `1`)

Errors are always sent in full. Some things are left out on purpose to save quota: CORS preflights and
404 responses aren't traced, and neither are River's own queue queries when you run the server locally.

## On AWS Lambda

On Lambda the server sends each event before the invocation returns, so nothing runs after the
response, in line with the API's Lambda-safe rule. The cost is a short call to Sentry at the end
of each traced request, after the measured duration. Lowering `SENTRY_TRACES_SAMPLE_RATE` also
lowers that cost.
