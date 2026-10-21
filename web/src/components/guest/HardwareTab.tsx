import { AddCdromDialog, ChangeCdromDialog } from "./CdromPanel";
import { AddAudioDialog, AddRngDialog, AddVirtiofsDialog, AddWatchdogDialog } from "./DevicesPanel";
import {
  AddDiskDialog,
  AddMountDialog,
  EditDiskDialog,
  ImportDiskDialog,
  MoveDiskDialog,
  ResizeDiskDialog,
} from "./DiskPanel";
import { AddEfiDialog, AddTpmDialog } from "./FirmwarePanel";
import { MachineDialog, MemoryDialog, ProcessorsDialog } from "./HardwareForm";
import { LxcPanel } from "./LxcPanel";
import { NicDialog } from "./NicPanel";
import { OptionsForm } from "./OptionsForm";
import { PciDialog } from "./PciPanel";
import { PendingChanges } from "./PendingChanges";
import { RawConfigEditor } from "./RawConfigEditor";
import { UsbDialog } from "./UsbPanel";
import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { MoreMenu } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { api } from "@/lib/api";
import { storagesForContent } from "@/lib/storage";
import { bytes } from "@/lib/format";
import {
  cdromEntries,
  configDigest,
  diskEntries,
  emptyNic,
  nextSlot,
  nicEntries,
  parseCPU,
  parseFeatures,
  parseKV,
  parseNet,
  parseSizeToBytes,
  parseStartup,
  slotEntries,
  unusedEntries,
} from "@/lib/guestHardware";
import { useCan } from "@/lib/perms";
import type { PendingChange } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

type Dialog =
  | { kind: "processors" }
  | { kind: "memory" }
  | { kind: "machine" }
  | { kind: "container" }
  | { kind: "options" }
  | { kind: "raw" }
  | { kind: "disk-edit"; slot: string; value: string }
  | { kind: "disk-resize"; slot: string; value: string }
  | { kind: "disk-move"; slot: string }
  | { kind: "disk-add" }
  | { kind: "disk-import" }
  | { kind: "mount-add" }
  | { kind: "cd-add" }
  | { kind: "cd-change"; slot: string }
  | { kind: "nic"; slot?: string }
  | { kind: "pci" }
  | { kind: "usb" }
  | { kind: "efi" }
  | { kind: "tpm" }
  | { kind: "audio" }
  | { kind: "watchdog" }
  | { kind: "rng" }
  | { kind: "virtiofs" }
  | { kind: "unlink"; slot: string }
  | { kind: "remove"; slot: string; label: string }
  | null;

type Row = {
  id: string;
  kind: string;
  summary: string;
  edit?: () => void;
  actions?: { label: string; danger?: boolean; run: () => void }[];
};

function join(parts: (string | false | undefined)[]) {
  return parts.filter(Boolean).join(" · ");
}

function diskSummary(slot: string, value: string) {
  const { head, extras } = parseKV(value);
  if (/cloudinit/i.test(head) || extras.media === "cloudinit") return join([slot, "cloud-init"]);
  if (extras["import-from"]) return join([slot, head, "import"]);
  const size = extras.size ? parseSizeToBytes(extras.size) : extras.mp ? parseSizeToBytes(head.split(":").pop()) : parseSizeToBytes(head.split(":").pop());
  const sizeLabel = size != null ? bytes(size) : extras.size || extras.mp;
  return join([slot, extras.mp ? `mp ${extras.mp}` : head, sizeLabel]);
}

