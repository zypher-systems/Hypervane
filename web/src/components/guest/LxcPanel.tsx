import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest, formatFeatures, lxcFeatureFlags, parseFeatures } from "@/lib/guestHardware";
import { useCan } from "@/lib/perms";
import { useState } from "react";

export function LxcPanel({
  vmid, node, type, config, onSaved, onClose,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>; onSaved: () => void; onClose: () => void;
}) {
  const can = useCan();
  const canOpts = can(`/vms/${vmid}`, "VM.Config") || can(`/vms/${vmid}`, "VM.Config.Options");
  const [features, setFeatures] = useState<string[]>(parseFeatures(config.features));
  const [swap, setSwap] = useState(config.swap === undefined ? "" : String(config.swap));
  const [cmode, setCmode] = useState(String(config.cmode ?? "tty"));
  const [tty, setTty] = useState(config.tty === undefined ? "2" : String(config.tty));
  const [timezone, setTimezone] = useState(String(config.timezone ?? ""));
  const [error, setError] = useState("");
  const unprivileged = String(config.unprivileged ?? "0") === "1";

  if (!canOpts) return null;

  return (
    <Modal
      title="Container"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              const fields: Record<string, unknown> = { cmode, tty };
              const del: string[] = [];
              const feat = formatFeatures(features);
              if (feat) fields.features = feat;
              else if (config.features !== undefined) del.push("features");
              if (swap !== "") fields.swap = swap;
              else if (config.swap !== undefined) del.push("swap");
              if (timezone) fields.timezone = timezone;
              else if (config.timezone !== undefined) del.push("timezone");
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields, delete: del }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save container options failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <p className="text-xs text-ink-dim">Unprivileged: {unprivileged ? "yes" : "no"} (set at create; PVE will not flip it here).</p>
      <div>
        <div className="mb-2 text-xs uppercase text-ink-dim">Features</div>
        <div className="flex flex-wrap gap-3 text-sm">
          {lxcFeatureFlags.map((flag) => (
            <label key={flag} className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={features.includes(flag)}
                onChange={(e) => setFeatures((cur) => e.target.checked ? [...cur, flag] : cur.filter((x) => x !== flag))}
              />
              {flag}
            </label>
          ))}
        </div>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Swap (MiB)"><Input type="number" value={swap} onChange={(e) => setSwap(e.target.value)} /></Field>
        <Field label="Console mode">
          <Select value={cmode} onChange={(e) => setCmode(e.target.value)}>
            <option value="tty">tty</option>
            <option value="console">console</option>
            <option value="shell">shell</option>
          </Select>
        </Field>
        <Field label="TTY count"><Input type="number" value={tty} onChange={(e) => setTty(e.target.value)} /></Field>
        <Field label="Timezone"><Input value={timezone} onChange={(e) => setTimezone(e.target.value)} placeholder="host or Europe/Berlin" /></Field>
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
