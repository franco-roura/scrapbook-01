import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Bubble,
  Button,
  Message,
  MessageAvatar,
  MessageContent,
  MessageHeader,
  NavBar,
  Panel,
  SectionHeader,
  ShareBar,
  Toaster,
} from "@franco_roura/module-01";
import { CompareList, CtaBanner, Footer, Hero, Tag } from "@franco_roura/module-01/site";

import { WaitlistForm } from "./WaitlistForm";

const sources = ["Instagram", "TikTok", "YouTube", "X", "Facebook", "WhatsApp", "Cualquier link"];

const pains = [
  {
    title: "Está todo desparramado",
    text: "Un poco en Instagram, otro en TikTok, links en WhatsApp, videos en YouTube…",
  },
  {
    title: "Nunca lo encontrás",
    text: "Sabés que lo guardaste, pero no en qué app ni en qué carpeta.",
  },
  {
    title: "No te acordás qué era",
    text: "Un link pelado no dice nada. Para cuando lo abrís, ya no sabés por qué lo guardaste.",
  },
  {
    title: "Te olvidás de que existía",
    text: "El restaurante que querías probar, el tip para tus anuncios: guardado y perdido.",
  },
];

const steps = [
  {
    title: "Compartí desde cualquier app",
    text: "Tocás “compartir” y elegís Scrapbook/01. Se abre encima de la app en la que estás, con el link ya puesto.",
  },
  {
    title: "Contá de qué se trata",
    text: "Una línea con tus palabras: “parrilla en Palermo”, “rutina de 20 minutos”. Podés dictarla o dejarla para después.",
  },
  {
    title: "Se ordena y se conecta solo",
    text: "Va a una de tus categorías, o a una nueva si no encaja en ninguna, y se enlaza con lo parecido. Te avisamos dónde quedó.",
  },
];

const examples = [
  {
    you: "¿Qué restaurantes de Buenos Aires tenía guardados?",
    ai: "Tenés 4 en “Viaje a Buenos Aires”: una parrilla en Palermo, dos bodegones y un bar de vermut. ¿Te los ordeno por barrio?",
  },
  {
    you: "¿Qué guardé sobre anuncios para mi emprendimiento?",
    ai: "Encontré 6 videos sobre anuncios. Te resumo los 3 consejos que más se repiten.",
  },
  {
    you: "Busco una cena rápida para hoy, ¿tengo algo guardado?",
    ai: "En “Cenas de semana” tenés un curry de garbanzos de 25 minutos y unos ñoquis al horno. ¿Te paso los links?",
  },
];

const faqs = [
  {
    q: "¿Con qué IA funciona?",
    a: "Con la que ya usás. La conectás desde Claude, ChatGPT u otras compatibles con conectores (MCP) y le preguntás por tus guardados cuando quieras. Lo que tu IA piensa corre en tu propio plan, no en tus créditos.",
  },
  {
    q: "¿Qué puedo guardar?",
    a: "Cualquier link que se pueda compartir: reels, TikToks, videos de YouTube, posts de X o Facebook, links que te llegan por WhatsApp y cualquier página web.",
  },
  {
    q: "¿Leen mis posts?",
    a: "No. Guardamos el link y la línea que escribiste vos; nunca entramos a la red social a leer el contenido. Por eso funciona igual con cualquier app.",
  },
  {
    q: "¿Qué pasa si el post se borra?",
    a: "El link deja de abrir, pero tu nota y su categoría quedan: sabés qué era y por dónde buscarlo.",
  },
  {
    q: "¿Quién ve mis guardados?",
    a: "Solo vos y la IA que conectes. Tu biblioteca es privada, y para desconectar tu IA alcanza con quitar el conector.",
  },
  {
    q: "¿Cuándo lo puedo usar?",
    a: "Estamos abriendo de a poco. Los primeros de la lista entran antes y con condición de fundador.",
  },
];

const section = "mx-auto w-full max-w-content-max px-6 md:px-12";

