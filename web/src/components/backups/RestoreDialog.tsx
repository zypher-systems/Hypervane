import { attachTask } from "@/components/layout/AppShell";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Advanced, CollapsedRaw, KeyValueTable } from "@/components/ui/item-details";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { guestKeys } from "@/lib/tasks";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

export function RestoreDialog({
  node,
  archive,
  storage,
  defaultType,
  defaultVmid,
  onClose,
}: {
  node: string;
  archive: string;
  storage?: string;
  defaultType?: string;
  defaultVmid?: number;
  onClose: () => void;
}) {
  const next = useQuery({ queryKey: ["nextid"], queryFn: api.nextid });
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const inferred = defaultType || (archive.toLowerCase().includes("lxc") || archive.includes("/ct/") ? "lxc" : "qemu");
  const [type, setType] = useState(inferred);
  const [vmid, setVmid] = useState(String(defaultVmid || next.data?.vmid || ""));
  const [dest, setDest] = useState(storage ?? "");
  const [unique, setUnique] = useState(true);
  const [start, setStart] = useState(false);
  const [force, setForce] = useState(!!defaultVmid);
  const [error, setError] = useState("");
  const [confirmForce, setConfirmForce] = useState(false);
  const destStores = (storages.data ?? []).filter((s) => !s.disable);
  const extracted = useQuery({
    queryKey: ["extract-config", node, archive],
    queryFn: () => api.extractConfig(node, archive),
  });
  const extractedFields = parseExtractedConfig(extracted.data?.config);

  const run = async () => {
    setError("");
    try {
      const res = await api.restore({
        node,
        archive,
        type,
        vmid: Number(vmid) || 0,
        storage: dest,
        unique,
        force,
        start,
      });
      attachTask(res.upid, [...guestKeys(Number(vmid) || 0), ["nextid"], ["guest-backups"]]);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Restore failed");
    }
  };

  return (
    <>
      <Modal
        title="Restore guest"
        onClose={onClose}
        footer={
          <Button
            onClick={() => {
              if (force) setConfirmForce(true);
              else void run();
            }}
          >
            Restore
          </Button>
        }
      >
        <p className="break-all font-mono text-xs text-ink-dim">{archive}</p>
        {extracted.data?.config && (
          <div className="space-y-1">
            <p className="text-xs uppercase text-ink-dim">Extracted config ({extracted.data.type})</p>
            {Object.keys(extractedFields).length ? (
              <KeyValueTable data={extractedFields} keys={["name", "hostname", "ostype", "cores", "memory", "scsihw", "net0", "scsi0", "rootfs"]} />
            ) : (
              <Advanced label="Advanced: raw extracted config" defaultOpen>
                <CollapsedRaw data={extracted.data.config} />
              </Advanced>
            )}
          </div>
        )}
        <Field label="Type">
          <Select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="qemu">QEMU</option>
            <option value="lxc">LXC</option>
          </Select>
        </Field>
        <Field label="VMID">
          <Input type="number" value={vmid} onChange={(e) => setVmid(e.target.value)} />
        </Field>
        <Field label="Target storage">
          <Select value={dest} onChange={(e) => setDest(e.target.value)}>
            <option value="">Default</option>
            {destStores.map((s) => <option key={s.storage}>{s.storage}</option>)}
          </Select>
        </Field>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={unique} onChange={(e) => setUnique(e.target.checked)} />
          Unique MAC / SMBIOS
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={start} onChange={(e) => setStart(e.target.checked)} />
          Start after restore
        </label>
        <label className="flex items-center gap-2 text-sm text-rose-300">
          <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} />
          Overwrite existing VMID
        </label>
        {error && <p className="text-sm text-rose-400">{error}</p>}
      </Modal>
      {confirmForce && (
        <ConfirmDialog
          title={`Overwrite guest ${vmid}?`}
          body="Force restore replaces the existing guest with this backup."
          danger
          confirmLabel="Overwrite"
          onClose={() => setConfirmForce(false)}
          onConfirm={() => {
            setConfirmForce(false);
            void run();
          }}
        />
      )}
    </>
  );
}

function parseExtractedConfig(raw?: string): Record<string, unknown> {
  if (!raw) return {};
  const out: Record<string, unknown> = {};
  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const i = trimmed.indexOf(":");
    if (i < 1) continue;
    out[trimmed.slice(0, i).trim()] = trimmed.slice(i + 1).trim();
  }
  return out;
}
