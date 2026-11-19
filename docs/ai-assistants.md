[← README](../README.md) · [Features](features.md) · [Getting started](getting-started.md) · [Sorting](sorting.md) · **AI assistants** · [Self-hosting](self-hosting.md) · [Architecture](architecture.md) · [Development](development.md)

# Ask your AI about your library

Scrapbook/01 is a [Model Context Protocol](https://modelcontextprotocol.io) server. Connect it to
Claude, ChatGPT or any MCP client, and your assistant can look things up in what you saved.

**On this page:** [Connect it](#connect-it) · [What it can do](#what-it-can-do) ·
[Who pays for what](#who-pays-for-what) · [Sign-in and access](#sign-in-and-access) ·
[For self-hosters](#for-self-hosters)

## Connect it

The quickest way is in the app: **Conectar IA** in the library header. **Agregar a Claude** opens
Claude's "Add custom connector" dialog already filled in, so you only tap Add. ChatGPT has no install
link, so **Copiar link y abrir ChatGPT** copies the server URL and opens ChatGPT for you to paste it.

To do it by hand, the server is at `https://<your API>/mcp`.

- **Claude:** Settings → Connectors → **Add custom connector**, and paste the URL.
- **ChatGPT, Claude Code and other MCP clients:** add it as a remote MCP server (Streamable HTTP)
  wherever the client takes one.

The client opens a sign-in page. Sign in with the same email as in the app, allow access, and the
library is connected.

Then ask in plain words:

- "What was that ramen place in Tokyo I saved?"
- "I'm cooking tonight. Anything quick from my saved recipes?"
- "What did I save about toddler sleep, and what's related to it?"
- "Save this for the Japan trip: https://… (a list of onsen near Hakone)."

## What it can do

| Tool | What it does |
| --- | --- |
| `search_library` | Searches your saves by meaning, optionally within one category. Returns short results |
| `get_item` | One save, with its full note and the saves most similar to it |
| `get_neighbors` | Saves linked to one in the graph, one or two hops out, with their categories |
| `save_item` | Saves a link with a note, sorted like any other save. Only when you ask |

Search embeds the question once and ranks your saves with pgvector. When the server has a Jev key, Jev
then re-ranks the top 10 by how well each answers the question (if Jev fails, the vector order
stands). Without an embeddings key, search falls back to keyword matching.

## Who pays for what

The expensive part, the assistant reading, reasoning and answering, runs on **your own** Claude or
ChatGPT plan. Scrapbook/01 only does retrieval:

- a search costs one small embedding, about $0.0000004, taken from your credits;
- out of credits, search still works, with free keyword matching;
- opening items and neighbors is a database query: free.

That's the point of the design: no markup on AI tokens, because Scrapbook/01 doesn't resell them.

## Sign-in and access

- MCP sign-in is OAuth 2.1 through [WorkOS AuthKit](https://workos.com/docs/authkit/mcp). The server
  publishes where to sign in at `/.well-known/oauth-protected-resource/mcp` (RFC 9728) and answers
  unauthenticated calls with `401` and a pointer to it.
- Clients register themselves, through Client ID Metadata Documents (the current MCP way) or dynamic
  client registration (older clients). AuthKit shows the sign-in and consent screens.
- Tokens are checked against AuthKit's published keys and must be issued for this server (their
  audience is `<API>/mcp`). Tokens for AI agents acting on their own, with no user, are refused.
- An assistant sees only your saves, the same as the app. It can add saves, but it can't edit or
  delete anything.
- Signing out of the app doesn't disconnect an assistant. Remove the connector in the assistant to
  revoke it.

## For self-hosters

In the WorkOS dashboard:

1. **Connect → Configuration:** turn on Client ID Metadata Documents and dynamic client registration.
2. **Resource Indicators:** add `<PUBLIC_URL>/mcp` and make it the default.
3. Set `PUBLIC_URL` (the API's public address) and `WORKOS_AUTHKIT_DOMAIN` on the API. On AWS, SST sets
   `PUBLIC_URL` for you.

To try it locally, the MCP client has to reach your machine: run a tunnel such as
`cloudflared tunnel --url http://localhost:8080` and use its address as `PUBLIC_URL`. The
[MCP Inspector](https://github.com/modelcontextprotocol/inspector) is handy for testing the tools.

---

[← How saves get sorted](sorting.md) · Next: [Self-hosting →](self-hosting.md)
