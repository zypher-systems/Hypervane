import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { BarButton, MoreMenu } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import type { NetIface } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

const types = [
  { id: "bridge", label: "Bridge" },
  { id: "eth", label: "Ethernet" },
  { id: "vlan", label: "VLAN" },
  { id: "bond", label: "Bond" },
  { id: "OVSBridge", label: "OVS bridge" },
  { id: "OVSBond", label: "OVS bond" },
  { id: "OVSIntPort", label: "OVS port" },
] as const;

function typeLabel(id?: string) {
  return types.find((t) => t.id === id)?.label ?? id ?? "Interface";
}

function join(parts: (string | false | undefined)[]) {
  return parts.filter(Boolean).join(" · ");
}

export function NetworkPanel({ node, writable }: { node: string; writable: boolean }) {
  const qc = useQueryClient();
  const ifaces = useQuery({ queryKey: ["node-net", node], queryFn: () => api.nodeNetwork(node) });
  const [dialog, setDialog] = useState<{ kind: "edit"; iface: NetIface } | { kind: "add"; type: string } | { kind: "delete"; iface: NetIface } | null>(null);

  function close() {
    setDialog(null);
  }
  function saved() {
    setDialog(null);
    void qc.invalidateQueries({ queryKey: ["node-net", node] });
  }

  const rows = ifaces.data ?? [];

  return (
    <div className="space-y-4">
      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/15 px-4 py-3">
          <div className="text-xs font-medium text-ink-dim">Network</div>
          {writable && (
            <div className="flex gap-2">
              <BarButton
                onClick={async () => {
                  attachTask((await api.applyNodeNetwork(node)).upid, [["node-net", node]]);
                }}
              >
                Apply
              </BarButton>
              <BarButton
                onClick={async () => {
                  await api.revertNodeNetwork(node);
                  void qc.invalidateQueries({ queryKey: ["node-net", node] });
                }}
              >
                Revert
              </BarButton>
              <MoreMenu
                label="Add"
                items={types.map((t) => ({
                  id: t.id,
                  label: t.label,
                  onClick: () => setDialog({ kind: "add", type: t.id }),
                }))}
              />
            </div>
          )}
        </div>
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No interfaces</td>
              </tr>
            )}
            {rows.map((n) => {
              const summary = join([
                n.iface,
                n.cidr || n.address,
                n.bridge_ports ? `ports ${n.bridge_ports}` : "",
                n.active ? "" : "pending",
              ]) || "—";
              const edit = writable ? () => setDialog({ kind: "edit", iface: n }) : undefined;
              return (
                <tr
                  key={n.iface}
                  className={`border-t border-line ${edit ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
                  onClick={edit}
                >
                  <td className="px-4 py-2 font-medium">{typeLabel(n.type)}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    {edit && <Button size="sm" variant="outline" onClick={edit}>Edit</Button>}
                    {writable && (
                      <Button size="sm" variant="danger" onClick={() => setDialog({ kind: "delete", iface: n })}>Delete</Button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Card>
      {(dialog?.kind === "add" || dialog?.kind === "edit") && (
        <IfaceDialog
          node={node}
          iface={dialog.kind === "edit" ? dialog.iface : null}
          type={dialog.kind === "add" ? dialog.type : dialog.iface.type}
          onClose={close}
          onSaved={saved}
        />
      )}
      {dialog?.kind === "delete" && (
        <ConfirmDialog
          title={`Delete ${dialog.iface.iface}?`}
          body="Removes the interface from the pending config. Apply to make it live."
          danger
          confirmLabel="Delete"
          onClose={close}
          onConfirm={async () => {
            await api.deleteNodeNetwork(node, dialog.iface.iface);
            saved();
          }}
        />
      )}
    </div>
  );
}

function IfaceDialog({
  node,
  iface,
  type,
  onClose,
  onSaved,
}: {
  node: string;
  iface: NetIface | null;
  type?: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [form, setForm] = useState({
    iface: iface?.iface || "",
    type: iface?.type || type || "bridge",
    cidr: iface?.cidr || iface?.address || "",
    gateway: iface?.gateway || "",
    bridge_ports: iface?.bridge_ports || "",
    comments: iface?.comments || "",
  });
  const [error, setError] = useState("");
  return (
    <Modal
      title={iface ? `Edit ${iface.iface}` : `Add ${typeLabel(form.type)}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              if (iface) await api.updateNodeNetwork(node, iface.iface, form);
              else await api.createNodeNetwork(node, form);
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
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Name"><Input value={form.iface} disabled={!!iface} onChange={(e) => setForm({ ...form, iface: e.target.value })} /></Field>
        <Field label="Type">
          <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
            {types.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
          </Select>
        </Field>
        <Field label="CIDR"><Input value={form.cidr} onChange={(e) => setForm({ ...form, cidr: e.target.value })} /></Field>
        <Field label="Gateway"><Input value={form.gateway} onChange={(e) => setForm({ ...form, gateway: e.target.value })} /></Field>
        <Field label="Bridge ports"><Input value={form.bridge_ports} onChange={(e) => setForm({ ...form, bridge_ports: e.target.value })} /></Field>
        <Field label="Comment"><Input value={form.comments} onChange={(e) => setForm({ ...form, comments: e.target.value })} /></Field>
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
