import { ObjectACL } from "@/components/access/ObjectACL";
import { ReplicationPanel } from "@/components/guest/ReplicationPanel";
import { ObjectTaskHistory } from "@/components/tasks/ObjectTaskHistory";
import { CephPanel } from "@/components/node/CephPanel";
import { SDNPanel } from "@/components/node/SDNPanel";
import { CertsPanel } from "@/components/node/CertsPanel";
import { DisksPanel } from "@/components/node/DisksPanel";
import { HardwarePanel } from "@/components/node/HardwarePanel";
import { NetworkPanel } from "@/components/node/NetworkPanel";
import { ServicesPanel } from "@/components/node/ServicesPanel";
import { SystemPanel } from "@/components/node/SystemPanel";
import { UpdatesPanel } from "@/components/node/UpdatesPanel";
import { ConsoleView } from "@/components/console/ConsoleView";
import { MetricChart } from "@/components/charts/MetricChart";
import { attachTask } from "@/components/layout/AppShell";
import { Badge, Card, statusTone } from "@/components/ui/card";
import { BarButton, MoreMenu, type MoreItem } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { fleetKeys, nodeKeys } from "@/lib/tasks";
import { storagesForContent } from "@/lib/storage";
import { MarkdownView } from "@/lib/markdown";
import { useCan } from "@/lib/perms";
import { bytes, cpuMeterLabel, cpuPct, uptime } from "@/lib/format";
import { live } from "@/lib/ws";
import type { NetIface, Resource } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { getActiveConnection } from "@/lib/connection";
import { Copy, Cpu, Database, FileKey, HardDrive, History, KeyRound, Network, RefreshCw, Settings, Share2, Terminal, Workflow } from "lucide-react";
import { useEffect, useState } from "react";
import { Segmented, Tabs } from "@/components/ui/tabs";
import { InsightsList } from "@/components/insights/InsightsList";
import type { Insight } from "@/types";
import { Meter } from "@/components/ui/meter";
import { CHART_CPU, CHART_MEM } from "@/lib/chartColors";

const TIMEFRAMES = ["hour", "day", "week", "month", "year"] as const;

const panels = [
  { id: "hardware", label: "Hardware", Icon: Cpu },
  { id: "network", label: "Network", Icon: Network },
  { id: "sdn", label: "SDN", Icon: Share2 },
  { id: "ceph", label: "Ceph", Icon: Database },
  { id: "disks", label: "Disks", Icon: HardDrive },
  { id: "updates", label: "Updates", Icon: RefreshCw },
  { id: "services", label: "Services", Icon: Workflow },
  { id: "system", label: "System", Icon: Settings },
  { id: "certificates", label: "Certificates", Icon: FileKey },
  { id: "console", label: "Console", Icon: Terminal },
  { id: "history", label: "Tasks", Icon: History },
  { id: "replication", label: "Replication", Icon: Copy },
  { id: "permissions", label: "Permissions", Icon: KeyRound },
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


function MetaRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-3 text-[11px]">
      <span className="text-ink-faint">{label}</span>
      <span className="font-mono text-right text-ink-dim">{value}</span>
    </div>
  );
}

function nodeOSName(pveversion?: string, kversion?: string) {
  const ver = (pveversion ?? "").split("/")[1] || "";
  const pve = ver ? `Proxmox VE ${ver}` : "";
  const kernel = (kversion ?? "").replace(/^Linux\s+/i, "").split(/\s+/)[0] || "";
  return [pve, kernel && `Linux ${kernel}`].filter(Boolean).join(" · ");
}

function nodeIPs(ifaces: NetIface[] | undefined): string[] {
  const skip = /^(lo|docker|veth|cni|flannel|calico|fwbr|fwln|fwpr|tap|vmbr\d+\.\d+)/i;
  const list = ifaces ?? [];
  const addrs = (pred: (n: NetIface) => boolean) =>
    list.filter(pred).flatMap((n) => [n.cidr, n.address].filter((v): v is string => !!v && v !== "dhcp"));
  const bridges = addrs((n) => !!n.active && /^vmbr\d+$/.test(n.iface) && !skip.test(n.iface));
  if (bridges.length) return [...new Set(bridges)];
  return [...new Set(addrs((n) => !!n.active && !skip.test(n.iface)))];
}

