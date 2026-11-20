[← README](../README.md) · [Features](features.md) · **Getting started** · [Sorting](sorting.md) · [AI assistants](ai-assistants.md) · [Self-hosting](self-hosting.md) · [Architecture](architecture.md) · [Observability](observability.md) · [Development](development.md)

# Getting started

Run Scrapbook/01 on your machine: Postgres in Docker, the Go API, and the app in a browser. No AI keys
needed: a fake AI server stands in for Jev, the embeddings API and Claude.

**On this page:** [What you need](#what-you-need) · [1. Sign-in (WorkOS)](#1-sign-in-workos) ·
[2. The API](#2-the-api) · [3. The app](#3-the-app) · [Your first saves](#your-first-saves) ·
[Real AI keys](#real-ai-keys) · [On a phone](#on-a-phone)

## What you need

- [Go](https://go.dev/dl/) 1.27 or newer
- [Bun](https://bun.sh)
- [Docker](https://docs.docker.com/get-docker/)
- A free [WorkOS](https://workos.com) account, for sign-in

```bash
git clone https://github.com/franco-roura/scrapbook-01.git
cd scrapbook-01
```

## 1. Sign-in (WorkOS)

In the WorkOS dashboard, stay in the **Sandbox** environment (it allows `localhost`):

1. **Authentication**: turn on **Magic Auth** (email codes).
2. **Redirects**: add `http://localhost:8081/auth/callback`.
3. **Authentication → Sessions → CORS**: add `http://localhost:8081`. The browser calls WorkOS
   directly to finish signing in.
4. Copy the **client id** (API Keys, `client_…`) and the **AuthKit domain** (Domains,
   `https://<something>.authkit.app`).

## 2. The API

```bash
cd server
docker compose up -d --wait        # Postgres with pgvector, on port 54329
cp .env.example .env
```

In `server/.env`, set the WorkOS values and point the AI clients at the fake server:

```ini
WORKOS_CLIENT_ID=client_...
WORKOS_AUTHKIT_DOMAIN=https://<something>.authkit.app
CORS_ORIGINS=http://localhost:8081

TYPESAFE_API_KEY=dev
TYPESAFE_BASE_URL=http://127.0.0.1:8787
EMBEDDINGS_API_KEY=dev
EMBEDDINGS_BASE_URL=http://127.0.0.1:8787
ANTHROPIC_API_KEY=dev
ANTHROPIC_BASE_URL=http://127.0.0.1:8787
# The fake embeddings score lower than real ones.
GRAPH_MIN_SIMILARITY=0.2
```

Then start the fake AI server and the API, each in its own terminal (both from `server/`):

```bash
go run ./cmd/fakeai
```

```bash
go run ./cmd/api
```

The API reads `.env` itself, applies migrations on start and listens on `http://localhost:8080`.
`curl localhost:8080/healthz` should answer `{"status":"ok"}`.

## 3. The app

```bash
cd apps/mobile
cp .env.example .env.local         # then set EXPO_PUBLIC_WORKOS_CLIENT_ID
bun install
bun run web
```

Open `http://localhost:8081`, choose **Seguir con email** (continue with email), and enter the code WorkOS emails you.

## Your first saves

The app is in Uruguayan Spanish; the labels below are what you'll see, with their meaning.

1. **Agregar link** (add link): paste any link, write what it's about ("pollo al limón, cena de
   semana"), save.
   Within a second or two it gets a category: the fake AI names it after the longest word.
2. Save a few more on the same topic. They join that category instead of making new ones.
3. Open one: change its category, or look at its **Conexiones** (connections).
4. **Mapa** (map) shows the whole library as a graph.
5. **Créditos** (credits) shows the free starting budget and what each save cost.

The fake AI is crude on purpose: it matches words, not meaning. With real keys, sorting and links
follow what the descriptions mean.

## Real AI keys

Replace the fake values in `server/.env` with real keys and remove the `*_BASE_URL` lines and
`GRAPH_MIN_SIMILARITY`. Each key is optional: a missing one turns its step off with a warning.

| Key | What it does | Where to get it |
| --- | --- | --- |
| `TYPESAFE_API_KEY` | Jev picks among your categories | [typesafe.ai](https://typesafe.ai) |
| `EMBEDDINGS_API_KEY` | Embeddings for links and search (OpenAI-compatible, 1536 dimensions; OpenRouter by default) | [openrouter.ai](https://openrouter.ai) |
| `ANTHROPIC_API_KEY` | Claude Haiku names new categories | [console.anthropic.com](https://console.anthropic.com) |

## On a phone

The share sheet needs a development build of the app, not Expo Go:

```bash
cd apps/mobile
bun x eas-cli build --profile development
```

Set `EXPO_PUBLIC_API_URL` in `apps/mobile/.env.local` to your computer's LAN address (for example
`http://192.168.1.20:8080`), since `localhost` on a phone is the phone. Add
`scrapbook01://auth/callback` to WorkOS's redirects for the phone builds. iOS builds need macOS or
EAS. See [apps/mobile/README.md](../apps/mobile/README.md) for the share sheet and notifications.

---

[← Features](features.md) · Next: [How saves get sorted →](sorting.md)
