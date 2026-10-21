import { ConsoleView } from "@/components/console/ConsoleView";
import { attachTask } from "@/components/layout/AppShell";
import { Button } from "@/components/ui/button";
import { CloneDialog } from "@/components/guest/CloneDialog";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { storagesForContent } from "@/lib/storage";
import { guestKeys, snapKeys } from "@/lib/tasks";
import { useCan } from "@/lib/perms";
import type { Resource } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";

type Dialog = "migrate" | "clone" | "snapshot" | "backup" | "stop" | "reset" | "console" | null;

export function GuestContextMenu({
  guest,
  nodes,
  pos,
  onClose,
}: {
  guest: Resource;
  nodes: Resource[];
  pos: { x: number; y: number };
  onClose: () => void;
}) {
  const can = useCan();
  const menu = useRef<HTMLDivElement>(null);
  const [dialog, setDialog] = useState<Dialog>(null);
  const [error, setError] = useState("");
  const vmid = guest.vmid ?? 0;
  const node = guest.node ?? "";
  const type = guest.type;
  const vmPath = `/vms/${vmid}`;
  const running = guest.status === "running";
  const stopped = guest.status === "stopped";
  const paused = guest.status === "paused" || guest.status === "suspended";
  const template = !!guest.template;
  const qemu = type === "qemu";
  const canPower = can(vmPath, "VM.PowerMgmt");
  const canMigrate = can(vmPath, "VM.Migrate");
  const canClone = can(vmPath, "VM.Clone");
  const canSnapshot = can(vmPath, "VM.Snapshot");
  const canBackup = can(vmPath, "VM.Backup");
  const canConsole = can(vmPath, "VM.Console");
  const targets = nodes.filter((n) => n.node && n.node !== node);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (dialog) return;
      if (menu.current && !menu.current.contains(e.target as Node)) onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !dialog) onClose();
    };
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [dialog, onClose]);

  async function power(action: string) {
    setError("");
    try {
      const res = await api.power(vmid, action, node, type);
      attachTask(res.upid, guestKeys(vmid));
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Action failed");
    }
  }

  const items: { id: string; label: string; enabled: boolean; danger?: boolean; run: () => void }[] = [
    { id: "start", label: "Start", enabled: canPower && !template && (stopped || paused), run: () => void power("start") },
    { id: "stop", label: "Stop", enabled: canPower && !template && (running || paused), danger: true, run: () => setDialog("stop") },
    { id: "clone", label: "Clone", enabled: canClone, run: () => setDialog("clone") },
    { id: "migrate", label: "Migrate", enabled: canMigrate && targets.length > 0, run: () => setDialog("migrate") },
    { id: "shutdown", label: "Shutdown", enabled: canPower && !template && running, run: () => void power("shutdown") },
    { id: "reboot", label: "Reboot", enabled: canPower && !template && running, run: () => void power("reboot") },
    { id: "reset", label: "Reset", enabled: canPower && !template && qemu && running, danger: true, run: () => setDialog("reset") },
    { id: "suspend", label: "Suspend", enabled: canPower && !template && running, run: () => void power("suspend") },
    { id: "resume", label: "Resume", enabled: canPower && !template && paused, run: () => void power("resume") },
    { id: "snapshot", label: "Snapshot", enabled: canSnapshot, run: () => setDialog("snapshot") },
    { id: "backup", label: "Backup", enabled: canBackup, run: () => setDialog("backup") },
    { id: "console", label: "Console", enabled: canConsole && !template && running, run: () => setDialog("console") },
  ];

  const left = Math.min(pos.x, window.innerWidth - 188);
  const top = Math.min(pos.y, window.innerHeight - 420);

  useEffect(() => {
    if (dialog) return;
    const first = menu.current?.querySelector<HTMLElement>("[role=menuitem]:not([disabled])");
    first?.focus();
  }, [dialog]);

  function onMenuKey(e: ReactKeyboardEvent<HTMLDivElement>) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp" && e.key !== "Home" && e.key !== "End") return;
    e.preventDefault();
    const items = Array.from(menu.current?.querySelectorAll<HTMLElement>("[role=menuitem]:not([disabled])") ?? []);
    if (!items.length) return;
    const idx = items.indexOf(document.activeElement as HTMLElement);
    let next = idx;
    if (e.key === "ArrowDown") next = idx < 0 ? 0 : (idx + 1) % items.length;
    else if (e.key === "ArrowUp") next = idx <= 0 ? items.length - 1 : idx - 1;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = items.length - 1;
    items[next]?.focus();
  }

  return (
    <>
      {!dialog && (
        <div
          ref={menu}
          role="menu"
          onKeyDown={onMenuKey}
          className="hv-glass fixed z-50 min-w-[11rem] rounded-md border py-1 text-xs shadow-lg"
          style={{ left, top }}
        >
          <p className="truncate px-3 py-1.5 text-xs font-medium text-ink-dim">
            {guest.name || vmid}
          </p>
          {items.map((item) => (
            <button
              key={item.id}
              type="button"
              role="menuitem"
              disabled={!item.enabled}
              onClick={item.run}
              className={`flex w-full px-3 py-1.5 text-left outline-none focus:bg-white/[0.08] disabled:opacity-35 ${
                item.danger ? "text-rose-300 hover:bg-rose-500/10 focus:bg-rose-500/10" : "text-ink hover:bg-white/[0.04]"
              }`}
            >
              {item.label}
            </button>
          ))}
          {error && <p className="px-3 py-1.5 text-rose-400">{error}</p>}
        </div>
      )}
      {dialog === "stop" && (
        <ConfirmDialog
          title={`Stop ${guest.name || vmid}`}
          body="Hard stop. Unsaved work in the guest may be lost."
          confirmLabel="Stop"
          danger
          onClose={() => setDialog(null)}
          onConfirm={() => void power("stop")}
        />
      )}
      {dialog === "reset" && (
        <ConfirmDialog
          title={`Reset ${guest.name || vmid}`}
          body="Hard reset. The guest will restart immediately."
          confirmLabel="Reset"
          danger
          onClose={() => setDialog(null)}
          onConfirm={() => void power("reset")}
        />
      )}
      {dialog === "clone" && (
        <CloneDialog
          vmid={guest.vmid ?? 0}
          node={guest.node ?? ""}
          type={guest.type}
          name={guest.name ?? ""}
          template={guest.template === 1}
          nodes={nodes}
          onClose={() => { setDialog(null); onClose(); }}
        />
      )}
      {dialog === "migrate" && (
        <TreeMigrateDialog guest={guest} targets={targets} onClose={() => { setDialog(null); onClose(); }} />
      )}
      {dialog === "snapshot" && (
        <TreeSnapshotDialog guest={guest} onClose={() => { setDialog(null); onClose(); }} />
      )}
      {dialog === "backup" && (
        <TreeBackupDialog guest={guest} onClose={() => { setDialog(null); onClose(); }} />
      )}
      {dialog === "console" && (
        <Modal title={`Console ${guest.name || vmid}`} onClose={() => { setDialog(null); onClose(); }} size="xl">
          <ConsoleView node={node} vmid={vmid} type={type} kind={qemu ? "vnc" : "term"} />
        </Modal>
      )}
    </>
  );
}

