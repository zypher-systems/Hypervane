import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest, formatCPU, formatHotplug, hotplugFlags, parseBoot, parseCPU, parseHotplug } from "@/lib/guestHardware";
import { useCan } from "@/lib/perms";
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

export function ProcessorsDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const caps = useQuery({ queryKey: ["caps", node], queryFn: () => api.capabilities(node), enabled: !!node && type === "qemu" });
  const [cores, setCores] = useState(String(config.cores ?? 1));
  const [cpu, setCpu] = useState(() => parseCPU(config.cpu).model);
  const [cpuFlags, setCpuFlags] = useState(() => parseCPU(config.cpu).flags);
  const [hostCPU, setHostCPU] = useState(() => parseCPU(config.cpu).model === "host");
  const [vcpus, setVcpus] = useState(config.vcpus === undefined ? "" : String(config.vcpus));
  const [sockets, setSockets] = useState(String(config.sockets ?? 1));
  const [numa, setNuma] = useState(String(config.numa ?? "0") === "1");
  const [affinity, setAffinity] = useState(String(config.affinity ?? ""));
  const [cpulimit, setCpulimit] = useState(config.cpulimit === undefined ? "" : String(config.cpulimit));
  const [cpuunits, setCpuunits] = useState(config.cpuunits === undefined ? "" : String(config.cpuunits));
  const [error, setError] = useState("");
  const cpus = (caps.data?.cpus ?? []).map((c) => String(c.name ?? c.id ?? "")).filter(Boolean);
  const can = useCan();
  const path = `/vms/${vmid}`;
  const canCPU = can(path, "VM.Config") || can(path, "VM.Config.CPU");
  const canOptions = can(path, "VM.Config") || can(path, "VM.Config.Options");

  return (
    <Modal
      title="Processors"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              const fields: Record<string, unknown> = { cores };
              const del: string[] = [];
              if (type === "qemu" && canCPU) {
                const model = hostCPU ? "host" : cpu;
                const cpuValue = formatCPU(model, cpuFlags);
                if (cpuValue) fields.cpu = cpuValue;
                else if (config.cpu !== undefined) del.push("cpu");
                if (sockets) fields.sockets = sockets;
                if (vcpus !== "") fields.vcpus = vcpus;
                else if (config.vcpus !== undefined) del.push("vcpus");
              }
              if (type === "qemu" && canOptions) {
                fields.numa = numa ? 1 : 0;
                if (affinity) fields.affinity = affinity;
                else if (config.affinity !== undefined) del.push("affinity");
                if (cpulimit !== "") fields.cpulimit = cpulimit;
                else if (config.cpulimit !== undefined) del.push("cpulimit");
                if (cpuunits !== "") fields.cpuunits = cpuunits;
                else if (config.cpuunits !== undefined) del.push("cpuunits");
              }
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields, delete: del }, node, type);
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
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Cores"><Input type="number" value={cores} onChange={(e) => setCores(e.target.value)} /></Field>
        {type === "qemu" && canCPU && (
          <>
            <Field label="CPU model">
              <Select
                value={hostCPU ? "host" : cpu}
                onChange={(e) => {
                  setHostCPU(e.target.value === "host");
                  setCpu(e.target.value);
                }}
              >
                <option value="">Default</option>
                <option value="host">host</option>
                {cpus.filter((c) => c !== "host").map((c) => <option key={c} value={c}>{c}</option>)}
              </Select>
            </Field>
            <Field label="Sockets">
              <Input type="number" value={sockets} onChange={(e) => setSockets(e.target.value)} />
            </Field>
            <Field label="vCPUs">
              <Input type="number" value={vcpus} onChange={(e) => setVcpus(e.target.value)} placeholder="same as cores" />
            </Field>
            <Field label="CPU flags">
              <Input value={cpuFlags} onChange={(e) => setCpuFlags(e.target.value)} placeholder="+aes;-spec-ctrl" />
            </Field>
            <label className="flex items-center gap-2 self-end text-sm">
              <input
                type="checkbox"
                checked={hostCPU}
                onChange={(e) => {
                  setHostCPU(e.target.checked);
                  if (e.target.checked) setCpu("host");
                }}
              />
              Host CPU
            </label>
          </>
        )}
        {type === "qemu" && canOptions && (
          <>
            <Field label="CPU affinity">
              <Input value={affinity} onChange={(e) => setAffinity(e.target.value)} placeholder="0-3,8" />
            </Field>
            <Field label="CPU limit">
              <Input value={cpulimit} onChange={(e) => setCpulimit(e.target.value)} placeholder="0" />
            </Field>
            <Field label="CPU units">
              <Input type="number" value={cpuunits} onChange={(e) => setCpuunits(e.target.value)} />
            </Field>
            <label className="flex items-center gap-2 self-end text-sm">
              <input type="checkbox" checked={numa} onChange={(e) => setNuma(e.target.checked)} />
              NUMA
            </label>
          </>
        )}
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function MemoryDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const [memory, setMemory] = useState(String(config.memory ?? 512));
  const [balloon, setBalloon] = useState(config.balloon === undefined ? "" : String(config.balloon));
  const [shares, setShares] = useState(config.shares === undefined ? "" : String(config.shares));
  const [error, setError] = useState("");
  const can = useCan();
  const path = `/vms/${vmid}`;
  const canMemory = can(path, "VM.Config") || can(path, "VM.Config.Memory");

  return (
    <Modal
      title="Memory"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              const fields: Record<string, unknown> = { memory };
              const del: string[] = [];
              if (type === "qemu" && canMemory) {
                if (balloon !== "") fields.balloon = balloon;
                else if (config.balloon !== undefined) del.push("balloon");
                if (shares !== "") fields.shares = shares;
                else if (config.shares !== undefined) del.push("shares");
              }
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields, delete: del }, node, type);
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
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Memory (MiB)"><Input type="number" value={memory} onChange={(e) => setMemory(e.target.value)} /></Field>
        {type === "qemu" && canMemory && (
          <>
            <Field label="Balloon (MiB)">
              <Input type="number" value={balloon} onChange={(e) => setBalloon(e.target.value)} placeholder="0 disables" />
            </Field>
            <Field label="Memory shares">
              <Input type="number" value={shares} onChange={(e) => setShares(e.target.value)} />
            </Field>
          </>
        )}
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function MachineDialog({ vmid, node, type, config, onClose, onSaved }: Shared) {
  const caps = useQuery({ queryKey: ["caps", node], queryFn: () => api.capabilities(node), enabled: !!node && type === "qemu" });
  const [bios, setBios] = useState(String(config.bios ?? "seabios"));
  const [machine, setMachine] = useState(String(config.machine ?? ""));
  const [scsihw, setScsihw] = useState(String(config.scsihw ?? "virtio-scsi-pci"));
  const [vga, setVga] = useState(String(config.vga ?? "std"));
  const [tablet, setTablet] = useState(config.tablet === undefined ? true : String(config.tablet) !== "0");
  const [args, setArgs] = useState(String(config.args ?? ""));
  const [showArgs, setShowArgs] = useState(!!config.args);
  const [boot, setBoot] = useState(parseBoot(config.boot).join(";"));
  const [hotplug, setHotplug] = useState<string[]>(parseHotplug(config.hotplug));
  const [error, setError] = useState("");
  const machines = (caps.data?.machines ?? []).map((m) => String(m.id ?? m.name ?? "")).filter(Boolean);
  const can = useCan();
  const path = `/vms/${vmid}`;
  const canDisplay = can(path, "VM.Config") || can(path, "VM.Config.HWType");
  const canOptions = can(path, "VM.Config") || can(path, "VM.Config.Options");

  return (
    <Modal
      title="Machine"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              const fields: Record<string, unknown> = { bios, scsihw };
              const del: string[] = [];
              if (machine) fields.machine = machine;
              else if (config.machine !== undefined) del.push("machine");
              if (boot) fields.boot = boot.includes("order=") ? boot : `order=${boot}`;
              if (canDisplay) {
                fields.vga = vga;
                fields.tablet = tablet ? 1 : 0;
                if (args) fields.args = args;
                else if (config.args !== undefined) del.push("args");
              }
              if (canOptions) fields.hotplug = formatHotplug(hotplug);
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields, delete: del }, node, type);
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
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="BIOS">
          <Select value={bios} onChange={(e) => setBios(e.target.value)}>
            <option value="seabios">SeaBIOS</option>
            <option value="ovmf">UEFI (OVMF)</option>
          </Select>
        </Field>
        <Field label="Machine">
          <Select value={machine} onChange={(e) => setMachine(e.target.value)}>
            <option value="">Default</option>
            {machines.map((m) => <option key={m}>{m}</option>)}
          </Select>
        </Field>
        <Field label="SCSI controller">
          <Select value={scsihw} onChange={(e) => setScsihw(e.target.value)}>
            <option value="virtio-scsi-pci">virtio-scsi-pci</option>
            <option value="virtio-scsi-single">virtio-scsi-single</option>
            <option value="lsi">lsi</option>
          </Select>
        </Field>
        <Field label="Boot order">
          <Input value={boot} onChange={(e) => setBoot(e.target.value)} placeholder="scsi0;ide2;net0" />
        </Field>
        {canDisplay && (
          <>
            <Field label="Display">
              <Select value={vga} onChange={(e) => setVga(e.target.value)}>
                <option value="std">std</option>
                <option value="qxl">qxl (SPICE)</option>
                <option value="virtio">virtio</option>
                <option value="virtio-gl">virtio-gl</option>
                <option value="serial0">serial0</option>
              </Select>
            </Field>
            <label className="flex items-center gap-2 self-end text-sm">
              <input type="checkbox" checked={tablet} onChange={(e) => setTablet(e.target.checked)} />
              USB tablet
            </label>
          </>
        )}
      </div>
      {canOptions && (
        <div className="space-y-2">
          <p className="text-xs uppercase text-ink-dim">Hotplug</p>
          <div className="flex flex-wrap gap-3 text-sm">
            {hotplugFlags.map((flag) => (
              <label key={flag} className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={hotplug.includes(flag)}
                  onChange={(e) => setHotplug((cur) => e.target.checked ? [...cur, flag] : cur.filter((x) => x !== flag))}
                />
                {flag}
              </label>
            ))}
          </div>
        </div>
      )}
      {canDisplay && (
        <div>
          <button type="button" className="text-xs text-ink-faint underline" onClick={() => setShowArgs((v) => !v)}>
            {showArgs ? "Hide extra args" : "Advanced: extra QEMU args"}
          </button>
          {showArgs && (
            <Field label="args">
              <Input value={args} onChange={(e) => setArgs(e.target.value)} placeholder="-cpu host,..." />
            </Field>
          )}
        </div>
      )}
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
