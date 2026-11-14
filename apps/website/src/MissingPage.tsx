// Temporary gallery (src/missing.tsx renders it) of what the website needed from Module/01 and didn't find. Dev only: open
// /missing.html. Each entry shows the gap live, the workaround the site uses, and a suggested API.
// `?demo=<name>` renders one demo alone, so phone-width problems can be shown in a 375px iframe
// (media queries follow the viewport, not a container).
import { Button, NavBar, Panel, SectionHeader, Toaster } from "@franco_roura/module-01";
import { BudgetMeter } from "@franco_roura/module-01/finance";
import { CtaBanner, Hero, Tag, Terminal } from "@franco_roura/module-01/site";
import type { ReactNode } from "react";

import { ChatExchange, CompareList, FaqItem, ShareBar } from "./parts";
import { WaitlistForm } from "./WaitlistForm";

const demos: Record<string, () => ReactNode> = {
  navbar: () => (
    <div className="p-4">
      <NavBar
        brand={<span className="font-display text-display-sm">Scrapbook/01</span>}
        links={[
          { label: "Cómo funciona", href: "#como-funciona" },
          { label: "Por qué", href: "#por-que" },
          { label: "Precio", href: "#precio" },
          { label: "Preguntas", href: "#preguntas" },
        ]}
        end={<Button size="sm">Sumarme</Button>}
      />
    </div>
  ),
  "hero-title": () => (
    <div className="p-4">
      <Hero title="Scrapbook/01" subtitle="Lista de espera" lead="The title at phone size." />
    </div>
  ),
};

function Entry({
  n,
  name,
  missing,
  workaround,
  api,
  children,
}: {
  n: number;
  name: string;
  missing: ReactNode;
  workaround: ReactNode;
  api?: string[];
  children: ReactNode;
}) {
  const code = `M-${String(n).padStart(2, "0")}`;
  return (
    <section id={code.toLowerCase()} className="scroll-mt-8">
      <Panel serial={`${code} · ${name.toUpperCase()}`} cross bodyClassName="grid gap-6 p-8 pt-12">
        <h2 className="font-display text-display-md text-ink">
          {n}. {name}
        </h2>
        <div className="grid gap-4 text-body-md md:grid-cols-2">
          <div>
            <p className="hud text-signal-ink">Missing</p>
            <p className="mt-1 text-ink">{missing}</p>
          </div>
          <div>
            <p className="hud text-ink-soft">The site does now</p>
            <p className="mt-1 text-ink-soft">{workaround}</p>
          </div>
        </div>
        <div className="rounded-lg bg-shell-recess p-6 shadow-press">{children}</div>
        {api ? <Terminal title="SUGGESTED API" lines={api} /> : null}
      </Panel>
    </section>
  );
}

function Phone({ demo, height = 260 }: { demo: string; height?: number }) {
  return (
    <div className="flex flex-col items-start gap-2">
      <p className="hud-sm text-ink-soft">375 px wide, as on a phone</p>
      <iframe
        title={`${demo} at phone width`}
        src={`/missing.html?demo=${demo}`}
        width={375}
        height={height}
        className="max-w-full rounded-md border border-seam bg-canvas"
      />
    </div>
  );
}

