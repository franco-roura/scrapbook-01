import { describe, expect, test, vi } from "vite-plus/test";

import { encode, FORM_URL, submit, validate } from "./waitlist";

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

describe("submit", () => {
  test("posts the trimmed fields to the Google Form", async () => {
    const send = vi.fn(async () => new Response(null));
    await submit({ name: " Ana ", email: " ana@example.com " }, send as unknown as typeof fetch);

    expect(send).toHaveBeenCalledTimes(1);
    const [url, init] = send.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe(FORM_URL);
    expect(init.mode).toBe("no-cors");
    const body = init.body as URLSearchParams;
    expect(body.toString()).toBe(encode({ name: "Ana", email: "ana@example.com" }).toString());
    expect(body.get("entry.1766984944")).toBe("ana@example.com");
  });
});