function TreeMigrateDialog({
  guest,
  targets,
  onClose,
}: {
  guest: Resource;
  targets: Resource[];
  onClose: () => void;
}) {
  const [target, setTarget] = useState(targets[0]?.node ?? "");
  const [online, setOnline] = useState(guest.status === "running");
  const [withLocalDisks, setWithLocalDisks] = useState(true);
  const [targetStorage, setTargetStorage] = useState("");
  const [migrationNetwork, setMigrationNetwork] = useState("");
  const [busy, setBusy] = useState(false);
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });

  return (
    <Modal
      title={`Migrate ${guest.name || guest.vmid}`}
      onClose={onClose}
      footer={
        <Button
          disabled={!target || busy}
          onClick={async () => {
            setBusy(true);
            try {
              const res = await api.migrate(guest.vmid ?? 0, {
                target,
                online: !!guest.template ? false : online,
                withLocalDisks,
                ...(targetStorage ? { targetStorage } : {}),
                ...(migrationNetwork ? { migrationNetwork } : {}),
              }, guest.node, guest.type);
              attachTask(res.upid, guestKeys(guest.vmid ?? 0));
              onClose();
            } finally {
              setBusy(false);
            }
          }}
        >
          Migrate
        </Button>
      }
    >
      <Field label="Target node">
        <Select value={target} onChange={(e) => setTarget(e.target.value)}>
          {targets.map((n) => <option key={n.id} value={n.node}>{n.node}</option>)}
        </Select>
      </Field>
      <Field label="Destination storage">
        <Select value={targetStorage} onChange={(e) => setTargetStorage(e.target.value)}>
          <option value="">Keep current</option>
          {storagesForContent(storages.data ?? [], guest.type === "lxc" ? "rootdir" : "images", target).map((s) => (
            <option key={s.storage} value={s.storage}>{s.storage}</option>
          ))}
        </Select>
      </Field>
      <Field label="Migration network">
        <Input value={migrationNetwork} onChange={(e) => setMigrationNetwork(e.target.value)} placeholder="10.0.0.0/24" />
      </Field>
      {guest.template ? null : (
        <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={online} onChange={(e) => setOnline(e.target.checked)} /> Live (online)</label>
      )}
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={withLocalDisks} onChange={(e) => setWithLocalDisks(e.target.checked)} /> With local disks</label>
    </Modal>
  );
}

