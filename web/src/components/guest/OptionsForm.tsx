import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { TagEditor } from "@/components/guest/TagEditor";
import { api } from "@/lib/api";
import { configDigest, formatStartup, parseStartup } from "@/lib/guestHardware";
import { useCan } from "@/lib/perms";
import { useState } from "react";

const keyboards = ["en-us", "en-gb", "de", "fr", "es", "it", "ja", "pt", "sv"];

export function OptionsForm({
  vmid, node, type, config, onSaved, onClose,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>; onSaved: () => void; onClose: () => void;
}) {
  const can = useCan();
  const canOptions = can(`/vms/${vmid}`, "VM.Config") || can(`/vms/${vmid}`, "VM.Config.Options");
  const [tags, setTags] = useState(String(config.tags ?? ""));
  const [onboot, setOnboot] = useState(String(config.onboot ?? "0") === "1");
  const [startup, setStartup] = useState(parseStartup(config.startup));
  const [protection, setProtection] = useState(String(config.protection ?? "0") === "1");
  const [hookscript, setHookscript] = useState(String(config.hookscript ?? ""));
  const [keyboard, setKeyboard] = useState(String(config.keyboard ?? ""));
  const [smbios, setSmbios] = useState(String(config.smbios1 ?? ""));
  const [freeze, setFreeze] = useState(String(config.freeze ?? "0") === "1");
  const [agent, setAgent] = useState(agentEnabled(config.agent));
  const [kvm, setKvm] = useState(config.kvm === undefined ? true : String(config.kvm) !== "0");
  const [acpi, setAcpi] = useState(config.acpi === undefined ? true : String(config.acpi) !== "0");
  const [localtime, setLocaltime] = useState(String(config.localtime ?? "0") === "1");
  const [error, setError] = useState("");

  if (!canOptions) return null;

  return (
    <Modal
      title="Options"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              const fields: Record<string, unknown> = {
                onboot: onboot ? 1 : 0,
                protection: protection ? 1 : 0,
              };
              const del: string[] = [];
              const start = formatStartup(startup);
              if (start) fields.startup = start;
              else if (config.startup !== undefined) del.push("startup");
              if (hookscript) fields.hookscript = hookscript;
              else if (config.hookscript !== undefined) del.push("hookscript");
              if (tags) fields.tags = tags;
              else if (config.tags !== undefined) del.push("tags");
              if (type === "qemu") {
                if (keyboard) fields.keyboard = keyboard;
                else if (config.keyboard !== undefined) del.push("keyboard");
                if (smbios) fields.smbios1 = smbios;
                else if (config.smbios1 !== undefined) del.push("smbios1");
                fields.kvm = kvm ? 1 : 0;
                fields.acpi = acpi ? 1 : 0;
                fields.localtime = localtime ? 1 : 0;
                fields.freeze = freeze ? 1 : 0;
                fields.agent = agent ? 1 : 0;
              }
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields, delete: del }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save options failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <div className="grid gap-4 md:grid-cols-2">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={onboot} onChange={(e) => setOnboot(e.target.checked)} />
          Start at boot
        </label>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={protection} onChange={(e) => setProtection(e.target.checked)} />
          Protection (prevent delete)
        </label>
        <Field label="Startup order"><Input value={startup.order} onChange={(e) => setStartup({ ...startup, order: e.target.value })} /></Field>
        <Field label="Startup delay (s)"><Input value={startup.up} onChange={(e) => setStartup({ ...startup, up: e.target.value })} /></Field>
        <Field label="Shutdown timeout (s)"><Input value={startup.down} onChange={(e) => setStartup({ ...startup, down: e.target.value })} /></Field>
        <Field label="Hookscript"><Input value={hookscript} onChange={(e) => setHookscript(e.target.value)} placeholder="local:snippets/hook.pl" /></Field>
        <Field label="Tags"><TagEditor value={tags} onChange={setTags} /></Field>
        {type === "qemu" && (
          <>
            <Field label="Keyboard">
              <Select value={keyboard} onChange={(e) => setKeyboard(e.target.value)}>
                <option value="">Default</option>
                {keyboards.map((k) => <option key={k}>{k}</option>)}
              </Select>
            </Field>
            <Field label="SMBIOS"><Input value={smbios} onChange={(e) => setSmbios(e.target.value)} /></Field>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={kvm} onChange={(e) => setKvm(e.target.checked)} />
              KVM hardware virtualization
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={acpi} onChange={(e) => setAcpi(e.target.checked)} />
              ACPI
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={localtime} onChange={(e) => setLocaltime(e.target.checked)} />
              Local time
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={freeze} onChange={(e) => setFreeze(e.target.checked)} />
              Freeze CPU at start
            </label>
            <label className="flex items-center gap-2 text-sm">
              <input type="checkbox" checked={agent} onChange={(e) => setAgent(e.target.checked)} />
              QEMU guest agent
            </label>
          </>
        )}
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function agentEnabled(raw: unknown): boolean {
  const v = String(raw ?? "");
  if (!v || v === "0") return false;
  if (v === "1") return true;
  return /(?:^|,)enabled=1(?:,|$)/.test(v) || !/(?:^|,)enabled=0(?:,|$)/.test(v);
}
