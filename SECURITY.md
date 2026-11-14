# Security policy

Scrapbook/01 holds what people save and why, so security reports come first.

## Reporting a vulnerability

Report it privately on GitHub: open the repository's **Security** tab and choose **Report a
vulnerability**. Please don't open a public issue or pull request for it.

Include what you found, how to reproduce it and what an attacker could do with it. Expect an
acknowledgement within a week. This is a personal project maintained in spare time, so a fix may take
longer, but you'll hear how it's going until it ships. You'll be credited in the fix unless you'd
rather not be.

## Supported versions

Only the latest `main` gets fixes. Self-hosted instances should update to it.

## Scope

In scope: the Go API and MCP server (`server/`), the app including its share extension
(`apps/mobile/`), and the deployment setup (`sst.config.ts`).

Out of scope: the setup of a particular self-hosted instance (its AWS account, WorkOS environment or
DNS), the AI providers and WorkOS themselves, volume-based denial of service, and anything that needs
an already compromised device or browser.

## How the app protects data

- Sign-in is WorkOS AuthKit. The app is a public OAuth client using PKCE: no secret ships in it. Its
  session lives in the device's keychain (localStorage on the web).
- The API checks every token's signature against WorkOS's published keys and its expiry. Tokens for
  MCP clients must also be issued for this server (their audience is `<API>/mcp`), and tokens for AI
  agents acting without a user are refused.
- `DEV_JWT_SECRET`, which accepts locally signed tokens, is for development only and logs a warning
  whenever it's set.
- Every query filters by the user from the token, never from the request. MCP clients get the same
  user's data and can only search, read and add saves.
- Configuration and secrets come from environment variables (SST secrets on AWS), so the repository
  holds none.
