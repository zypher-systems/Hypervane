import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { cloneCanLeaveNode } from "@/lib/clone";
import { storagesForContent } from "@/lib/storage";
import { attachTask, guestKeys } from "@/lib/tasks";
import type { Resource } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";

/**
 * Clone a guest or template. Proxmox only clones to another node when the
 * guest's volumes are on shared storage, so the node picker appears only when
 * that's possible, and says why when it isn't.
 */
export function CloneDialog({
  vmid,
  node,
  type,
  name,
  template,
  nodes,
  onClose,
}: {
  vmid: number;
  node: string;
  type: string;
  name: string;
  template: boolean;
  nodes: Resource[];
  onClose: () => void;
}) {
  const next = useQuery({ queryKey: ["nextid"], queryFn: api.nextid });
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const guest = useQuery({ queryKey: ["guest", vmid, node, type], queryFn: () => api.guest(vmid, node, type) });
  const [newid, setNewid] = useState(0);
  const [cloneName, setCloneName] = useState(`${name || `guest-${vmid}`}-clone`);
  const [target, setTarget] = useState(node);
  // Only a template can be linked-cloned.
  const [full, setFull] = useState(true);
  const [storage, setStorage] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!newid && next.data?.vmid) setNewid(next.data.vmid);
  }, [next.data, newid]);

  const all = storages.data ?? [];
  const config = guest.data?.config ?? {};
  const diskKind = type === "lxc" ? "rootdir" : "images";
  const sharedDisk = all.filter((s) => s.shared && storagesForContent([s], diskKind).length > 0);
  // Could any choice in this dialog take the clone off this node?
  const leave = cloneCanLeaveNode(type, config, all, true, sharedDisk[0]?.storage);
  const canPickNode = guest.isSuccess && storages.isSuccess && leave.allowed && nodes.length > 1;
  const elsewhere = target !== node;

  // Storage choices: on another node the clone must land on shared storage.
  const choices = useMemo(
    () => storagesForContent(all, diskKind, target).filter((s) => !elsewhere || s.shared),
    [all, diskKind, target, elsewhere],
  );
  const needsStorage = elsewhere && type === "lxc" && !cloneCanLeaveNode(type, config, all, false).allowed;
  useEffect(() => {
    if (storage && !choices.some((s) => s.storage === storage)) setStorage("");
  }, [choices, storage]);
  useEffect(() => {
    if (needsStorage && !storage && choices[0]) setStorage(choices[0].storage);
  }, [needsStorage, storage, choices]);

  const onlineNodes = nodes.filter((n) => n.node && (!n.status || n.status === "online"));
  const blocked = elsewhere && !cloneCanLeaveNode(type, config, all, full, storage).allowed;

  async function submit() {
    setBusy(true);
    setError("");
    try {
      const body: Record<string, unknown> = { newid, name: cloneName.trim(), full };
      if (elsewhere) body.target = target;
      if (full && storage) body.storage = storage;
      const res = await api.clone(vmid, body, node, type);
      attachTask(res.upid, [...guestKeys(vmid), ...guestKeys(res.vmid || newid), ["nextid"]]);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "The clone couldn't start");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      title={`Clone ${name || vmid}`}
      onClose={onClose}
      footer={
        <Button disabled={busy || !newid || !cloneName.trim() || blocked || guest.isLoading} onClick={() => void submit()}>
          {busy ? "Starting…" : "Clone"}
        </Button>
      }
    >
      <Field label="New VMID">
        <Input type="number" value={newid || ""} onChange={(e) => setNewid(Number(e.target.value))} />
      </Field>
      <Field label="Name">
        <Input value={cloneName} onChange={(e) => setCloneName(e.target.value)} />
      </Field>
      <Field label="Node">
        {canPickNode ? (
          <Select value={target} onChange={(e) => setTarget(e.target.value)}>
            {onlineNodes.map((n) => (
              <option key={n.id} value={n.node}>
                {n.node}
                {n.node === node ? " (where it is now)" : ""}
              </option>
            ))}
          </Select>
        ) : (
          <p className="text-sm leading-6 text-ink-dim">
            <span className="text-ink">{node}</span>
            {guest.isSuccess && leave.local.length > 0 && (
              <>
                {" "}
                — its disks are on {leave.local.join(", ")}, which only {node} has, so the clone stays here. Migrate it afterwards to move it.
              </>
            )}
          </p>
        )}
      </Field>
      {template && (
        <label className="flex items-center gap-2 text-sm text-ink">
          <input type="checkbox" checked={full} onChange={(e) => setFull(e.target.checked)} />
          Full clone
          <span className="text-xs text-ink-faint">{full ? "independent copy" : "linked: faster and smaller, keeps the template in use"}</span>
        </label>
      )}
      {full ? (
        <Field label="Storage">
          <Select value={storage} onChange={(e) => setStorage(e.target.value)}>
            {!needsStorage && <option value="">Same as the source</option>}
            {choices.map((s) => (
              <option key={s.storage} value={s.storage}>
                {s.storage}
                {s.shared ? " (shared)" : ""}
              </option>
            ))}
          </Select>
        </Field>
      ) : (
        <p className="text-xs text-ink-faint">A linked clone stays on the template's storage.</p>
      )}
      {blocked && <p className="text-sm text-warn">A linked clone can't go to another node from local storage. Use a full clone onto shared storage.</p>}
      {error && <p className="text-sm text-danger">{error}</p>}
    </Modal>
  );
}
