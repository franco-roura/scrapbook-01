import { describe, expect, test, vi } from "vite-plus/test";

import { submit, validate } from "./waitlist";

describe("validate", () => {
  test("accepts a name and an email", () => {
    expect(validate({ name: "Ana", email: "ana@example.com" })).toBeNull();
  });

  test("asks for a name first", () => {
    expect(validate({ name: "  ", email: "nope" })?.field).toBe("name");
  });

  test("rejects an email without a domain", () => {
    expect(validate({ name: "Ana", email: "ana@example" })?.field).toBe("email");
  });
});

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

describe("submit", () => {
  test("posts the trimmed fields to the API's waitlist", async () => {
    const send = vi.fn(async () => json(200, { state: "pending" }));
    const out = await submit(
      { name: " Ana ", email: " ana@example.com " },
      send as unknown as typeof fetch,
      "https://api.example.test",
    );

    expect(out).toBe("joined");
    const [url, init] = send.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe("https://api.example.test/v1/waitlist");
    expect(init.method).toBe("POST");
    expect(JSON.parse(init.body as string)).toEqual({ name: "Ana", email: "ana@example.com" });
  });

  test("tells existing users apart", async () => {
    const send = vi.fn(async () =>
      json(409, { error: { code: "already_user", message: "Ya tenés cuenta" } }),
    );
    const out = await submit(
      { name: "Ana", email: "ana@example.com" },
      send as unknown as typeof fetch,
      "",
    );
    expect(out).toBe("already_user");
  });

  test("throws the API's message on failure", async () => {
    const send = vi.fn(async () =>
      json(502, { error: { code: "waitlist_failed", message: "No pudimos anotarte." } }),
    );
    await expect(
      submit({ name: "Ana", email: "ana@example.com" }, send as unknown as typeof fetch, ""),
    ).rejects.toThrow("No pudimos anotarte.");
  });
});
