import { bytes } from "@/lib/format";
import type { PendingChange } from "@/types";

const memoryKeys = new Set(["memory", "balloon", "min_memory", "swap"]);

const labels: Record<string, string> = {
  memory: "Memory",
  balloon: "Balloon",
  min_memory: "Minimum memory",
  swap: "Swap",
  shares: "Memory shares",
  cores: "Cores",
  sockets: "Sockets",
  vcpus: "vCPUs",
  cpu: "CPU type",
  cpulimit: "CPU limit",
  cpuunits: "CPU units",
  numa: "NUMA",
  ostype: "OS type",
  machine: "Machine",
  bios: "Firmware",
  scsihw: "SCSI controller",
  onboot: "Start at boot",
  agent: "QEMU guest agent",
  name: "Name",
  hostname: "Hostname",
  tags: "Tags",
  description: "Description",
  rootfs: "Root disk",
};

export function pendingLabel(key: string): string {
  if (labels[key]) return labels[key];
  const net = key.match(/^net(\d+)$/);
  if (net) return `NIC ${net[1]}`;
  const disk = key.match(/^(scsi|sata|virtio|ide|mp)(\d+)$/);
  if (disk) return `${disk[1].toUpperCase()} ${disk[2]}`;
  const unused = key.match(/^unused(\d+)$/);
  if (unused) return `Unused disk ${unused[1]}`;
  return key.replace(/[_-]/g, " ");
}

export function formatPendingValue(key: string, value: unknown): string {
  if (value === undefined || value === null || value === "") return "—";
  if (memoryKeys.has(key)) {
    const n = Number(value);
    if (Number.isFinite(n)) return bytes(n * 1024 * 1024);
  }
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

export function describePending(change: PendingChange, config?: Record<string, unknown>): {
  label: string;
  from: string;
  to: string;
  summary: string;
} {
  const current = change.value !== undefined && change.value !== "" ? change.value : config?.[change.key];
  const next = change.delete ? undefined : change.pending;
  const from = formatPendingValue(change.key, current);
  const to = change.delete ? "removed" : formatPendingValue(change.key, next);
  const label = pendingLabel(change.key);
  return { label, from, to, summary: `${label} ${from} → ${to}` };
}
