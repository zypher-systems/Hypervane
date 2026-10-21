import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import { snapKeys } from "@/lib/tasks";
import type { Snapshot } from "@/types";
import { useState } from "react";

function join(parts: (string | false | undefined)[]) {
  return parts.filter(Boolean).join(" · ");
}

export function SnapshotsPanel({
  vmid, node, type, items, canSnapshot, onChange,
}: {
  vmid: number; node: string; type: string; items: Snapshot[];
  canSnapshot: boolean; onChange: () => void;
}) {
  const can = useCan();
  const canWrite = can(`/vms/${vmid}`, "VM.Snapshot");
  const snaps = items.filter((s) => s.name !== "current");
  const [dialog, setDialog] = useState<
    | { kind: "add" }
    | { kind: "edit"; snap: Snapshot }
    | { kind: "rollback"; name: string }
    | { kind: "delete"; name: string }
    | null
  >(null);

  function close() {
    setDialog(null);
  }

  return (
    <div className="space-y-4">
      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/15 px-4 py-3">
          <div className="text-xs font-medium text-ink-dim">Snapshots</div>
          {canWrite && canSnapshot && <BarButton onClick={() => setDialog({ kind: "add" })}>Add</BarButton>}
        </div>
        {!canSnapshot && (
          <p className="px-4 pt-3 text-xs text-ink-dim">This guest storage cannot take snapshots.</p>
        )}
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {snaps.length === 0 && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No snapshots</td>
              </tr>
            )}
            {snaps.map((s) => {
              const summary = join([
                s.description,
                s.vmstate ? "RAM" : "",
                s.snaptime ? new Date(s.snaptime * 1000).toLocaleString() : "",
              ]) || "—";
              const edit = canWrite ? () => setDialog({ kind: "edit", snap: s }) : undefined;
              return (
                <tr
                  key={s.name}
                  className={`border-t border-line ${edit ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
                  onClick={edit}
                >
                  <td className="px-4 py-2 font-medium">{s.name}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    {edit && <Button size="sm" variant="outline" onClick={edit}>Edit</Button>}
                    <Button size="sm" variant="outline" onClick={() => setDialog({ kind: "rollback", name: s.name })}>Rollback</Button>
                    {canWrite && (
                      <Button size="sm" variant="danger" onClick={() => setDialog({ kind: "delete", name: s.name })}>Delete</Button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Card>
      {dialog?.kind === "add" && (
        <AddSnapshotDialog vmid={vmid} node={node} type={type} onClose={close} />
      )}
      {dialog?.kind === "edit" && (
        <EditSnapshotDialog
          vmid={vmid}
          node={node}
          type={type}
          snap={dialog.snap}
          onClose={close}
          onSaved={onChange}
        />
      )}
      {(dialog?.kind === "rollback" || dialog?.kind === "delete") && (
        <ConfirmDialog
          title={`${dialog.kind} ${dialog.name}?`}
          body={dialog.kind === "rollback" ? "The guest will revert to this snapshot." : "The snapshot will be permanently removed."}
          danger
          onClose={close}
          onConfirm={async () => {
            const res = dialog.kind === "delete"
              ? await api.deleteSnapshot(vmid, dialog.name, node, type)
              : await api.rollbackSnapshot(vmid, dialog.name, node, type);
            attachTask(res.upid, snapKeys(vmid));
            close();
          }}
        />
      )}
    </div>
  );
}

function AddSnapshotDialog({
  vmid, node, type, onClose,
}: {
  vmid: number; node: string; type: string; onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [ram, setRam] = useState(type === "qemu");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Add snapshot"
      onClose={onClose}
      footer={
        <Button
          disabled={!name}
          onClick={async () => {
            setError("");
            try {
              const res = await api.createSnapshot(vmid, { name, description, vmstate: type === "qemu" && ram }, node, type);
              attachTask(res.upid, snapKeys(vmid));
              onClose();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Create snapshot failed");
            }
          }}
        >
          Create
        </Button>
      }
    >
      <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
      <Field label="Description"><Input value={description} onChange={(e) => setDescription(e.target.value)} /></Field>
      {type === "qemu" && (
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={ram} onChange={(e) => setRam(e.target.checked)} />
          Include RAM
        </label>
      )}
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function EditSnapshotDialog({
  vmid, node, type, snap, onClose, onSaved,
}: {
  vmid: number; node: string; type: string; snap: Snapshot; onClose: () => void; onSaved: () => void;
}) {
  const [description, setDescription] = useState(snap.description ?? "");
  const [error, setError] = useState("");
  return (
    <Modal
      title={`Edit ${snap.name}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              await api.updateSnapshot(vmid, snap.name, { description }, node, type);
              onClose();
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <Field label="Description"><Input value={description} onChange={(e) => setDescription(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