function Gallery() {
  return (
    <main className="mx-auto flex w-full max-w-content-max flex-col gap-12 px-6 py-12 md:px-12">
      <SectionHeader
        code="T-00"
        label="Temporary · dev only"
        title="Missing from Module/01"
        ruler
      />
      <p className="-mt-6 max-w-3xl text-body-lg text-ink-soft">
        What the Scrapbook/01 website needed and didn't find in{" "}
        <code className="font-mono">@franco_roura/module-01</code> 1.0.1. Entries 1–4 are built in{" "}
        <code className="font-mono">src/parts.tsx</code> from the kit's tokens; 5–10 are gaps in
        existing components. Delete this page once they're in the kit.
      </p>
      <nav aria-label="Entries" className="flex flex-wrap gap-2">
        {[
          "FAQ accordion",
          "Chat exchange",
          "Share bar",
          "Compare list",
          "CtaBanner form",
          "NavBar menu",
          "Hero image",
          "Hero title",
          "Tag wrapping",
          "Dark mode",
        ].map((label, i) => (
          <a key={label} href={`#m-${String(i + 1).padStart(2, "0")}`}>
            <Tag tone="outline">{`${i + 1} · ${label}`}</Tag>
          </a>
        ))}
      </nav>

      <Entry
        n={1}
        name="FAQ accordion"
        missing="An open/close question and answer. Nothing in the kit discloses content in place (Tabs and Popover don't fit a FAQ)."
        workaround={
          <>
            <code className="font-mono">FaqItem</code>: a styled native{" "}
            <code className="font-mono">&lt;details&gt;</code>, keyboard and screen-reader friendly
            for free, with a plus key that turns 45° when open.
          </>
        }
        api={[
          '<Accordion type="single" collapsible>',
          '  <AccordionItem value="ai" question="¿Con qué IA funciona?">…</AccordionItem>',
          "</Accordion>",
        ]}
      >
        <div className="grid max-w-2xl gap-4">
          <FaqItem question="¿Con qué IA funciona?">
            Con la que ya usás: Claude, ChatGPT u otras compatibles con MCP.
          </FaqItem>
          <FaqItem question="¿Leen mis posts?">
            No. Guardamos el link y la línea que escribiste vos.
          </FaqItem>
        </div>
      </Entry>

      <Entry
        n={2}
        name="Chat exchange"
        missing="A short conversation: what you asked, then the assistant's answer. Terminal is the closest part, but every line gets a $ prompt, so it reads as shell commands."
        workaround={
          <>
            <code className="font-mono">ChatExchange</code>: a small Panel with your line in quotes
            and the answer in a <code className="font-mono">signal-tint</code> bubble.
          </>
        }
        api={[
          "<ChatExchange>",
          '  <ChatMessage from="you">¿Qué guardé sobre anuncios?</ChatMessage>',
          '  <ChatMessage from="assistant" label="Tu IA">Encontré 6 videos…</ChatMessage>',
          "</ChatExchange>",
        ]}
      >
        <div className="grid gap-6 md:grid-cols-[1fr_1fr]">
          <ChatExchange
            serial="E-01"
            you="¿Qué guardé sobre anuncios?"
            ai="Encontré 6 videos sobre anuncios. Te resumo los 3 consejos que más se repiten."
          />
          <div className="flex flex-col gap-2">
            <p className="hud-sm text-ink-soft">The kit's Terminal, for comparison</p>
            <Terminal
              title="NOT A CHAT"
              lines={["¿Qué guardé sobre anuncios?", "Encontré 6 videos…"]}
            />
          </div>
        </div>
      </Entry>

      <Entry
        n={3}
        name="Share bar"
        missing="Two (or more) shares of one whole on one track, each labeled: the 80/20 split. BudgetMeter shows one share of a budget, in minor units, with a today tick."
        workaround={
          <>
            <code className="font-mono">ShareBar</code>: a sunken stadium track with an{" "}
            <code className="font-mono">ink</code> part and a{" "}
            <code className="font-mono">signal</code> part, HUD labels inside, one accessible label
            for the whole.
          </>
        }
        api={[
          '<ShareBar label="80% to models, 20% to us" parts={[',
          '  { share: 80, label: "80% tus guardados", tone: "ink" },',
          '  { share: 20, label: "20%", tone: "signal" },',
          "]} />",
        ]}
      >
        <div className="grid gap-6">
          <ShareBar
            label="80% procesa tus guardados, 20% es lo que cobramos"
            parts={[
              { share: 80, label: "80% tus guardados" },
              { share: 20, label: "20%" },
            ]}
          />
          <div className="flex flex-col gap-2">
            <p className="hud-sm text-ink-soft">
              The kit's BudgetMeter at 80 of 100, for comparison
            </p>
            <BudgetMeter spent={80n} budget={100n} label="80 of 100" />
          </div>
        </div>
      </Entry>

      <Entry
        n={4}
        name="Compare list"
        missing="“Instead of / you get” lines: a muted cross, then a check. Common on landing pages for a before/after."
        workaround={
          <>
            <code className="font-mono">CompareList</code>: Lucide X and Check at the kit's stroke,
            with screen-reader prefixes so the icons aren't the only cue.
          </>
        }
        api={['<CompareList instead="Buscar a mano en cinco apps" get="Preguntás y te lo trae" />']}
      >
        <div className="max-w-md">
          <CompareList
            instead="Buscar a mano en cinco apps"
            get="Preguntás en tu chat y te lo trae"
          />
        </div>
      </Entry>

      <Entry
        n={5}
        name="CtaBanner form slot"
        missing="CtaBanner lays its action out beside the title, sized for one button. A form there (two fields and a button) crushes the headline, and Field labels in ink-soft are hard to read on signal."
        workaround="The banner has a single button that jumps to the hero's form and focuses its first field."
        api={[
          '<CtaBanner title="…" text="…">',
          "  <WaitlistForm />   {/* children: full width under the text */}",
          "</CtaBanner>",
        ]}
      >
        <CtaBanner
          code="S-06 · AS IS"
          title="Lo que guardaste, cuando lo necesitás."
          text="Sumate a la lista y asegurá tu lugar entre los primeros 1.000."
          action={<WaitlistForm />}
        />
      </Entry>

      <Entry
        n={6}
        name="NavBar mobile menu"
        missing="On phones the links don't collapse into a menu: they stay inline and push the bar past the screen edge. desktopOnly can hide them, but then they're unreachable."
        workaround="Every link is desktopOnly, so phones get the brand and the “Sumarme” button only."
        api={[
          '<NavBar links={…} mobileMenu="sheet" />   // a menu key that opens a Sheet of links',
        ]}
      >
        <Phone demo="navbar" height={120} />
      </Entry>

      <Entry
        n={7}
        name="Hero image size"
        missing="HeroFrame is fixed at 200 × 240 px, a portrait-photo size. A product screenshot is either a tiny letterbox or a crop."
        workaround="The top of the app's share-sheet screenshot, cropped into the portrait frame."
        api={['<Hero image={…} imageSize="lg" imageAspect="16/10" />']}
      >
        <Hero
          title="Scrapbook/01"
          subtitle="A wide screenshot in the hero frame"
          lead="The 1280 × 640 social image, letterboxed into 200 × 240."
          image={
            <img src="/og.png" alt="The Scrapbook/01 social image" className="!object-contain" />
          }
          imageLabel="1280 × 640"
        />
      </Entry>

      <Entry
        n={8}
        name="Hero title on phones"
        missing="At the hero's phone size, a 12-character name in Michroma is wider than a 375 px screen, so “Scrapbook/01” is cut off."
        workaround={
          <>
            The title is wrapped in <code className="font-mono">max-md:text-display-md</code>, one
            step smaller on phones.
          </>
        }
        api={[
          '<Hero title="Scrapbook/01" />   // fit long titles: clamp() or a step down below md',
        ]}
      >
        <Phone demo="hero-title" height={300} />
      </Entry>

      <Entry
        n={9}
        name="Tag wrapping"
        missing="Tags never wrap (they're shrink-0 and nowrap), so a sentence-long tag overflows a phone-width card. The guideline says one or two words, but sites want short taglines in that style too."
        workaround="Long ones became plain HUD overlines (hud, signal-ink) that wrap."
        api={['<Tag tone="signal" wrap>Tus palabras, no un robot</Tag>   // or a HudOverline part']}
      >
        <div className="w-72 overflow-visible rounded-md border border-dashed border-hud-mark p-4">
          <p className="hud-sm mb-3 text-ink-soft">A 288 px card</p>
          <Tag tone="signal">No guardamos links. Guardamos ideas.</Tag>
        </div>
      </Entry>

      <Entry
        n={10}
        name="Dark mode"
        missing="Module/01 has one light theme. There are no dark values for canvas, shell, ink or seam, so the site ignores the system's dark setting. The old page followed it."
        workaround="Light only, for now."
        api={[
          "@media (prefers-color-scheme: dark) { :root { --color-canvas: …; --color-shell: …; } }",
          '// or [data-theme="dark"], with a visor-like shell and signal kept as is',
        ]}
      >
        <div className="grid gap-4 md:grid-cols-2">
          <div className="rounded-md bg-canvas p-6 shadow-float">
            <p className="hud text-ink-soft">Light (what exists)</p>
            <p className="mt-2 font-display text-display-sm text-ink">Scrapbook/01</p>
          </div>
          <div className="rounded-md bg-visor p-6">
            <p className="hud text-visor-mark">Dark (not defined)</p>
            <p className="mt-2 font-display text-display-sm text-shell">Scrapbook/01</p>
          </div>
        </div>
      </Entry>
      <Toaster />
    </main>
  );
}

/** The gallery, or one demo alone with `?demo=<name>`. */
export function MissingPage() {
  const demo = new URLSearchParams(location.search).get("demo");
  return demo && demos[demo] ? demos[demo]() : <Gallery />;
}
