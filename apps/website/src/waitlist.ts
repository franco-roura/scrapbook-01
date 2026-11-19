// Waitlist sign-ups go to the API (POST /v1/waitlist), which adds them to the WorkOS AuthKit
// waitlist. WorkOS emails a confirmation; approving the entry in the WorkOS dashboard (Users →
// Waitlist) emails an invitation to create an account. The API URL is set at build time by SST.

export const API_URL: string = (import.meta.env.VITE_API_URL ?? "").replace(/\/$/, "");

export type Signup = { name: string; email: string };

export type SignupError = { field: keyof Signup; message: string };

/** What happened to a sign-up the API answered. */
export type Outcome = "joined" | "already_user";

/**
 * The API answered with an error. `message` is the API's text, for the toast; `report` is what goes
 * to Sentry, built only from the status and error code so nothing the user typed can ride along.
 */
export class SubmitError extends Error {
  readonly status: number;
  readonly code: string | undefined;

  constructor(message: string, status: number, code: string | undefined) {
    super(message);
    this.name = "SubmitError";
    this.status = status;
    this.code = code;
  }

  get report(): Error {
    const e = new Error(`Waitlist sign-up failed: HTTP ${this.status} ${this.code ?? "unknown"}`);
    e.name = "SubmitError";
    return e;
  }
}

/** Checks a sign-up before it's sent. Returns the first problem, or null. */
export function validate(s: Signup): SignupError | null {
  if (!s.name.trim()) return { field: "name", message: "Escribí tu nombre." };
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s.email.trim())) {
    return { field: "email", message: "Revisá tu email." };
  }
  return null;
}

/** Sends a sign-up. Throws when it couldn't be saved, with a message to show. */
export async function submit(
  s: Signup,
  send: typeof fetch = fetch,
  apiUrl: string = API_URL,
): Promise<Outcome> {
  const res = await send(`${apiUrl}/v1/waitlist`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name: s.name.trim(), email: s.email.trim() }),
  });
  if (res.ok) return "joined";
  const body = (await res.json().catch(() => null)) as {
    error?: { code?: string; message?: string };
  } | null;
  if (res.status === 409 && body?.error?.code === "already_user") return "already_user";
  throw new SubmitError(
    body?.error?.message ?? "No pudimos anotarte. Probá de nuevo en un minuto.",
    res.status,
    body?.error?.code,
  );
}
