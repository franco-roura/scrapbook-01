// Parts this page needs that Module/01 doesn't have yet, built from its tokens. Each is a
// candidate for the kit; keep them token-only so moving them there is a copy.
import { Panel } from "@franco_roura/module-01";
import { Check, Plus, X } from "lucide-react";
import type { ReactNode } from "react";

/** A short exchange with an assistant: what you said, then its answer in a signal-tint bubble. */
export function ChatExchange({ you, ai, serial }: { you: string; ai: string; serial?: string }) {
  return (
    <Panel serial={serial} notch="sm" bodyClassName="flex h-full flex-col gap-4 p-6">
      <p className="hud text-ink-soft">Vos</p>
      <p className="text-body-md text-ink">“{you}”</p>
      <div className="mt-auto rounded-md bg-signal-tint px-4 py-3">
        <p className="hud-sm mb-1 text-signal-ink">Tu IA</p>
        <p className="text-body-sm text-ink">{ai}</p>
      </div>
    </Panel>
  );
}

/** Two shares of one whole on a sunken track: the first in ink, the second in signal. */
export function ShareBar({ parts, label }: { parts: [ShareBarPart, ShareBarPart]; label: string }) {
  const [a, b] = parts;
  return (
    <div
      role="img"
      aria-label={label}
      className="flex h-12 overflow-hidden rounded-full bg-shell-recess shadow-press"
    >
      <span
        className="hud grid place-items-center bg-ink text-shell"
        style={{ flexGrow: a.share, flexBasis: 0 }}
      >
        {a.label}
      </span>
      <span
        className="hud grid place-items-center bg-signal text-ink"
        style={{ flexGrow: b.share, flexBasis: 0 }}
      >
        {b.label}
      </span>
    </div>
  );
}
type ShareBarPart = { share: number; label: string };

/** "Instead of / you get" lines: a muted cross, then a green check. */
export function CompareList({ instead, get }: { instead: string; get: string }) {
  return (
    <ul className="mt-6 grid gap-3 border-t border-seam pt-5 text-body-sm">
      <li className="flex items-start gap-3 text-ink-soft">
        <X aria-hidden className="mt-0.5 size-4 shrink-0" strokeWidth={1.75} />
        <span>
          <span className="sr-only">En lugar de: </span>
          {instead}
        </span>
      </li>
      <li className="flex items-start gap-3 text-ink">
        <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-income-ink" strokeWidth={2} />
        <span>
          <span className="sr-only">Con Scrapbook/01: </span>
          {get}
        </span>
      </li>
    </ul>
  );
}

/** A question that opens to its answer: a molded part with a plus key that turns when open. */
export function FaqItem({ question, children }: { question: string; children: ReactNode }) {
  return (
    <details className="group rounded-lg bg-shell shadow-float open:shadow-float">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-4 rounded-lg px-6 py-5 text-label text-ink outline-offset-2 focus-visible:outline-2 focus-visible:outline-signal-ink [&::-webkit-details-marker]:hidden">
        {question}
        <span className="grid size-8 shrink-0 place-items-center rounded-full bg-shell-recess shadow-press transition-transform group-open:rotate-45">
          <Plus aria-hidden className="size-4" strokeWidth={1.75} />
        </span>
      </summary>
      <p className="px-6 pb-6 text-body-md text-ink-soft">{children}</p>
    </details>
  );
}
