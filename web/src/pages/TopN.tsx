import { MetricChart } from "@/components/charts/MetricChart";
import { Card } from "@/components/ui/card";
import { InventoryCard } from "@/components/ui/inventory";
import { api, ApiError } from "@/lib/api";
import { getActiveConnection, onActiveConnection } from "@/lib/connection";
import { bytes, bytesRate, cpuPct, ops, usageHex, usageTextClass } from "@/lib/format";
import {
  agentDiskUsed,
  guestMemScale,
  iopsTotals,
  isRankableGuest,
  integrateRates,
  overlayGuests,
  rankGuests,
  type TopNMetric,
} from "@/lib/topN";
import { live } from "@/lib/ws";
import type { Resource } from "@/types";
import { useQueries, useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Segmented } from "@/components/ui/tabs";
import { CHART_CPU, CHART_MEM } from "@/lib/chartColors";
import { CHART_PALETTE } from "@/lib/chartColors";

const TIMEFRAMES = ["hour", "day", "week", "month", "year"] as const;
const TABS = [
  { id: "cpu", label: "CPU", rank: "cpu" as TopNMetric },
  { id: "mem", label: "Memory", rank: "mem" as TopNMetric },
  { id: "storage", label: "Storage", rank: "disk" as TopNMetric },
  { id: "net", label: "Network", rank: "net" as TopNMetric },
] as const;
const STORAGE_TABS = [
  { id: "usage", label: "Usage", rank: "disk" as TopNMetric },
  { id: "rw", label: "Read / Write", rank: "diskio" as TopNMetric },
  { id: "iops", label: "IOPS", rank: "iops" as TopNMetric },
] as const;
const PALETTE = CHART_PALETTE;

type Tab = (typeof TABS)[number]["id"];
type StorageTab = (typeof STORAGE_TABS)[number]["id"];
type RankKind = "cpu" | "mem" | "usage" | "rw" | "iops" | "net";

