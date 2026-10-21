import type { Resource } from "@/types";

export type TopNMetric = "cpu" | "mem" | "disk" | "diskio" | "net" | "iops";

export type TopNRow = {
  guest: Resource;
  value: number;
};

export function isRankableGuest(r: Resource) {
  return (r.type === "qemu" || r.type === "lxc") && !r.template;
}

export function guestScore(r: Resource, metric: TopNMetric, iopsByVmid: Record<number, number> = {}): number | null {
  switch (metric) {
    case "cpu":
      return Math.min(100, Math.max(0, (r.cpu ?? 0) * 100));
    case "mem":
      return r.maxmem ? Math.min(100, Math.max(0, (100 * (r.mem ?? 0)) / r.maxmem)) : 0;
    case "disk":
      return r.maxdisk ? Math.min(100, Math.max(0, (100 * (r.disk ?? 0)) / r.maxdisk)) : 0;
    case "diskio":
      return (r.diskread ?? 0) + (r.diskwrite ?? 0);
    case "net":
      return (r.netin ?? 0) + (r.netout ?? 0);
    case "iops": {
      if (r.vmid == null) return null;
      const n = iopsByVmid[r.vmid];
      return n == null || Number.isNaN(n) ? null : n;
    }
    default:
      return null;
  }
}

export function rankGuests(
  resources: Resource[],
  metric: TopNMetric,
  iopsByVmid: Record<number, number> = {},
  limit = 10,
): TopNRow[] {
  const rows: TopNRow[] = [];
  for (const guest of resources) {
    if (!isRankableGuest(guest)) continue;
    const value = guestScore(guest, metric, iopsByVmid);
    if (value == null) continue;
    rows.push({ guest, value });
  }
  rows.sort((a, b) => {
    if (b.value !== a.value) return b.value - a.value;
    const av = a.guest.vmid ?? 0;
    const bv = b.guest.vmid ?? 0;
    if (av !== bv) return av - bv;
    return (a.guest.name ?? "").localeCompare(b.guest.name ?? "", undefined, { numeric: true });
  });
  return rows.slice(0, limit);
}

export type OverlayGuest = {
  id: string;
  label: string;
  color: string;
  points: Record<string, number>[];
  value: (p: Record<string, number>) => number | null;
};

export function overlayGuests(guests: OverlayGuest[]): {
  points: Record<string, number>[];
  series: { key: string; label: string; color: string }[];
} {
  const times = new Set<number>();
  const indexed = guests.map((g) => {
    const byTime = new Map<number, Record<string, number>>();
    for (const p of g.points) {
      const t = Number(p.time ?? p.ts ?? 0);
      if (!t) continue;
      times.add(t);
      byTime.set(t, p);
    }
    return { g, byTime };
  });
  const points = [...times]
    .sort((a, b) => a - b)
    .map((time) => {
      const row: Record<string, number> = { time };
      for (const { g, byTime } of indexed) {
        const p = byTime.get(time);
        if (!p) continue;
        const v = g.value(p);
        if (v == null || Number.isNaN(v)) continue;
        row[g.id] = v;
      }
      return row;
    });
  return {
    points,
    series: guests.map((g) => ({ key: g.id, label: g.label, color: g.color })),
  };
}

export function sumGuestKeys(sets: Record<string, number>[][], keys: string[]): Record<string, number>[] {
  const byTime = new Map<number, Record<string, number>>();
  for (const points of sets) {
    for (const p of points) {
      const t = Number(p.time ?? p.ts ?? 0);
      if (!t) continue;
      const row = byTime.get(t) ?? { time: t };
      for (const key of keys) {
        const v = Number(p[key]);
        if (!Number.isFinite(v)) continue;
        row[key] = (row[key] ?? 0) + v;
      }
      byTime.set(t, row);
    }
  }
  return [...byTime.values()].sort((a, b) => a.time - b.time);
}

export function guestMemScale(points: Record<string, number>[], liveBytes = 0, maxBytes = 0) {
  const peak = Math.max(0, ...points.map((p) => Number(p.mem)).filter((n) => Number.isFinite(n)));
  if (!peak) return (v: number) => v;
  if (liveBytes > 0) {
    const err = (scale: number) => Math.abs(peak * scale - liveBytes) / liveBytes;
    const best = [1, 1024, 1024 * 1024].reduce((a, b) => (err(b) < err(a) ? b : a));
    return (v: number) => v * best;
  }
  if (maxBytes > 0 && peak <= maxBytes * 1.5) return (v: number) => v;
  if (peak >= 1024 * 1024) return (v: number) => v;
  if (peak >= 1024) return (v: number) => v * 1024;
  return (v: number) => v * 1024 * 1024;
}

export function agentDiskUsed(raw: unknown): number {
  let used = 0;
  let cur: unknown = raw;
  for (let i = 0; i < 4; i++) {
    if (Array.isArray(cur)) {
      for (const item of cur) {
        if (!item || typeof item !== "object" || Array.isArray(item)) continue;
        const rec = item as Record<string, unknown>;
        const typ = String(rec.type ?? "");
        if (/^(iso9660|udf|nfs|cifs|smb|9p|squashfs|tmpfs|devtmpfs|overlay|proc|sysfs|devfs)$/i.test(typ)) continue;
        const n = Number(rec["used-bytes"] ?? rec.used ?? 0);
        if (Number.isFinite(n) && n > 0) used += n;
      }
      return used;
    }
    if (cur && typeof cur === "object") {
      const rec = cur as Record<string, unknown>;
      cur = rec.result !== undefined ? rec.result : rec.data;
      continue;
    }
    break;
  }
  return used;
}

export function integrateRates(points: Record<string, number>[], keys: string[]): Record<string, number> {
  const out: Record<string, number> = {};
  for (const key of keys) out[key] = 0;
  for (let i = 1; i < points.length; i++) {
    const t0 = Number(points[i - 1].time ?? points[i - 1].ts ?? 0);
    const t1 = Number(points[i].time ?? points[i].ts ?? 0);
    const dt = t1 > t0 ? t1 - t0 : 0;
    if (!dt) continue;
    for (const key of keys) {
      const a = Number(points[i - 1][key]);
      const b = Number(points[i][key]);
      const va = Number.isFinite(a) ? a : 0;
      const vb = Number.isFinite(b) ? b : 0;
      out[key] += ((va + vb) / 2) * dt;
    }
  }
  return out;
}

export function iopsTotals(samples: Record<string, { rd?: number; wr?: number }> | undefined): Record<number, number> {
  const out: Record<number, number> = {};
  for (const [k, v] of Object.entries(samples ?? {})) {
    const id = Number(k);
    if (!Number.isFinite(id)) continue;
    out[id] = (v.rd ?? 0) + (v.wr ?? 0);
  }
  return out;
}
