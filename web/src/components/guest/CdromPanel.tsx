import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { busMax, configDigest, nextSlot } from "@/lib/guestHardware";
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

export function AddCdromDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const media = useQuery({ queryKey: ["media", "iso", node], queryFn: () => api.media("iso", node), enabled: !!node });
  const [bus, setBus] = useState("ide");
  const [iso, setIso] = useState("");
  const [error, setError] = useState("");
  const next = nextSlot(config, bus, busMax(bus));

  return (
    <Modal
      title={`Add CD/DVD ${next}`}
      onClose={onClose}
      footer={
        <Button
          disabled={!iso}
          onClick={async () => {
            setError("");
            try {
              await api.updateGuest(vmid, {
                node, type, digest: configDigest(config), fields: { [next]: `${iso},media=cdrom` },
              }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add CD/DVD failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <Field label="Bus">
        <Select value={bus} onChange={(e) => setBus(e.target.value)}>
          <option value="ide">ide</option>
          <option value="sata">sata</option>
        </Select>
      </Field>
      <Field label="ISO">
        <Select value={iso} onChange={(e) => setIso(e.target.value)}>
          <option value="">Select…</option>
          {(media.data ?? []).map((m) => (
            <option key={m.volid} value={m.volid}>{m.volid}</option>
          ))}
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function ChangeCdromDialog({
  slot, vmid, node, type, config, onClose, onSaved,
}: Shared & { slot: string }) {
  const media = useQuery({ queryKey: ["media", "iso", node], queryFn: () => api.media("iso", node), enabled: !!node });
  const [iso, setIso] = useState("");
  const [error, setError] = useState("");

  return (
    <Modal
      title={`Change ${slot}`}
      onClose={onClose}
      footer={
        <Button
          disabled={!iso}
          onClick={async () => {
            setError("");
            try {
              await api.updateGuest(vmid, {
                node, type, digest: configDigest(config), fields: { [slot]: `${iso},media=cdrom` },
              }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Change ISO failed");
            }
          }}
        >
          Change
        </Button>
      }
    >
      <Field label="ISO">
        <Select value={iso} onChange={(e) => setIso(e.target.value)}>
          <option value="">Select…</option>
          {(media.data ?? []).map((m) => (
            <option key={m.volid} value={m.volid}>{m.volid}</option>
          ))}
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