export default function App() {
  return (
    <>
      <div className="sticky top-4 z-40 mx-auto w-full max-w-content-max px-4 md:px-12">
        <NavBar
          brand={<span className="font-display text-display-sm">Scrapbook/01</span>}
          links={[
            { label: "Cómo funciona", href: "#como-funciona" },
            { label: "Por qué", href: "#por-que" },
            { label: "Precio", href: "#precio" },
            { label: "Preguntas", href: "#preguntas" },
          ]}
          mobileMenu
          menuLabel="Menú"
          end={
            <Button asChild size="sm">
              <a href="#lista">Sumarme</a>
            </Button>
          }
        />
      </div>

      <main className="flex flex-col gap-24 pt-12 pb-16">
        <div className={section}>
          <Hero
            // Michroma is wide: at the kit's sizes "Scrapbook/01" overflows a 375px screen, and on
            // desktop the column beside the wide image frame. One step down fits both.
            title={<span className="text-display-md md:text-display-lg">Scrapbook/01</span>}
            subtitle="Lista de espera · primeros 1.000 lugares"
            lead={
              <>
                <strong className="text-ink">
                  Guardaste mil cosas “para después”. ¿Cuándo es después?
                </strong>{" "}
                Compartí cualquier reel, TikTok, post o link y escribí en una línea de qué se trata.
                Se ordena solo en tus categorías, se conecta con lo que ya guardaste y tu IA lo
                encuentra cuando se lo pedís.
              </>
            }
            actions={
              <div id="lista" className="flex w-full flex-col gap-3">
                <WaitlistForm />
                <p className="text-body-sm text-ink-soft">
                  Sin spam. Te escribimos solo cuando abramos lugares.
                </p>
              </div>
            }
            image={
              <img
                src="/app.png"
                alt="Tres pantallas de la app: compartir un link y contar de qué se trata, la biblioteca ordenada en categorías y las conexiones de un guardado"
              />
            }
            imageShape="wide"
            serial="S-00 · WAITLIST"
          />
          <ul aria-label="Funciona con" className="mt-8 flex flex-wrap gap-2">
            {sources.map((s) => (
              <li key={s}>
                <Tag tone="outline">{s}</Tag>
              </li>
            ))}
          </ul>
        </div>

        <section className={section} aria-labelledby="dolor">
          <SectionHeader
            as="h2"
            code="S-01"
            label="El problema"
            title={<span id="dolor">Tus guardados son un cajón desordenado</span>}
            ruler
          />
          <p className="mt-4 max-w-2xl text-body-lg text-ink-soft">
            Guardar es fácil. Volver a encontrarlo, no.
          </p>
          <div className="mt-12 grid gap-8 md:grid-cols-2">
            {pains.map((p, i) => (
              <Panel key={p.title} serial={`P-0${i + 1}`} notch="sm" bodyClassName="p-6 pt-10">
                <h3 className="text-label text-ink">{p.title}</h3>
                <p className="mt-1 text-body-md text-ink-soft">{p.text}</p>
              </Panel>
            ))}
          </div>
        </section>

        <section id="como-funciona" className={section}>
          <SectionHeader
            as="h2"
            code="S-02"
            label="Cómo funciona"
            title="Compartí, contá y listo"
            ruler
          />
          <ol className="mt-12 grid gap-8 md:grid-cols-3">
            {steps.map((s, i) => (
              <li key={s.title}>
                <Panel
                  className="h-full"

                  serial={`F-0${i + 1}`}
                  hinge={["24px", "50%", "70%"][i]}
                  cross
                  bodyClassName="flex h-full flex-col gap-3 p-8 pt-12"
                >
                  <span className="grid size-10 place-items-center rounded-md bg-signal font-mono text-amount-md text-ink">
                    {i + 1}
                  </span>
                  <h3 className="font-display text-display-sm text-ink">{s.title}</h3>
                  <p className="text-body-md text-ink-soft">{s.text}</p>
                </Panel>
              </li>
            ))}
          </ol>
        </section>

        <section id="por-que" className={section}>
          <SectionHeader
            as="h2"
            code="S-03"
            label="Por qué es distinto"
            title="No es otra app de marcadores"
            ruler
          />
          <div className="mt-12 grid gap-8 md:grid-cols-2">
            <Panel serial="D-01" hinge="32px" bodyClassName="p-8 pt-12">
              <Tag tone="signal" wrap>
                Tus palabras, no un robot
              </Tag>
              <h3 className="mt-4 font-display text-display-sm text-ink">
                Lo encontrás por lo que es
              </h3>
              <p className="mt-3 text-body-md text-ink-soft">
                No leemos los posts ni adivinamos por el link: vos contás de qué se trata en una
                línea. Por eso funciona con cualquier app, y cada guardado se ordena por lo que
                realmente es.
              </p>
              <CompareList
                className="mt-6"
                insteadLabel="En lugar de"
                getLabel="Con Scrapbook/01"
                items={[
                  {
                    instead: "Un link pelado que no dice qué era",
                    get: "Tu nota, una categoría y lo que se le parece",
                  },
                ]}
              />
            </Panel>
            <Panel serial="D-02" hinge="65%" bodyClassName="p-8 pt-12">
              <Tag tone="signal" wrap>
                Con la IA que ya usás
              </Tag>
              <h3 className="mt-4 font-display text-display-sm text-ink">Preguntale a tu IA</h3>
              <p className="mt-3 text-body-md text-ink-soft">
                Conectás Claude, ChatGPT u otra compatible con MCP y le preguntás por lo que
                guardaste. Busca en tu biblioteca, abre lo que encuentra y te muestra lo
                relacionado. Lo que piensa corre en tu propio plan: no te cobramos tokens.
              </p>
              <CompareList
                className="mt-6"
                insteadLabel="En lugar de"
                getLabel="Con Scrapbook/01"
                items={[
                  {
                    instead: "Buscar a mano en cinco apps",
                    get: "Preguntás en tu chat y te lo trae",
                  },
                ]}
              />
            </Panel>
          </div>
          <div className="mt-8 grid gap-8 md:grid-cols-3">
            {examples.map((e, i) => (
              <Panel
                key={e.you}
                serial={`E-0${i + 1}`}
                notch="sm"
                className="h-full"
                bodyClassName="flex h-full flex-col gap-4 p-6 pt-10"
              >
                <Message align="end">
                  <MessageContent>
                    <Bubble>{e.you}</Bubble>
                  </MessageContent>
                </Message>
                <Message>
                  <MessageAvatar fallback="IA" />
                  <MessageContent>
                    <MessageHeader>Tu IA</MessageHeader>
                    <Bubble>{e.ai}</Bubble>
                  </MessageContent>
                </Message>
              </Panel>
            ))}
          </div>
        </section>

        <section id="precio" className={section}>
          <SectionHeader
            as="h2"
            code="S-04"
            label="Precio honesto"
            title="Pagás lo que usás. Ganamos solo si te sirve."
            ruler
          />
          <div className="mt-12 grid items-center gap-8 md:grid-cols-[1.1fr_0.9fr]">
            <p className="text-body-lg text-ink-soft">
              Sin suscripción que te olvidás de cancelar. Comprás packs de créditos de IA y se usan
              para ordenar y conectar tus guardados. Cada uno cuesta una fracción de centavo, así
              que un pack rinde miles. Lo que hace tu propia IA corre en tu plan, no en tus
              créditos.
            </p>
            <Panel serial="K-01 · 80/20" cross bodyClassName="flex flex-col gap-4 p-8 pt-12">
              <p className="text-label text-ink">De cada peso que gastás en créditos</p>
              <ShareBar
                label="Cómo se reparte lo que gastás en créditos"
                parts={[
                  { share: 80, label: "Modelos de IA que ordenan y conectan tus guardados" },
                  { share: 20, label: "Lo que cobramos nosotros. Si no lo usás, no ganamos" },
                ]}
              />
              <Panel recess notch={false} bodyClassName="p-4 text-body-sm text-ink">
                <strong>Condición de fundador:</strong> este modelo está garantizado para los
                primeros 1.000 de la lista.
              </Panel>
            </Panel>
          </div>
        </section>

        <section id="preguntas" className={section}>
          <div className="max-w-3xl">
            <SectionHeader
              as="h2"
              code="S-05"
              label="Preguntas"
              title="Preguntas frecuentes"
              ruler
            />
            <Accordion type="single" collapsible className="mt-12">
              {faqs.map((f, i) => (
                <AccordionItem key={f.q} value={f.q}>
                  <AccordionTrigger code={`Q-0${i + 1}`}>{f.q}</AccordionTrigger>
                  <AccordionContent>{f.a}</AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </div>
        </section>

        <div className={section}>
          <CtaBanner
            code="S-06 · LISTA DE ESPERA"
            title="Lo que guardaste, cuando lo necesitás."
            text="Sumate a la lista y asegurá tu lugar entre los primeros 1.000."
          >
            <WaitlistForm onSignal />
          </CtaBanner>
        </div>
      </main>

      <div className={section}>
        <Footer
          left="© 2026 · Hecho en Uruguay"
          right="SCRAPBOOK/01"
          links={[
            {
              label: "GitHub",
              href: "https://github.com/franco-roura/scrapbook-01",
              target: "_blank",
            },
          ]}
        />
      </div>
      <Toaster />
    </>
  );
}
