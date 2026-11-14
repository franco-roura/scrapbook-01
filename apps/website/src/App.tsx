import { Button, NavBar, Panel, SectionHeader, Toaster } from "@franco_roura/module-01";
import { CtaBanner, Footer, Hero, Tag } from "@franco_roura/module-01/site";

import { ChatExchange, CompareList, FaqItem, ShareBar } from "./parts";
import { WaitlistForm } from "./WaitlistForm";

const sources = [
  "Instagram",
  "TikTok",
  "YouTube",
  "X",
  "Facebook",
  "WhatsApp",
  "Telegram",
  "Cualquier web",
];

const pains = [
  {
    title: "Está todo desparramado",
    text: "Un poco en Instagram, otro en TikTok, links en WhatsApp, videos en YouTube…",
  },
  {
    title: "Nunca lo encontrás",
    text: "Sabés que lo guardaste, pero no en qué app ni en qué carpeta.",
  },
  { title: "Desaparece", text: "La cuenta se cerró, el post se borró, y tu guardado quedó vacío." },
  {
    title: "Te olvidás de que existía",
    text: "El restaurante que querías probar, el tip para tus anuncios: guardado y perdido.",
  },
];

const steps = [
  {
    title: "Compartí desde cualquier app",
    text: "Tocás “compartir” como siempre y lo mandás a tu biblioteca. Un toque, sin pensar dónde va.",
  },
  {
    title: "Lo respaldamos y lo entendemos",
    text: "Guardamos el contenido y su idea central: qué dice, qué lugar es, qué tip da. Y lo etiquetamos solos.",
  },
  {
    title: "Vuelve cuando importa",
    text: "Tu IA (Claude, ChatGPT u otra) se conecta a tu biblioteca y te lo trae en el momento justo.",
  },
];

const examples = [
  {
    you: "Estoy armando el viaje a Buenos Aires",
    ai: "Guardaste 9 reels de BA: 4 restaurantes, 2 bares en Palermo y un tip para cambiar plata. ¿Te los sumo al itinerario?",
  },
  {
    you: "Ayudame con los anuncios de mi emprendimiento",
    ai: "Antes de arrancar: tenés 6 videos guardados sobre anuncios. Te resumo los 3 consejos que más se repiten.",
  },
  {
    you: "¿Dónde ceno hoy?",
    ai: "Hace dos meses guardaste un reel de una parrilla a 10 cuadras tuyas que querías probar.",
  },
];

const faqs = [
  {
    q: "¿Con qué IA funciona?",
    a: "Con la que ya usás. Te conectás desde Claude, ChatGPT u otras compatibles con conectores (MCP), y tus guardados pasan a ser parte de su memoria.",
  },
  {
    q: "¿Qué puedo guardar?",
    a: "Reels, TikToks, videos de YouTube, posts de X o Facebook, mensajes reenviados de WhatsApp o Telegram y cualquier página web.",
  },
  {
    q: "¿Quién ve mis guardados?",
    a: "Solo vos y la IA que vos conectes. Tu biblioteca es privada.",
  },
  {
    q: "¿Cuándo lo puedo usar?",
    a: "Estamos abriendo de a poco. Los primeros de la lista entran antes y con condición de fundador.",
  },
];

/** Jumps to the waitlist form at the top and puts the cursor in its first field. */
function focusWaitlist() {
  document.querySelector<HTMLInputElement>("#lista input")?.focus({ preventScroll: true });
}

const section = "mx-auto w-full max-w-content-max px-6 md:px-12";

