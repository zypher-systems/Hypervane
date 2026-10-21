import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest, formatKV, nextSlot } from "@/lib/guestHardware";
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

async function put(
  vmid: number, node: string, type: string, config: Record<string, unknown>,
  fields: Record<string, unknown>,
) {
  await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields }, node, type);
}

export function AddAudioDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const [device, setDevice] = useState("ich9-intel-hda");
  const [driver, setDriver] = useState("spice");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Add audio"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              await put(vmid, node, type, config, { audio0: formatKV("", { device, driver }) });
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add audio failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <Field label="Device">
        <Select value={device} onChange={(e) => setDevice(e.target.value)}>
          <option value="ich9-intel-hda">ich9-intel-hda</option>
          <option value="intel-hda">intel-hda</option>
          <option value="AC97">AC97</option>
        </Select>
      </Field>
      <Field label="Driver">
        <Select value={driver} onChange={(e) => setDriver(e.target.value)}>
          <option value="spice">spice</option>
          <option value="none">none</option>
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function AddWatchdogDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const [model, setModel] = useState("i6300esb");
  const [action, setAction] = useState("none");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Add watchdog"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              await put(vmid, node, type, config, { watchdog: formatKV("", { model, action }) });
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add watchdog failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <Field label="Model">
        <Select value={model} onChange={(e) => setModel(e.target.value)}>
          <option value="i6300esb">i6300esb</option>
          <option value="ib700">ib700</option>
        </Select>
      </Field>
      <Field label="Action">
        <Select value={action} onChange={(e) => setAction(e.target.value)}>
          <option value="none">none</option>
          <option value="reset">reset</option>
          <option value="shutdown">shutdown</option>
          <option value="poweroff">poweroff</option>
          <option value="pause">pause</option>
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function AddRngDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const [source, setSource] = useState("/dev/urandom");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Add RNG"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              await put(vmid, node, type, config, { rng0: formatKV("", { source }) });
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add RNG failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <Field label="Source"><Input value={source} onChange={(e) => setSource(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function AddVirtiofsDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const dirs = useQuery({ queryKey: ["maps", "dir"], queryFn: () => api.mappings("dir").catch(() => ({ supported: false, items: [] })) });
  const [mapping, setMapping] = useState("");
  const [error, setError] = useState("");
  const slot = nextSlot(config, "virtiofs", 15);
  return (
    <Modal
      title={`Add ${slot}`}
      onClose={onClose}
      footer={
        <Button
          disabled={!mapping}
          onClick={async () => {
            setError("");
            try {
              await put(vmid, node, type, config, { [slot]: mapping });
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add virtiofs failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <Field label="Directory mapping">
        <Select value={mapping} onChange={(e) => setMapping(e.target.value)}>
          <option value="">Select…</option>
          {(dirs.data?.items ?? []).map((item) => {
            const id = String(item.id ?? "");
            return <option key={id} value={id}>{id}</option>;
          })}
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