export function HardwareTab({
  vmid, node, type, config, pending, onSaved,
}: {
  vmid: number;
  node: string;
  type: string;
  config: Record<string, unknown>;
  pending: PendingChange[];
  onSaved: () => void;
}) {
  const qemu = type === "qemu";
  const can = useCan();
  const path = `/vms/${vmid}`;
  const canConfig = can(path, "VM.Config");
  const canCPU = canConfig || can(path, "VM.Config.CPU");
  const canMemory = canConfig || can(path, "VM.Config.Memory");
  const canDisk = canConfig || can(path, "VM.Config.Disk");
  const canNet = canConfig || can(path, "VM.Config.Network");
  const canHW = canConfig || can(path, "VM.Config.HWType");
  const canOptions = canConfig || can(path, "VM.Config.Options");
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const storeNames = storagesForContent(storages.data ?? [], type === "lxc" ? "rootdir" : "images", node).map((s) => s.storage);
  const importStores = storagesForContent(storages.data ?? [], "import", node).map((s) => s.storage);
  const [dialog, setDialog] = useState<Dialog>(null);
  const [rescanMsg, setRescanMsg] = useState("");

  function close() {
    setDialog(null);
  }
  function saved() {
    setDialog(null);
    onSaved();
  }

  const rows = useMemo<Row[]>(() => {
    const out: Row[] = [];
    const cpu = parseCPU(config.cpu);
    const sockets = Number(config.sockets ?? 1);
    out.push({
      id: "processors",
      kind: "Processors",
      summary: join([
        `${config.cores ?? 1} cores`,
        qemu ? (cpu.model || "default") : "",
        qemu && sockets > 1 ? `${sockets} sockets` : "",
      ]),
      edit: canCPU ? () => setDialog({ kind: "processors" }) : undefined,
    });
    const memMiB = Number(config.memory ?? 0);
    const balloon = config.balloon !== undefined ? Number(config.balloon) : null;
    out.push({
      id: "memory",
      kind: "Memory",
      summary: join([
        memMiB ? bytes(memMiB * 1024 * 1024) : "—",
        qemu && balloon != null && Number.isFinite(balloon) ? `balloon ${bytes(balloon * 1024 * 1024)}` : "",
      ]),
      edit: canMemory ? () => setDialog({ kind: "memory" }) : undefined,
    });
    if (qemu) {
      out.push({
        id: "machine",
        kind: "Machine",
        summary: join([
          String(config.bios ?? "seabios") === "ovmf" ? "OVMF" : "SeaBIOS",
          String(config.machine || "default"),
          String(config.scsihw || "virtio-scsi-pci"),
          `${String(config.vga || "std")} display`,
        ]),
        edit: canHW || canOptions ? () => setDialog({ kind: "machine" }) : undefined,
      });
    }
    if (!qemu) {
      const feats = parseFeatures(config.features);
      out.push({
        id: "container",
        kind: "Container",
        summary: join([
          String(config.unprivileged ?? "0") === "1" ? "unprivileged" : "privileged",
          feats.length ? feats.join(",") : "",
          config.swap !== undefined ? `swap ${config.swap} MiB` : "",
          String(config.cmode || "tty"),
        ]),
        edit: canOptions ? () => setDialog({ kind: "container" }) : undefined,
      });
    }
    for (const [slot, val] of diskEntries(config)) {
      const mount = slot.startsWith("mp") || parseKV(val).extras.mp;
      const cloudinit = /cloudinit/i.test(val);
      out.push({
        id: slot,
        kind: cloudinit ? "Cloud-init" : mount ? "Mount" : "Hard Disk",
        summary: diskSummary(slot, val),
        edit: canDisk && !cloudinit ? () => setDialog({ kind: "disk-edit", slot, value: val }) : undefined,
        actions: canDisk ? [
          ...(!cloudinit && !mount ? [
            { label: "Resize", run: () => setDialog({ kind: "disk-resize", slot, value: val }) },
            { label: "Move", run: () => setDialog({ kind: "disk-move", slot }) },
          ] : []),
          { label: "Unlink", danger: true, run: () => setDialog({ kind: "unlink", slot }) },
        ] : undefined,
      });
    }
    for (const [slot, val] of unusedEntries(config)) {
      out.push({
        id: slot,
        kind: "Unused",
        summary: join([slot, String(val)]),
        actions: canDisk ? [{ label: "Unlink", danger: true, run: () => setDialog({ kind: "unlink", slot }) }] : undefined,
      });
    }
    if (qemu) {
      for (const [slot, val] of cdromEntries(config)) {
        const vol = val.split(",")[0] === "none" ? "" : val.split(",")[0];
        out.push({
          id: slot,
          kind: "CD/DVD",
          summary: join([slot, vol || "empty"]),
          edit: canDisk ? () => setDialog({ kind: "cd-change", slot }) : undefined,
          actions: canDisk ? [
            { label: "Eject", run: async () => {
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields: { [slot]: "none,media=cdrom" } }, node, type);
              onSaved();
            } },
            { label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot, label: "CD/DVD" }) },
          ] : undefined,
        });
      }
    }
    for (const [slot, val] of nicEntries(config)) {
      const nic = parseNet(slot, val, type);
      out.push({
        id: slot,
        kind: "Network",
        summary: join([
          slot,
          type === "lxc" ? nic.name : nic.model,
          nic.bridge,
          nic.tag ? `tag ${nic.tag}` : "",
        ]),
        edit: canNet ? () => setDialog({ kind: "nic", slot }) : undefined,
        actions: canNet ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot, label: "network device" }) }] : undefined,
      });
    }
    if (qemu) {
      for (const [slot, val] of slotEntries(config, "hostpci")) {
        out.push({
          id: slot,
          kind: "PCI",
          summary: join([slot, val]),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot, label: "PCI device" }) }] : undefined,
        });
      }
      for (const [slot, val] of slotEntries(config, "usb")) {
        out.push({
          id: slot,
          kind: "USB",
          summary: join([slot, val]),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot, label: "USB device" }) }] : undefined,
        });
      }
      if (config.efidisk0 !== undefined) {
        out.push({
          id: "efidisk0",
          kind: "EFI Disk",
          summary: String(config.efidisk0),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot: "efidisk0", label: "EFI disk" }) }] : undefined,
        });
      }
      if (config.tpmstate0 !== undefined) {
        out.push({
          id: "tpmstate0",
          kind: "TPM",
          summary: String(config.tpmstate0),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot: "tpmstate0", label: "TPM" }) }] : undefined,
        });
      }
      if (config.audio0 !== undefined) {
        out.push({
          id: "audio0",
          kind: "Audio",
          summary: String(config.audio0),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot: "audio0", label: "audio device" }) }] : undefined,
        });
      }
      if (config.watchdog !== undefined) {
        out.push({
          id: "watchdog",
          kind: "Watchdog",
          summary: String(config.watchdog),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot: "watchdog", label: "watchdog" }) }] : undefined,
        });
      }
      if (config.rng0 !== undefined) {
        out.push({
          id: "rng0",
          kind: "RNG",
          summary: String(config.rng0),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot: "rng0", label: "RNG" }) }] : undefined,
        });
      }
      for (const [slot, val] of slotEntries(config, "serial")) {
        out.push({
          id: slot,
          kind: "Serial",
          summary: join([slot, val]),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot, label: "serial" }) }] : undefined,
        });
      }
      for (const [slot, val] of slotEntries(config, "virtiofs")) {
        out.push({
          id: slot,
          kind: "Virtiofs",
          summary: join([slot, val]),
          actions: canHW ? [{ label: "Remove", danger: true, run: () => setDialog({ kind: "remove", slot, label: "virtiofs" }) }] : undefined,
        });
      }
    }
    const start = parseStartup(config.startup);
    out.push({
      id: "options",
      kind: "Options",
      summary: join([
        String(config.onboot ?? "0") === "1" ? "onboot" : "",
        String(config.protection ?? "0") === "1" ? "protected" : "",
        start.order ? `order ${start.order}` : "",
        config.tags ? String(config.tags) : "",
      ]) || "defaults",
      edit: canOptions ? () => setDialog({ kind: "options" }) : undefined,
    });
    out.push({
      id: "raw",
      kind: "Raw config",
      summary: "qemu/lxc config file",
      edit: canConfig ? () => setDialog({ kind: "raw" }) : undefined,
    });
    return out;
  }, [canCPU, canConfig, canDisk, canHW, canMemory, canNet, canOptions, config, onSaved, qemu, type, vmid, node]);

  const addItems = [
    qemu && canDisk && { id: "disk", label: "Hard disk", run: () => setDialog({ kind: "disk-add" }) },
    qemu && canDisk && { id: "import", label: "Import disk", run: () => setDialog({ kind: "disk-import" }) },
    !qemu && canDisk && { id: "mount", label: "Mount", run: () => setDialog({ kind: "mount-add" }) },
    qemu && canDisk && { id: "cd", label: "CD/DVD", run: () => setDialog({ kind: "cd-add" }) },
    canNet && { id: "nic", label: "Network", run: () => setDialog({ kind: "nic" }) },
    qemu && canHW && { id: "pci", label: "PCI", run: () => setDialog({ kind: "pci" }) },
    qemu && canHW && { id: "usb", label: "USB", run: () => setDialog({ kind: "usb" }) },
    qemu && canHW && config.efidisk0 === undefined && String(config.bios ?? "seabios") === "ovmf" && { id: "efi", label: "EFI disk", run: () => setDialog({ kind: "efi" }) },
    qemu && canHW && config.tpmstate0 === undefined && { id: "tpm", label: "TPM", run: () => setDialog({ kind: "tpm" }) },
    qemu && canHW && config.audio0 === undefined && { id: "audio", label: "Audio", run: () => setDialog({ kind: "audio" }) },
    qemu && canHW && config.watchdog === undefined && { id: "watchdog", label: "Watchdog", run: () => setDialog({ kind: "watchdog" }) },
    qemu && canHW && config.rng0 === undefined && { id: "rng", label: "RNG", run: () => setDialog({ kind: "rng" }) },
    qemu && canHW && { id: "serial", label: "Serial", run: async () => {
      await api.updateGuest(vmid, {
        node, type, digest: configDigest(config),
        fields: { [nextSlot(config, "serial", 3)]: "socket" },
      }, node, type);
      onSaved();
    } },
    qemu && canHW && { id: "virtiofs", label: "Virtiofs", run: () => setDialog({ kind: "virtiofs" }) },
  ].filter((item): item is { id: string; label: string; run: () => void } => !!item);

  const nicForm = dialog?.kind === "nic" && dialog.slot
    ? parseNet(dialog.slot, String(config[dialog.slot] ?? ""), type)
    : emptyNic(nextSlot(config, "net", 31), type);

  return (
    <div className="space-y-4">
      <PendingChanges
        pending={pending}
        vmid={vmid}
        node={node}
        type={type}
        config={config}
        onReverted={onSaved}
      />
      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/15 px-4 py-3">
          <div className="text-xs font-medium text-ink-dim">Hardware</div>
          <div className="flex gap-2">
            {qemu && canDisk && (
              <Button
                size="sm"
                variant="outline"
                onClick={async () => {
                  setRescanMsg("");
                  const res = await api.rescanDisks(vmid, node, type);
                  if (!res.supported) {
                    setRescanMsg("Rescan is not available on this node");
                    return;
                  }
                  if (res.upid) attachTask(res.upid, [["guest", vmid]]);
                  else onSaved();
                }}
              >
                Rescan
              </Button>
            )}
            {addItems.length > 0 && (
              <MoreMenu label="Add" items={addItems.map((item) => ({ id: item.id, label: item.label, onClick: item.run }))} />
            )}
          </div>
        </div>
        {rescanMsg && <p className="px-4 pt-3 text-xs text-ink-dim">{rescanMsg}</p>}
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={row.id}
                className={`border-t border-line ${row.edit ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
                onClick={row.edit}
              >
                <td className="px-4 py-2 font-medium">{row.kind}</td>
                <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={row.summary}>{row.summary}</td>
                <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                  {row.edit && <Button size="sm" variant="outline" onClick={row.edit}>Edit</Button>}
                  {row.actions?.map((action) => (
                    <Button key={action.label} size="sm" variant={action.danger ? "danger" : "outline"} onClick={action.run}>
                      {action.label}
                    </Button>
                  ))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>

      {dialog?.kind === "processors" && <ProcessorsDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "memory" && <MemoryDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "machine" && <MachineDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "container" && <LxcPanel vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "options" && <OptionsForm vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "raw" && <RawConfigEditor vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "disk-edit" && (
        <EditDiskDialog slot={dialog.slot} value={dialog.value} vmid={vmid} node={node} type={type} onClose={close} onSaved={saved} />
      )}
      {dialog?.kind === "disk-resize" && (
        <ResizeDiskDialog
          slot={dialog.slot}
          current={parseKV(dialog.value).extras.size}
          vmid={vmid}
          node={node}
          type={type}
          onClose={close}
          onResized={() => close()}
        />
      )}
      {dialog?.kind === "disk-move" && (
        <MoveDiskDialog
          slot={dialog.slot}
          vmid={vmid}
          node={node}
          type={type}
          storages={storeNames}
          digest={configDigest(config)}
          onClose={close}
          onDone={close}
        />
      )}
      {dialog?.kind === "disk-add" && (
        <AddDiskDialog
          vmid={vmid}
          node={node}
          type={type}
          config={config}
          storages={storeNames}
          unused={unusedEntries(config)}
          onClose={close}
          onSaved={saved}
        />
      )}
      {dialog?.kind === "disk-import" && (
        <ImportDiskDialog vmid={vmid} node={node} type={type} config={config} storages={importStores.length ? importStores : storeNames} onClose={close} onSaved={saved} />
      )}
      {dialog?.kind === "mount-add" && (
        <AddMountDialog vmid={vmid} node={node} type={type} config={config} storages={storeNames} onClose={close} onSaved={saved} />
      )}
      {dialog?.kind === "cd-add" && <AddCdromDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "cd-change" && (
        <ChangeCdromDialog slot={dialog.slot} vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />
      )}
      {dialog?.kind === "nic" && (
        <NicDialog
          vmid={vmid}
          node={node}
          type={type}
          config={config}
          nic={nicForm}
          isNew={!dialog.slot}
          onClose={close}
          onSaved={saved}
        />
      )}
      {dialog?.kind === "pci" && <PciDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "usb" && <UsbDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "efi" && <AddEfiDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "tpm" && <AddTpmDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "audio" && <AddAudioDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "watchdog" && <AddWatchdogDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "rng" && <AddRngDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "virtiofs" && <AddVirtiofsDialog vmid={vmid} node={node} type={type} config={config} onClose={close} onSaved={saved} />}
      {dialog?.kind === "unlink" && (
        <ConfirmDialog
          title={`Unlink ${dialog.slot}?`}
          body="Removes the disk from this guest. The volume becomes unused until you attach or delete it."
          danger
          confirmLabel="Unlink"
          onClose={close}
          onConfirm={async () => {
            await api.unlinkDisk(vmid, dialog.slot, node, type);
            saved();
          }}
        />
      )}
      {dialog?.kind === "remove" && (
        <ConfirmDialog
          title={`Remove ${dialog.slot}?`}
          body={`Detaches this ${dialog.label} from the guest.`}
          danger
          confirmLabel="Remove"
          onClose={close}
          onConfirm={async () => {
            await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields: {}, delete: [dialog.slot] }, node, type);
            saved();
          }}
        />
      )}
    </div>
  );
}
