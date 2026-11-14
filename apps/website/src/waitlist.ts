// Waitlist sign-ups go to the "Lista de espera" Google Form (answers land in Google Sheets, with an
// email notice). Google Forms accepts a plain form post but sends no CORS headers, so the request is
// "no-cors": it's delivered, but the response can't be read. Only network failures are reported.

export const FORM_URL =
  "https://docs.google.com/forms/d/e/1FAIpQLSdrA_gm_D_eVkIi9yQSZwIz776cye3_4ham1l0pwbaaqF2-Tg/formResponse";

const FIELDS = { name: "entry.1777572684", email: "entry.1766984944" } as const;

export type Signup = { name: string; email: string };

export type SignupError = { field: keyof Signup; message: string };

/** Checks a sign-up before it's sent. Returns the first problem, or null. */
export function validate(s: Signup): SignupError | null {
  if (!s.name.trim()) return { field: "name", message: "Escribí tu nombre." };
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.email.trim())) {
    return { field: "email", message: "Revisá tu email." };
  }
  return null;
}

/** The Google Form body for a sign-up. */
export function encode(s: Signup): URLSearchParams {
  const body = new URLSearchParams();
  body.append(FIELDS.name, s.name.trim());
  body.append(FIELDS.email, s.email.trim());
  return body;
}

/** Sends a sign-up. Throws only when the request couldn't be made. */
export async function submit(s: Signup, send: typeof fetch = fetch): Promise<void> {
  await send(FORM_URL, { method: "POST", mode: "no-cors", body: encode(s) });
}
