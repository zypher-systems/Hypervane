export type NicForm = {
  slot: string;
  model: string;
  name: string;
  bridge: string;
  tag: string;
  trunks: string;
  firewall: boolean;
  rate: string;
  mtu: string;
  mac: string;
  linkDown: boolean;
  ip: string;
  gw: string;
  ip6: string;
  gw6: string;
};

export type StartupForm = {
  order: string;
  up: string;
  down: string;
};

const diskSlotRe = /^(scsi|sata|virtio|ide|rootfs|mp)\d+$/;
const unusedRe = /^unused\d+$/;
const netRe = /^net\d+$/;

export function configDigest(config: Record<string, unknown>): string {
  return String(config.digest ?? "");
}

export function parseBoot(boot?: unknown): string[] {
  const raw = String(boot ?? "");
  const m = raw.match(/order=([^;]+(?:;[^;]+)*)/);
  if (m) return m[1].split(";").filter(Boolean);
  return raw ? raw.split(";").filter(Boolean) : [];
}

export function isCdrom(value: string): boolean {
  return /(?:^|,)media=cdrom(?:,|$)/i.test(value);
}

export function diskEntries(config: Record<string, unknown>): [string, string][] {
  return Object.entries(config)
    .filter(([k, v]) => (diskSlotRe.test(k) || k === "rootfs") && !isCdrom(String(v ?? "")))
    .map(([k, v]) => [k, String(v ?? "")] as [string, string]);
}

export function unusedEntries(config: Record<string, unknown>): [string, string][] {
  return Object.entries(config)
    .filter(([k]) => unusedRe.test(k))
    .map(([k, v]) => [k, String(v ?? "")] as [string, string]);
}

export function nicEntries(config: Record<string, unknown>): [string, string][] {
  return Object.entries(config)
    .filter(([k]) => netRe.test(k))
    .map(([k, v]) => [k, String(v ?? "")] as [string, string]);
}

export function cdromEntries(config: Record<string, unknown>): [string, string][] {
  return Object.entries(config)
    .filter(([k, v]) => /^(ide|sata)\d+$/.test(k) && isCdrom(String(v ?? "")))
    .map(([k, v]) => [k, String(v ?? "")] as [string, string]);
}

export function nextSlot(config: Record<string, unknown>, prefix: string, max: number): string {
  for (let i = 0; i <= max; i++) {
    const key = `${prefix}${i}`;
    if (config[key] === undefined) return key;
  }
  return `${prefix}${max}`;
}

export function busMax(bus: string): number {
  if (bus === "ide") return 3;
  if (bus === "sata") return 5;
  if (bus === "virtio") return 15;
  return 30;
}

export function parseSizeToBytes(raw?: string): number | null {
  const s = String(raw ?? "").trim();
  if (!s) return null;
  const m = s.match(/^(\d+(?:\.\d+)?)\s*([kmgtp]i?b?)?$/i);
  if (!m) return null;
  const n = Number(m[1]);
  if (!Number.isFinite(n) || n < 0) return null;
  const unit = (m[2] || "").toLowerCase();
  if (!unit && n >= 1024 ** 3) return n;
  const pow = unit.startsWith("k") ? 1 : unit.startsWith("m") ? 2 : unit.startsWith("t") ? 4 : unit.startsWith("p") ? 5 : 3;
  return n * 1024 ** pow;
}

export function diskSizeGiB(raw?: string): number | null {
  const bytes = parseSizeToBytes(raw);
  if (bytes == null) return null;
  return bytes / 1024 ** 3;
}

export function parseCPU(raw: unknown): { model: string; flags: string } {
  const { head, extras } = parseKV(String(raw ?? ""));
  return { model: head, flags: extras.flags ?? "" };
}

export function formatCPU(model: string, flags: string): string {
  return formatKV(model, { flags: flags || undefined });
}

export function parseKV(raw: string): { head: string; extras: Record<string, string> } {
  const extras: Record<string, string> = {};
  const parts = String(raw ?? "").split(",").filter(Boolean);
  let head = "";
  for (const part of parts) {
    const eq = part.indexOf("=");
    if (eq === -1) {
      if (!head) head = part;
      continue;
    }
    extras[part.slice(0, eq)] = part.slice(eq + 1);
  }
  return { head, extras };
}

export function formatKV(head: string, extras: Record<string, string | number | boolean | undefined>): string {
  const parts: string[] = [];
  if (head) parts.push(head);
  for (const [k, v] of Object.entries(extras)) {
    if (v === undefined || v === "" || v === false) continue;
    if (v === true) {
      parts.push(`${k}=1`);
      continue;
    }
    parts.push(`${k}=${v}`);
  }
  return parts.join(",");
}

export function emptyNic(slot: string, kind: string): NicForm {
  return {
    slot,
    model: "virtio",
    name: "eth0",
    bridge: "vmbr0",
    tag: "",
    trunks: "",
    firewall: false,
    rate: "",
    mtu: "",
    mac: "",
    linkDown: false,
    ip: kind === "lxc" ? "dhcp" : "",
    gw: "",
    ip6: "",
    gw6: "",
  };
}

