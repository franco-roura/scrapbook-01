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
- **Layout:** Module/01 1.1.0 throughout: `NavBar` (with its phone menu), `Hero` (wide image
  frame), `SectionHeader`, `Panel`, `Tag`, `CompareList`, `Message`/`Bubble`, `ShareBar`,
  `Accordion`, `CtaBanner` (with the signup form inside), `Footer` and `Toaster`. One local tweak
  remains: the hero title steps down a size on phones, since "Scrapbook/01" in Michroma is wider
  than a 375px screen at the hero's phone size.
- **Images:** `public/app.png` (the three-screen hero), `public/og.png` and `public/icon.png` come
  from `docs/brand`. Re-copy them when those change.
