import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

export function SetupPage() {
  const nav = useNavigate();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [form, setForm] = useState({
    username: "admin",
    password: "",
  });

  return (
    <div className="mx-auto flex min-h-full max-w-xl flex-col items-center justify-center px-4">
      <img src="/hypervane.svg" alt="Hypervane" className="mb-6 h-16 w-16" />
      <Card className="w-full p-8">
        <p className="font-mono text-xs uppercase tracking-[0.25em] text-accent">First run</p>
        <h1 className="mt-2 text-3xl font-semibold">Create the local admin</h1>
        <p className="mt-2 text-sm text-ink-dim">
          This account stays in Hypervane. You will add Proxmox API connections after you sign in.
        </p>
        <form
          className="mt-6 grid gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setError("");
            try {
              await api.setup(form);
              nav({ to: "/settings" });
            } catch (err) {
              setError(err instanceof Error ? err.message : "Setup failed");
            } finally {
              setBusy(false);
            }
          }}
        >
          <Field label="Hypervane username">
            <Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} />
          </Field>
          <Field label="Hypervane password">
            <Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
          </Field>
          {error && <p className="text-sm text-rose-400">{error}</p>}
          <Button disabled={busy}>{busy ? "Creating…" : "Create admin and continue"}</Button>
        </form>
      </Card>
    </div>
  );
}
