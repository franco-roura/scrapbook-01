[← README](../README.md) · [Features](features.md) · [Getting started](getting-started.md) · [Sorting](sorting.md) · **AI assistants** · [Self-hosting](self-hosting.md) · [Architecture](architecture.md) · [Observability](observability.md) · [Development](development.md)

# Ask your AI about your library

Scrapbook/01 is a [Model Context Protocol](https://modelcontextprotocol.io) server. Connect it to
Claude, ChatGPT or any MCP client, and your assistant can look things up in what you saved.

**On this page:** [Connect it](#connect-it) · [What it can do](#what-it-can-do) ·
[Analyses](#analyses) · [Who pays for what](#who-pays-for-what) · [Sign-in and access](#sign-in-and-access) ·
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
- "Here's what that reel says. Add it to the save so you remember it."

## What it can do

| Tool | What it does |
| --- | --- |
| `search_library` | Searches your saves by meaning, optionally within one category. Returns short results |
| `get_item` | One save, with its full note, its analysis if it has one, and the saves most similar to it |
| `get_neighbors` | Saves linked to one in the graph, one or two hops out, with their categories |
| `save_item` | Saves a link with a note, sorted like any other save. Only when you ask |
| `save_analysis` | Records what a save contains (see [Analyses](#analyses)). Replaces the previous one |

Search embeds the question once and ranks your saves with pgvector. When the server has a Jev key, Jev
then re-ranks the top 10 by how well each answers the question (if Jev fails, the vector order
stands). Without an embeddings key, search falls back to keyword matching.

Search looks at analyses too: a save matches through your note or through its analysis, whichever is
closer, and results show the analysis summary.

## Analyses

Your note says why you saved something ("for the Japan trip"). An analysis says what's in it: the
five onsen and their prices, the steps of the recipe, the books in the chart. Many posts carry
their content only in the video or the image, behind a caption like "Comment GUIDE", so a short note
is often all the library knows.

The assistant writes the analysis, not the server: Scrapbook/01 still never reads the post. You
show your assistant the content (paste the caption, share a screenshot, or let it open the link if
it can), and it records what it learned with `save_analysis`:

| Field | What goes in it |
| --- | --- |
| `summary` | The central idea, in two or three sentences. Required |
| `details` | The concrete content: lists, steps, places, prices, names, links |
| `tags` | Short topic tags, up to 20 |
| `useWhen` | Situations in which the assistant should bring the save up on its own, e.g. "planning a trip to Buenos Aires", up to 10 |
| `actions` | Things you could do with it, e.g. "book the ferry early" |
| `caveats` | What to keep in mind: sponsored content, audio it couldn't hear, prices that may be outdated |
| `sources` | What the analysis was made from, e.g. `caption`, `video_frames`, `transcript` |

Your note and your category stay as they are; sorting still reads only your note. Each save has at
most one analysis, and saving another replaces it.

`useWhen` is what makes the assistant proactive: when what you're doing matches a situation in an
analysis, a search finds that save even if you never asked for it by name.

## Who pays for what

The expensive part, the assistant reading, reasoning and answering, runs on **your own** Claude or
ChatGPT plan. Scrapbook/01 only does retrieval:

- a search costs one small embedding, about $0.0000004, taken from your credits;
- saving an analysis costs one small embedding too (its summary, tags and situations), and out of
  credits it's saved without one, for keyword search;
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
- An assistant sees only your saves, the same as the app. It can add saves and analyses, but it
  can't edit your notes or delete anything.
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
