# Scrapbook/01 website

The landing page and waitlist, in Spanish. Built with [Vite+](https://viteplus.dev) (Vite, Vitest,
Oxlint and Oxfmt behind one `vp` CLI), React and the [Module/01](https://github.com/franco-roura/module-01)
design system.

```bash
bun install
bun run dev      # http://localhost:5173
bun run check    # format, lint and type-check
bun run test
bun run build    # dist/, served by CloudFront in production (sst.config.ts, "Landing")
```

- **Waitlist:** sign-ups post to the "Lista de espera" Google Form (`src/waitlist.ts`). Google Forms
  sends no CORS headers, so the request is fire-and-forget: only network errors are reported. A bad
  name or email faults its field and shows a toast, as Module/01's Field guideline describes.
- **Layout:** Module/01's site parts (`NavBar`, `Hero`, `SectionHeader`, `Panel`, `CtaBanner`,
  `Footer`, `Tag`, `Toaster`). The few parts the kit doesn't have yet are in `src/parts.tsx`, built
  from its tokens only so they can move into Module/01 as they are.
- **Images:** `public/share.png` is the app's share sheet from `docs/screenshots`; `public/og.png`
  and `public/icon.png` come from `docs/brand`. Re-copy them when those change.