function TreeSnapshotDialog({ guest, onClose }: { guest: Resource; onClose: () => void }) {
  const now = new Date();
  const stamp = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, "0")}${String(now.getDate()).padStart(2, "0")}-${String(now.getHours()).padStart(2, "0")}${String(now.getMinutes()).padStart(2, "0")}`;
  const [name, setName] = useState(`snap-${stamp}`);
  const [description, setDescription] = useState("");
  const [ram, setRam] = useState(guest.type === "qemu");
  const [busy, setBusy] = useState(false);

  return (
    <Modal
      title={`Snapshot ${guest.name || guest.vmid}`}
      onClose={onClose}
      footer={
        <Button
          disabled={!name || busy}
          onClick={async () => {
            setBusy(true);
            try {
              const res = await api.createSnapshot(guest.vmid ?? 0, {
                name,
                description,
                vmstate: guest.type === "qemu" && ram,
              }, guest.node, guest.type);
              attachTask(res.upid, snapKeys(guest.vmid ?? 0));
              onClose();
            } finally {
              setBusy(false);
            }
          }}
        >
          Create snapshot
        </Button>
      }
    >
      <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
      <Field label="Description"><Input value={description} onChange={(e) => setDescription(e.target.value)} /></Field>
      {guest.type === "qemu" && (
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={ram} onChange={(e) => setRam(e.target.checked)} />
          Include RAM
        </label>
      )}
    </Modal>
  );
}

function TreeBackupDialog({ guest, onClose }: { guest: Resource; onClose: () => void }) {
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const backupStores = (storages.data ?? []).filter((s) => s.content?.includes("backup") && !s.disable);
  const [storage, setStorage] = useState("");
  const [mode, setMode] = useState("snapshot");
  const [busy, setBusy] = useState(false);
  const dest = storage || backupStores[0]?.storage || "";

  return (
    <Modal
      title={`Backup ${guest.name || guest.vmid}`}
      onClose={onClose}
      footer={
        <Button
          disabled={!dest || busy}
          onClick={async () => {
            setBusy(true);
            try {
              const res = await api.guestBackup(guest.vmid ?? 0, { storage: dest, mode, compress: "zstd" }, guest.node, guest.type);
              attachTask(res.upid, [...guestKeys(guest.vmid ?? 0), ["guest-backups"]]);
              onClose();
            } finally {
              setBusy(false);
            }
          }}
        >
          Backup now
        </Button>
      }
    >
      <Field label="Storage">
        <Select value={dest} onChange={(e) => setStorage(e.target.value)}>
          {backupStores.map((s) => <option key={s.storage}>{s.storage}</option>)}
        </Select>
      </Field>
      <Field label="Mode">
        <Select value={mode} onChange={(e) => setMode(e.target.value)}>
          <option value="snapshot">Snapshot</option>
          <option value="suspend">Suspend</option>
          <option value="stop">Stop</option>
        </Select>
      </Field>
      {!backupStores.length && <p className="text-sm text-ink-dim">No backup storage is available.</p>}
    </Modal>
  );
}
