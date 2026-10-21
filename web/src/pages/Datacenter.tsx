import { ObjectACL } from "@/components/access/ObjectACL";
import { JoinInfoCard, JoinPaste } from "@/components/datacenter/ClusterJoin";
import { SupportPanel } from "@/components/datacenter/SupportPanel";
import { MetricChart } from "@/components/charts/MetricChart";
import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard, InventoryEmpty, InventoryRow, joinSummary } from "@/components/ui/inventory";
import { BarButton, MoreMenu } from "@/components/ui/action-bar";
import { Advanced, CollapsedRaw, ItemDetails, KeyValueTable, asRecord, asRecords } from "@/components/ui/item-details";
import { Field, Input, Select } from "@/components/ui/input";
import { ContentTypesField } from "@/components/storage/ContentTypes";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import type { FeatureList, FeatureObject, Pool, Resource, Storage } from "@/types";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { Activity, Bell, FileKey, Globe, HardDrive, Layers, LifeBuoy, Map as MapIcon, ScrollText, Settings, Share2, Workflow } from "lucide-react";
import { memo, useEffect, useMemo, useState, type ReactNode } from "react";
import { Link, useSearch } from "@tanstack/react-router";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";
import { Segmented } from "@/components/ui/tabs";
import { CHART_CPU, CHART_MEM } from "@/lib/chartColors";

const TIMEFRAMES = ["hour", "day", "week", "month", "year"] as const;

const panels = [
  { id: "options", label: "Options", Icon: Settings },
  { id: "storage", label: "Storage", Icon: HardDrive },
  { id: "cluster", label: "Cluster", Icon: Share2 },
  { id: "remotes", label: "Remotes", Icon: Globe },
  { id: "log", label: "Log", Icon: ScrollText },
  { id: "metrics", label: "Metric servers", Icon: Activity },
  { id: "jobs", label: "Jobs", Icon: Workflow },
  { id: "pools", label: "Pools", Icon: Layers },
  { id: "acme", label: "ACME", Icon: FileKey },
  { id: "notifications", label: "Notifications", Icon: Bell },
  { id: "mappings", label: "Mappings", Icon: MapIcon },
  { id: "support", label: "Support", Icon: LifeBuoy },
] as const;
type Panel = (typeof panels)[number]["id"];


function hasSeries(points: Record<string, number>[], key: string) {
  return points.some((p) => Number.isFinite(Number(p[key])));
}

function memToBytes(points: Record<string, number>[]) {
  const max = Math.max(0, ...points.map((p) => Number(p.memused)).filter((n) => Number.isFinite(n)));
  if (max > 1e9) return (v: number) => v;
  if (max > 1e6) return (v: number) => v * 1024;
  return (v: number) => v * 1024 * 1024;
}

type SeriesMode = "sum" | "avg" | "wavg";

function mergePoints(
  series: Record<string, number>[][],
  keys: { key: string; mode: SeriesMode }[],
  weights: number[],
): Record<string, number>[] {
  const byTime = new Map<number, { sums: Record<string, number>; counts: Record<string, number>; weight: Record<string, number> }>();
  series.forEach((points, i) => {
    const w = weights[i] || 1;
    for (const p of points) {
      const t = Number(p.time ?? p.ts ?? 0);
      if (!t) continue;
      let row = byTime.get(t);
      if (!row) {
        row = { sums: {}, counts: {}, weight: {} };
        byTime.set(t, row);
      }
      for (const { key, mode } of keys) {
        const v = Number(p[key]);
        if (!Number.isFinite(v)) continue;
        row.sums[key] = (row.sums[key] ?? 0) + (mode === "wavg" ? v * w : v);
        row.counts[key] = (row.counts[key] ?? 0) + 1;
        row.weight[key] = (row.weight[key] ?? 0) + w;
      }
    }
  });
  return [...byTime.entries()]
    .sort((a, b) => a[0] - b[0])
    .map(([time, row]) => {
      const out: Record<string, number> = { time };
      for (const { key, mode } of keys) {
        if (!row.counts[key]) continue;
        if (mode === "sum") out[key] = row.sums[key];
        else if (mode === "wavg") out[key] = row.weight[key] ? row.sums[key] / row.weight[key] : 0;
        else out[key] = row.sums[key] / row.counts[key];
      }
      return out;
    });
}

export type HistoryNode = { node: string; maxcpu: number };

export const ClusterHistory = memo(function ClusterHistory({
  nodes,
  clusterName,
}: {
  nodes: HistoryNode[];
  clusterName: string;
}) {
  const [tf, setTf] = useState<(typeof TIMEFRAMES)[number]>("hour");
  const names = nodes.map((n) => n.node);
  const weights = nodes.map((n) => n.maxcpu || 1);
  const rrdQueries = useQueries({
    queries: names.map((node) => ({
      queryKey: ["node-rrd", node, tf],
      queryFn: () => api.nodeRrd(node, tf),
      enabled: !!node,
      refetchInterval: 15000,
    })),
  });
  const ioQueries = useQueries({
    queries: names.map((node) => ({
      queryKey: ["node-io-rrd", node, tf],
      queryFn: () => api.nodeIoRrd(node, tf),
      enabled: !!node,
      refetchInterval: 10000,
    })),
  });
  const rrdUpdated = rrdQueries.map((q) => q.dataUpdatedAt).join(",");
  const ioUpdated = ioQueries.map((q) => q.dataUpdatedAt).join(",");
  const points = useMemo(
    () => mergePoints(rrdQueries.map((q) => q.data ?? []), [
      { key: "cpu", mode: "wavg" },
      { key: "memused", mode: "sum" },
      { key: "netin", mode: "sum" },
      { key: "netout", mode: "sum" },
      { key: "iowait", mode: "avg" },
      { key: "wait", mode: "avg" },
    ], weights),
    [rrdUpdated, names.join("|"), weights.join("|")],
  );
  const diskPoints = useMemo(
    () => mergePoints(ioQueries.map((q) => q.data?.disk ?? []), [
      { key: "diskread", mode: "sum" },
      { key: "diskwrite", mode: "sum" },
    ], weights),
    [ioUpdated, names.join("|"), weights.join("|")],
  );
  const iopsPoints = useMemo(
    () => mergePoints(ioQueries.map((q) => q.data?.iops ?? []), [
      { key: "rd_requests", mode: "sum" },
      { key: "wr_requests", mode: "sum" },
    ], weights),
    [ioUpdated, names.join("|"), weights.join("|")],
  );
  const delayKey = hasSeries(points, "iowait") ? "iowait" : hasSeries(points, "wait") ? "wait" : "";

  return (
    <Card className="p-5">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-xs font-medium text-ink-dim">Historical</p>
          <h2 className="mt-1 text-sm font-medium">{clusterName}</h2>
        </div>
        <Segmented items={TIMEFRAMES} value={tf} onChange={setTf} ariaLabel="Time range" />
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <div>
          <p className="mb-2 text-xs font-medium text-ink-dim">CPU</p>
          <MetricChart
            points={points}
            format="percent"
            series={[{ key: "cpu", label: "CPU", color: CHART_CPU, scale: (v) => v * 100 }]}
          />
        </div>
        <div>
          <p className="mb-2 text-xs font-medium text-ink-dim">Memory</p>
          <MetricChart
            points={points}
            format="bytes"
            tipSide="left"
            series={[{ key: "memused", label: "Memory", color: CHART_MEM, scale: memToBytes(points) }]}
          />
        </div>
        <div>
          <p className="mb-2 text-xs font-medium text-ink-dim">Network</p>
          <MetricChart
            points={points}
            format="rate"
            series={[
              { key: "netin", label: "In", color: CHART_MEM },
              { key: "netout", label: "Out", color: CHART_CPU },
            ]}
          />
        </div>
        <div>
          <p className="mb-2 text-xs font-medium text-ink-dim">Disk</p>
          <MetricChart
            points={diskPoints}
            format="rate"
            tipSide="left"
            series={[
              { key: "diskread", label: "Read", color: CHART_MEM },
              { key: "diskwrite", label: "Write", color: CHART_CPU },
            ]}
          />
        </div>
        <div>
          <p className="mb-2 text-xs font-medium text-ink-dim">IOPS</p>
          <MetricChart
            points={iopsPoints}
            format="ops"
            series={[
              { key: "rd_requests", label: "Read", color: CHART_MEM },
              { key: "wr_requests", label: "Write", color: CHART_CPU },
            ]}
          />
        </div>
        <div>
          <p className="mb-2 text-xs font-medium text-ink-dim">IO delay</p>
          <MetricChart
            points={delayKey ? points : []}
            format="percent"
            tipSide="left"
            series={[
              {
                key: delayKey || "iowait",
                label: "IO delay",
                color: CHART_CPU,
                scale: (v) => v * 100,
              },
            ]}
          />
        </div>
      </div>
    </Card>
  );
});

export function DatacenterPage() {
  const notify = useQuery({ queryKey: ["dc-notify-smtp"], queryFn: () => api.notifyEndpoints("smtp") });
  const log = useQuery({ queryKey: ["dc-log-probe"], queryFn: () => api.clusterLog(1) });
  const metrics = useQuery({ queryKey: ["dc-metrics"], queryFn: api.metricServers });
  const jobs = useQuery({ queryKey: ["dc-jobs"], queryFn: api.clusterJobs });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary, refetchInterval: 15000 });
  const search = useSearch({ from: "/app/datacenter" });
  const view = search.tab;

  const notifyOn = notify.data?.supported !== false || notify.isLoading;
  const visiblePanels = panels.filter((p) => {
    if (p.id === "storage") return false;
    if (p.id === "notifications" && !notifyOn) return false;
    if (p.id === "log" && log.data?.supported === false) return false;
    if (p.id === "metrics" && metrics.data?.supported === false) return false;
    if (p.id === "jobs" && jobs.data?.supported === false) return false;
    return true;
  });
  const nodes = (summary.data?.resources ?? [])
    .filter((r) => r.type === "node")
    .sort((a, b) => (a.node ?? "").localeCompare(b.node ?? "", undefined, { numeric: true }));

  return (
    <div className="space-y-5">
      <DatacenterHeader />
      {view === "storage" && (
        <div className="hv-form-stack space-y-4">
          <StorageTab />
        </div>
      )}
      {view === "config" && <ConfigView panels={visiblePanels} section={search.section} nodes={nodes} />}
    </div>
  );
}

function ConfigView({
  panels: items,
  section,
  nodes,
}: {
  panels: readonly (typeof panels)[number][];
  section?: string;
  nodes: Resource[];
}) {
  const current: Panel = items.find((p) => p.id === section)?.id ?? items[0]?.id ?? "options";
  return (
    <div className="grid gap-6 lg:grid-cols-[13rem_minmax(0,1fr)]">
      <nav aria-label="Configuration" className="flex flex-col gap-0.5 lg:sticky lg:top-0 lg:self-start">
        {items.map((p) => (
          <Link
            key={p.id}
            to="/datacenter"
            search={{ tab: "config", section: p.id }}
            aria-current={p.id === current ? "page" : undefined}
            className={`flex h-8 items-center gap-2 rounded-md px-2.5 text-sm ${
              p.id === current ? "bg-raised text-ink" : "text-ink-dim hover:bg-white/[0.04] hover:text-ink"
            }`}
          >
            <p.Icon className="h-3.5 w-3.5 shrink-0" />
            {p.label}
          </Link>
        ))}
      </nav>
      <div className="hv-form-stack min-w-0 space-y-4">
        {current === "options" && <OptionsTab />}
        {current === "cluster" && <ClusterTab />}
        {current === "remotes" && <RemotesTab />}
        {current === "log" && <LogTab />}
        {current === "metrics" && <MetricsTab />}
        {current === "jobs" && <JobsTab />}
        {current === "pools" && <PoolsTab />}
        {current === "acme" && <ACMETab />}
        {current === "notifications" && <NotifyTab />}
        {current === "mappings" && <MappingsTab />}
        {current === "support" && <SupportPanel nodes={nodes} />}
      </div>
    </div>
  );
}

const optionKnown = [
  "description", "keyboard", "language", "tag-style", "next-id", "u2f", "webauthn",
  "email_from", "http_proxy", "max_workers", "mac_prefix", "console", "migration", "ha",
  "bwlimit", "registered-tags", "user-tag-access", "crs", "fencing", "notify",
];

function kvMap(v: unknown): Record<string, string> {
  const obj = asObj(v);
  if (Object.keys(obj).length) {
    return Object.fromEntries(Object.entries(obj).map(([k, val]) => [k, String(val ?? "")]));
  }
  const out: Record<string, string> = {};
  for (const part of String(v ?? "").split(",")) {
    const eq = part.indexOf("=");
    if (eq < 1) continue;
    out[part.slice(0, eq).trim()] = part.slice(eq + 1).trim();
  }
  return out;
}

function joinBits(parts: (string | false | undefined)[]): string {
  return parts.filter(Boolean).join(",");
}

function asObj(v: unknown): Record<string, unknown> {
  if (typeof v === "string") {
    try {
      const parsed = JSON.parse(v);
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) return parsed as Record<string, unknown>;
    } catch {
      return {};
    }
    return {};
  }
  if (v && typeof v === "object" && !Array.isArray(v)) return v as Record<string, unknown>;
  return {};
}

function extraPairs(obj: Record<string, unknown>, known: string[]): string {
  return Object.entries(obj)
    .filter(([k]) => !known.includes(k))
    .map(([k, v]) => (v && typeof v === "object" ? `${k}=${JSON.stringify(v)}` : `${k}=${String(v ?? "")}`))
    .filter((p) => !p.endsWith("="))
    .join(" ");
}

function parsePairs(text: string): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  for (const part of text.trim().split(/\s+/)) {
    const eq = part.indexOf("=");
    if (eq < 1) continue;
    const k = part.slice(0, eq);
    const v = part.slice(eq + 1);
    if (!k || v === "") continue;
    try {
      body[k] = JSON.parse(v);
    } catch {
      body[k] = v;
    }
  }
  return body;
}

function OptionsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const opts = useQuery({ queryKey: ["dc-opts"], queryFn: api.dcOptions });
  const [keyboard, setKeyboard] = useState("");
  const [language, setLanguage] = useState("");
  const [tagShape, setTagShape] = useState("");
  const [tagOrdering, setTagOrdering] = useState("");
  const [tagColors, setTagColors] = useState("");
  const [nextStart, setNextStart] = useState("");
  const [nextEnd, setNextEnd] = useState("");
  const [u2fAppid, setU2fAppid] = useState("");
  const [u2fOrigin, setU2fOrigin] = useState("");
  const [waRp, setWaRp] = useState("");
  const [waOrigin, setWaOrigin] = useState("");
  const [waId, setWaId] = useState("");
  const [emailFrom, setEmailFrom] = useState("");
  const [httpProxy, setHttpProxy] = useState("");
  const [maxWorkers, setMaxWorkers] = useState("");
  const [macPrefix, setMacPrefix] = useState("");
  const [consoleView, setConsoleView] = useState("");
  const [migType, setMigType] = useState("");
  const [migNet, setMigNet] = useState("");
  const [haPolicy, setHaPolicy] = useState("");
  const [bwDefault, setBwDefault] = useState("");
  const [bwMig, setBwMig] = useState("");
  const [bwRestore, setBwRestore] = useState("");
  const [bwClone, setBwClone] = useState("");
  const [bwMove, setBwMove] = useState("");
  const [regTags, setRegTags] = useState("");
  const [userAllow, setUserAllow] = useState("");
  const [userAllowList, setUserAllowList] = useState("");
  const [crsHa, setCrsHa] = useState("");
  const [description, setDescription] = useState("");
  const [fencing, setFencing] = useState("");
  const [notify, setNotify] = useState("");
  const [extra, setExtra] = useState("");
  const cpus = useQuery({ queryKey: ["dc-cpus"], queryFn: api.cpuModels });
  const [cpuName, setCpuName] = useState("");
  const [cpuModel, setCpuModel] = useState("");
  const [cpuFlags, setCpuFlags] = useState("");
  const [editCPU, setEditCPU] = useState<Record<string, unknown> | null>(null);

  useEffect(() => {
    if (!opts.data) return;
    const src = opts.data;
    const tags = asObj(src["tag-style"]);
    const next = asObj(src["next-id"]);
    const u2f = asObj(src.u2f);
    const wa = asObj(src.webauthn);
    setKeyboard(String(src.keyboard ?? ""));
    setLanguage(String(src.language ?? ""));
    setTagShape(String(tags.shape ?? tags["tag-style"] ?? ""));
    setTagOrdering(String(tags.ordering ?? ""));
    setTagColors(String(tags["color-map"] ?? tags.color ?? ""));
    setNextStart(String(next.start ?? ""));
    setNextEnd(String(next.end ?? ""));
    setU2fAppid(String(u2f.appid ?? ""));
    setU2fOrigin(String(u2f.origin ?? ""));
    setWaRp(String(wa.rp ?? ""));
    setWaOrigin(String(wa.origin ?? ""));
    setWaId(String(wa.id ?? ""));
    setEmailFrom(String(src.email_from ?? ""));
    setHttpProxy(String(src.http_proxy ?? ""));
    setMaxWorkers(String(src.max_workers ?? ""));
    setMacPrefix(String(src.mac_prefix ?? ""));
    setConsoleView(String(src.console ?? ""));
    const mig = kvMap(src.migration);
    setMigType(String(mig.type ?? ""));
    setMigNet(String(mig.network ?? ""));
    const ha = kvMap(src.ha);
    setHaPolicy(String(ha.shutdown_policy ?? ""));
    const bw = kvMap(src.bwlimit);
    setBwDefault(String(bw.default ?? ""));
    setBwMig(String(bw.migration ?? ""));
    setBwRestore(String(bw.restore ?? ""));
    setBwClone(String(bw.clone ?? ""));
    setBwMove(String(bw.move ?? ""));
    setRegTags(String(src["registered-tags"] ?? ""));
    const userTags = kvMap(src["user-tag-access"]);
    setUserAllow(String(userTags["user-allow"] ?? ""));
    setUserAllowList(String(userTags["user-allow-list"] ?? ""));
    const crs = kvMap(src.crs);
    setCrsHa(String(crs.ha ?? ""));
    setDescription(String(src.description ?? ""));
    setFencing(String(src.fencing ?? ""));
    setNotify(String(src.notify ?? ""));
    setExtra(extraPairs(src, optionKnown));
  }, [opts.data]);

  return (
    <div className="space-y-4">
      <Card className="space-y-3 p-4">
        <h2 className="font-medium">Options</h2>
        <div className="grid gap-3 md:grid-cols-2">
          <Field label="Description"><Input value={description} onChange={(e) => setDescription(e.target.value)} disabled={!writable} /></Field>
          <Field label="Keyboard"><Input value={keyboard} onChange={(e) => setKeyboard(e.target.value)} placeholder="en-us" disabled={!writable} /></Field>
          <Field label="Language"><Input value={language} onChange={(e) => setLanguage(e.target.value)} placeholder="en" disabled={!writable} /></Field>
          <Field label="Tag style">
            <Select value={tagShape} onChange={(e) => setTagShape(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="full">full</option>
              <option value="circle">circle</option>
              <option value="dense">dense</option>
            </Select>
          </Field>
          <Field label="Tag ordering">
            <Select value={tagOrdering} onChange={(e) => setTagOrdering(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="config">config</option>
              <option value="alphabetical">alphabetical</option>
            </Select>
          </Field>
          <Field label="Tag colors"><Input value={tagColors} onChange={(e) => setTagColors(e.target.value)} placeholder="prod:FF0000;dev:00AA00" disabled={!writable} /></Field>
          <Field label="Next ID start"><Input value={nextStart} onChange={(e) => setNextStart(e.target.value)} placeholder="100" disabled={!writable} /></Field>
          <Field label="Next ID end"><Input value={nextEnd} onChange={(e) => setNextEnd(e.target.value)} placeholder="999999999" disabled={!writable} /></Field>
          <Field label="U2F appid"><Input value={u2fAppid} onChange={(e) => setU2fAppid(e.target.value)} disabled={!writable} /></Field>
          <Field label="U2F origin"><Input value={u2fOrigin} onChange={(e) => setU2fOrigin(e.target.value)} disabled={!writable} /></Field>
          <Field label="WebAuthn RP"><Input value={waRp} onChange={(e) => setWaRp(e.target.value)} disabled={!writable} /></Field>
          <Field label="WebAuthn origin"><Input value={waOrigin} onChange={(e) => setWaOrigin(e.target.value)} disabled={!writable} /></Field>
          <Field label="WebAuthn id"><Input value={waId} onChange={(e) => setWaId(e.target.value)} disabled={!writable} /></Field>
          <Field label="Email from"><Input value={emailFrom} onChange={(e) => setEmailFrom(e.target.value)} placeholder="root@pve" disabled={!writable} /></Field>
          <Field label="HTTP proxy"><Input value={httpProxy} onChange={(e) => setHttpProxy(e.target.value)} placeholder="http://proxy:3128" disabled={!writable} /></Field>
          <Field label="Max workers"><Input value={maxWorkers} onChange={(e) => setMaxWorkers(e.target.value)} disabled={!writable} /></Field>
          <Field label="MAC prefix"><Input value={macPrefix} onChange={(e) => setMacPrefix(e.target.value)} placeholder="BC:24:11" disabled={!writable} /></Field>
          <Field label="Console">
            <Select value={consoleView} onChange={(e) => setConsoleView(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="html5">html5</option>
              <option value="xtermjs">xtermjs</option>
              <option value="vv">vv (SPICE)</option>
              <option value="applet">applet</option>
            </Select>
          </Field>
          <Field label="Migration type">
            <Select value={migType} onChange={(e) => setMigType(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="secure">secure</option>
              <option value="insecure">insecure</option>
            </Select>
          </Field>
          <Field label="Migration network"><Input value={migNet} onChange={(e) => setMigNet(e.target.value)} placeholder="10.0.0.0/24" disabled={!writable} /></Field>
          <Field label="HA shutdown policy">
            <Select value={haPolicy} onChange={(e) => setHaPolicy(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="conditional">conditional</option>
              <option value="migrate">migrate</option>
              <option value="failover">failover</option>
              <option value="freeze">freeze</option>
            </Select>
          </Field>
          <Field label="bwlimit default"><Input value={bwDefault} onChange={(e) => setBwDefault(e.target.value)} placeholder="KiB/s" disabled={!writable} /></Field>
          <Field label="bwlimit migration"><Input value={bwMig} onChange={(e) => setBwMig(e.target.value)} disabled={!writable} /></Field>
          <Field label="bwlimit restore"><Input value={bwRestore} onChange={(e) => setBwRestore(e.target.value)} disabled={!writable} /></Field>
          <Field label="bwlimit clone"><Input value={bwClone} onChange={(e) => setBwClone(e.target.value)} disabled={!writable} /></Field>
          <Field label="bwlimit move"><Input value={bwMove} onChange={(e) => setBwMove(e.target.value)} disabled={!writable} /></Field>
          <Field label="Registered tags"><Input value={regTags} onChange={(e) => setRegTags(e.target.value)} placeholder="prod;pci" disabled={!writable} /></Field>
          <Field label="User tag access">
            <Select value={userAllow} onChange={(e) => setUserAllow(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="free">free</option>
              <option value="existing">existing</option>
              <option value="list">list</option>
              <option value="none">none</option>
            </Select>
          </Field>
          <Field label="User allow list"><Input value={userAllowList} onChange={(e) => setUserAllowList(e.target.value)} placeholder="dev;lab" disabled={!writable} /></Field>
          <Field label="CRS HA">
            <Select value={crsHa} onChange={(e) => setCrsHa(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="basic">basic</option>
              <option value="static">static</option>
              <option value="dynamic">dynamic</option>
            </Select>
          </Field>
          <Field label="Fencing">
            <Select value={fencing} onChange={(e) => setFencing(e.target.value)} disabled={!writable}>
              <option value="">Default</option>
              <option value="hardware">hardware</option>
              <option value="watchdog">watchdog</option>
              <option value="both">both</option>
            </Select>
          </Field>
          <Field label="Notify"><Input value={notify} onChange={(e) => setNotify(e.target.value)} placeholder="fencing=always,replication=always" disabled={!writable} /></Field>
        </div>
        <Advanced label="Advanced: extra keys" defaultOpen={!!extra}>
          <Field label="Extra keys">
            <Input value={extra} onChange={(e) => setExtra(e.target.value)} placeholder="key=value leftover keys" disabled={!writable} />
          </Field>
        </Advanced>
        {writable && (
          <Button size="sm" onClick={async () => {
            const body: Record<string, unknown> = { ...parsePairs(extra) };
            if (keyboard) body.keyboard = keyboard;
            if (language) body.language = language;
            const tagBits = [
              tagShape && `shape=${tagShape}`,
              tagOrdering && `ordering=${tagOrdering}`,
              tagColors && `color-map=${tagColors}`,
            ].filter(Boolean);
            if (tagBits.length) body["tag-style"] = tagBits.join(",");
            const nextBits = [
              nextStart && `start=${nextStart}`,
              nextEnd && `end=${nextEnd}`,
            ].filter(Boolean);
            if (nextBits.length) body["next-id"] = nextBits.join(",");
            if (u2fAppid || u2fOrigin) body.u2f = JSON.stringify({ ...(u2fAppid ? { appid: u2fAppid } : {}), ...(u2fOrigin ? { origin: u2fOrigin } : {}) });
            if (waRp || waOrigin || waId) {
              body.webauthn = JSON.stringify({ ...(waRp ? { rp: waRp } : {}), ...(waOrigin ? { origin: waOrigin } : {}), ...(waId ? { id: waId } : {}) });
            }
            if (emailFrom) body.email_from = emailFrom;
            if (httpProxy) body.http_proxy = httpProxy;
            if (maxWorkers) body.max_workers = maxWorkers;
            if (macPrefix) body.mac_prefix = macPrefix;
            if (consoleView) body.console = consoleView;
            const migBits = joinBits([migType && `type=${migType}`, migNet && `network=${migNet}`]);
            if (migBits) body.migration = migBits;
            if (haPolicy) body.ha = `shutdown_policy=${haPolicy}`;
            const bwBits = joinBits([
              bwDefault && `default=${bwDefault}`,
              bwMig && `migration=${bwMig}`,
              bwRestore && `restore=${bwRestore}`,
              bwClone && `clone=${bwClone}`,
              bwMove && `move=${bwMove}`,
            ]);
            if (bwBits) body.bwlimit = bwBits;
            if (regTags) body["registered-tags"] = regTags;
            const userBits = joinBits([userAllow && `user-allow=${userAllow}`, userAllowList && `user-allow-list=${userAllowList}`]);
            if (userBits) body["user-tag-access"] = userBits;
            if (crsHa) body.crs = `ha=${crsHa}`;
            if (description) body.description = description;
            if (fencing) body.fencing = fencing;
            if (notify) body.notify = notify;
            await api.updateDCOptions(body);
            void qc.invalidateQueries({ queryKey: ["dc-opts"] });
          }}>Save options</Button>
        )}
      </Card>
      {cpus.data?.supported !== false && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Custom CPU models</h2>
          {writable && (
            <div className="flex flex-wrap gap-2">
              <Input className="max-w-xs" placeholder="name" value={cpuName} onChange={(e) => setCpuName(e.target.value)} />
              <Input className="max-w-xs" placeholder="reported-model" value={cpuModel} onChange={(e) => setCpuModel(e.target.value)} />
              <Input className="max-w-sm" placeholder="flags (+aes;-ssbd)" value={cpuFlags} onChange={(e) => setCpuFlags(e.target.value)} />
              <Button size="sm" onClick={async () => {
                const body: Record<string, unknown> = { name: cpuName, cputype: cpuName };
                if (cpuModel) body["reported-model"] = cpuModel;
                if (cpuFlags) body.flags = cpuFlags;
                await api.createCPUModel(body);
                setCpuName("");
                setCpuModel("");
                setCpuFlags("");
                void qc.invalidateQueries({ queryKey: ["dc-cpus"] });
              }}>Add</Button>
            </div>
          )}
          <FeatureTable
            list={cpus.data}
            idKey="name"
            writable={writable}
            detailKeys={["reported-model", "flags", "comment"]}
            onEdit={(item) => {
              setEditCPU(item);
              setCpuModel(String(item["reported-model"] ?? ""));
              setCpuFlags(String(item.flags ?? ""));
            }}
            onDelete={async (id) => {
              await api.deleteCPUModel(id);
              void qc.invalidateQueries({ queryKey: ["dc-cpus"] });
            }}
          />
          {editCPU && (
            <Modal
              title={`Edit ${String(editCPU.name ?? "")}`}
              onClose={() => setEditCPU(null)}
              footer={
                <Button onClick={async () => {
                  const body: Record<string, unknown> = {};
                  if (cpuModel) body["reported-model"] = cpuModel;
                  if (cpuFlags) body.flags = cpuFlags;
                  await api.updateCPUModel(String(editCPU.name ?? ""), body);
                  setEditCPU(null);
                  void qc.invalidateQueries({ queryKey: ["dc-cpus"] });
                }}>Save</Button>
              }
            >
              <Field label="Reported model"><Input value={cpuModel} onChange={(e) => setCpuModel(e.target.value)} /></Field>
              <Field label="Flags"><Input value={cpuFlags} onChange={(e) => setCpuFlags(e.target.value)} placeholder="+aes;-ssbd" /></Field>
            </Modal>
          )}
        </Card>
      )}
    </div>
  );
}

function LogTab() {
  const can = useCan();
  const canRead = can("/", "Sys.Audit") || can("/", "Sys.Modify");
  const [max, setMax] = useState("80");
  const [filter, setFilter] = useState("");
  const log = useQuery({
    queryKey: ["dc-log", max],
    queryFn: () => api.clusterLog(Number(max) || 80),
    enabled: canRead,
  });
  if (!canRead) {
    return <p className="text-sm text-ink-dim">Sys.Audit is required to read the cluster log.</p>;
  }
  if (log.data && !log.data.supported) {
    return <p className="text-sm text-ink-dim">Cluster log is not available.</p>;
  }
  const needle = filter.trim().toLowerCase();
  const lines = (log.data?.items ?? [])
    .map((item) => {
      const time = String(item.time ?? item.uid ?? "");
      const msg = String(item.msg ?? item.message ?? JSON.stringify(item));
      const user = String(item.user ?? "");
      return `${time}  ${user}  ${msg}`;
    })
    .filter((line) => !needle || line.toLowerCase().includes(needle));
  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-end gap-2 px-4 pt-4">
        <h2 className="mr-auto font-medium">Cluster log</h2>
        <Field label="Max">
          <Input className="w-24" value={max} onChange={(e) => setMax(e.target.value)} />
        </Field>
        <Field label="Filter">
          <Input className="w-56" value={filter} onChange={(e) => setFilter(e.target.value)} placeholder="user or message" />
        </Field>
      </div>
      <pre className="max-h-[32rem] overflow-auto p-4 font-mono text-xs text-ink-dim">
        {lines.join("\n") || "No log lines."}
      </pre>
    </Card>
  );
}

function MetricsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const list = useQuery({ queryKey: ["dc-metrics"], queryFn: api.metricServers });
  const [id, setId] = useState("");
  const [type, setType] = useState("influxdb");
  const [server, setServer] = useState("");
  const [port, setPort] = useState("");
  const [proto, setProto] = useState("udp");
  const [bucket, setBucket] = useState("");
  const [org, setOrg] = useState("");
  const [token, setToken] = useState("");
  const [path, setPath] = useState("");
  const [verify, setVerify] = useState(true);
  const [edit, setEdit] = useState<Record<string, unknown> | null>(null);

  function metricBody(includeId?: string): Record<string, unknown> {
    const body: Record<string, unknown> = { type, server };
    if (includeId) body.id = includeId;
    if (port) body.port = port;
    if (type === "influxdb") {
      body.influxdbproto = proto;
      if (bucket) body.bucket = bucket;
      if (org) body.organization = org;
      if (token) body.token = token;
      if (!verify) body["verify-certificate"] = 0;
    } else {
      body.proto = proto;
      if (path) body.path = path;
    }
    return body;
  }

  function loadMetric(item: Record<string, unknown>) {
    const kind = String(item.type ?? "influxdb");
    setType(kind);
    setServer(String(item.server ?? ""));
    setPort(String(item.port ?? ""));
    setProto(String(item.influxdbproto ?? item.proto ?? (kind === "graphite" ? "udp" : "udp")));
    setBucket(String(item.bucket ?? ""));
    setOrg(String(item.organization ?? ""));
    setToken(String(item.token ?? ""));
    setPath(String(item.path ?? ""));
    setVerify(item["verify-certificate"] !== 0 && item["verify-certificate"] !== "0");
    setEdit(item);
  }

  if (list.data && !list.data.supported) {
    return <p className="text-sm text-ink-dim">Metric servers are not available on this cluster.</p>;
  }
  return (
    <div className="space-y-4">
      {writable && (
        <div className="flex flex-wrap gap-2">
          <Input className="max-w-xs" placeholder="id" value={id} onChange={(e) => setId(e.target.value)} />
          <Select className="max-w-[10rem]" value={type} onChange={(e) => setType(e.target.value)}>
            <option value="influxdb">influxdb</option>
            <option value="graphite">graphite</option>
          </Select>
          <Input className="max-w-xs" placeholder="server" value={server} onChange={(e) => setServer(e.target.value)} />
          <Input className="max-w-[8rem]" placeholder="port" value={port} onChange={(e) => setPort(e.target.value)} />
          {type === "influxdb" && (
            <>
              <Select className="max-w-[8rem]" value={proto} onChange={(e) => setProto(e.target.value)}>
                <option value="udp">udp</option>
                <option value="http">http</option>
                <option value="https">https</option>
              </Select>
              <Input className="max-w-xs" placeholder="bucket" value={bucket} onChange={(e) => setBucket(e.target.value)} />
              <Input className="max-w-xs" placeholder="organization" value={org} onChange={(e) => setOrg(e.target.value)} />
              <Input className="max-w-xs" placeholder="token" value={token} onChange={(e) => setToken(e.target.value)} />
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={verify} onChange={(e) => setVerify(e.target.checked)} />
                Verify TLS
              </label>
            </>
          )}
          {type === "graphite" && (
            <>
              <Select className="max-w-[8rem]" value={proto} onChange={(e) => setProto(e.target.value)}>
                <option value="udp">udp</option>
                <option value="tcp">tcp</option>
              </Select>
              <Input className="max-w-xs" placeholder="path" value={path} onChange={(e) => setPath(e.target.value)} />
            </>
          )}
          <Button size="sm" onClick={async () => {
            await api.createMetricServer(metricBody(id));
            setId("");
            void qc.invalidateQueries({ queryKey: ["dc-metrics"] });
          }}>Add</Button>
        </div>
      )}
      <FeatureTable
        list={list.data}
        idKey="id"
        writable={writable}
        detailKeys={["type", "server", "port", "disable"]}
        onEdit={loadMetric}
        onDelete={async (name) => {
          await api.deleteMetricServer(name);
          void qc.invalidateQueries({ queryKey: ["dc-metrics"] });
        }}
      />
      {edit && (
        <Modal
          title={`Edit ${String(edit.id ?? "")}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              await api.updateMetricServer(String(edit.id ?? ""), metricBody());
              setEdit(null);
              void qc.invalidateQueries({ queryKey: ["dc-metrics"] });
            }}>Save</Button>
          }
        >
          <Field label="Type">
            <Select value={type} onChange={(e) => setType(e.target.value)}>
              <option value="influxdb">influxdb</option>
              <option value="graphite">graphite</option>
            </Select>
          </Field>
          <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
          <Field label="Port"><Input value={port} onChange={(e) => setPort(e.target.value)} /></Field>
          {type === "influxdb" && (
            <>
              <Field label="Protocol">
                <Select value={proto} onChange={(e) => setProto(e.target.value)}>
                  <option value="udp">udp</option>
                  <option value="http">http</option>
                  <option value="https">https</option>
                </Select>
              </Field>
              <Field label="Bucket"><Input value={bucket} onChange={(e) => setBucket(e.target.value)} /></Field>
              <Field label="Organization"><Input value={org} onChange={(e) => setOrg(e.target.value)} /></Field>
              <Field label="Token"><Input value={token} onChange={(e) => setToken(e.target.value)} /></Field>
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={verify} onChange={(e) => setVerify(e.target.checked)} />
                Verify TLS
              </label>
            </>
          )}
          {type === "graphite" && (
            <>
              <Field label="Protocol">
                <Select value={proto} onChange={(e) => setProto(e.target.value)}>
                  <option value="udp">udp</option>
                  <option value="tcp">tcp</option>
                </Select>
              </Field>
              <Field label="Path"><Input value={path} onChange={(e) => setPath(e.target.value)} /></Field>
            </>
          )}
        </Modal>
      )}
    </div>
  );
}

function JobsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const jobs = useQuery({ queryKey: ["dc-realm-jobs"], queryFn: api.realmSyncJobs });
  const [adding, setAdding] = useState(false);
  const [id, setId] = useState("");
  const [realm, setRealm] = useState("");
  const [schedule, setSchedule] = useState("0 3 * * *");
  const [analyze, setAnalyze] = useState<FeatureList | null>(null);
  const [analyzeMsg, setAnalyzeMsg] = useState("");
  const [editJob, setEditJob] = useState<Record<string, unknown> | null>(null);
  const [editEnabled, setEditEnabled] = useState(true);
  if (jobs.data && !jobs.data.supported) {
    return <p className="text-sm text-ink-dim">Cluster jobs are not available on this cluster.</p>;
  }

  async function analyzeSchedule() {
    const res = await api.scheduleAnalyze(schedule);
    if (!res.supported) {
      setAnalyzeMsg("Schedule analyze is not available.");
      setAnalyze(null);
      return;
    }
    setAnalyzeMsg("");
    setAnalyze(res);
  }

  return (
    <div className="space-y-4">
      <FeatureTable
        title="Realm-sync"
        actions={writable ? <BarButton onClick={() => {
          setId("");
          setRealm("");
          setSchedule("0 3 * * *");
          setAnalyze(null);
          setAnalyzeMsg("");
          setAdding(true);
        }}>Add</BarButton> : undefined}
        list={jobs.data}
        idKey="id"
        writable={writable}
        detailKeys={["realm", "schedule", "enabled", "comment"]}
        onEdit={(item) => {
          setEditJob(item);
          setRealm(String(item.realm ?? ""));
          setSchedule(String(item.schedule ?? ""));
          setEditEnabled(item.enabled !== 0 && item.enabled !== "0");
          setAnalyze(null);
          setAnalyzeMsg("");
        }}
        onDelete={async (name) => {
          await api.deleteRealmSyncJob(name);
          void qc.invalidateQueries({ queryKey: ["dc-realm-jobs"] });
        }}
      />
      {adding && (
        <Modal
          title="Add realm-sync"
          onClose={() => setAdding(false)}
          footer={
            <>
              <Button variant="outline" onClick={() => void analyzeSchedule()}>Analyze schedule</Button>
              <Button onClick={async () => {
                await api.createRealmSyncJob({ id, realm, schedule, enabled: 1 });
                setId("");
                setAdding(false);
                void qc.invalidateQueries({ queryKey: ["dc-realm-jobs"] });
              }}>Save</Button>
            </>
          }
        >
          <Field label="Job id"><Input value={id} onChange={(e) => setId(e.target.value)} /></Field>
          <Field label="Realm"><Input value={realm} onChange={(e) => setRealm(e.target.value)} /></Field>
          <Field label="Schedule"><Input value={schedule} onChange={(e) => setSchedule(e.target.value)} /></Field>
          {analyzeMsg && <p className="text-sm text-ink-dim">{analyzeMsg}</p>}
          {analyze && <ScheduleAnalyzeTable items={analyze.items} />}
        </Modal>
      )}
      {editJob && (
        <Modal
          title={`Edit ${String(editJob.id ?? "")}`}
          onClose={() => setEditJob(null)}
          footer={
            <>
              <Button variant="outline" onClick={() => void analyzeSchedule()}>Analyze schedule</Button>
              <Button onClick={async () => {
                await api.updateRealmSyncJob(String(editJob.id ?? ""), { realm, schedule, enabled: editEnabled ? 1 : 0 });
                setEditJob(null);
                void qc.invalidateQueries({ queryKey: ["dc-realm-jobs"] });
              }}>Save</Button>
            </>
          }
        >
          <Field label="Realm"><Input value={realm} onChange={(e) => setRealm(e.target.value)} /></Field>
          <Field label="Schedule"><Input value={schedule} onChange={(e) => setSchedule(e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={editEnabled} onChange={(e) => setEditEnabled(e.target.checked)} />
            Enabled
          </label>
          {analyzeMsg && <p className="text-sm text-ink-dim">{analyzeMsg}</p>}
          {analyze && <ScheduleAnalyzeTable items={analyze.items} />}
        </Modal>
      )}
    </div>
  );
}

function PoolsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Pool.Allocate") || can("/", "Sys.Modify");
  const pools = useQuery({ queryKey: ["dc-pools"], queryFn: api.dcPools });
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [comment, setComment] = useState("");
  const [open, setOpen] = useState<string | null>(null);
  const [del, setDel] = useState<Pool | null>(null);
  const [vms, setVms] = useState<Set<number>>(new Set());
  const [stores, setStores] = useState<Set<string>>(new Set());
  const detail = useQuery({ queryKey: ["dc-pool", open], queryFn: () => api.dcPool(open!), enabled: !!open });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const guests = (summary.data?.resources ?? []).filter((r) => r.type === "qemu" || r.type === "lxc");
  const storages = (summary.data?.resources ?? []).filter((r) => r.type === "storage");

  useEffect(() => {
    const members = (detail.data?.members ?? []) as Record<string, unknown>[];
    setVms(new Set(members.map((m) => Number(m.vmid)).filter((n) => n > 0)));
    setStores(new Set(members.map((m) => String(m.storage ?? "")).filter(Boolean)));
  }, [detail.data]);

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Pools"
        actions={writable ? <BarButton onClick={() => { setName(""); setComment(""); setAdding(true); }}>Add</BarButton> : undefined}
      >
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {!(pools.data ?? []).length && <InventoryEmpty>No pools</InventoryEmpty>}
            {(pools.data ?? []).map((p) => (
              <InventoryRow key={p.poolid} kind={p.poolid} summary={p.comment || "—"}>
                <Button size="sm" variant="outline" onClick={() => setOpen(p.poolid)}>Members</Button>
                {writable && <Button size="sm" variant="danger" onClick={() => setDel(p)}>Delete</Button>}
              </InventoryRow>
            ))}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <Modal
          title="Add pool"
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              await api.createDCPool({ poolid: name, comment });
              setName("");
              setComment("");
              setAdding(false);
              void qc.invalidateQueries({ queryKey: ["dc-pools"] });
            }}>Save</Button>
          }
        >
          <Field label="Pool id"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
        </Modal>
      )}
      {open && (
        <Modal
          title={`Members of ${open}`}
          onClose={() => setOpen(null)}
          size="xl"
          footer={writable ? (
            <Button onClick={async () => {
              await api.updateDCPool(open, {
                vms: [...vms].sort((a, b) => a - b).join(","),
                storage: [...stores].sort().join(","),
              });
              void qc.invalidateQueries({ queryKey: ["dc-pool", open] });
            }}>Save</Button>
          ) : undefined}
        >
          <div className="grid gap-4 md:grid-cols-2">
            <div>
              <p className="mb-2 text-xs uppercase text-ink-dim">Guests</p>
              <div className="max-h-56 space-y-1 overflow-auto text-sm">
                {guests.map((g) => {
                  const id = g.vmid ?? 0;
                  return (
                    <label key={g.id} className="flex items-center gap-2 text-ink">
                      <input
                        type="checkbox"
                        checked={vms.has(id)}
                        disabled={!writable}
                        onChange={(e) => {
                          setVms((prev) => {
                            const next = new Set(prev);
                            if (e.target.checked) next.add(id);
                            else next.delete(id);
                            return next;
                          });
                        }}
                      />
                      {id} {g.name || g.type}
                    </label>
                  );
                })}
                {!guests.length && <p className="text-ink-dim">No guests.</p>}
              </div>
            </div>
            <div>
              <p className="mb-2 text-xs uppercase text-ink-dim">Storage</p>
              <div className="max-h-56 space-y-1 overflow-auto text-sm">
                {storages.map((s) => {
                  const storeName = s.storage || s.id;
                  return (
                    <label key={s.id} className="flex items-center gap-2 text-ink">
                      <input
                        type="checkbox"
                        checked={stores.has(storeName)}
                        disabled={!writable}
                        onChange={(e) => {
                          setStores((prev) => {
                            const next = new Set(prev);
                            if (e.target.checked) next.add(storeName);
                            else next.delete(storeName);
                            return next;
                          });
                        }}
                      />
                      {storeName}
                    </label>
                  );
                })}
                {!storages.length && <p className="text-ink-dim">No storage.</p>}
              </div>
            </div>
          </div>
          <ObjectACL path={`/pool/${open}`} />
        </Modal>
      )}
      {del && (
        <ConfirmDialog
          title={`Delete pool ${del.poolid}?`}
          body="Removes the pool. Guests are not deleted."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteDCPool(del.poolid);
            setDel(null);
            if (open === del.poolid) setOpen(null);
            void qc.invalidateQueries({ queryKey: ["dc-pools"] });
          }}
        />
      )}
    </div>
  );
}

function StorageTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/storage", "Datastore.Allocate") || can("/storage", "Sys.Modify") || can("/", "Sys.Modify");
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const nodes = (summary.data?.resources ?? []).filter((r) => r.type === "node").map((r) => r.node).filter(Boolean) as string[];
  const [wizard, setWizard] = useState<StorageWizardKind | null>(null);
  const [edit, setEdit] = useState<Storage | null>(null);
  const [del, setDel] = useState<Storage | null>(null);
  const [aclFor, setAclFor] = useState<string | null>(null);
  const [error, setError] = useState("");
  const canACL = can("/access", "Permissions.Modify") || can("/access", "Sys.Audit") || can("/", "Permissions.Modify");

  const kinds = ["pbs", "nfs", "cifs", "iscsi", "rbd", "cephfs", "zfs-iscsi", "dir", "lvm", "lvmthin", "zfspool", "btrfs", "esxi", "glusterfs"] as const;
  return (
    <div className="space-y-4">
      {error && <p className="text-sm text-rose-400">{error}</p>}
      <InventoryCard
        title="Storage"
        actions={writable ? (
          <MoreMenu
            label="Add"
            items={kinds.map((k) => ({
              id: k,
              label: wizardLabel(k),
              onClick: () => { setWizard(k); setError(""); },
            }))}
          />
        ) : undefined}
      >
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(storages.data ?? []).map((s) => {
              const summary = [s.type, s.content, s.disable ? "disabled" : "enabled"].filter(Boolean).join(" · ");
              return (
                <tr
                  key={s.storage}
                  className={`border-t border-line ${writable ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
                  onClick={writable ? () => setEdit(s) : undefined}
                >
                  <td className="px-4 py-2 font-medium">{s.storage}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    {canACL && <Button size="sm" variant="outline" onClick={() => setAclFor(s.storage)}>Permissions</Button>}
                    {writable && (
                      <>
                        <Button size="sm" variant="outline" onClick={async () => {
                          await api.updateStorage(s.storage, { disable: s.disable ? 0 : 1 });
                          void qc.invalidateQueries({ queryKey: ["storages"] });
                        }}>{s.disable ? "Enable" : "Disable"}</Button>
                        <Button size="sm" variant="outline" onClick={() => setEdit(s)}>Edit</Button>
                        <Button size="sm" variant="danger" onClick={() => setDel(s)}>Delete</Button>
                      </>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {wizard && (
        <StorageWizard
          kind={wizard}
          nodes={nodes}
          onClose={() => setWizard(null)}
          onError={setError}
          onDone={() => { setWizard(null); void qc.invalidateQueries({ queryKey: ["storages"] }); }}
        />
      )}
      {edit && (
        <StorageEdit
          item={edit}
          onClose={() => setEdit(null)}
          onDone={() => { setEdit(null); void qc.invalidateQueries({ queryKey: ["storages"] }); }}
        />
      )}
      {aclFor && (
        <Card className="p-4">
          <div className="mb-2 flex justify-end">
            <Button size="sm" variant="ghost" onClick={() => setAclFor(null)}>Close</Button>
          </div>
          <ObjectACL path={`/storage/${aclFor}`} />
        </Card>
      )}
      {del && (
        <ConfirmDialog
          title={`Delete storage ${del.storage}?`}
          body="Removes the storage definition. Content on the backend is not deleted."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteStorage(del.storage);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["storages"] });
          }}
        />
      )}
    </div>
  );
}

type StorageWizardKind = "pbs" | "nfs" | "cifs" | "iscsi" | "rbd" | "cephfs" | "zfs-iscsi" | "dir" | "lvm" | "lvmthin" | "zfspool" | "btrfs" | "esxi" | "glusterfs";

function wizardLabel(kind: StorageWizardKind | string) {
  if (kind === "zfs-iscsi") return "ZFS-iSCSI";
  if (kind === "zfspool") return "ZFS pool";
  if (kind === "lvmthin") return "LVM-thin";
  if (kind === "cephfs") return "CephFS";
  if (kind === "esxi") return "ESXi";
  if (kind === "glusterfs") return "GlusterFS";
  return kind.toUpperCase();
}

function StorageWizard({
  kind, nodes, onClose, onDone, onError,
}: {
  kind: StorageWizardKind;
  nodes: string[];
  onClose: () => void;
  onDone: () => void;
  onError: (msg: string) => void;
}) {
  const [node, setNode] = useState(nodes[0] ?? "");
  const [id, setId] = useState("");
  const [server, setServer] = useState("");
  const [datastore, setDatastore] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [fingerprint, setFingerprint] = useState("");
  const [exportPath, setExportPath] = useState("");
  const [share, setShare] = useState("");
  const [domain, setDomain] = useState("");
  const [portal, setPortal] = useState("");
  const [target, setTarget] = useState("");
  const [iscsiType, setIscsiType] = useState("iscsi");
  const [pool, setPool] = useState("");
  const [monhost, setMonhost] = useState("");
  const [namespace, setNamespace] = useState("");
  const [pbsNamespace, setPbsNamespace] = useState("");
  const [fsName, setFsName] = useState("");
  const [content, setContent] = useState(kind === "pbs" ? "backup" : "images");
  const [path, setPath] = useState("");
  const [vgname, setVgname] = useState("");
  const [thinpool, setThinpool] = useState("");
  const [shared, setShared] = useState(kind === "dir");
  const [skipCert, setSkipCert] = useState(false);
  const [volume, setVolume] = useState("");
  const [transport, setTransport] = useState("tcp");
  const [restrictNodes, setRestrictNodes] = useState("");
  const [extra, setExtra] = useState("");
  const [scan, setScan] = useState<FeatureList | null>(null);

  async function runScan(scanKind: "nfs" | "cifs" | "iscsi" | "lvm" | "lvmthin" | "zfs" | "pbs", query: Record<string, string>) {
    try {
      const res = await api.scanStorage(scanKind, node, query);
      setScan(res);
      if (res.supported === false) onError(`${scanKind.toUpperCase()} scan is not available on this node.`);
    } catch (e) {
      onError(e instanceof Error ? e.message : "Scan failed");
    }
  }

  function extras(): Record<string, unknown> {
    const body: Record<string, unknown> = {};
    for (const part of extra.split(/\s+/)) {
      const [k, v] = part.split("=");
      if (k && v) body[k] = v;
    }
    return body;
  }

  return (
    <Modal
      title={`Add ${wizardLabel(kind)}`}
      onClose={onClose}
      footer={
        <Button onClick={async () => {
          try {
            const body: Record<string, unknown> = { storage: id, ...extras() };
            if (restrictNodes) body.nodes = restrictNodes;
            if (kind === "pbs") {
              Object.assign(body, { type: "pbs", server, datastore, username, password, fingerprint, content: content || "backup", ...(pbsNamespace ? { namespace: pbsNamespace } : {}) });
            } else if (kind === "nfs") {
              Object.assign(body, { type: "nfs", server, export: exportPath, content: content || "images" });
            } else if (kind === "cifs") {
              Object.assign(body, { type: "cifs", server, share, username, password, domain, content: content || "images" });
            } else if (kind === "iscsi") {
              Object.assign(body, { type: iscsiType, portal, target, content: content || "images" });
            } else if (kind === "rbd") {
              Object.assign(body, { type: "rbd", pool, monhost, namespace, content: content || "images" });
            } else if (kind === "cephfs") {
              Object.assign(body, { type: "cephfs", monhost, path: fsName, content: content || "images" });
            } else if (kind === "zfs-iscsi") {
              Object.assign(body, { type: "zfs", portal, target, pool, content: content || "images" });
            } else if (kind === "dir") {
              Object.assign(body, { type: "dir", path, content: content || "images", shared: shared ? 1 : 0, nodes: node });
            } else if (kind === "lvm") {
              Object.assign(body, { type: "lvm", vgname, content: content || "images", nodes: node });
            } else if (kind === "lvmthin") {
              Object.assign(body, { type: "lvmthin", vgname, thinpool, content: content || "images", nodes: node });
            } else if (kind === "zfspool") {
              Object.assign(body, { type: "zfspool", pool, content: content || "images", nodes: node });
            } else if (kind === "btrfs") {
              Object.assign(body, { type: "btrfs", path, content: content || "images" });
            } else if (kind === "esxi") {
              Object.assign(body, {
                type: "esxi",
                server,
                username,
                password,
                "skip-cert-verification": skipCert ? 1 : 0,
              });
            } else if (kind === "glusterfs") {
              Object.assign(body, { type: "glusterfs", server, volume, transport, content: content || "images" });
            }
            await api.createStorage(body);
            onDone();
          } catch (e) {
            onError(e instanceof Error ? e.message : "Create failed");
          }
        }}>Create</Button>
      }
    >
      <Field label="ID"><Input value={id} onChange={(e) => setId(e.target.value)} /></Field>
      {nodes.length > 0 && (
        <Field label="Scan node">
          <Select value={node} onChange={(e) => setNode(e.target.value)}>
            {nodes.map((n) => <option key={n}>{n}</option>)}
          </Select>
        </Field>
      )}
      {kind === "pbs" && (
        <>
          <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
          <Field label="User"><Input value={username} onChange={(e) => setUsername(e.target.value)} placeholder="user@pbs" /></Field>
          <Field label="Password"><Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></Field>
          <Field label="Fingerprint"><Input value={fingerprint} onChange={(e) => setFingerprint(e.target.value)} /></Field>
          <Button size="sm" variant="outline" onClick={async () => {
            await runScan("pbs", { server, username, password, fingerprint });
          }}>Scan datastores</Button>
          <Field label="Datastore">
            <Select value={datastore} onChange={(e) => setDatastore(e.target.value)}>
              <option value="">Select datastore</option>
              {(scan?.items ?? []).map((item, i) => {
                const name = String(item.store ?? item.datastore ?? item.name ?? "");
                return <option key={name || i} value={name}>{name}</option>;
              })}
            </Select>
          </Field>
          {!scan?.items?.length && (
            <Field label="Datastore (manual)"><Input value={datastore} onChange={(e) => setDatastore(e.target.value)} /></Field>
          )}
          <Field label="Namespace"><Input value={pbsNamespace} onChange={(e) => setPbsNamespace(e.target.value)} placeholder="optional" /></Field>
        </>
      )}
      {kind === "nfs" && (
        <>
          <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
          <Button size="sm" variant="outline" onClick={() => void runScan("nfs", { server })}>Scan exports</Button>
          <Field label="Export">
            <Select value={exportPath} onChange={(e) => setExportPath(e.target.value)}>
              <option value="">Select export</option>
              {(scan?.items ?? []).map((item, i) => {
                const path = String(item.path ?? item.export ?? "");
                return <option key={path || i} value={path}>{path}</option>;
              })}
            </Select>
          </Field>
        </>
      )}
      {kind === "cifs" && (
        <>
          <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
          <Field label="Username"><Input value={username} onChange={(e) => setUsername(e.target.value)} /></Field>
          <Field label="Password"><Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></Field>
          <Field label="Domain"><Input value={domain} onChange={(e) => setDomain(e.target.value)} /></Field>
          <Button size="sm" variant="outline" onClick={() => void runScan("cifs", { server, username, password, domain })}>Scan shares</Button>
          <Field label="Share">
            <Select value={share} onChange={(e) => setShare(e.target.value)}>
              <option value="">Select share</option>
              {(scan?.items ?? []).map((item, i) => {
                const name = String(item.share ?? item.name ?? "");
                return <option key={name || i} value={name}>{name}</option>;
              })}
            </Select>
          </Field>
        </>
      )}
      {kind === "iscsi" && (
        <>
          <Field label="Portal"><Input value={portal} onChange={(e) => setPortal(e.target.value)} /></Field>
          <Button size="sm" variant="outline" onClick={() => void runScan("iscsi", { portal })}>Scan LUNs</Button>
          <Field label="Target">
            <Select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">Select target</option>
              {(scan?.items ?? []).map((item, i) => {
                const name = String(item.target ?? item.portal ?? "");
                return <option key={name || i} value={name}>{name}</option>;
              })}
            </Select>
          </Field>
          <Field label="Type">
            <Select value={iscsiType} onChange={(e) => setIscsiType(e.target.value)}>
              <option value="iscsi">iscsi</option>
              <option value="iscsi-direct">iscsi-direct</option>
            </Select>
          </Field>
        </>
      )}
      {kind === "rbd" && (
        <>
          <Field label="Monitors"><Input value={monhost} onChange={(e) => setMonhost(e.target.value)} placeholder="10.0.0.1 10.0.0.2" /></Field>
          <Field label="Pool"><Input value={pool} onChange={(e) => setPool(e.target.value)} /></Field>
          <Field label="Namespace"><Input value={namespace} onChange={(e) => setNamespace(e.target.value)} /></Field>
        </>
      )}
      {kind === "cephfs" && (
        <>
          <Field label="Monitors"><Input value={monhost} onChange={(e) => setMonhost(e.target.value)} placeholder="10.0.0.1 10.0.0.2" /></Field>
          <Field label="FS / path"><Input value={fsName} onChange={(e) => setFsName(e.target.value)} /></Field>
        </>
      )}
      {kind === "zfs-iscsi" && (
        <>
          <Field label="Portal"><Input value={portal} onChange={(e) => setPortal(e.target.value)} /></Field>
          <Button size="sm" variant="outline" onClick={() => void runScan("iscsi", { portal })}>Scan targets</Button>
          <Field label="Target">
            <Select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">Select target</option>
              {(scan?.items ?? []).map((item, i) => {
                const name = String(item.target ?? item.portal ?? "");
                return <option key={name || i} value={name}>{name}</option>;
              })}
            </Select>
          </Field>
          {!scan?.items?.length && (
            <Field label="Target (manual)"><Input value={target} onChange={(e) => setTarget(e.target.value)} /></Field>
          )}
          <Field label="Pool"><Input value={pool} onChange={(e) => setPool(e.target.value)} /></Field>
        </>
      )}
      {kind === "dir" && (
        <>
          <Field label="Path"><Input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/mnt/data" /></Field>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={shared} onChange={(e) => setShared(e.target.checked)} />
            Shared
          </label>
        </>
      )}
      {kind === "lvm" && (
        <>
          <Button size="sm" variant="outline" onClick={() => void runScan("lvm", {})}>Scan volume groups</Button>
          <Field label="Volume group">
            <Select value={vgname} onChange={(e) => setVgname(e.target.value)}>
              <option value="">Select VG</option>
              {(scan?.items ?? []).map((item, i) => {
                const name = String(item.vg ?? item.vgname ?? item.name ?? "");
                return <option key={name || i} value={name}>{name}</option>;
              })}
            </Select>
          </Field>
          {!scan?.items?.length && (
            <Field label="Volume group (manual)"><Input value={vgname} onChange={(e) => setVgname(e.target.value)} /></Field>
          )}
        </>
      )}
      {kind === "lvmthin" && (
        <>
          <Button size="sm" variant="outline" onClick={() => void runScan("lvmthin", {})}>Scan thin pools</Button>
          <Field label="Volume group / thin pool">
            <Select value={vgname && thinpool ? `${vgname}/${thinpool}` : ""} onChange={(e) => {
              const [vg, lv] = e.target.value.split("/");
              setVgname(vg ?? "");
              setThinpool(lv ?? "");
            }}>
              <option value="">Select thin pool</option>
              {(scan?.items ?? []).map((item, i) => {
                const vg = String(item.vg ?? item.vgname ?? "");
                const lv = String(item.lv ?? item.thinpool ?? "");
                const value = vg && lv ? `${vg}/${lv}` : String(item.name ?? "");
                return <option key={value || i} value={value}>{value}</option>;
              })}
            </Select>
          </Field>
          {!scan?.items?.length && (
            <>
              <Field label="Volume group"><Input value={vgname} onChange={(e) => setVgname(e.target.value)} /></Field>
              <Field label="Thin pool"><Input value={thinpool} onChange={(e) => setThinpool(e.target.value)} /></Field>
            </>
          )}
        </>
      )}
      {kind === "zfspool" && (
        <>
          <Button size="sm" variant="outline" onClick={() => void runScan("zfs", {})}>Scan ZFS pools</Button>
          <Field label="Pool">
            <Select value={pool} onChange={(e) => setPool(e.target.value)}>
              <option value="">Select pool</option>
              {(scan?.items ?? []).map((item, i) => {
                const name = String(item.pool ?? item.name ?? "");
                return <option key={name || i} value={name}>{name}</option>;
              })}
            </Select>
          </Field>
          {!scan?.items?.length && (
            <Field label="Pool (manual)"><Input value={pool} onChange={(e) => setPool(e.target.value)} /></Field>
          )}
        </>
      )}
      {kind === "btrfs" && (
        <Field label="Path"><Input value={path} onChange={(e) => setPath(e.target.value)} placeholder="/mnt/btrfs" /></Field>
      )}
      {kind === "esxi" && (
        <>
          <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
          <Field label="Username"><Input value={username} onChange={(e) => setUsername(e.target.value)} /></Field>
          <Field label="Password"><Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={skipCert} onChange={(e) => setSkipCert(e.target.checked)} />
            Skip certificate verification
          </label>
        </>
      )}
      {kind === "glusterfs" && (
        <>
          <Field label="Peer"><Input value={server} onChange={(e) => setServer(e.target.value)} placeholder="gluster1.example" /></Field>
          <Field label="Volume"><Input value={volume} onChange={(e) => setVolume(e.target.value)} /></Field>
          <Field label="Transport">
            <Select value={transport} onChange={(e) => setTransport(e.target.value)}>
              <option value="tcp">tcp</option>
              <option value="unix">unix</option>
              <option value="rdma">rdma</option>
            </Select>
          </Field>
        </>
      )}
      <ContentTypesField value={content} onChange={setContent} allowed={kind === "pbs" ? ["backup"] : undefined} />
      <Field label="Nodes"><Input value={restrictNodes} onChange={(e) => setRestrictNodes(e.target.value)} placeholder="pve1,pve2" /></Field>
      <Advanced label="Advanced: extra keys" defaultOpen={!!extra}>
        <Field label="Extra keys"><Input value={extra} onChange={(e) => setExtra(e.target.value)} placeholder="key=value leftover keys" /></Field>
      </Advanced>
    </Modal>
  );
}

const storageKnown = ["storage", "type", "content", "nodes", "shared", "disable", "digest", "path", "vgname", "thinpool", "pool", "server", "username", "password", "export", "share", "domain", "portal", "target", "monhost", "namespace", "datastore", "fingerprint", "skip-cert-verification", "volume", "transport"];

function StorageEdit({ item, onClose, onDone }: { item: Storage; onClose: () => void; onDone: () => void }) {
  const cfg = useQuery({ queryKey: ["storage-cfg", item.storage], queryFn: () => api.storageConfig(item.storage) });
  const [fields, setFields] = useState<Record<string, string>>({});
  const [extra, setExtra] = useState("");
  const kind = String(cfg.data?.type ?? item.type);

  useEffect(() => {
    if (!cfg.data) return;
    const src = cfg.data;
    const next: Record<string, string> = {};
    for (const key of storageKnown) {
      if (src[key] !== undefined) next[key] = String(src[key] ?? "");
    }
    setFields({
      content: String(src.content ?? item.content ?? ""),
      nodes: String(src.nodes ?? item.nodes ?? ""),
      path: String(src.path ?? ""),
      vgname: String(src.vgname ?? ""),
      thinpool: String(src.thinpool ?? ""),
      pool: String(src.pool ?? ""),
      server: String(src.server ?? ""),
      username: String(src.username ?? ""),
      shared: String(src.shared ?? item.shared ?? ""),
      "skip-cert-verification": String(src["skip-cert-verification"] ?? ""),
      ...next,
    });
    setExtra(extraPairs(src, storageKnown));
  }, [cfg.data, item.content, item.nodes, item.shared]);

  const set = (key: string, value: string) => setFields((prev) => ({ ...prev, [key]: value }));

  return (
    <Modal
      title={`Edit ${item.storage} (${wizardLabel(kind)})`}
      onClose={onClose}
      footer={
        <Button onClick={async () => {
          const body: Record<string, unknown> = { ...parsePairs(extra) };
          for (const [k, v] of Object.entries(fields)) {
            if (k === "storage" || k === "type" || k === "digest" || k === "disable") continue;
            if (v === "") continue;
            body[k] = k === "shared" || k === "skip-cert-verification" ? (v === "1" || v === "true" ? 1 : 0) : v;
          }
          await api.updateStorage(item.storage, body);
          onDone();
        }}>Save</Button>
      }
    >
      <ContentTypesField value={fields.content ?? ""} onChange={(v) => set("content", v)} allowed={kind === "pbs" ? ["backup"] : undefined} />
      <Field label="Nodes"><Input value={fields.nodes ?? ""} onChange={(e) => set("nodes", e.target.value)} /></Field>
      {(kind === "dir" || kind === "btrfs" || kind === "cephfs") && (
        <Field label="Path"><Input value={fields.path ?? ""} onChange={(e) => set("path", e.target.value)} /></Field>
      )}
      {(kind === "lvm" || kind === "lvmthin") && (
        <Field label="Volume group"><Input value={fields.vgname ?? ""} onChange={(e) => set("vgname", e.target.value)} /></Field>
      )}
      {kind === "lvmthin" && (
        <Field label="Thin pool"><Input value={fields.thinpool ?? ""} onChange={(e) => set("thinpool", e.target.value)} /></Field>
      )}
      {(kind === "zfspool" || kind === "zfs" || kind === "rbd") && (
        <Field label="Pool"><Input value={fields.pool ?? ""} onChange={(e) => set("pool", e.target.value)} /></Field>
      )}
      {(kind === "pbs" || kind === "nfs" || kind === "cifs" || kind === "esxi" || kind === "glusterfs") && (
        <Field label={kind === "glusterfs" ? "Peer" : "Server"}><Input value={fields.server ?? ""} onChange={(e) => set("server", e.target.value)} /></Field>
      )}
      {kind === "nfs" && (
        <Field label="Export"><Input value={fields.export ?? ""} onChange={(e) => set("export", e.target.value)} /></Field>
      )}
      {(kind === "iscsi" || kind === "iscsi-direct" || kind === "zfs") && (
        <>
          <Field label="Portal"><Input value={fields.portal ?? ""} onChange={(e) => set("portal", e.target.value)} /></Field>
          <Field label="Target"><Input value={fields.target ?? ""} onChange={(e) => set("target", e.target.value)} /></Field>
          <Field label="Password"><Input type="password" value={fields.password ?? ""} onChange={(e) => set("password", e.target.value)} /></Field>
        </>
      )}
      {kind === "glusterfs" && (
        <>
          <Field label="Volume"><Input value={fields.volume ?? ""} onChange={(e) => set("volume", e.target.value)} /></Field>
          <Field label="Transport">
            <Select value={fields.transport || "tcp"} onChange={(e) => set("transport", e.target.value)}>
              <option value="tcp">tcp</option>
              <option value="unix">unix</option>
              <option value="rdma">rdma</option>
            </Select>
          </Field>
        </>
      )}
      {(kind === "cifs" || kind === "esxi" || kind === "pbs") && (
        <Field label="Username"><Input value={fields.username ?? ""} onChange={(e) => set("username", e.target.value)} /></Field>
      )}
      {(kind === "cifs" || kind === "esxi" || kind === "pbs") && (
        <Field label="Password"><Input type="password" value={fields.password ?? ""} onChange={(e) => set("password", e.target.value)} /></Field>
      )}
      {kind === "dir" && (
        <label className="flex items-center gap-2 text-sm text-ink-dim">
          <input type="checkbox" checked={fields.shared === "1"} onChange={(e) => set("shared", e.target.checked ? "1" : "0")} />
          Shared
        </label>
      )}
      {kind === "esxi" && (
        <label className="flex items-center gap-2 text-sm text-ink-dim">
          <input type="checkbox" checked={fields["skip-cert-verification"] === "1"} onChange={(e) => set("skip-cert-verification", e.target.checked ? "1" : "0")} />
          Skip certificate verification
        </label>
      )}
      <Advanced label="Advanced: extra keys" defaultOpen={!!extra}>
        <Field label="Extra keys"><Input value={extra} onChange={(e) => setExtra(e.target.value)} /></Field>
      </Advanced>
    </Modal>
  );
}

function remotesBody(form: {
  type: string;
  server: string;
  port: string;
  fingerprint: string;
  token: string;
  comment: string;
  digest?: string;
}) {
  const body: Record<string, unknown> = {};
  if (form.type) body.type = form.type;
  if (form.server) body.server = form.server;
  if (form.port) body.port = form.port;
  if (form.fingerprint) body.fingerprint = form.fingerprint;
  if (form.token) body.token = form.token;
  if (form.comment) body.comment = form.comment;
  if (form.digest) body.digest = form.digest;
  return body;
}

function RemotesTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const remotes = useQuery({ queryKey: ["cluster-remotes"], queryFn: api.clusterRemotes });
  const empty = { remote: "", type: "evpn", server: "", port: "", fingerprint: "", token: "", comment: "" };
  const [form, setForm] = useState(empty);
  const [adding, setAdding] = useState(false);
  const [edit, setEdit] = useState<Record<string, unknown> | null>(null);
  const [del, setDel] = useState<string | null>(null);

  function fill(item: Record<string, unknown>) {
    setForm({
      remote: String(item.remote ?? item.name ?? item.id ?? ""),
      type: String(item.type ?? "evpn"),
      server: String(item.server ?? item.host ?? ""),
      port: String(item.port ?? ""),
      fingerprint: String(item.fingerprint ?? ""),
      token: "",
      comment: String(item.comment ?? ""),
    });
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">
        Named remotes used by remote migrate. Token is only sent when you type a new value.
      </p>
      {remotes.data?.supported === false && (
        <p className="text-sm text-ink-dim">
          This cluster does not expose remotes. Use Remote migrate with a target endpoint instead.
        </p>
      )}
      <FeatureTable
        title="Remotes"
        actions={writable && remotes.data?.supported !== false ? (
          <BarButton onClick={() => { setForm(empty); setAdding(true); }}>Add</BarButton>
        ) : undefined}
        list={remotes.data}
        idKey="remote"
        writable={writable && remotes.data?.supported !== false}
        detailKeys={["type", "server", "host", "port", "fingerprint", "comment"]}
        onEdit={(item) => { fill(item); setEdit(item); }}
        onDelete={async (id) => setDel(id)}
      />
      {adding && (
        <Modal
          title="Add remote"
          onClose={() => setAdding(false)}
          footer={
            <Button
              disabled={!form.remote || !form.server}
              onClick={async () => {
                await api.createClusterRemote({ remote: form.remote, ...remotesBody(form) });
                setAdding(false);
                void qc.invalidateQueries({ queryKey: ["cluster-remotes"] });
              }}
            >
              Save
            </Button>
          }
        >
          <RemoteFields form={form} setForm={setForm} create />
        </Modal>
      )}
      {edit && (
        <Modal
          title={`Edit ${form.remote}`}
          onClose={() => setEdit(null)}
          footer={
            <Button
              onClick={async () => {
                await api.updateClusterRemote(form.remote, remotesBody({
                  ...form,
                  digest: String(edit.digest ?? ""),
                }));
                setEdit(null);
                void qc.invalidateQueries({ queryKey: ["cluster-remotes"] });
              }}
            >
              Save
            </Button>
          }
        >
          <RemoteFields form={form} setForm={setForm} />
        </Modal>
      )}
      {del && (
        <ConfirmDialog
          title={`Delete remote ${del}?`}
          body="Remote migrate will no longer list this remote."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteClusterRemote(del);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["cluster-remotes"] });
          }}
        />
      )}
    </div>
  );
}

function RemoteFields({
  form,
  setForm,
  create,
}: {
  form: { remote: string; type: string; server: string; port: string; fingerprint: string; token: string; comment: string };
  setForm: (next: typeof form) => void;
  create?: boolean;
}) {
  return (
    <>
      {create && <Field label="Remote"><Input value={form.remote} onChange={(e) => setForm({ ...form, remote: e.target.value })} /></Field>}
      <Field label="Type">
        <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
          <option value="evpn">evpn</option>
          <option value="pve">pve</option>
        </Select>
      </Field>
      <Field label="Server"><Input value={form.server} onChange={(e) => setForm({ ...form, server: e.target.value })} placeholder="10.0.0.8" /></Field>
      <Field label="Port"><Input value={form.port} onChange={(e) => setForm({ ...form, port: e.target.value })} placeholder="8006" /></Field>
      <Field label="Fingerprint"><Input value={form.fingerprint} onChange={(e) => setForm({ ...form, fingerprint: e.target.value })} /></Field>
      <Field label="Token"><Input type="password" value={form.token} onChange={(e) => setForm({ ...form, token: e.target.value })} placeholder={create ? "" : "unchanged"} /></Field>
      <Field label="Comment"><Input value={form.comment} onChange={(e) => setForm({ ...form, comment: e.target.value })} /></Field>
    </>
  );
}

function ClusterTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const config = useQuery({ queryKey: ["dc-cluster"], queryFn: api.clusterConfig });
  const nodes = useQuery({ queryKey: ["dc-cluster-nodes"], queryFn: api.clusterNodes });
  const join = useQuery({ queryKey: ["dc-cluster-join"], queryFn: api.clusterJoinInfo });
  const qdev = useQuery({ queryKey: ["dc-cluster-qdev"], queryFn: api.clusterQDevice });
  const [createName, setCreateName] = useState("");
  const [link0, setLink0] = useState("");
  const [link1, setLink1] = useState("");
  const [link2, setLink2] = useState("");
  const [votes, setVotes] = useState("1");
  const [joinHost, setJoinHost] = useState("");
  const [fingerprint, setFingerprint] = useState("");
  const [password, setPassword] = useState("");
  const [addNode, setAddNode] = useState("");
  const [addingNode, setAddingNode] = useState(false);
  const [qdevHost, setQdevHost] = useState("");
  const [confirm, setConfirm] = useState<"create" | "join" | "qdev" | "totem" | null>(null);
  const [remove, setRemove] = useState<string | null>(null);
  const [consensus, setConsensus] = useState("");
  const [linkMode, setLinkMode] = useState("");
  const [totemLink0, setTotemLink0] = useState("");
  const [totemLink1, setTotemLink1] = useState("");
  const [totemLink2, setTotemLink2] = useState("");

  const clustered = config.data?.supported !== false && !!(config.data?.cluster_name || config.data?.totem || (nodes.data?.items?.length ?? 0) > 1);

  return (
    <div className="space-y-4">
      <Card className="space-y-2 p-4">
        <h2 className="font-medium">Cluster</h2>
        {config.data?.supported === false ? (
          <p className="text-sm text-ink-dim">Cluster config is not available on this node.</p>
        ) : (
          <ClusterConfigTable data={config.data ?? {}} members={nodes.data?.items} />
        )}
      </Card>
      {clustered && join.data && <JoinInfoCard data={join.data} />}
      {writable && clustered && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Totem / links</h2>
          <p className="text-sm text-ink-dim">Membership-sensitive. Confirm before applying.</p>
          <div className="grid gap-3 md:grid-cols-2">
            <Field label="Consensus"><Input value={consensus} onChange={(e) => setConsensus(e.target.value)} placeholder="ms" /></Field>
            <Field label="Link mode">
              <Select value={linkMode} onChange={(e) => setLinkMode(e.target.value)}>
                <option value="">Keep current</option>
                <option value="passive">passive</option>
                <option value="active">active</option>
                <option value="rr">rr</option>
              </Select>
            </Field>
            <Field label="link0"><Input value={totemLink0} onChange={(e) => setTotemLink0(e.target.value)} /></Field>
            <Field label="link1"><Input value={totemLink1} onChange={(e) => setTotemLink1(e.target.value)} /></Field>
            <Field label="link2"><Input value={totemLink2} onChange={(e) => setTotemLink2(e.target.value)} /></Field>
          </div>
          <Button size="sm" onClick={() => setConfirm("totem")}>Save totem</Button>
        </Card>
      )}
      {nodes.data?.supported !== false && (
        <InventoryCard
          title="Nodes"
          actions={writable && clustered ? <BarButton onClick={() => setAddingNode(true)}>Add</BarButton> : undefined}
        >
          <table className="w-full text-sm">
            <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
              <tr>
                <th className="px-4 py-2">Kind</th>
                <th>Summary</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {!(nodes.data?.items ?? []).length && <InventoryEmpty>No cluster nodes</InventoryEmpty>}
              {(nodes.data?.items ?? []).map((item, i) => {
                const name = String(item.name || item.node || i);
                return (
                  <InventoryRow
                    key={name}
                    kind={name}
                    summary={joinSummary([
                      item.nodeid != null && `id ${item.nodeid}`,
                      (item.quorum_votes ?? item.votes) != null && `votes ${item.quorum_votes ?? item.votes}`,
                      String(item.ring0_addr ?? item.link0 ?? ""),
                    ])}
                  >
                    {writable && <Button size="sm" variant="danger" onClick={() => setRemove(name)}>Remove</Button>}
                  </InventoryRow>
                );
              })}
            </tbody>
          </table>
        </InventoryCard>
      )}
      {writable && !clustered && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Create cluster</h2>
          <div className="grid gap-3 md:grid-cols-3">
            <Field label="Name"><Input value={createName} onChange={(e) => setCreateName(e.target.value)} /></Field>
            <Field label="link0 address"><Input value={link0} onChange={(e) => setLink0(e.target.value)} /></Field>
            <Field label="link1 address"><Input value={link1} onChange={(e) => setLink1(e.target.value)} /></Field>
            <Field label="link2 address"><Input value={link2} onChange={(e) => setLink2(e.target.value)} /></Field>
            <Field label="Votes"><Input value={votes} onChange={(e) => setVotes(e.target.value)} /></Field>
          </div>
          <Button size="sm" onClick={() => setConfirm("create")}>Create</Button>
        </Card>
      )}
      {writable && !clustered && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Join cluster</h2>
          <p className="text-sm text-ink-dim">Add this node to an existing cluster.</p>
          <JoinPaste
            onFill={(j) => {
              setJoinHost(j.ip);
              setFingerprint(j.fingerprint);
            }}
          />
          <div className="grid gap-3 md:grid-cols-2">
            <Field label="Hostname"><Input value={joinHost} onChange={(e) => setJoinHost(e.target.value)} /></Field>
            <Field label="Fingerprint"><Input value={fingerprint} onChange={(e) => setFingerprint(e.target.value)} /></Field>
            <Field label="Password"><Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></Field>
            <Field label="link0 address"><Input value={link0} onChange={(e) => setLink0(e.target.value)} /></Field>
            <Field label="link1 address"><Input value={link1} onChange={(e) => setLink1(e.target.value)} /></Field>
            <Field label="link2 address"><Input value={link2} onChange={(e) => setLink2(e.target.value)} /></Field>
          </div>
          <Button size="sm" onClick={() => setConfirm("join")}>Join</Button>
        </Card>
      )}
      {addingNode && (
        <Modal
          title="Add node"
          onClose={() => setAddingNode(false)}
          footer={
            <Button onClick={async () => {
              await api.addClusterNode({ node: addNode, ...clusterLinks(link0, link1, link2), votes });
              setAddNode("");
              setAddingNode(false);
              void qc.invalidateQueries({ queryKey: ["dc-cluster-nodes"] });
            }}>Save</Button>
          }
        >
          <Field label="Node"><Input value={addNode} onChange={(e) => setAddNode(e.target.value)} /></Field>
          <Field label="link0"><Input value={link0} onChange={(e) => setLink0(e.target.value)} /></Field>
          <Field label="link1"><Input value={link1} onChange={(e) => setLink1(e.target.value)} /></Field>
          <Field label="link2"><Input value={link2} onChange={(e) => setLink2(e.target.value)} /></Field>
        </Modal>
      )}
      {qdev.data?.supported !== false && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">QDevice</h2>
          <QDeviceTable items={qdev.data?.items ?? []} />
          {writable && (
            <div className="flex flex-wrap gap-2">
              <Input className="max-w-xs" placeholder="qdevice host" value={qdevHost} onChange={(e) => setQdevHost(e.target.value)} />
              <Button size="sm" onClick={() => setConfirm("qdev")}>Create</Button>
              <Button size="sm" variant="danger" onClick={async () => {
                await api.deleteClusterQDevice();
                void qc.invalidateQueries({ queryKey: ["dc-cluster-qdev"] });
              }}>Destroy</Button>
            </div>
          )}
        </Card>
      )}
      {qdev.data?.supported === false && (
        <p className="text-sm text-ink-dim">QDevice is not available on this cluster.</p>
      )}
      {confirm && (
        <ConfirmDialog
          title={confirm === "create" ? "Create this cluster?" : confirm === "join" ? "Join this cluster?" : confirm === "totem" ? "Update corosync totem?" : "Create QDevice?"}
          body="This changes corosync membership and can interrupt quorum."
          confirmLabel={confirm === "join" ? "Join" : "Confirm"}
          onClose={() => setConfirm(null)}
          onConfirm={async () => {
            if (confirm === "create") await api.createCluster({ clustername: createName, ...clusterLinks(link0, link1, link2), votes });
            if (confirm === "join") await api.joinCluster({ hostname: joinHost, fingerprint, password, ...clusterLinks(link0, link1, link2), votes });
            if (confirm === "qdev") await api.createClusterQDevice({ address: qdevHost });
            if (confirm === "totem") {
              const body: Record<string, unknown> = {};
              if (consensus) body.consensus = consensus;
              if (linkMode) body.link_mode = linkMode;
              if (totemLink0) body.link0 = totemLink0;
              if (totemLink1) body.link1 = totemLink1;
              if (totemLink2) body.link2 = totemLink2;
              await api.updateClusterTotem(body);
            }
            setConfirm(null);
            void qc.invalidateQueries({ queryKey: ["dc-cluster"] });
            void qc.invalidateQueries({ queryKey: ["dc-cluster-nodes"] });
            void qc.invalidateQueries({ queryKey: ["dc-cluster-qdev"] });
          }}
        />
      )}
      {remove && (
        <ConfirmDialog
          title={`Remove node ${remove}?`}
          body="The node is removed from corosync. Guests are not deleted."
          danger
          confirmLabel="Remove"
          onClose={() => setRemove(null)}
          onConfirm={async () => {
            await api.deleteClusterNode(remove);
            setRemove(null);
            void qc.invalidateQueries({ queryKey: ["dc-cluster-nodes"] });
          }}
        />
      )}
    </div>
  );
}

function ACMETab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const accounts = useQuery({ queryKey: ["acme-accounts"], queryFn: api.acmeAccounts });
  const plugins = useQuery({ queryKey: ["acme-plugins"], queryFn: api.acmePlugins });
  const directories = useQuery({ queryKey: ["acme-dirs"], queryFn: api.acmeDirectories });
  const [name, setName] = useState("default");
  const [contact, setContact] = useState("");
  const [directory, setDirectory] = useState("");
  const [plugin, setPlugin] = useState("");
  const [apiName, setApiName] = useState("standalone");
  const [pluginData, setPluginData] = useState<Record<string, string>>({});
  const [addingAccount, setAddingAccount] = useState(false);
  const [addingPlugin, setAddingPlugin] = useState(false);
  const [editAccount, setEditAccount] = useState<Record<string, unknown> | null>(null);
  const [editPlugin, setEditPlugin] = useState<Record<string, unknown> | null>(null);
  const [editContact, setEditContact] = useState("");
  const [editDirectory, setEditDirectory] = useState("");
  const [editApi, setEditApi] = useState("");
  const [editPluginData, setEditPluginData] = useState<Record<string, string>>({});
  const meta = useQuery({
    queryKey: ["acme-meta", directory],
    queryFn: () => api.acmeMeta(directory || undefined),
    enabled: directories.data?.supported !== false,
  });
  const tos = useQuery({
    queryKey: ["acme-tos", directory],
    queryFn: () => api.acmeTos(directory || undefined),
    enabled: directories.data?.supported !== false,
  });
  const schema = useQuery({
    queryKey: ["acme-schema", apiName],
    queryFn: () => api.acmeChallengeSchema(apiName),
    enabled: /dns/i.test(apiName),
  });
  const editSchema = useQuery({
    queryKey: ["acme-schema", editApi],
    queryFn: () => api.acmeChallengeSchema(editApi),
    enabled: !!editPlugin && /dns/i.test(editApi),
  });

  if (accounts.data && !accounts.data.supported) {
    return <p className="text-sm text-ink-dim">ACME is not available on this cluster.</p>;
  }

  return (
    <div className="space-y-4">
      {directories.data?.supported !== false && (
        <Card className="space-y-2 p-4">
          <h2 className="font-medium">Directory</h2>
          <Select className="max-w-xl" value={directory} onChange={(e) => setDirectory(e.target.value)}>
            <option value="">Default directory</option>
            {(directories.data?.items ?? []).map((item, i) => {
              const url = String(item.url ?? item.name ?? "");
              return <option key={url || i} value={url}>{String(item.name ?? url)}</option>;
            })}
          </Select>
          {tos.data?.supported !== false && tos.data?.value && (
            <p className="text-xs text-ink-dim">TOS: {tos.data.value}</p>
          )}
          {meta.data?.supported !== false && meta.data?.item && (
            <KeyValueTable data={asRecord(meta.data.item)} />
          )}
        </Card>
      )}
      <FeatureTable
        title="Accounts"
        actions={writable ? <BarButton onClick={() => setAddingAccount(true)}>Add</BarButton> : undefined}
        list={accounts.data}
        idKey="name"
        writable={writable}
        detailKeys={["contact", "directory", "tos"]}
        onEdit={(item) => {
          setEditAccount(item);
          setEditContact(String(item.contact ?? item.email ?? ""));
          setEditDirectory(String(item.directory ?? directory));
        }}
        onDelete={async (id) => {
          await api.deleteACMEAccount(id);
          void qc.invalidateQueries({ queryKey: ["acme-accounts"] });
        }}
      />
      <FeatureTable
        title="Plugins"
        actions={writable ? <BarButton onClick={() => setAddingPlugin(true)}>Add</BarButton> : undefined}
        list={plugins.data}
        idKey="plugin"
        writable={writable}
        detailKeys={["api", "validation", "comment"]}
        onEdit={(item) => {
          setEditPlugin(item);
          setEditApi(String(item.api ?? ""));
          setEditPluginData(parseACMEData(item.data));
        }}
        onDelete={async (id) => {
          await api.deleteACMEPlugin(id);
          void qc.invalidateQueries({ queryKey: ["acme-plugins"] });
        }}
      />
      {addingAccount && (
        <Modal
          title="Add account"
          onClose={() => setAddingAccount(false)}
          footer={
            <Button onClick={async () => {
              const body: Record<string, unknown> = { name, contact };
              if (directory) body.directory = directory;
              attachTask((await api.createACMEAccount(body)).upid, [["acme-accounts"]]);
              setAddingAccount(false);
            }}>Save</Button>
          }
        >
          <Field label="Account name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="Contact email"><Input value={contact} onChange={(e) => setContact(e.target.value)} /></Field>
        </Modal>
      )}
      {addingPlugin && (
        <Modal
          title="Add plugin"
          onClose={() => setAddingPlugin(false)}
          footer={
            <Button onClick={async () => {
              const data = Object.entries(pluginData).filter(([, v]) => v).map(([k, v]) => `${k}=${v}`).join("\n");
              await api.createACMEPlugin({ id: plugin, api: apiName, ...(data ? { data } : {}) });
              setPlugin("");
              setPluginData({});
              setAddingPlugin(false);
              void qc.invalidateQueries({ queryKey: ["acme-plugins"] });
            }}>Save</Button>
          }
        >
          <Field label="Plugin id"><Input value={plugin} onChange={(e) => setPlugin(e.target.value)} /></Field>
          <Field label="API"><Input value={apiName} onChange={(e) => setApiName(e.target.value)} placeholder="standalone/dns" /></Field>
          <ACMESchemaFields schema={schema.data} values={pluginData} onChange={setPluginData} />
        </Modal>
      )}
      {editAccount && (
        <Modal
          title={`Edit account ${String(editAccount.name ?? "default")}`}
          onClose={() => setEditAccount(null)}
          footer={
            <Button onClick={async () => {
              const body: Record<string, unknown> = { contact: editContact };
              if (editDirectory) body.directory = editDirectory;
              const res = await api.updateACMEAccount(String(editAccount.name ?? name), body);
              if (res.upid) attachTask(res.upid, [["acme-accounts"]]);
              else void qc.invalidateQueries({ queryKey: ["acme-accounts"] });
              setEditAccount(null);
            }}>Save</Button>
          }
        >
          <Field label="Contact"><Input value={editContact} onChange={(e) => setEditContact(e.target.value)} /></Field>
          <Field label="Directory"><Input value={editDirectory} onChange={(e) => setEditDirectory(e.target.value)} placeholder="https://acme-v02.api.letsencrypt.org/directory" /></Field>
        </Modal>
      )}
      {editPlugin && (
        <Modal
          title={`Edit plugin ${String(editPlugin.plugin ?? editPlugin.id ?? "")}`}
          onClose={() => setEditPlugin(null)}
          footer={
            <Button onClick={async () => {
              const data = Object.entries(editPluginData).filter(([, v]) => v).map(([k, v]) => `${k}=${v}`).join("\n");
              await api.updateACMEPlugin(String(editPlugin.plugin ?? editPlugin.id ?? ""), { api: editApi, ...(data ? { data } : {}) });
              setEditPlugin(null);
              void qc.invalidateQueries({ queryKey: ["acme-plugins"] });
            }}>Save</Button>
          }
        >
          <Field label="API"><Input value={editApi} onChange={(e) => setEditApi(e.target.value)} /></Field>
          <ACMESchemaFields schema={editSchema.data} values={editPluginData} onChange={setEditPluginData} />
        </Modal>
      )}
    </div>
  );
}

function NotifyTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const [kind, setKind] = useState("smtp");
  const endpoints = useQuery({ queryKey: ["notify-ep", kind], queryFn: () => api.notifyEndpoints(kind) });
  const matchers = useQuery({ queryKey: ["notify-match"], queryFn: api.notifyMatchers });
  const targets = useQuery({ queryKey: ["notify-targets"], queryFn: api.notifyTargets });
  const [name, setName] = useState("");
  const [server, setServer] = useState("");
  const [token, setToken] = useState("");
  const [url, setUrl] = useState("");
  const [method, setMethod] = useState("POST");
  const [secret, setSecret] = useState("");
  const [fromAddress, setFromAddress] = useState("");
  const [mailto, setMailto] = useState("");
  const [author, setAuthor] = useState("");
  const [smtpPort, setSmtpPort] = useState("");
  const [smtpMode, setSmtpMode] = useState("tls");
  const [smtpUser, setSmtpUser] = useState("");
  const [smtpPass, setSmtpPass] = useState("");
  const [matcherName, setMatcherName] = useState("");
  const [matcherTarget, setMatcherTarget] = useState("");
  const [matchField, setMatchField] = useState("");
  const [matchCalendar, setMatchCalendar] = useState("");
  const [matcherComment, setMatcherComment] = useState("");
  const [matcherDisable, setMatcherDisable] = useState(false);
  const [severities, setSeverities] = useState<string[]>(["info", "notice", "warning", "error"]);
  const [addingEp, setAddingEp] = useState(false);
  const [addingMatch, setAddingMatch] = useState(false);
  const [editEp, setEditEp] = useState<Record<string, unknown> | null>(null);
  const [editMatch, setEditMatch] = useState<Record<string, unknown> | null>(null);
  const [testMsg, setTestMsg] = useState("");

  if (endpoints.data && !endpoints.data.supported) {
    return <p className="text-sm text-ink-dim">Notifications require PVE 8.1+.</p>;
  }

  function typedBody(base: Record<string, unknown> = {}) {
    const body: Record<string, unknown> = { ...base };
    if (kind === "gotify") {
      if (server) body.server = server;
      if (token) body.token = token;
    } else if (kind === "webhook") {
      if (url) body.url = url;
      if (method) body.method = method;
      if (secret) body.secret = secret;
    } else if (kind === "smtp") {
      if (server) body.server = server;
      if (fromAddress) body["from-address"] = fromAddress;
      if (mailto) body.mailto = mailto;
      if (author) body.author = author;
      if (smtpPort) body.port = Number(smtpPort);
      if (smtpMode) body.mode = smtpMode;
      if (smtpUser) body.username = smtpUser;
      if (smtpPass) body.password = smtpPass;
    } else if (kind === "sendmail") {
      if (fromAddress) body["from-address"] = fromAddress;
      if (mailto) body.mailto = mailto;
      if (author) body.author = author;
    }
    return body;
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        {["smtp", "gotify", "webhook", "sendmail"].map((k) => (
          <Button key={k} size="sm" variant={kind === k ? "selected" : "outline"} onClick={() => setKind(k)}>{k}</Button>
        ))}
      </div>
      <FeatureTable
        title="Endpoints"
        actions={writable ? <BarButton onClick={() => { setName(""); setAddingEp(true); }}>Add</BarButton> : undefined}
        list={endpoints.data}
        idKey="name"
        writable={writable}
        detailKeys={["server", "url", "mailto", "disable"]}
        onEdit={(item) => {
          setServer(String(item.server ?? ""));
          setToken(String(item.token ?? ""));
          setUrl(String(item.url ?? ""));
          setMethod(String(item.method ?? "POST"));
          setSecret(String(item.secret ?? ""));
          setFromAddress(String(item["from-address"] ?? item["mail-from"] ?? ""));
          setMailto(String(item.mailto ?? ""));
          setAuthor(String(item.author ?? ""));
          setSmtpPort(String(item.port ?? ""));
          setSmtpMode(String(item.mode ?? "tls"));
          setSmtpUser(String(item.username ?? ""));
          setEditEp(item);
        }}
        onDelete={async (id) => {
          await api.deleteNotifyEndpoint(kind, id);
          void qc.invalidateQueries({ queryKey: ["notify-ep", kind] });
        }}
      />
      {testMsg && <p className="text-sm text-ink-dim">{testMsg}</p>}
      <FeatureTable
        title="Targets"
        list={targets.data}
        idKey="name"
        writable={writable}
        detailKeys={["type", "comment", "disable"]}
        actionLabel="Test"
        onAction={async (id) => {
          const res = await api.testNotifyTarget(id);
          setTestMsg(res.supported === false ? "Target test is not available." : `Tested ${id}.`);
        }}
      />
      <FeatureTable
        title="Matchers"
        actions={writable ? <BarButton onClick={() => {
          setMatcherName("");
          setMatcherTarget("");
          setMatchField("");
          setMatchCalendar("");
          setMatcherComment("");
          setMatcherDisable(false);
          setSeverities(["info", "notice", "warning", "error"]);
          setAddingMatch(true);
        }}>Add</BarButton> : undefined}
        list={matchers.data}
        idKey="name"
        writable={writable}
        detailKeys={["target", "match-severity", "match-field", "match-calendar", "comment", "disable"]}
        onEdit={(item) => {
          setEditMatch(item);
          setMatcherTarget(String(item.target ?? ""));
          setMatchField(String(item["match-field"] ?? ""));
          setMatchCalendar(String(item["match-calendar"] ?? ""));
          setMatcherComment(String(item.comment ?? ""));
          setMatcherDisable(String(item.disable ?? "") === "1" || item.disable === true);
          setSeverities(parseSeverities(item["match-severity"]));
        }}
        onDelete={async (id) => {
          await api.deleteNotifyMatcher(id);
          void qc.invalidateQueries({ queryKey: ["notify-match"] });
        }}
      />
      {addingEp && (
        <Modal
          title={`Add ${kind} endpoint`}
          onClose={() => setAddingEp(false)}
          footer={
            <Button onClick={async () => {
              await api.createNotifyEndpoint(kind, typedBody({ name }));
              setName("");
              setAddingEp(false);
              void qc.invalidateQueries({ queryKey: ["notify-ep", kind] });
              void qc.invalidateQueries({ queryKey: ["notify-targets"] });
            }}>Save</Button>
          }
        >
          <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          {kind === "gotify" && (
            <>
              <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
              <Field label="Token"><Input value={token} onChange={(e) => setToken(e.target.value)} /></Field>
            </>
          )}
          {kind === "webhook" && (
            <>
              <Field label="URL"><Input value={url} onChange={(e) => setUrl(e.target.value)} /></Field>
              <Field label="Method">
                <Select value={method} onChange={(e) => setMethod(e.target.value)}>
                  <option>POST</option>
                  <option>PUT</option>
                </Select>
              </Field>
              <Field label="Secret"><Input value={secret} onChange={(e) => setSecret(e.target.value)} /></Field>
            </>
          )}
          {kind === "smtp" && (
            <>
              <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
              <Field label="From"><Input value={fromAddress} onChange={(e) => setFromAddress(e.target.value)} /></Field>
              <Field label="Mailto"><Input value={mailto} onChange={(e) => setMailto(e.target.value)} /></Field>
              <Field label="Port"><Input value={smtpPort} onChange={(e) => setSmtpPort(e.target.value)} /></Field>
              <Field label="Mode">
                <Select value={smtpMode} onChange={(e) => setSmtpMode(e.target.value)}>
                  <option value="tls">tls</option>
                  <option value="starttls">starttls</option>
                  <option value="insecure">insecure</option>
                </Select>
              </Field>
              <Field label="Username"><Input value={smtpUser} onChange={(e) => setSmtpUser(e.target.value)} /></Field>
              <Field label="Password"><Input type="password" value={smtpPass} onChange={(e) => setSmtpPass(e.target.value)} /></Field>
              <Field label="Author"><Input value={author} onChange={(e) => setAuthor(e.target.value)} /></Field>
            </>
          )}
          {kind === "sendmail" && (
            <>
              <Field label="From"><Input value={fromAddress} onChange={(e) => setFromAddress(e.target.value)} /></Field>
              <Field label="Mailto"><Input value={mailto} onChange={(e) => setMailto(e.target.value)} /></Field>
              <Field label="Author"><Input value={author} onChange={(e) => setAuthor(e.target.value)} /></Field>
            </>
          )}
        </Modal>
      )}
      {addingMatch && (
        <Modal
          title="Add matcher"
          onClose={() => setAddingMatch(false)}
          footer={
            <Button onClick={async () => {
              const body: Record<string, unknown> = {
                name: matcherName,
                target: matcherTarget,
                "match-severity": severities.join(","),
              };
              if (matchField) body["match-field"] = matchField;
              if (matchCalendar) body["match-calendar"] = matchCalendar;
              if (matcherComment) body.comment = matcherComment;
              if (matcherDisable) body.disable = 1;
              await api.createNotifyMatcher(body);
              setMatcherName("");
              setMatchField("");
              setMatchCalendar("");
              setMatcherComment("");
              setMatcherDisable(false);
              setAddingMatch(false);
              void qc.invalidateQueries({ queryKey: ["notify-match"] });
            }}>Save</Button>
          }
        >
          <Field label="Name"><Input value={matcherName} onChange={(e) => setMatcherName(e.target.value)} /></Field>
          <Field label="Target"><Input value={matcherTarget} onChange={(e) => setMatcherTarget(e.target.value)} /></Field>
          <Field label="Severity">
            <SeverityChecks value={severities} onChange={setSeverities} />
          </Field>
          <Field label="Match field"><Input value={matchField} onChange={(e) => setMatchField(e.target.value)} placeholder="type:vzdump" /></Field>
          <Field label="Match calendar"><Input value={matchCalendar} onChange={(e) => setMatchCalendar(e.target.value)} placeholder="mon..fri" /></Field>
          <Field label="Comment"><Input value={matcherComment} onChange={(e) => setMatcherComment(e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={matcherDisable} onChange={(e) => setMatcherDisable(e.target.checked)} />
            Disable
          </label>
        </Modal>
      )}
      {editEp && (
        <Modal
          title={`Edit ${kind} ${String(editEp.name ?? "")}`}
          onClose={() => setEditEp(null)}
          footer={
            <Button onClick={async () => {
              await api.updateNotifyEndpoint(kind, String(editEp.name ?? ""), typedBody());
              setEditEp(null);
              void qc.invalidateQueries({ queryKey: ["notify-ep", kind] });
            }}>Save</Button>
          }
        >
          {kind === "gotify" && (
            <>
              <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
              <Field label="Token"><Input value={token} onChange={(e) => setToken(e.target.value)} /></Field>
            </>
          )}
          {kind === "webhook" && (
            <>
              <Field label="URL"><Input value={url} onChange={(e) => setUrl(e.target.value)} /></Field>
              <Field label="Method"><Input value={method} onChange={(e) => setMethod(e.target.value)} /></Field>
              <Field label="Secret"><Input value={secret} onChange={(e) => setSecret(e.target.value)} /></Field>
            </>
          )}
          {kind === "smtp" && (
            <>
              <Field label="Server"><Input value={server} onChange={(e) => setServer(e.target.value)} /></Field>
              <Field label="From"><Input value={fromAddress} onChange={(e) => setFromAddress(e.target.value)} /></Field>
              <Field label="Mailto"><Input value={mailto} onChange={(e) => setMailto(e.target.value)} /></Field>
              <Field label="Port"><Input value={smtpPort} onChange={(e) => setSmtpPort(e.target.value)} /></Field>
              <Field label="Mode">
                <Select value={smtpMode} onChange={(e) => setSmtpMode(e.target.value)}>
                  <option value="tls">tls</option>
                  <option value="starttls">starttls</option>
                  <option value="insecure">insecure</option>
                </Select>
              </Field>
              <Field label="Username"><Input value={smtpUser} onChange={(e) => setSmtpUser(e.target.value)} /></Field>
              <Field label="Password"><Input type="password" value={smtpPass} onChange={(e) => setSmtpPass(e.target.value)} /></Field>
              <Field label="Author"><Input value={author} onChange={(e) => setAuthor(e.target.value)} /></Field>
            </>
          )}
          {kind === "sendmail" && (
            <>
              <Field label="From"><Input value={fromAddress} onChange={(e) => setFromAddress(e.target.value)} /></Field>
              <Field label="Mailto"><Input value={mailto} onChange={(e) => setMailto(e.target.value)} /></Field>
              <Field label="Author"><Input value={author} onChange={(e) => setAuthor(e.target.value)} /></Field>
            </>
          )}
        </Modal>
      )}
      {editMatch && (
        <Modal
          title={`Edit matcher ${String(editMatch.name ?? "")}`}
          onClose={() => setEditMatch(null)}
          footer={
            <Button onClick={async () => {
              await api.updateNotifyMatcher(String(editMatch.name ?? ""), {
                target: matcherTarget || String(editMatch.target ?? ""),
                "match-severity": severities.join(","),
                "match-field": matchField,
                "match-calendar": matchCalendar,
                comment: matcherComment,
                disable: matcherDisable ? 1 : 0,
              });
              setEditMatch(null);
              void qc.invalidateQueries({ queryKey: ["notify-match"] });
            }}>Save</Button>
          }
        >
          <Field label="Target"><Input value={matcherTarget} onChange={(e) => setMatcherTarget(e.target.value)} /></Field>
          <Field label="Severity">
            <SeverityChecks value={severities} onChange={setSeverities} />
          </Field>
          <Field label="Match field"><Input value={matchField} onChange={(e) => setMatchField(e.target.value)} placeholder="type:vzdump" /></Field>
          <Field label="Match calendar"><Input value={matchCalendar} onChange={(e) => setMatchCalendar(e.target.value)} placeholder="mon..fri" /></Field>
          <Field label="Comment"><Input value={matcherComment} onChange={(e) => setMatcherComment(e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={matcherDisable} onChange={(e) => setMatcherDisable(e.target.checked)} />
            Disable
          </label>
        </Modal>
      )}
    </div>
  );
}

function MappingsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can("/", "Mapping.Modify");
  const [kind, setKind] = useState("pci");
  const list = useQuery({ queryKey: ["maps", kind], queryFn: () => api.mappings(kind) });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const nodes = [...new Set((summary.data?.resources ?? []).filter((r) => r.type === "node" && r.node).map((r) => r.node as string))];
  const [id, setId] = useState("");
  const [paths, setPaths] = useState<Record<string, string>>({});
  const [adding, setAdding] = useState(false);
  const [edit, setEdit] = useState<Record<string, unknown> | null>(null);
  const [editPaths, setEditPaths] = useState<Record<string, string>>({});

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        {["pci", "usb", "dir"].map((k) => (
          <Button key={k} size="sm" variant={kind === k ? "selected" : "outline"} onClick={() => { setKind(k); setPaths({}); }}>{k}</Button>
        ))}
      </div>
      <FeatureTable
        title="Mappings"
        actions={writable ? <BarButton onClick={() => { setId(""); setPaths({}); setAdding(true); }}>Add</BarButton> : undefined}
        list={list.data}
        idKey="id"
        writable={writable}
        renderDetails={(item) => {
          const map = item.map;
          const extra = Array.isArray(map) ? map.map(String).join(" · ") : String(map ?? "");
          return <ItemDetails item={item} keys={["comment", "description"]} extra={extra} />;
        }}
        onEdit={(item) => {
          setEdit(item);
          const next: Record<string, string> = {};
          const map = item.map;
          const entries = Array.isArray(map) ? map : String(map ?? "").split(/[;,]/);
          for (const raw of entries) {
            const s = String(raw ?? "").trim();
            const i = s.indexOf(":");
            if (i > 0) next[s.slice(0, i)] = s.slice(i + 1);
          }
          setEditPaths(next);
        }}
        onDelete={async (name) => {
          await api.deleteMapping(kind, name);
          void qc.invalidateQueries({ queryKey: ["maps", kind] });
        }}
      />
      {adding && (
        <Modal
          title={`Add ${kind} mapping`}
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              const map = Object.entries(paths)
                .filter(([, path]) => path.trim())
                .map(([node, path]) => `${node}:${path.trim()}`);
              if (!id || !map.length) return;
              await api.createMapping(kind, { id, map });
              setId("");
              setPaths({});
              setAdding(false);
              void qc.invalidateQueries({ queryKey: ["maps", kind] });
            }}>Save</Button>
          }
        >
          <Field label="Mapping id">
            <Input placeholder="gpu0" value={id} onChange={(e) => setId(e.target.value)} />
          </Field>
          <div className="grid gap-3">
            {nodes.map((node) => (
              <MappingNodeRow
                key={node}
                kind={kind}
                node={node}
                value={paths[node] ?? ""}
                onChange={(path) => setPaths((cur) => ({ ...cur, [node]: path }))}
              />
            ))}
            {!nodes.length && <p className="text-sm text-ink-dim">No nodes in inventory.</p>}
          </div>
        </Modal>
      )}
      {edit && (
        <Modal
          title={`Edit mapping ${String(edit.id ?? "")}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              const map = Object.entries(editPaths)
                .filter(([, path]) => path.trim())
                .map(([node, path]) => `${node}:${path.trim()}`);
              if (!map.length) return;
              await api.updateMapping(kind, String(edit.id ?? ""), { map });
              setEdit(null);
              void qc.invalidateQueries({ queryKey: ["maps", kind] });
            }}>Save</Button>
          }
        >
          <div className="grid gap-3">
            {nodes.map((node) => (
              <MappingNodeRow
                key={node}
                kind={kind}
                node={node}
                value={editPaths[node] ?? ""}
                onChange={(path) => setEditPaths((cur) => ({ ...cur, [node]: path }))}
              />
            ))}
          </div>
        </Modal>
      )}
    </div>
  );
}

function MappingNodeRow({
  kind, node, value, onChange,
}: {
  kind: string;
  node: string;
  value: string;
  onChange: (path: string) => void;
}) {
  const hw = useQuery({
    queryKey: ["hw", node, kind],
    queryFn: () => api.nodeHardware(node, kind === "usb" ? "usb" : "pci"),
    enabled: kind !== "dir",
  });
  if (kind === "dir") {
    return (
      <Field label={node}>
        <Input value={value} onChange={(e) => onChange(e.target.value)} placeholder="/mnt/shared" />
      </Field>
    );
  }
  return (
    <Field label={node}>
      <Select value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">None</option>
        {(hw.data?.items ?? []).map((item, i) => {
          const id = kind === "usb"
            ? [item.vendid ?? item.vendor, item.prodid ?? item.product_id].filter(Boolean).join(":") || String(item.id ?? "")
            : String(item.id ?? item.pci ?? "");
          const label = kind === "usb"
            ? String(item.product ?? item.manufacturer ?? item.name ?? id)
            : String(item.device_name ?? item.vendor_name ?? id);
          return <option key={id || i} value={id}>{id}{label && label !== id ? ` ${label}` : ""}</option>;
        })}
      </Select>
      {hw.data && hw.data.supported === false && (
        <p className="mt-1 text-xs text-ink-dim">Inventory is not available; type a path below if needed.</p>
      )}
      {hw.data?.supported === false && (
        <Input className="mt-2" value={value} onChange={(e) => onChange(e.target.value)} placeholder={kind === "usb" ? "1234:5678" : "0000:01:00.0"} />
      )}
    </Field>
  );
}

function parseACMEData(v: unknown): Record<string, string> {
  const text = typeof v === "string" ? v : "";
  const out: Record<string, string> = {};
  for (const line of text.split(/[\n,]/)) {
    const eq = line.indexOf("=");
    if (eq < 1) continue;
    out[line.slice(0, eq).trim()] = line.slice(eq + 1).trim();
  }
  return out;
}

function schemaProperties(item: unknown): { key: string; label: string }[] {
  const rec = asObj(item);
  let props = asObj(rec.properties);
  if (!Object.keys(props).length) props = asObj(asObj(rec.schema).properties);
  if (!Object.keys(props).length) {
    for (const value of Object.values(rec)) {
      const found = asObj(asObj(value).properties);
      if (Object.keys(found).length) {
        props = found;
        break;
      }
    }
  }
  return Object.entries(props).map(([key, spec]) => {
    const s = asObj(spec);
    return { key, label: String(s.title ?? s.description ?? key) };
  });
}

function ACMESchemaFields({
  schema, values, onChange,
}: {
  schema?: FeatureObject;
  values: Record<string, string>;
  onChange: (next: Record<string, string>) => void;
  plugin?: string;
}) {
  if (!schema || schema.supported === false || !schema.item) return null;
  const fields = schemaProperties(schema.item);
  if (!fields.length) {
    const rec = asRecord(schema.item);
    const keys = Object.keys(rec).filter((k) => typeof rec[k] !== "object");
    if (keys.length) {
      return (
        <div className="grid gap-3 md:grid-cols-2">
          {keys.map((key) => (
            <Field key={key} label={key}>
              <Input
                value={values[key] ?? String(rec[key] ?? "")}
                onChange={(e) => onChange({ ...values, [key]: e.target.value })}
              />
            </Field>
          ))}
        </div>
      );
    }
    return <KeyValueTable data={rec} />;
  }
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {fields.map((f) => (
        <Field key={f.key} label={f.label}>
          <Input
            value={values[f.key] ?? ""}
            onChange={(e) => onChange({ ...values, [f.key]: e.target.value })}
          />
        </Field>
      ))}
    </div>
  );
}

function ScheduleAnalyzeTable({ items }: { items: Record<string, unknown>[] }) {
  if (!items.length) return <p className="text-sm text-ink-dim">No upcoming runs.</p>;
  const cols = Object.keys(items[0]);
  return (
    <Card className="overflow-hidden">
      <table className="w-full text-sm">
        <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
          <tr>
            {cols.map((c) => (
              <th key={c} className="px-4 py-2">{c.replace(/_/g, " ")}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {items.map((row, i) => (
            <tr key={i} className="border-t border-line">
              {cols.map((c) => (
                <td key={c} className="px-4 py-2 font-mono text-xs text-ink-dim">{String(row[c] ?? "")}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}

const MATCH_SEVERITIES = ["info", "notice", "warning", "error"] as const;

function parseSeverities(raw: unknown): string[] {
  const parts = String(raw ?? "").split(",").map((s) => s.trim()).filter(Boolean);
  return parts.length ? parts : [...MATCH_SEVERITIES];
}

function SeverityChecks({ value, onChange }: { value: string[]; onChange: (next: string[]) => void }) {
  return (
    <div className="flex flex-wrap gap-3 text-sm text-ink-dim">
      {MATCH_SEVERITIES.map((s) => (
        <label key={s} className="flex items-center gap-1.5">
          <input
            type="checkbox"
            checked={value.includes(s)}
            onChange={(e) => onChange(e.target.checked ? [...value, s] : value.filter((x) => x !== s))}
          />
          {s}
        </label>
      ))}
    </div>
  );
}

function clusterLinks(link0: string, link1: string, link2: string) {
  const out: Record<string, string> = {};
  if (link0) out.link0 = link0;
  if (link1) out.link1 = link1;
  if (link2) out.link2 = link2;
  return out;
}

function ClusterConfigTable({ data, members }: { data: Record<string, unknown>; members?: Record<string, unknown>[] }) {
  const src = data.totem || data.cluster_name || data.nodelist ? data : asRecord(data.data ?? data.item ?? data.config);
  const totem = asRecord(src.totem);
  // Membership comes from /cluster/config/nodes; totem carries only the ring settings.
  const nodeRows = asRecords(src.nodelist).length ? asRecords(src.nodelist) : (members ?? []);
  const name = String(src.cluster_name || totem.cluster_name || "—");
  const votes = nodeRows.reduce((sum, n) => sum + Number(n.quorum_votes ?? n.votes ?? 0), 0);
  const links = ["link0", "link1", "link2"]
    .map((k) => totem[k] != null ? `${k} ${String(totem[k])}` : "")
    .filter(Boolean);
  const iface = asRecord(totem.interface);
  if (!links.length) {
    for (const [k, v] of Object.entries(iface)) {
      const row = asRecord(v);
      const addr = row.bindnetaddr ?? row.address;
      links.push(`link${String(row.linknumber ?? k)}${addr ? ` ${String(addr)}` : ""}`);
    }
  }
  return (
    <div className="space-y-2">
      <table className="w-full text-sm">
        <tbody>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs text-ink-dim">Name</td><td className="px-4 py-1.5">{name}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs text-ink-dim">Nodes</td><td className="px-4 py-1.5">{nodeRows.length || "—"}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs text-ink-dim">Votes</td><td className="px-4 py-1.5">{votes || String(totem.votes ?? "—")}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs text-ink-dim">Links</td><td className="px-4 py-1.5 font-mono text-xs text-ink">{links.join(" · ") || "—"}</td></tr>
          {totem.consensus != null && <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs text-ink-dim">Consensus</td><td className="px-4 py-1.5">{String(totem.consensus)}</td></tr>}
          {totem.link_mode != null && <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs text-ink-dim">Link mode</td><td className="px-4 py-1.5">{String(totem.link_mode)}</td></tr>}
        </tbody>
      </table>
      {nodeRows.length > 0 && (
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Node</th><th>ID</th><th>Votes</th><th>Address</th></tr>
          </thead>
          <tbody>
            {nodeRows.map((n, i) => (
              <tr key={String(n.name || n.node || i)} className="border-t border-line">
                <td className="px-4 py-2">{String(n.name || n.node || "—")}</td>
                <td>{String(n.nodeid ?? n.id ?? "—")}</td>
                <td>{String(n.quorum_votes ?? n.votes ?? "—")}</td>
                <td className="font-mono text-xs text-ink-dim">{String(n.ring0_addr ?? n.link0 ?? n.pve_addr ?? "—")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <Advanced label="Advanced: raw config">
        <CollapsedRaw data={src} />
      </Advanced>
    </div>
  );
}

function QDeviceTable({ items }: { items: Record<string, unknown>[] }) {
  if (!items.length) return <p className="text-sm text-ink-dim">No QDevice configured.</p>;
  return (
    <table className="w-full text-sm">
      <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
        <tr><th className="px-4 py-2">Name</th><th>State</th><th>Votes</th></tr>
      </thead>
      <tbody>
        {items.map((item, i) => (
          <tr key={String(item.name || item.id || i)} className="border-t border-line">
            <td className="px-4 py-2">{String(item.name || item.id || "—")}</td>
            <td>{String(item.state ?? item.status ?? "—")}</td>
            <td>{String(item.votes ?? item.quorum_votes ?? "—")}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function FeatureTable({
  title,
  actions,
  list,
  idKey,
  writable,
  onDelete,
  onEdit,
  onAction,
  actionLabel,
  detailKeys,
  renderDetails,
}: {
  title?: string;
  actions?: ReactNode;
  list?: FeatureList;
  idKey: string;
  writable: boolean;
  onDelete?: (id: string) => Promise<void>;
  onEdit?: (item: Record<string, unknown>) => void;
  onAction?: (id: string, item: Record<string, unknown>) => void | Promise<void>;
  actionLabel?: string;
  detailKeys?: string[];
  renderDetails?: (item: Record<string, unknown>) => ReactNode;
}) {
  return (
    <InventoryCard title={title || "Items"} actions={actions}>
      <table className="w-full text-sm">
        <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
          <tr><th className="px-4 py-2">Kind</th><th>Summary</th><th></th></tr>
        </thead>
        <tbody>
          {!(list?.items ?? []).length && (
            <tr className="border-t border-line">
              <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>None</td>
            </tr>
          )}
          {(list?.items ?? []).map((item, i) => {
            const id = String(item[idKey] || item.name || item.id || i);
            return (
              <tr
                key={id}
                className={`border-t border-line ${onEdit ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
                onClick={onEdit ? () => onEdit(item) : undefined}
              >
                <td className="px-4 py-2 font-medium">{id}</td>
                <td className="max-w-xl px-0 py-2 text-ink-dim">
                  {renderDetails ? renderDetails(item) : <ItemDetails item={item} keys={detailKeys} />}
                </td>
                <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                  {writable && onAction && <Button size="sm" variant="outline" onClick={() => void onAction(id, item)}>{actionLabel || "Action"}</Button>}
                  {writable && onEdit && <Button size="sm" variant="outline" onClick={() => onEdit(item)}>Edit</Button>}
                  {writable && onDelete && <Button size="sm" variant="danger" onClick={() => void onDelete(id)}>Delete</Button>}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </InventoryCard>
  );
}
