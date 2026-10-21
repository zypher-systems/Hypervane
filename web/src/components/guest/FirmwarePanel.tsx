import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { storagesForContent } from "@/lib/storage";
import { configDigest, formatKV } from "@/lib/guestHardware";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

type Shared = {
  vmid: number;
  node: string;
  type: string;
  config: Record<string, unknown>;
  onClose: () => void;
  onSaved: () => void;
};

export function AddEfiDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const stores = storagesForContent(storages.data ?? [], "images", node).map((s) => s.storage);
  const [storage, setStorage] = useState(stores[0] ?? "");
  const [secure, setSecure] = useState(true);
  const [error, setError] = useState("");
  const store = storage || stores[0] || "";

  return (
    <Modal
      title="Add EFI disk"
      onClose={onClose}
      footer={
        <Button
          disabled={!store}
          onClick={async () => {
            setError("");
            try {
              await api.updateGuest(vmid, {
                node, type, digest: configDigest(config),
                fields: { efidisk0: formatKV(`${store}:1`, { efitype: "4m", "pre-enrolled-keys": secure ? 1 : 0 }) },
              }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add EFI disk failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      {String(config.bios ?? "seabios") !== "ovmf" && (
        <p className="text-sm text-ink-dim">Set BIOS to UEFI (OVMF) on Machine before EFI will apply cleanly.</p>
      )}
      <Field label="Storage">
        <Select value={store} onChange={(e) => setStorage(e.target.value)}>
          {stores.map((s) => <option key={s}>{s}</option>)}
        </Select>
      </Field>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={secure} onChange={(e) => setSecure(e.target.checked)} />
        Pre-enroll keys (secure boot)
      </label>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function AddTpmDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const stores = storagesForContent(storages.data ?? [], "images", node).map((s) => s.storage);
  const [storage, setStorage] = useState(stores[0] ?? "");
  const [error, setError] = useState("");
  const store = storage || stores[0] || "";

  return (
    <Modal
      title="Add TPM v2.0"
      onClose={onClose}
      footer={
        <Button
          disabled={!store}
          onClick={async () => {
            setError("");
            try {
              await api.updateGuest(vmid, {
                node, type, digest: configDigest(config),
                fields: { tpmstate0: formatKV(`${store}:1`, { version: "v2.0" }) },
              }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add TPM failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <Field label="Storage">
        <Select value={store} onChange={(e) => setStorage(e.target.value)}>
          {stores.map((s) => <option key={s}>{s}</option>)}
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
