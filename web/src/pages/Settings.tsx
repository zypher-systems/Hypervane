import { Badge, Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api, ApiError } from "@/lib/api";
import { composePVEToken } from "@/lib/pveToken";
import { getActiveConnection, setActiveConnection } from "@/lib/connection";
import type { Connection } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";

type FormState = {
  name: string;
  pveUrl: string;
  tokenUser: string;
  tokenSecret: string;
  tlsMode: string;
  tlsPin: string;
};

const emptyForm: FormState = {
  name: "",
  pveUrl: "https://192.168.1.10:8006",
  tokenUser: "",
  tokenSecret: "",
  tlsMode: "skip-verify",
  tlsPin: "",
};

export function SettingsPage() {
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ["admin-connections"], queryFn: api.adminConnections });
  const [form, setForm] = useState<FormState>(emptyForm);
  const [editing, setEditing] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [remove, setRemove] = useState<Connection | null>(null);

  const connections = list.data ?? [];
  const empty = !list.isLoading && connections.length === 0;

  function startAdd() {
    setEditing(null);
    setForm(emptyForm);
    setError("");
    setOpen(true);
  }

  function startEdit(c: Connection) {
    setEditing(c.id);
    setForm({
      name: c.name,
      pveUrl: c.pveUrl,
      tokenUser: "",
      tokenSecret: "",
      tlsMode: c.tlsMode || "skip-verify",
      tlsPin: "",
    });
    setError("");
    setOpen(true);
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const body: Record<string, unknown> = {
        name: form.name,
        pveUrl: form.pveUrl,
        tlsMode: form.tlsMode,
        tlsPin: form.tlsPin,
      };
      const token = composePVEToken(form.tokenUser, form.tokenSecret);
      if (token) body.token = token;
      const saved = editing
        ? await api.updateConnection(editing, body)
        : await api.createConnection({ ...body, token });
      if (!getActiveConnection()) setActiveConnection(saved.id);
      setOpen(false);
      setForm(emptyForm);
      await qc.invalidateQueries();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Save failed");
    } finally {
      setBusy(false);
    }
  }

  async function onTest(c?: Connection) {
    setBusy(true);
    setError("");
    try {
      if (c) {
        await api.testSavedConnection(c.id);
      } else {
        await api.testConnection({
          name: form.name,
          pveUrl: form.pveUrl,
          token: composePVEToken(form.tokenUser, form.tokenSecret),
          tlsMode: form.tlsMode,
          tlsPin: form.tlsPin,
        });
      }
      await qc.invalidateQueries({ queryKey: ["admin-connections"] });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Test failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <header className="flex items-end justify-between gap-4">
        <div>
          <p className="text-xs font-medium text-ink-dim">Settings</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">Connections</h1>
          <p className="mt-1 text-sm text-ink-dim">
            Each connection is one Proxmox API endpoint. Tokens are encrypted in Postgres and never sent back to the browser.
          </p>
        </div>
        {!empty && (
          <Button size="sm" onClick={startAdd}>
            Add connection
          </Button>
        )}
      </header>

      {list.error instanceof ApiError && (
        <p className="text-sm text-rose-400">{list.error.message}</p>
      )}

      {empty && !open && (
        <Card className="p-6">
          <h2 className="text-lg font-semibold">Add your first connection</h2>
          <p className="mt-1 text-sm text-ink-dim">
            Point Hypervane at a cluster or a standalone node. You can add more later and switch between them in the header.
          </p>
          <div className="mt-4">
            <Button onClick={startAdd}>Add a connection</Button>
          </div>
        </Card>
      )}

      {open && (
        <Card className="p-6">
          <h2 className="text-lg font-semibold">{editing ? "Edit connection" : "New connection"}</h2>
          <form className="mt-4 grid gap-4" onSubmit={onSubmit}>
            <Field label="Name">
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} placeholder="zypher-systems" />
            </Field>
            <Field label="Proxmox URL">
              <Input value={form.pveUrl} onChange={(e) => setForm({ ...form, pveUrl: e.target.value })} />
            </Field>
            <Field label="User">
              <Input
                autoComplete="off"
                placeholder="root@pam!hypervane"
                value={form.tokenUser}
                onChange={(e) => setForm({ ...form, tokenUser: e.target.value })}
              />
              <p className="mt-1 text-xs text-ink-faint">PVE user and token id, as shown when you create the token.</p>
            </Field>
            <Field label={editing ? "API key (leave blank to keep)" : "API key"}>
              <Input
                type="password"
                autoComplete="off"
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                value={form.tokenSecret}
                onChange={(e) => setForm({ ...form, tokenSecret: e.target.value })}
              />
              <p className="mt-1 text-xs text-ink-faint">The secret shown once. Pasting the full user@realm!id=secret still works.</p>
            </Field>
            <Field label="TLS">
              <Select value={form.tlsMode} onChange={(e) => setForm({ ...form, tlsMode: e.target.value })}>
                <option value="skip-verify">Skip verify (self-signed)</option>
                <option value="pin">SHA-256 pin</option>
              </Select>
            </Field>
            {form.tlsMode === "pin" && (
              <Field label="Certificate SHA-256 pin">
                <Input value={form.tlsPin} onChange={(e) => setForm({ ...form, tlsPin: e.target.value })} />
              </Field>
            )}
            {error && <p className="text-sm text-rose-400">{error}</p>}
            <div className="flex flex-wrap gap-2">
              <Button disabled={busy}>{busy ? "Saving…" : "Save"}</Button>
              <Button type="button" variant="ghost" disabled={busy || (!composePVEToken(form.tokenUser, form.tokenSecret) && !editing)} onClick={() => void onTest()}>
                Test
              </Button>
              <Button type="button" variant="ghost" onClick={() => setOpen(false)}>
                Cancel
              </Button>
            </div>
          </form>
        </Card>
      )}

      <div className="grid gap-3">
        {connections.map((c) => (
          <Card key={c.id} className="p-5">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-base font-semibold">{c.name}</h3>
                  {c.online === true && <Badge tone="green">online</Badge>}
                  {c.online === false && <Badge tone="rose">error</Badge>}
                  {!c.enabled && <Badge>disabled</Badge>}
                </div>
                <p className="mt-1 font-mono text-xs text-ink-dim">{c.pveUrl}</p>
                {c.clusterName && <p className="mt-1 text-xs text-ink-dim">Cluster {c.clusterName}</p>}
                {c.nodes && c.nodes.length > 0 && (
                  <p className="mt-1 text-xs text-ink-dim">{c.nodes.join(" · ")}</p>
                )}
                {c.error && <p className="mt-2 text-sm text-rose-400">{c.error}</p>}
              </div>
              <div className="flex flex-wrap gap-2">
                <Button size="sm" variant="ghost" onClick={() => setActiveConnection(c.id)}>
                  Use
                </Button>
                <Button size="sm" variant="ghost" disabled={busy} onClick={() => void onTest(c)}>
                  Test
                </Button>
                <Button size="sm" variant="ghost" onClick={() => startEdit(c)}>
                  Edit
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setRemove(c)}>
                  Remove
                </Button>
              </div>
            </div>
          </Card>
        ))}
      </div>

      {remove && (
        <ConfirmDialog
          title={`Remove ${remove.name}?`}
          body="The encrypted token is deleted. Guests on other connections are unchanged."
          confirmLabel="Remove"
          danger
          onClose={() => setRemove(null)}
          onConfirm={async () => {
            await api.deleteConnection(remove.id);
            if (getActiveConnection() === remove.id) setActiveConnection("");
            setRemove(null);
            await qc.invalidateQueries();
          }}
        />
      )}
    </div>
  );
}