export function NodeDetailPage() {
  const { node } = useParams({ from: "/app/nodes/$node" });
  const search = useSearch({ from: "/app/nodes/$node" });
  const qc = useQueryClient();
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const insights = useQuery({
    queryKey: ["insights", getActiveConnection()],
    queryFn: api.insights,
    refetchInterval: 60_000,
    staleTime: 30_000,
  });
  // Anything about this node, or about a guest running on it.
  const aboutThisNode = (i: Insight) =>
    !!i.subjects?.some((s) => (s.kind === "node" && s.node === node) || (s.kind === "guest" && s.node === node));
  const nodeInsights = (insights.data?.insights ?? []).filter(aboutThisNode);
  const detail = useQuery({ queryKey: ["node", node], queryFn: () => api.node(node) });
  const nets = useQuery({ queryKey: ["node-net", node], queryFn: () => api.nodeNetwork(node) });
  const [tf, setTf] = useState<(typeof TIMEFRAMES)[number]>("hour");
  const rrd = useQuery({ queryKey: ["node-rrd", node, tf], queryFn: () => api.nodeRrd(node, tf), refetchInterval: 15000 });
  const ioRrd = useQuery({
    queryKey: ["node-io-rrd", node, tf],
    queryFn: () => api.nodeIoRrd(node, tf),
    refetchInterval: 10000,
  });
  const notesQ = useQuery({ queryKey: ["node-config", node], queryFn: () => api.nodeConfig(node) });
  // The open section lives in the URL (?tab=) so Back, reload, and links land on it.
  const panel: Panel | null = panels.find((p) => p.id === search.tab)?.id ?? null;
  const [power, setPower] = useState<"reboot" | "shutdown" | null>(null);
  const [report, setReport] = useState<string | null>(null);
  const [reportMissing, setReportMissing] = useState(false);
  const [bulk, setBulk] = useState<"start" | "stop" | "suspend" | "migrate" | null>(null);
  const [picked, setPicked] = useState<number[]>([]);
  const [bulkError, setBulkError] = useState("");
  const [migrateTarget, setMigrateTarget] = useState("");
  const [migrateStorage, setMigrateStorage] = useState("");
  const [migrateNetwork, setMigrateNetwork] = useState("");
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const [wolMsg, setWolMsg] = useState("");
  const [editNotes, setEditNotes] = useState(false);
  const [notesDraft, setNotesDraft] = useState("");
  const [notesError, setNotesError] = useState("");
  const [shellKind, setShellKind] = useState<"term" | "vnc" | "spice">("term");
  const nav = useNavigate();
  const can = useCan();
  const nodePath = `/nodes/${node}`;
  const canShell = can(nodePath, "Sys.Console");
  const canAudit = can(nodePath, "Sys.Audit") || can(nodePath, "Sys.Modify") || can("/", "Sys.Audit");
  const writable = can(nodePath, "Sys.Modify");
  const canSDN = can("/sdn", "SDN.Audit") || can("/sdn", "SDN.Allocate") || can(nodePath, "Sys.Audit");
  const canCeph = can(nodePath, "Sys.Audit") || can(nodePath, "Sys.Modify") || can("/", "Sys.Audit");
  const canACL = can("/access", "Permissions.Modify") || can("/access", "Sys.Audit") || can("/", "Permissions.Modify");
  const [liveNode, setLiveNode] = useState<Record<string, unknown> | null>(null);

  const visiblePanels = panels.filter((p) => {
    if (p.id === "sdn") return canSDN;
    if (p.id === "ceph") return canCeph;
    if (p.id === "permissions") return canACL;
    if (p.id === "console") return canShell;
    if (p.id === "history") return canAudit;
    if (p.id === "replication") return canAudit;
    return canAudit;
  });

  useEffect(() => {
    setLiveNode(null);
    live.subscribe([`node:${node}`]);
    const off = live.on((topic, data) => {
      if (topic === `node:${node}`) setLiveNode(data as Record<string, unknown>);
    });
    return () => {
      off();
      live.unsubscribe([`node:${node}`]);
    };
  }, [node]);

  useEffect(() => {
    setReport(null);
    setReportMissing(false);
    setBulk(null);
    setPicked([]);
    setBulkError("");
    setEditNotes(false);
    setNotesDraft("");
    setNotesError("");
    setWolMsg("");
  }, [node]);

  const guests = (summary.data?.resources ?? [])
    .filter((r: Resource) => r.node === node && (r.type === "qemu" || r.type === "lxc"))
    .sort((a, b) => (a.name ?? "").localeCompare(b.name ?? "", undefined, { numeric: true }));
  const nodes = (summary.data?.resources ?? []).filter((r: Resource) => r.type === "node" && r.node && r.node !== node).map((r) => r.node as string);
  const nodeRes = (summary.data?.resources ?? []).find((r: Resource) => r.type === "node" && (r.node === node || r.id === `node/${node}`));
  const st = { ...(detail.data ?? {}), ...(liveNode ?? {}) } as {
    cpu?: number;
    uptime?: number;
    memory?: { used: number; total: number };
    pveversion?: string;
    kversion?: string;
    cpuinfo?: { cpus?: number; cores?: number };
  };
  const nodeCores = Number(st.cpuinfo?.cpus ?? st.cpuinfo?.cores ?? nodeRes?.maxcpu) || 0;
  const offline = nodeRes?.status === "offline" || nodeRes?.status === "unknown";
  const notes = notesQ.data ? String(notesQ.data.description ?? "") : "";
  const memUsed = st?.memory?.used ?? nodeRes?.mem;
  const memMax = st?.memory?.total ?? nodeRes?.maxmem;
  const points = rrd.data ?? [];
  const diskPoints = ioRrd.data?.disk ?? [];
  const iopsPoints = ioRrd.data?.iops ?? [];
  const delayKey = hasSeries(points, "iowait") ? "iowait" : hasSeries(points, "wait") ? "wait" : "";
  const osName = nodeOSName(st?.pveversion, st?.kversion);
  const ips = nodeIPs(nets.data);
  const runningCount = guests.filter((g) => g.status === "running").length;
  const moreItems: MoreItem[] = [];
  if (canAudit) {
    moreItems.push({
      id: "report",
      label: "System report",
      onClick: () => {
        void (async () => {
          setReportMissing(false);
          const res = await api.nodeReport(node);
          if (!res.supported) {
            setReportMissing(true);
            setReport(null);
            return;
          }
          setReport(res.report);
        })();
      },
    });
  }
  const canPower = can("/", "VM.PowerMgmt") || writable;
  const canMigrate = can("/", "VM.Migrate") || writable;
  const bulkGuests = guests.filter((g) => !g.template);
  if (bulkGuests.length && (canPower || canMigrate)) {
    moreItems.push({ id: "bulk-head", label: "Bulk actions", heading: true });
    if (canPower) {
      moreItems.push({ id: "start", label: "Start", onClick: () => openBulk("start") });
      moreItems.push({ id: "stop", label: "Stop", danger: true, onClick: () => openBulk("stop") });
      moreItems.push({ id: "suspend", label: "Suspend", onClick: () => openBulk("suspend") });
    }
    if (canMigrate && nodes.length) {
      moreItems.push({ id: "migrate", label: "Migrate", onClick: () => openBulk("migrate") });
    }
  }

  function openBulk(kind: "start" | "stop" | "suspend" | "migrate") {
    setPicked([]);
    setBulkError("");
    if (kind === "migrate") setMigrateTarget(nodes[0] ?? "");
    setBulk(kind);
  }

  function setPanel(next: Panel | null) {
    void nav({ to: "/nodes/$node", params: { node }, search: next ? { tab: next } : {} });
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-xs text-ink-faint">{st?.pveversion || "Node"}</div>
          <h1 className="text-2xl font-semibold">{node}</h1>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <Badge tone={statusTone(nodeRes?.status)}>{nodeRes?.status || "unknown"}</Badge>
            <span className="text-xs text-ink-faint">up {uptime(st?.uptime)}</span>
          </div>
          {wolMsg && <p className="mt-1 text-sm text-ink-dim">{wolMsg}</p>}
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          {writable && offline && (
            <BarButton
              accent
              onClick={async () => {
                setWolMsg("");
                const res = await api.wakeOnLan(node);
                setWolMsg(res.supported ? `Wake sent${res.mac ? ` (${res.mac})` : ""}` : "Wake-on-LAN is not available on this node.");
              }}
            >
              Wake-on-LAN
            </BarButton>
          )}
          {writable && !offline && (
            <>
              <BarButton onClick={() => setPower("reboot")}>Reboot</BarButton>
              <BarButton onClick={() => setPower("shutdown")}>Shutdown</BarButton>
            </>
          )}
          <MoreMenu items={moreItems} />
        </div>
      </div>

      <Tabs<Panel | "summary">
        items={["summary", ...visiblePanels.map((p) => p.id)]}
        value={panel ?? "summary"}
        onChange={(t) => setPanel(t === "summary" ? null : t)}
        label={(t) => (t === "summary" ? "Summary" : visiblePanels.find((p) => p.id === t)?.label ?? t)}
      />

      {!panel && (
      <>
      {nodeInsights.length > 0 && (
        <Card className="overflow-hidden">
          <h2 className="px-5 pt-4 text-sm font-semibold">Needs attention</h2>
          <InsightsList report={insights.data} filter={aboutThisNode} />
        </Card>
      )}
      <div className="grid gap-4 lg:grid-cols-2">
        <Card className="p-5">
          <p className="text-xs font-medium text-ink-dim">Live</p>
          <div className="mt-4 space-y-4">
            <Meter label={cpuMeterLabel(nodeCores)} used={cpuPct(st?.cpu ?? nodeRes?.cpu)} total={100} text={`${cpuPct(st?.cpu ?? nodeRes?.cpu).toFixed(0)}%`} />
            <Meter label="Memory" used={memUsed} total={memMax} text={`${bytes(memUsed)} / ${bytes(memMax)}`} />
            <Meter label="Disk" used={nodeRes?.disk} total={nodeRes?.maxdisk} text={`${bytes(nodeRes?.disk)} / ${bytes(nodeRes?.maxdisk)}`} />
          </div>
          <div className="mt-5 space-y-2 border-t border-white/[0.06] pt-4">
            <MetaRow label="Operating system" value={osName || "—"} />
            <MetaRow label="IP address" value={ips.length ? ips.join(", ") : "—"} />
          </div>
          <div className="mt-5 border-t border-white/[0.06] pt-4">
            <p className="text-[11px] text-ink-dim">{runningCount} running · {guests.length} guests</p>
          </div>
        </Card>

        <Card className="flex min-h-[220px] flex-col p-5">
          <div className="mb-3 flex items-center justify-between gap-2">
            <p className="text-xs font-medium text-ink-dim">Notes</p>
            {writable && !editNotes && (
              <button
                type="button"
                className="font-mono text-[11px] text-ink-dim hover:text-ink"
                onClick={() => { setNotesDraft(notes); setNotesError(""); setEditNotes(true); }}
              >
                Edit
              </button>
            )}
          </div>
          {editNotes ? (
            <div className="flex min-h-0 flex-1 flex-col gap-3">
              <Textarea
                className="min-h-32 flex-1 font-mono text-[12px]"
                value={notesDraft}
                onChange={(e) => setNotesDraft(e.target.value)}
                placeholder={"Lite markdown — headings, lists, tables, bold, links."}
              />
              {notesError && <p className="text-xs text-rose-400">{notesError}</p>}
              <div className="flex gap-2">
                <Button
                  size="sm"
                  onClick={async () => {
                    setNotesError("");
                    try {
                      await api.updateNodeConfig(node, { description: notesDraft });
                      setEditNotes(false);
                      void qc.invalidateQueries({ queryKey: ["node-config", node] });
                    } catch (e) {
                      setNotesError(e instanceof Error ? e.message : "Save failed");
                    }
                  }}
                >
                  Save
                </Button>
                <Button size="sm" variant="outline" onClick={() => setEditNotes(false)}>Cancel</Button>
              </div>
            </div>
          ) : (
            <div className="min-h-0 flex-1 overflow-y-auto">
              <MarkdownView text={notes} />
            </div>
          )}
        </Card>

      </div>

      <Card className="p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-xs font-medium text-ink-dim">Historical</p>
            <h2 className="mt-1 text-sm font-medium">{node}</h2>
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

      {reportMissing && <p className="text-sm text-ink-dim">System report is not available on this node.</p>}
      </>
      )}

      {report !== null && (
        <Card className="p-4">
          <div className="mb-2 flex justify-between">
            <h2 className="text-xs font-medium text-ink-dim">System report</h2>
            <BarButton onClick={() => {
              const blob = new Blob([report], { type: "text/plain" });
              const url = URL.createObjectURL(blob);
              const a = document.createElement("a");
              a.href = url;
              a.download = `${node}-report.txt`;
              a.click();
              URL.revokeObjectURL(url);
            }}>Download</BarButton>
          </div>
          <pre className="max-h-96 overflow-auto font-mono text-xs text-ink-dim">{report}</pre>
        </Card>
      )}

      {panel && (
          <div className="hv-form-stack space-y-4">
            {panel === "hardware" && <HardwarePanel key={node} node={node} />}
            {panel === "network" && <NetworkPanel key={node} node={node} writable={writable} />}
            {panel === "sdn" && <SDNPanel key={node} node={node} />}
            {panel === "ceph" && <CephPanel key={node} node={node} writable={writable} />}
            {panel === "disks" && <DisksPanel key={node} node={node} writable={writable} />}
            {panel === "updates" && <UpdatesPanel key={node} node={node} writable={writable} />}
            {panel === "services" && <ServicesPanel key={node} node={node} writable={writable} />}
            {panel === "system" && <SystemPanel key={node} node={node} writable={writable} />}
            {panel === "certificates" && <CertsPanel key={node} node={node} writable={writable} />}
            {panel === "permissions" && <ObjectACL key={nodePath} path={nodePath} />}
            {panel === "history" && <ObjectTaskHistory key={node} node={node} />}
            {panel === "replication" && (
              <ReplicationPanel key={node} node={node} nodes={(summary.data?.resources ?? []).filter((r: Resource) => r.type === "node")} sourceNode={node} />
            )}
            {panel === "console" && (
              <div className="space-y-2">
                <div className="flex flex-wrap gap-2">
                  {(["term", "vnc", "spice"] as const).map((k) => (
                    <BarButton key={k} selected={shellKind === k} onClick={() => setShellKind(k)}>
                      {k === "term" ? "Serial / term" : k.toUpperCase()}
                    </BarButton>
                  ))}
                </div>
                <ConsoleView key={`${node}:${shellKind}`} node={node} kind={shellKind} />
              </div>
            )}
          </div>
      )}

      {power && (
        <ConfirmDialog
          title={`${power === "reboot" ? "Reboot" : "Shutdown"} ${node}?`}
          body="Guests on this node will stop unless HA relocates them."
          danger
          confirmLabel={power === "reboot" ? "Reboot" : "Shutdown"}
          onClose={() => setPower(null)}
          onConfirm={async () => {
            attachTask((await api.nodeCommand(node, power)).upid, nodeKeys(node));
            setPower(null);
          }}
        />
      )}
      {bulk && (
        <Modal
          title={`Bulk ${bulk === "start" ? "Start" : bulk === "stop" ? "Stop" : bulk === "suspend" ? "Suspend" : "Migrate"}`}
          onClose={() => setBulk(null)}
          size="lg"
          footer={
            <>
              <Button size="sm" variant="ghost" onClick={() => setBulk(null)}>Cancel</Button>
              <Button
                size="sm"
                variant={bulk === "stop" ? "danger" : "primary"}
                disabled={!picked.length || (bulk === "migrate" && !migrateTarget)}
                onClick={async () => {
                  setBulkError("");
                  const selected = bulkGuests
                    .filter((g) => picked.includes(g.vmid ?? 0))
                    .map((g) => ({ vmid: g.vmid ?? 0, node, type: g.type }));
                  try {
                    const extras: Record<string, unknown> = {};
                    if (bulk === "migrate") {
                      extras.target = migrateTarget;
                      if (migrateStorage) extras.targetStorage = migrateStorage;
                      if (migrateNetwork) extras.migrationNetwork = migrateNetwork;
                    }
                    const res = await api.bulk(bulk, selected, extras);
                    (res.upids ?? []).forEach((upid) => attachTask(upid, [...fleetKeys(), ...picked.map((vmid) => ["guest", vmid] as const)]));
                    setBulk(null);
                  } catch (e) {
                    setBulkError(e instanceof Error ? e.message : "Bulk action failed");
                  }
                }}
              >
                {bulk === "start" ? "Start" : bulk === "stop" ? "Stop" : bulk === "suspend" ? "Suspend" : "Migrate"} {picked.length || ""}
              </Button>
            </>
          }
        >
          <p className="text-sm text-ink-dim">
            {bulk === "start" && "Start the selected guests on this node."}
            {bulk === "stop" && "Hard-stop the selected guests. Unsaved work may be lost."}
            {bulk === "suspend" && "Suspend the selected guests. Memory stays allocated until you resume them."}
            {bulk === "migrate" && "Move the selected guests to the target node."}
          </p>
          <div className="flex gap-3 text-xs">
            <button type="button" className="text-ink-dim hover:text-ink" onClick={() => setPicked(bulkGuests.map((g) => g.vmid ?? 0).filter(Boolean))}>
              Select all
            </button>
            <button type="button" className="text-ink-dim hover:text-ink" onClick={() => setPicked([])}>
              Select none
            </button>
          </div>
          <div className="max-h-72 space-y-1 overflow-y-auto rounded-lg border border-white/10 p-2">
            {bulkGuests.map((g) => {
              const id = g.vmid ?? 0;
              const on = picked.includes(id);
              return (
                <label key={g.id} className="flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-white/[0.04]">
                  <input
                    type="checkbox"
                    className="accent-accent"
                    checked={on}
                    onChange={() => setPicked((cur) => (on ? cur.filter((n) => n !== id) : [...cur, id]))}
                  />
                  <span className="w-10 font-mono text-xs text-ink-dim">{g.vmid}</span>
                  <span className="min-w-0 flex-1 truncate">{g.name || g.type}</span>
                  <span className="font-mono text-[10px] uppercase text-ink-faint">{g.type}</span>
                  <Badge tone={statusTone(g.status)}>{g.status}</Badge>
                </label>
              );
            })}
            {bulkGuests.length === 0 && <p className="px-2 py-3 text-sm text-ink-dim">No guests on this node.</p>}
          </div>
          {bulk === "migrate" && (
            <div className="space-y-3">
              <Field label="Target node">
                <Select value={migrateTarget} onChange={(e) => setMigrateTarget(e.target.value)}>
                  {nodes.map((n) => <option key={n}>{n}</option>)}
                </Select>
              </Field>
              <Field label="Destination storage">
                <Select value={migrateStorage} onChange={(e) => setMigrateStorage(e.target.value)}>
                  <option value="">Keep current</option>
                  {storagesForContent(storages.data ?? [], "images", migrateTarget).map((s) => (
                    <option key={s.storage} value={s.storage}>{s.storage}</option>
                  ))}
                </Select>
              </Field>
              <Field label="Migration network">
                <Input value={migrateNetwork} onChange={(e) => setMigrateNetwork(e.target.value)} placeholder="10.0.0.0/24" />
              </Field>
            </div>
          )}
          {bulkError && <p className="text-xs text-rose-400">{bulkError}</p>}
        </Modal>
      )}
    </div>
  );
}