export function parseNet(slot: string, raw: string, kind: string): NicForm {
  const { head, extras } = parseKV(raw);
  const form = emptyNic(slot, kind);
  if (kind === "lxc") {
    form.name = extras.name || "eth0";
    form.bridge = extras.bridge || "vmbr0";
    form.ip = extras.ip || "";
    form.gw = extras.gw || "";
    form.ip6 = extras.ip6 || "";
    form.gw6 = extras.gw6 || "";
  } else {
    form.model = extras.model || head || "virtio";
    form.bridge = extras.bridge || "vmbr0";
    form.mac = extras.macaddr || "";
    form.linkDown = extras.link_down === "1";
  }
  form.tag = extras.tag || "";
  form.trunks = extras.trunks || "";
  form.firewall = extras.firewall === "1";
  form.rate = extras.rate || "";
  form.mtu = extras.mtu || "";
  return form;
}

export function formatNet(form: NicForm, kind: string): string {
  if (kind === "lxc") {
    return formatKV("", {
      name: form.name || "eth0",
      bridge: form.bridge || "vmbr0",
      ip: form.ip || undefined,
      gw: form.gw || undefined,
      ip6: form.ip6 || undefined,
      gw6: form.gw6 || undefined,
      tag: form.tag || undefined,
      trunks: form.trunks || undefined,
      firewall: form.firewall || undefined,
      rate: form.rate || undefined,
      mtu: form.mtu || undefined,
      hwaddr: form.mac || undefined,
    });
  }
  return formatKV(form.model || "virtio", {
    bridge: form.bridge || "vmbr0",
    tag: form.tag || undefined,
    trunks: form.trunks || undefined,
    firewall: form.firewall || undefined,
    rate: form.rate || undefined,
    mtu: form.mtu || undefined,
    macaddr: form.mac || undefined,
    link_down: form.linkDown || undefined,
  });
}

export function parseStartup(raw: unknown): StartupForm {
  const { extras } = parseKV(String(raw ?? ""));
  return { order: extras.order || "", up: extras.up || "", down: extras.down || "" };
}

export function formatStartup(form: StartupForm): string {
  return formatKV("", {
    order: form.order || undefined,
    up: form.up || undefined,
    down: form.down || undefined,
  });
}

export const hotplugFlags = ["disk", "network", "usb", "memory", "cpu"] as const;

export function parseHotplug(raw: unknown): string[] {
  const s = String(raw ?? "").trim();
  if (!s) return ["disk", "network", "usb"];
  if (s === "1" || s === "on") return [...hotplugFlags];
  return s.split(",").map((x) => x.trim()).filter(Boolean);
}

export function formatHotplug(flags: string[]): string {
  return flags.join(",");
}

export function slotEntries(config: Record<string, unknown>, prefix: string): [string, string][] {
  const re = new RegExp(`^${prefix}\\d+$`);
  return Object.entries(config)
    .filter(([k]) => re.test(k))
    .map(([k, v]) => [k, String(v ?? "")] as [string, string]);
}

export function formatHostPCI(opts: { mapping?: string; host?: string; pcie?: boolean; xvga?: boolean; rombar?: boolean; mdev?: string }): string {
  if (opts.mapping) {
    return formatKV("", { mapping: opts.mapping, mdev: opts.mdev, pcie: opts.pcie, "x-vga": opts.xvga, rombar: opts.rombar === false ? 0 : undefined });
  }
  return formatKV(opts.host || "", { mdev: opts.mdev, pcie: opts.pcie, "x-vga": opts.xvga, rombar: opts.rombar === false ? 0 : undefined });
}

export function formatUSB(opts: { mapping?: string; host?: string; usb3?: boolean }): string {
  if (opts.mapping) {
    return formatKV("", { mapping: opts.mapping, usb3: opts.usb3 });
  }
  const host = opts.host?.includes(":") && !opts.host.startsWith("host=") ? `host=${opts.host}` : (opts.host || "");
  return formatKV(host, { usb3: opts.usb3 });
}

export const lxcFeatureFlags = ["nesting", "keyctl", "fuse", "mknod"] as const;

export function parseFeatures(raw: unknown): string[] {
  const { extras } = parseKV(String(raw ?? ""));
  return lxcFeatureFlags.filter((f) => extras[f] === "1");
}

export function formatFeatures(flags: string[]): string {
  return formatKV("", Object.fromEntries(flags.map((f) => [f, true])));
}

export function formatMount(opts: { storage?: string; sizeGb?: string; bind?: string; mp: string }): string {
  const head = opts.bind ? opts.bind : `${opts.storage}:${opts.sizeGb || "8"}`;
  return formatKV(head, { mp: opts.mp || "/mnt/data" });
}

export function formatImport(opts: { storage: string; importFrom: string }): string {
  return formatKV(`${opts.storage}:0`, { "import-from": opts.importFrom });
}

export function formatDisk(opts: {
  storage: string;
  sizeGb: string;
  format?: string;
  cache?: string;
  discard?: boolean;
  iothread?: boolean;
  backup?: boolean;
  cloudinit?: boolean;
}): string {
  if (opts.cloudinit) return `${opts.storage}:cloudinit`;
  return formatKV(`${opts.storage}:${opts.sizeGb}`, {
    format: opts.format || undefined,
    cache: opts.cache || undefined,
    discard: opts.discard || undefined,
    iothread: opts.iothread || undefined,
    backup: opts.backup === false ? 0 : undefined,
  });
}
