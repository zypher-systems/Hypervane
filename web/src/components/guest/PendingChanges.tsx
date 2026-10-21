import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest } from "@/lib/guestHardware";
import { describePending } from "@/lib/pending";
import { useCan } from "@/lib/perms";
import type { PendingChange } from "@/types";
import { useState } from "react";

export function appliedPending(pending: PendingChange[] | undefined) {
  return (pending ?? []).filter((p) => p.pending !== undefined || !!p.delete);
}

export function PendingChanges({
  pending, vmid, node, type, config, onReverted,
}: {
  pending: PendingChange[];
  vmid?: number;
  node?: string;
  type?: string;
  config?: Record<string, unknown>;
  onReverted?: () => void;
}) {
  const can = useCan();
  const canEdit = vmid !== undefined && (can(`/vms/${vmid}`, "VM.Config") || can(`/vms/${vmid}`, "VM.Config.Options"));
  const items = appliedPending(pending);
  const [edits, setEdits] = useState<Record<string, string>>(() => {
    const next: Record<string, string> = {};
    for (const p of items) next[p.key] = p.delete ? "" : String(p.pending ?? "");
    return next;
  });
  const [editing, setEditing] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState("");
  if (!items.length) return null;
  return (
    <Card className="p-4">
      <div className="mb-3 flex items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-medium">Pending changes</h3>
          <p className="mt-0.5 text-xs text-ink-dim">These values apply after the guest restarts.</p>
        </div>
        <div className="flex gap-2">
          {canEdit && (
            <Button size="sm" variant="ghost" onClick={() => setEditing((v) => !v)}>
              {editing ? "Done" : "Edit"}
            </Button>
          )}
          {canEdit && editing && (
            <Button size="sm" onClick={async () => {
              if (vmid === undefined) return;
              setError("");
              try {
                const body: Record<string, unknown> = { digest: configDigest(config ?? {}) };
                for (const p of items) {
                  if (edits[p.key] !== undefined) body[p.key] = edits[p.key];
                }
                await api.updateGuest(vmid, body, node, type);
                onReverted?.();
              } catch (e) {
                setError(e instanceof Error ? e.message : "Save failed");
              }
            }}>Save</Button>
          )}
          {canEdit && onReverted && (
            <Button size="sm" variant="outline" onClick={() => setConfirm(true)}>Revert pending</Button>
          )}
        </div>
      </div>
      <div className="divide-y divide-white/[0.06]">
        {items.map((p) => {
          const row = describePending(p, config);
          return (
            <div key={p.key} className="py-2 first:pt-0 last:pb-0">
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <p className="text-sm font-medium">{row.label}</p>
                <p className="font-mono text-sm tabular-nums text-ink-dim">
                  <span>{row.from}</span>
                  <span className="mx-2 text-ink-faint">→</span>
                  <span className="text-ink">{row.to}</span>
                </p>
              </div>
              {editing && (
                <Field label={`Raw ${p.key}`}>
                  <Input
                    className="mt-2 font-mono text-xs"
                    value={edits[p.key] ?? ""}
                    onChange={(e) => setEdits((prev) => ({ ...prev, [p.key]: e.target.value }))}
                    disabled={!canEdit}
                    placeholder={p.delete ? "deleted" : ""}
                  />
                </Field>
              )}
            </div>
          );
        })}
      </div>
      {error && <p className="mt-2 text-sm text-rose-400">{error}</p>}
      {confirm && vmid !== undefined && (
        <ConfirmDialog
          title="Revert pending changes?"
          body="Discards config that has not been applied yet (needs a reboot or hotplug)."
          danger
          confirmLabel="Revert"
          onClose={() => setConfirm(false)}
          onConfirm={async () => {
            setError("");
            try {
              await api.revertPending(vmid, configDigest(config ?? {}), node, type);
              setConfirm(false);
              onReverted?.();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Revert failed");
              setConfirm(false);
            }
          }}
        />
      )}
    </Card>
  );
}
