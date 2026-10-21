export function bytesRate(n?: number | null) {
  return `${bytes(n)}/s`;
}

export function ops(n?: number | null) {
  if (n == null || Number.isNaN(n)) return "—";
  const v = Math.abs(n);
  if (v >= 1000) return `${(v / 1000).toFixed(v >= 10000 ? 0 : 1)}k`;
  return v < 10 ? v.toFixed(1) : v.toFixed(0);
}

export function bytes(n?: number | null) {
  if (n == null || Number.isNaN(n)) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let v = Math.abs(n);
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
}

export function pct(used?: number, max?: number) {
  if (!max) return 0;
  return Math.min(100, Math.max(0, (100 * (used ?? 0)) / max));
}

export function cpuPct(cpu?: number) {
  return Math.min(100, Math.max(0, (cpu ?? 0) * 100));
}

// Usage reads neutral until it needs attention (same thresholds as meters).
export const USAGE_OK = "#c9ced6";
export const USAGE_WARN = "#d9a441";
export const USAGE_HIGH = "#f0857a";

export function usageHex(pct: number) {
  if (pct >= 95) return USAGE_HIGH;
  if (pct >= 80) return USAGE_WARN;
  return USAGE_OK;
}

export function usageTextClass(pct: number) {
  if (pct >= 95) return "text-danger";
  if (pct >= 80) return "text-warn";
  return "text-ink";
}

export function cpuMeterLabel(cores?: number | null) {
  const n = Math.round(Number(cores) || 0);
  if (n <= 0) return "CPU";
  return `CPU · ${n} ${n === 1 ? "core" : "cores"}`;
}

export function uptime(seconds?: number) {
  if (!seconds) return "—";
  const d = Math.floor(seconds / 86400);
  const h = Math.floor((seconds % 86400) / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (d) return `${d}d ${h}h`;
  if (h) return `${h}h ${m}m`;
  return `${m}m`;
}

export function relTime(epoch?: number) {
  if (!epoch) return "—";
  const ms = epoch > 1e12 ? epoch : epoch * 1000;
  return new Date(ms).toLocaleString();
}

/** "today 02:00", "yesterday 23:10", or "Sep 12 06:00" in the viewer's time zone. */
export function formatWhen(unix: number): string {
  const d = new Date(unix * 1000);
  const today = new Date();
  const yesterday = new Date(today.getTime() - 86_400_000);
  const time = d.toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  if (d.toDateString() === today.toDateString()) return `today ${time}`;
  if (d.toDateString() === yesterday.toDateString()) return `yesterday ${time}`;
  return `${d.toLocaleDateString(undefined, { month: "short", day: "numeric" })} ${time}`;
}

/** Whole days since a Unix time. */
export function daysSince(unix: number): number {
  return Math.max(0, Math.floor((Date.now() / 1000 - unix) / 86_400));
}
