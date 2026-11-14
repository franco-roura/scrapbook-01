import { Button, Field, fault, Input } from "@franco_roura/module-01";
import { useId, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { submit, validate, type Signup } from "./waitlist";

/** Name, email and "Quiero mi lugar", in one row on wide screens. */
export function WaitlistForm() {
  const id = useId();
  const [values, setValues] = useState<Signup>({ name: "", email: "" });
  const [error, setError] = useState<{ field: keyof Signup; message: string } | null>(null);
  const [sending, setSending] = useState(false);

  const fieldId = (f: keyof Signup) => `${id}-${f}`;

  function change(field: keyof Signup, value: string) {
    setValues((v) => ({ ...v, [field]: value }));
    if (error?.field === field) setError(null);
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    const problem = validate(values);
    if (problem) {
      setError(problem);
      fault(fieldId(problem.field));
      toast.error(problem.message);
      return;
    }
    setSending(true);
    try {
      await submit(values);
      setValues({ name: "", email: "" });
      toast.success("¡Listo! Ya estás en la lista. Te escribimos cuando abramos tu lugar");
    } catch {
      toast.error("No pudimos anotarte. Probá de nuevo en un minuto");
    } finally {
      setSending(false);
    }
  }

  return (
    <form
      noValidate
      onSubmit={onSubmit}
      className="flex w-full max-w-2xl flex-col gap-4 md:flex-row md:items-end"
    >
      <Field
        label="Tu nombre"
        htmlFor={fieldId("name")}
        error={error?.field === "name" ? error.message : undefined}
        className="flex-1"
      >
        <Input
          id={fieldId("name")}
          name="name"
          autoComplete="name"
          value={values.name}
          onChange={(e) => change("name", e.target.value)}
          aria-invalid={error?.field === "name" || undefined}
        />
      </Field>
      <Field
        label="Tu email"
        htmlFor={fieldId("email")}
        error={error?.field === "email" ? error.message : undefined}
        className="flex-1"
      >
        <Input
          id={fieldId("email")}
          name="email"
          type="email"
          autoComplete="email"
          inputMode="email"
          value={values.email}
          onChange={(e) => change("email", e.target.value)}
          aria-invalid={error?.field === "email" || undefined}
        />
      </Field>
      <Button type="submit" size="lg" disabled={sending} arrow>
        {sending ? "Enviando…" : "Quiero mi lugar"}
      </Button>
    </form>
  );
}