export default function App() {
  return (
    <>
      <div className="sticky top-4 z-40 mx-auto w-full max-w-content-max px-4 md:px-12">
        <NavBar
          brand={<span className="font-display text-display-sm">Scrapbook/01</span>}
          links={[
            { label: "Cómo funciona", href: "#como-funciona", desktopOnly: true },
            { label: "Por qué", href: "#por-que", desktopOnly: true },
            { label: "Precio", href: "#precio", desktopOnly: true },
            { label: "Preguntas", href: "#preguntas", desktopOnly: true },
          ]}
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
            // Michroma is wide: the hero's phone size doesn't fit the full name on a 375px screen.
            title={<span className="max-md:text-display-md">Scrapbook/01</span>}
            subtitle="Lista de espera · primeros 1.000 lugares"
            lead={
              <>
                <strong className="text-ink">
                  Guardaste mil cosas “para después”. ¿Cuándo es después?
                </strong>{" "}
                Compartí cualquier reel, TikTok, post o link. Nosotros respaldamos el contenido, lo
                ordenamos solos y tu IA te lo trae justo cuando lo necesitás, sin que tengas que
                buscarlo.
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
                src="/share.png"
                alt="La hoja para compartir de la app: un link de Instagram y el campo “What is it about?” con una descripción escrita"
                className="object-top"
              />
            }
            imageLabel="SHARE SHEET"
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
            title="Compartí, olvidate, y listo"
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
              <p className="hud text-signal-ink">No guardamos links. Guardamos ideas.</p>
              <h3 className="mt-4 font-display text-display-sm text-ink">
                Si el post desaparece, vos no lo perdés
              </h3>
              <p className="mt-3 text-body-md text-ink-soft">
                Otras apps guardan el enlace. Nosotros respaldamos el contenido y extraemos lo que
                importa: el lugar, la receta, el consejo, el dato. Aunque el original se borre, la
                idea queda tuya.
              </p>
              <CompareList
                instead="Un link que mañana puede dar error"
                get="El contenido respaldado y resumido en una línea"
              />
            </Panel>
            <Panel serial="D-02" hinge="65%" bodyClassName="p-8 pt-12">
              <p className="hud text-signal-ink">Tu IA se acuerda por vos</p>
              <h3 className="mt-4 font-display text-display-sm text-ink">
                No tenés que ponerte a buscar
              </h3>
              <p className="mt-3 text-body-md text-ink-soft">
                Nuestro conector para tu IA no espera a que le preguntes. Está atento a lo que estás
                haciendo y aparece con lo que guardaste justo cuando te sirve, incluso cuando menos
                lo esperás.
              </p>
              <CompareList
                instead="Una carpeta más que tenés que revisar"
                get="Tus guardados aparecen solos en la conversación"
              />
            </Panel>
          </div>
          <div className="mt-8 grid gap-8 md:grid-cols-3">
            {examples.map((e, i) => (
              <ChatExchange key={e.you} serial={`E-0${i + 1}`} you={e.you} ai={e.ai} />
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
              Sin suscripción que te olvidás de cancelar. Comprás packs de créditos de IA y los usás
              para procesar tus guardados. Nosotros elegimos y optimizamos los mejores modelos para
              que cada crédito rinda.
            </p>
            <Panel serial="K-01 · 80/20" cross bodyClassName="flex flex-col gap-4 p-8 pt-12">
              <p className="text-label text-ink">De cada peso que gastás en créditos</p>
              <ShareBar
                label="80% procesa tus guardados, 20% es lo que cobramos"
                parts={[
                  { share: 80, label: "80% tus guardados" },
                  { share: 20, label: "20%" },
                ]}
              />
              <ul className="grid gap-1 text-body-sm text-ink-soft">
                <li>
                  <span aria-hidden className="mr-2 inline-block size-2.5 rounded-xs bg-ink" />
                  80%: va directo a los modelos de IA que ordenan tu contenido.
                </li>
                <li>
                  <span aria-hidden className="mr-2 inline-block size-2.5 rounded-xs bg-signal" />
                  20%: es lo que cobramos nosotros. Si no lo usás, no ganamos.
                </li>
              </ul>
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
            <div className="mt-12 grid gap-4">
              {faqs.map((f) => (
                <FaqItem key={f.q} question={f.q}>
                  {f.a}
                </FaqItem>
              ))}
            </div>
          </div>
        </section>

        <div className={section}>
          <CtaBanner
            id="lista"
            code="S-06 · LISTA DE ESPERA"
            title="Lo que guardaste, cuando lo necesitás."
            text="Sumate a la lista y asegurá tu lugar entre los primeros 1.000."
            action={
              <Button asChild variant="secondary" size="lg">
                <a href="#lista" onClick={focusWaitlist}>
                  Quiero mi lugar →
                </a>
              </Button>
            }
          />
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