export function TopNPage() {
  const [tab, setTab] = useState<Tab>("cpu");
  const [storageTab, setStorageTab] = useState<StorageTab>("usage");
  const [tf, setTf] = useState<(typeof TIMEFRAMES)[number]>("hour");
  const q = useQuery({ queryKey: ["summary", getActiveConnection()], queryFn: api.summary, refetchInterval: 15000 });
  const [liveResources, setLiveResources] = useState<Resource[] | null>(null);

  useEffect(() => {
    const off = live.on((topic, data) => {
      if (topic !== "resources") return;
      const payload = data as { connection?: string; resources?: Resource[] };
      if (payload.connection && payload.connection !== getActiveConnection()) return;
      if (payload.resources) setLiveResources(payload.resources);
    });
    const unsub = onActiveConnection(() => setLiveResources(null));
    return () => {
      off();
      unsub();
    };
  }, []);

  const resources = liveResources ?? q.data?.resources ?? [];
  const guests = resources.filter((r) => r.type === "qemu" || r.type === "lxc");
  const iopsLatest = useQuery({
    queryKey: ["guest-iops-latest", getActiveConnection()],
    queryFn: api.guestIopsLatest,
    refetchInterval: 15000,
  });
  const iopsByVmid = useMemo(() => iopsTotals(iopsLatest.data?.samples), [iopsLatest.data]);
  const rankable = useMemo(() => guests.filter(isRankableGuest), [guests]);
  const fsQueries = useQueries({
    queries: rankable.map((g) => ({
      queryKey: ["guest-fsinfo", g.vmid, g.node, g.type],
      queryFn: () => api.agentCommand(g.vmid!, "get-fsinfo", g.node, g.type),
      enabled: tab === "storage" && g.type === "qemu" && g.status === "running" && g.vmid != null,
      refetchInterval: 30000,
      retry: false,
    })),
  });
  const guestsForRank = useMemo(() => {
    if (tab !== "storage") return guests;
    const usedById = new Map<string, number>();
    rankable.forEach((g, i) => {
      const used = agentDiskUsed(fsQueries[i]?.data) || g.disk || 0;
      usedById.set(g.id, used);
    });
    return guests.map((g) => {
      const used = usedById.get(g.id);
      return used != null ? { ...g, disk: used } : g;
    });
  }, [tab, guests, rankable, fsQueries]);
  const rankMetric = tab === "storage"
    ? (STORAGE_TABS.find((t) => t.id === storageTab)?.rank ?? "disk")
    : (TABS.find((t) => t.id === tab)?.rank ?? "cpu");
  const prelimRows = rankGuests(guestsForRank, rankMetric, iopsByVmid);
  const rrdTargets = (tab === "net" || (tab === "storage" && storageTab === "rw")) ? rankable : prelimRows.map((r) => r.guest);
  const rrdQueries = useQueries({
    queries: rrdTargets.map((g) => ({
      queryKey: ["guest-rrd", g.vmid, g.node, g.type, tf],
      queryFn: () => api.guestRrd(g.vmid!, tf, g.node, g.type),
      enabled: g.vmid != null && !(tab === "storage" && storageTab === "usage"),
      refetchInterval: 15000,
    })),
  });
  const ioQueries = useQueries({
    queries: prelimRows.map((row) => ({
      queryKey: ["guest-io-rrd", row.guest.vmid, row.guest.node, row.guest.type, tf],
      queryFn: () => api.guestIoRrd(row.guest.vmid!, tf, row.guest.node, row.guest.type),
      enabled: tab === "storage" && storageTab === "iops" && row.guest.vmid != null,
      refetchInterval: 15000,
    })),
  });
  const rrdById = useMemo(() => {
    const out = new Map<string, Record<string, number>[]>();
    rrdTargets.forEach((g, i) => out.set(g.id, rrdQueries[i]?.data ?? []));
    return out;
  }, [rrdTargets, rrdQueries]);
  const rows = useMemo(() => {
    if (tab === "net") {
      return guestsForRank
        .filter(isRankableGuest)
        .map((guest) => ({
          guest,
          value: lastSum(rrdById.get(guest.id) ?? [], ["netin", "netout"]),
        }))
        .sort((a, b) => b.value - a.value || (a.guest.vmid ?? 0) - (b.guest.vmid ?? 0))
        .slice(0, 10);
    }
    if (tab !== "storage" || storageTab !== "rw") return prelimRows;
    return guestsForRank
      .filter(isRankableGuest)
      .map((guest) => ({
        guest,
        value: lastSum(rrdById.get(guest.id) ?? [], ["diskread", "diskwrite"]) || (guest.diskread ?? 0) + (guest.diskwrite ?? 0),
      }))
      .sort((a, b) => b.value - a.value || (a.guest.vmid ?? 0) - (b.guest.vmid ?? 0))
      .slice(0, 10);
  }, [tab, storageTab, prelimRows, guestsForRank, rrdById]);
  const rrdSets = rows.map((row) => rrdById.get(row.guest.id) ?? []);
  const iopsSets = ioQueries.map((q) => q.data?.iops ?? []);

  if (q.error instanceof ApiError && q.error.status === 503) {
    return (
      <div className="mx-auto max-w-lg py-16 text-center">
        <h1 className="text-2xl font-semibold">Add a connection</h1>
        <p className="mt-2 text-sm text-ink-dim">
          Hypervane is ready. Connect a Proxmox cluster or standalone node from Settings.
        </p>
        <Link to="/settings" className="mt-6 inline-block text-sm text-accent hover:underline">
          Open Settings
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-xs font-medium text-ink-dim">Hot guests</p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight">Top 10</h1>
          <p className="mt-1 text-sm text-ink-dim">Current pressure on this cluster. Click through to the guest.</p>
        </div>
        <Segmented items={TIMEFRAMES} value={tf} onChange={setTf} ariaLabel="Time range" />
      </header>

      <div className="grid grid-cols-2 gap-1 sm:grid-cols-4">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            onClick={() => setTab(t.id)}
            className={`rounded-md px-3 py-2 text-sm ${
              tab === t.id ? "bg-raised text-ink shadow-[inset_0_-2px_0_rgb(var(--hv-accent))]" : "bg-white/[0.02] text-ink-dim hover:bg-white/[0.05] hover:text-ink"
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {tab === "storage" && (
        <div className="grid grid-cols-3 gap-1">
          {STORAGE_TABS.map((t) => (
            <button
              key={t.id}
              type="button"
              onClick={() => setStorageTab(t.id)}
              className={`rounded-md border px-3 py-2 text-sm ${
                storageTab === t.id
                  ? "border-accent text-ink"
                  : "border-transparent text-ink-dim hover:border-white/10 hover:text-ink"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>
      )}

      {(tab !== "storage" || storageTab !== "usage") && (
      <Card className="space-y-6 p-5">
        {tab === "cpu" && (
          <FullChart title="CPU">
            <OverlayChart
              rows={rows}
              sets={rrdSets}
              format="percent"
              value={(p) => {
                const v = Number(p.cpu);
                return Number.isFinite(v) ? v * 100 : null;
              }}
            />
          </FullChart>
        )}
        {tab === "mem" && (
          <FullChart title="Memory">
            <OverlayChart
              rows={rows}
              sets={rrdSets}
              format="percent"
              value={(p, i) => {
                const v = Number(p.mem);
                const g = rows[i]?.guest;
                if (!Number.isFinite(v) || !g?.maxmem) return null;
                return Math.min(100, (100 * guestMemScale(rrdSets[i] ?? [], g.mem, g.maxmem)(v)) / g.maxmem);
              }}
            />
          </FullChart>
        )}
        {tab === "storage" && storageTab === "rw" && (
          <FullChart title="Read / Write">
            <OverlayChart
              rows={rows}
              sets={rrdSets}
              format="rate"
              value={(p) => {
                const r = Number(p.diskread);
                const w = Number(p.diskwrite);
                const total = (Number.isFinite(r) ? r : 0) + (Number.isFinite(w) ? w : 0);
                return total || null;
              }}
            />
          </FullChart>
        )}
        {tab === "storage" && storageTab === "iops" && (
          <FullChart title="IOPS">
            <OverlayChart
              rows={rows}
              sets={iopsSets}
              format="ops"
              value={(p) => {
                const r = Number(p.rd_requests);
                const w = Number(p.wr_requests);
                const total = (Number.isFinite(r) ? r : 0) + (Number.isFinite(w) ? w : 0);
                return total || null;
              }}
            />
          </FullChart>
        )}
        {tab === "net" && (
          <FullChart title="Network">
            <OverlayChart
              rows={rows}
              sets={rrdSets}
              format="rate"
              value={(p) => {
                const inn = Number(p.netin);
                const out = Number(p.netout);
                const total = (Number.isFinite(inn) ? inn : 0) + (Number.isFinite(out) ? out : 0);
                return total || null;
              }}
            />
          </FullChart>
        )}
      </Card>
      )}

      <InventoryCard title={tab === "storage"
        ? (STORAGE_TABS.find((t) => t.id === storageTab)?.label ?? "Storage")
        : (TABS.find((t) => t.id === tab)?.label ?? "Guests")}
      >
        <RankTable
          kind={tab === "storage" ? storageTab : tab}
          rows={rows}
          rrdSets={rrdSets}
          iopsSets={iopsSets}
          iopsByVmid={iopsByVmid}
        />
      </InventoryCard>
    </div>
  );
}

function FullChart({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div>
      <p className="mb-2 text-xs font-medium text-ink-dim">{title}</p>
      {children}
    </div>
  );
}

function OverlayChart({
  rows,
  sets,
  format,
  value,
}: {
  rows: { guest: Resource }[];
  sets: Record<string, number>[][];
  format: "percent" | "bytes" | "rate" | "ops";
  value: (p: Record<string, number>, index: number) => number | null;
}) {
  const overlay = overlayGuests(
    rows.map((row, i) => ({
      id: `g${row.guest.vmid}`,
      label: row.guest.name || `VM ${row.guest.vmid}`,
      color: PALETTE[i % PALETTE.length],
      points: sets[i] ?? [],
      value: (p) => value(p, i),
    })),
  );
  return (
    <div>
      <MetricChart points={overlay.points} format={format} height={240} series={overlay.series} />
      {overlay.series.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-x-4 gap-y-1">
          {overlay.series.map((s, i) => (
            <Link
              key={s.key}
              to="/guests/$vmid"
              params={{ vmid: String(rows[i]?.guest.vmid) }}
              search={{ node: rows[i]?.guest.node, type: rows[i]?.guest.type }}
              className="flex items-center gap-1.5 text-[11px] text-ink-dim hover:text-ink"
            >
              <span className="h-1.5 w-1.5 rounded-full" style={{ background: s.color }} />
              {s.label}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function RankTable({
  kind,
  rows,
  rrdSets,
  iopsSets,
  iopsByVmid,
}: {
  kind: RankKind;
  rows: { guest: Resource; value: number }[];
  rrdSets: Record<string, number>[][];
  iopsSets?: Record<string, number>[][];
  iopsByVmid?: Record<number, number>;
}) {
  const leader = kind === "rw"
    ? Math.max(0, ...rows.map((_, i) => lastSum(rrdSets[i] ?? [], ["diskread", "diskwrite"])))
    : kind === "iops"
      ? Math.max(0, ...rows.map((row, i) => lastSum(iopsSets?.[i] ?? [], ["rd_requests", "wr_requests"]) || (row.guest.vmid != null ? iopsByVmid?.[row.guest.vmid] ?? 0 : 0)))
      : (rows[0]?.value ?? 0);
  const extraHeader = kind === "cpu" ? "CPUs" : kind === "mem" ? "Memory" : "Disk";
  return (
    <table className="w-full table-fixed text-sm">
      <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
        <tr>
          <th className="w-1/5 px-3 py-2">Guest</th>
          <th className="w-1/5 px-3 py-2">Node</th>
          {kind === "net" ? (
            <>
              <th className="w-1/5 px-3 py-2">In/Out Rate</th>
              <th className="w-1/5 px-3 py-2">In/Out Totals</th>
            </>
          ) : (
            <>
              <th className="w-1/5 px-3 py-2">{extraHeader}</th>
              <th className="w-1/5 px-3 py-2">Value</th>
            </>
          )}
          <th className="w-1/5 px-3 py-2">History</th>
        </tr>
      </thead>
      <tbody>
        {rows.length === 0 && (
          <tr className="border-t border-line">
            <td className="px-4 py-6 text-sm text-ink-dim" colSpan={5}>
              No guests to rank yet.
            </td>
          </tr>
        )}
        {rows.map((row, i) => {
          const tone = valueTone(kind, row, leader, rrdSets[i] ?? [], iopsSets?.[i] ?? [], iopsByVmid);
          return (
            <tr key={row.guest.id} className="border-t border-line">
              <td className="truncate px-3 py-3 align-middle">
                <Link
                  to="/guests/$vmid"
                  params={{ vmid: String(row.guest.vmid) }}
                  search={{ node: row.guest.node, type: row.guest.type }}
                  className="flex items-baseline gap-2 text-ink hover:text-accent"
                >
                  <span className="font-mono text-[11px] text-ink-faint">{i + 1}</span>
                  <span className="truncate font-medium">{row.guest.name || `VM ${row.guest.vmid}`}</span>
                </Link>
              </td>
              <td className="truncate px-3 py-3 align-middle font-mono text-xs text-ink-dim">{row.guest.node || "—"}</td>
              {kind === "net" ? (
                <>
                  <td className="px-3 py-3 align-middle font-mono text-sm text-ink">{netPair(rrdSets[i] ?? [], "rate")}</td>
                  <td className="px-3 py-3 align-middle font-mono text-sm text-ink">{netPair(rrdSets[i] ?? [], "total")}</td>
                </>
              ) : (
                <>
                  <td className={`truncate px-3 py-3 align-middle font-mono text-sm ${extraClass(kind, row.guest)}`}>{extraCell(kind, row.guest)}</td>
                  <td className="px-3 py-3 align-middle">
                    <span className={`text-base font-semibold tabular-nums ${kind === "rw" || kind === "iops" ? "text-ink" : usageTextClass(tone)}`}>
                      {valueLabel(kind, row, rrdSets[i] ?? [], iopsSets?.[i] ?? [], iopsByVmid)}
                    </span>
                  </td>
                </>
              )}
              <td className="px-3 py-2">
                <ValueChart
                  kind={kind}
                  points={kind === "iops" ? (iopsSets?.[i] ?? []) : (rrdSets[i] ?? [])}
                  guest={row.guest}
                  color={PALETTE[i % PALETTE.length]}
                />
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function extraClass(kind: RankKind, guest: Resource) {
  if (kind === "rw" || kind === "iops" || kind === "usage") return usageTextClass(diskPct(guest));
  return "text-ink";
}

function extraCell(kind: RankKind, guest: Resource) {
  if (kind === "cpu") return Math.round(guest.maxcpu || 0) || "—";
  if (kind === "mem") return `${bytes(guest.mem)} / ${bytes(guest.maxmem)}`;
  return `${bytes(guest.disk)} / ${bytes(guest.maxdisk)}`;
}

function diskPct(guest: Resource) {
  return guest.maxdisk ? Math.min(100, Math.max(0, (100 * (guest.disk ?? 0)) / guest.maxdisk)) : 0;
}

function lastSum(points: Record<string, number>[], keys: string[]) {
  const last = points[points.length - 1];
  if (!last) return 0;
  return keys.reduce((sum, key) => sum + (Number(last[key]) || 0), 0);
}

function valueTone(
  kind: RankKind,
  row: { guest: Resource; value: number },
  leader: number,
  rrd: Record<string, number>[],
  iops: Record<string, number>[],
  iopsByVmid?: Record<number, number>,
) {
  if (kind === "cpu") return cpuPct(row.guest.cpu);
  if (kind === "mem") return memPct(row.guest);
  if (kind === "usage") return diskPct(row.guest);
  const live = kind === "iops"
    ? (lastSum(iops, ["rd_requests", "wr_requests"]) || (row.guest.vmid != null ? iopsByVmid?.[row.guest.vmid] ?? 0 : 0))
    : lastSum(rrd, ["diskread", "diskwrite"]);
  return leader > 0 ? Math.min(100, (100 * live) / leader) : 0;
}

function valueLabel(
  kind: RankKind,
  row: { guest: Resource; value: number },
  rrd: Record<string, number>[],
  iops: Record<string, number>[],
  iopsByVmid?: Record<number, number>,
) {
  if (kind === "cpu") return `${cpuPct(row.guest.cpu).toFixed(0)}%`;
  if (kind === "mem") return `${memPct(row.guest).toFixed(0)}%`;
  if (kind === "usage") return `${diskPct(row.guest).toFixed(0)}%`;
  if (kind === "rw") {
    const last = rrd[rrd.length - 1];
    const rate = last
      ? (Number(last.diskread) || 0) + (Number(last.diskwrite) || 0)
      : (row.guest.diskread ?? 0) + (row.guest.diskwrite ?? 0);
    return bytesRate(rate);
  }
  const last = iops[iops.length - 1];
  const n = last
    ? (Number(last.rd_requests) || 0) + (Number(last.wr_requests) || 0)
    : (row.guest.vmid != null ? iopsByVmid?.[row.guest.vmid] : undefined);
  return n == null ? "—" : ops(n);
}

function memPct(guest: Resource) {
  return guest.maxmem ? Math.min(100, Math.max(0, (100 * (guest.mem ?? 0)) / guest.maxmem)) : 0;
}

function ValueChart({
  kind,
  points,
  guest,
  color = CHART_CPU,
}: {
  kind: RankKind;
  points: Record<string, number>[];
  guest?: Resource;
  color?: string;
}) {
  if (kind === "cpu") {
    return (
      <MetricChart
        compact
        threshold
        height={56}
        points={points}
        format="percent"
        series={[{ key: "cpu", label: "CPU", color: CHART_CPU, scale: (v) => v * 100 }]}
      />
    );
  }
  if (kind === "mem") {
    const max = guest?.maxmem || 0;
    return (
      <MetricChart
        compact
        threshold
        height={56}
        points={points}
        format="percent"
        series={[
          {
            key: "mem",
            label: "Memory",
            color: CHART_MEM,
            scale: (v) => {
              if (!max) return 0;
              return Math.min(100, (100 * guestMemScale(points, guest?.mem, guest?.maxmem)(v)) / max);
            },
          },
        ]}
      />
    );
  }
  if (kind === "usage") {
    const pct = guest ? diskPct(guest) : 0;
    return (
      <div className="flex h-14 items-center">
        <div className="h-2 w-full overflow-hidden rounded-full bg-white/10">
          <div className="h-full rounded-full" style={{ width: `${pct}%`, background: usageBarColor(pct) }} />
        </div>
      </div>
    );
  }
  if (kind === "rw") {
    return (
      <MetricChart
        compact
        height={56}
        points={combineKeys(points, "io", ["diskread", "diskwrite"])}
        format="rate"
        series={[{ key: "io", label: "Disk", color }]}
      />
    );
  }
  if (kind === "net") {
    return (
      <MetricChart
        compact
        height={56}
        points={combineKeys(points, "io", ["netin", "netout"])}
        format="rate"
        series={[{ key: "io", label: "Net", color }]}
      />
    );
  }
  return (
    <MetricChart
      compact
      height={56}
      points={combineKeys(points, "io", ["rd_requests", "wr_requests"])}
      format="ops"
      series={[{ key: "io", label: "IOPS", color }]}
    />
  );
}

function netPair(points: Record<string, number>[], mode: "rate" | "total") {
  const pair = mode === "total" ? integrateRates(points, ["netin", "netout"]) : null;
  const inn = pair ? (pair.netin ?? 0) : lastKey(points, "netin");
  const out = pair ? (pair.netout ?? 0) : lastKey(points, "netout");
  const fmt = mode === "total" ? bytes : bytesRate;
  return (
    <span className="block leading-5">
      <span className="text-ink">{fmt(inn)}</span> in
      <br />
      <span className="text-ink">{fmt(out)}</span> out
    </span>
  );
}

function lastKey(points: Record<string, number>[], key: string) {
  const last = points[points.length - 1];
  return last ? Number(last[key]) || 0 : 0;
}

function combineKeys(points: Record<string, number>[], key: string, keys: string[]) {
  return points.map((p) => ({
    time: Number(p.time ?? p.ts ?? 0),
    [key]: keys.reduce((sum, k) => sum + (Number(p[k]) || 0), 0),
  }));
}

function usageBarColor(pct: number) {
  return usageHex(pct);
}

