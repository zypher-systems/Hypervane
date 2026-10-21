import { ObjectACL } from "@/components/access/ObjectACL";
import { BackupPanel } from "@/components/backups/BackupPanel";
import { ObjectTaskHistory } from "@/components/tasks/ObjectTaskHistory";
import { AgentPanel } from "@/components/guest/AgentPanel";
import { CloudInitPanel } from "@/components/guest/CloudInitPanel";
import { HardwareTab } from "@/components/guest/HardwareTab";
import { appliedPending, PendingChanges } from "@/components/guest/PendingChanges";
import { describePending } from "@/lib/pending";
import { ReplicationPanel } from "@/components/guest/ReplicationPanel";
import { SnapshotsPanel } from "@/components/guest/SnapshotsPanel";
import { HAPanel } from "@/components/ha/HAPanel";
import { FirewallPanel } from "@/components/firewall/FirewallPanel";
import { ConsoleView } from "@/components/console/ConsoleView";
import { MetricChart } from "@/components/charts/MetricChart";
import { attachTask } from "@/components/layout/AppShell";
import { Badge, Card, statusTone } from "@/components/ui/card";
import { BarButton, MoreMenu } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { CloneDialog } from "@/components/guest/CloneDialog";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { TagList } from "@/components/ui/tags";
import { api } from "@/lib/api";
import { pveMigrateToken } from "@/lib/pveToken";
import { guestKeys } from "@/lib/tasks";
import { storagesForContent } from "@/lib/storage";
import { configDigest, parseKV } from "@/lib/guestHardware";
import { MarkdownView } from "@/lib/markdown";
import { useCan } from "@/lib/perms";
import { bytes, cpuMeterLabel, cpuPct, daysSince, formatWhen, uptime } from "@/lib/format";
import { getActiveConnection } from "@/lib/connection";
import { InsightsList } from "@/components/insights/InsightsList";
import { live } from "@/lib/ws";
import type { GuestStatus, Insight, Resource } from "@/types";
import { haSid, haTone } from "@/types";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { Archive, Bot, Camera, Cloud, Copy, Cpu, HeartPulse, History, KeyRound, RotateCw, Shield, Terminal } from "lucide-react";
import { useEffect, useState } from "react";
import { Segmented, Tabs } from "@/components/ui/tabs";
import { Meter } from "@/components/ui/meter";
import { CHART_CPU, CHART_MEM } from "@/lib/chartColors";

type ConfirmKind = "shutdown" | "stop" | "reboot" | "reset" | "suspend" | "hibernate" | "template";

const CONFIRM: Record<ConfirmKind, { title: (name: string) => string; body: string; label: string }> = {
  shutdown: {
    title: (name) => `Shutdown ${name}?`,
    body: "Asks the guest OS to power off. Unsaved work may be lost if the OS ignores it.",
    label: "Shutdown",
  },
  stop: {
    title: (name) => `Stop ${name}?`,
    body: "Hard stop. Unsaved work in the guest may be lost.",
    label: "Stop",
  },
  reboot: {
    title: (name) => `Reboot ${name}?`,
    body: "Asks the guest OS to reboot. Unsaved work may be lost.",
    label: "Reboot",
  },
  reset: {
    title: (name) => `Reset ${name}?`,
    body: "Hard reset. The guest will restart immediately.",
    label: "Reset",
  },
  suspend: {
    title: (name) => `Suspend ${name}?`,
    body: "Pauses the guest. Memory stays allocated on the node until you resume.",
    label: "Suspend",
  },
  hibernate: {
    title: (name) => `Hibernate ${name}?`,
    body: "Suspends the guest to disk. Memory is written out and freed on the node until you resume.",
    label: "Hibernate",
  },
  template: {
    title: (name) => `Convert ${name} to a template?`,
    body: "The guest must be stopped. This cannot be undone from Hypervane without cloning.",
    label: "Convert",
  },
};

const panels = [
  { id: "hardware", label: "Hardware", Icon: Cpu },
  { id: "console", label: "Console", Icon: Terminal },
  { id: "snapshots", label: "Snapshots", Icon: Camera },
  { id: "backup", label: "Backup", Icon: Archive },
  { id: "replication", label: "Replication", Icon: Copy },
  { id: "ha", label: "HA", Icon: HeartPulse },
  { id: "firewall", label: "Firewall", Icon: Shield },
  { id: "cloud-init", label: "Cloud-init", Icon: Cloud },
  { id: "agent", label: "Agent", Icon: Bot },
  { id: "history", label: "Tasks", Icon: History },
  { id: "permissions", label: "Permissions", Icon: KeyRound },
] as const;
type Panel = (typeof panels)[number]["id"];
const TIMEFRAMES = ["hour", "day", "week", "month", "year"] as const;

function hasKey(points: Record<string, number>[], key: string) {
  return points.some((p) => p[key] !== undefined && Number.isFinite(Number(p[key])));
}

function agentFsList(raw: unknown): Record<string, unknown>[] {
  let cur: unknown = raw;
  for (let i = 0; i < 4; i++) {
    if (Array.isArray(cur)) {
      return cur.filter((item): item is Record<string, unknown> => !!item && typeof item === "object" && !Array.isArray(item));
    }
    if (cur && typeof cur === "object") {
      const rec = cur as Record<string, unknown>;
      if (rec.result !== undefined) {
        cur = rec.result;
        continue;
      }
      if (rec.data !== undefined) {
        cur = rec.data;
        continue;
      }
    }
    break;
  }
  return [];
}

function unwrapAgent(raw: unknown): unknown {
  let cur: unknown = raw;
  for (let i = 0; i < 4; i++) {
    if (!cur || typeof cur !== "object" || Array.isArray(cur)) return cur;
    const rec = cur as Record<string, unknown>;
    if (rec.result !== undefined) {
      cur = rec.result;
      continue;
    }
    if (rec.data !== undefined) {
      cur = rec.data;
      continue;
    }
    break;
  }
  return cur;
}

function agentOSName(raw: unknown): string {
  const rec = unwrapAgent(raw);
  if (!rec || typeof rec !== "object" || Array.isArray(rec)) return "";
  const o = rec as Record<string, unknown>;
  const pretty = String(o["pretty-name"] ?? o.pretty_name ?? "").trim();
  if (pretty) return pretty;
  const name = String(o.name ?? "").trim();
  const version = String(o.version ?? o["version-id"] ?? "").trim();
  return [name, version].filter(Boolean).join(" ");
}

function agentDiskUsed(raw: unknown): number {
  let used = 0;
  for (const f of agentFsList(raw)) {
    const typ = String(f.type ?? "");
    if (/^(iso9660|udf|nfs|cifs|smb|9p|squashfs|tmpfs|devtmpfs|overlay|proc|sysfs|devfs)$/i.test(typ)) continue;
    const u = Number(f["used-bytes"] ?? f.used ?? 0);
    if (Number.isFinite(u) && u > 0) used += u;
  }
  return used;
}

function usableIP(addr: string) {
  if (!addr || addr === "127.0.0.1" || addr === "::1") return false;
  if (addr.startsWith("fe80:") || addr.startsWith("169.254.")) return false;
  return true;
}

function normalizeMac(s: string) {
  return s.toLowerCase().replace(/-/g, ":");
}

function looksLikeMac(s: string) {
  return /^([0-9a-f]{2}:){5}[0-9a-f]{2}$/i.test(s.trim());
}

const NIC_MODELS = ["virtio", "e1000", "e1000e", "rtl8139", "vmxnet3", "virtio-net"];
const CONTAINER_IF = /^(lo|docker\d*|br-[0-9a-f]{6,}|veth|cni|flannel|calico|kube|weave|virbr|lxc|podman|cni-podman|dummy|tunl|sit|ip6tnl)/i;
const HOST_IF = /^(eth\d+|ens\d+|enp\d+s\d+|eno\d+|em\d+|bond\d+|br\d+|vmbr\d+|wlan\d+)$/i;

function configMACs(cfg: Record<string, unknown>): string[] {
  const macs: string[] = [];
  for (const [k, v] of Object.entries(cfg)) {
    if (!/^net\d+$/.test(k)) continue;
    const { extras } = parseKV(String(v ?? ""));
    for (const key of ["macaddr", "hwaddr", ...NIC_MODELS]) {
      const c = extras[key];
      if (c && looksLikeMac(c)) macs.push(normalizeMac(c));
    }
  }
  return macs;
}

function nicAddrs(n: Record<string, unknown>): string[] {
  const addrs = n["ip-addresses"] ?? n.ip_addresses ?? n.ip;
  const list = Array.isArray(addrs) ? addrs : addrs ? [addrs] : [];
  const ips: string[] = [];
  for (const item of list) {
    const addr = typeof item === "string"
      ? item.split("/")[0]
      : String((item && typeof item === "object" ? (item as Record<string, unknown>)["ip-address"] ?? (item as Record<string, unknown>).ip : "") ?? "").split("/")[0];
    if (usableIP(addr) && !ips.includes(addr)) ips.push(addr);
  }
  return ips;
}

function agentHostIPs(raw: unknown, cfg: Record<string, unknown>): string[] {
  const nics = agentFsList(raw);
  const hostMacs = configMACs(cfg);
  const collect = (pred: (n: Record<string, unknown>) => boolean) => {
    const ips: string[] = [];
    for (const n of nics) {
      if (!pred(n)) continue;
      for (const ip of nicAddrs(n)) if (!ips.includes(ip)) ips.push(ip);
    }
    return ips.sort((a, b) => Number(a.includes(":")) - Number(b.includes(":")));
  };
  if (hostMacs.length) {
    const byMac = collect((n) => {
      const mac = normalizeMac(String(n["hardware-address"] ?? n.hardware_address ?? ""));
      return !!mac && hostMacs.includes(mac);
    });
    if (byMac.length) return byMac;
  }
  return collect((n) => {
    const name = String(n.name ?? "");
    return HOST_IF.test(name) && !CONTAINER_IF.test(name);
  });
}

function configIPs(cfg: Record<string, unknown>): string[] {
  const ips: string[] = [];
  for (const [k, v] of Object.entries(cfg)) {
    if (!/^net\d+$/.test(k)) continue;
    const raw = String(v ?? "");
    for (const key of ["ip", "ip6"]) {
      const m = raw.match(new RegExp(`(?:^|,)${key}=([^,]+)`));
      const addr = (m?.[1] ?? "").split("/")[0];
      if (addr && addr !== "dhcp" && addr !== "auto" && usableIP(addr) && !ips.includes(addr)) ips.push(addr);
    }
  }
  return ips;
}

function MetaRow({ label, value, on }: { label: string; value: string; on?: boolean }) {
  return (
    <div className="flex items-start justify-between gap-3 text-[11px]">
      <span className="text-ink-faint">{label}</span>
      <span className={`font-mono text-right ${on === false ? "text-ink-faint" : "text-ink-dim"}`}>{value}</span>
    </div>
  );
}

function guestMemToBytes(points: Record<string, number>[]) {
  const max = Math.max(0, ...points.map((p) => Number(p.mem)).filter((n) => Number.isFinite(n)));
  if (max > 1e9) return (v: number) => v;
  if (max > 1e6) return (v: number) => v * 1024;
  return (v: number) => v * 1024 * 1024;
}


export function GuestDetailPage() {
  const { vmid } = useParams({ from: "/app/guests/$vmid" });
  const search = useSearch({ from: "/app/guests/$vmid" });
  const id = Number(vmid);
  const qc = useQueryClient();
  const guest = useQuery({
    queryKey: ["guest", id, search.node, search.type],
    queryFn: () => api.guest(id, search.node, search.type),
  });
  const snaps = useQuery({
    queryKey: ["snaps", id],
    queryFn: () => api.snapshots(id, search.node, search.type),
  });
  const insights = useQuery({
    queryKey: ["insights", getActiveConnection()],
    queryFn: api.insights,
    refetchInterval: 60_000,
    staleTime: 30_000,
  });
  const snapFeat = useQuery({
    queryKey: ["feature", id, "snapshot"],
    queryFn: () => api.guestFeature(id, "snapshot", search.node, search.type),
  });
  const cloneFeat = useQuery({
    queryKey: ["feature", id, "clone"],
    queryFn: () => api.guestFeature(id, "clone", search.node, search.type),
  });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  // The open section lives in the URL (?tab=) so Back, reload, and links land on it.
  const panel: Panel | null = panels.find((p) => p.id === search.tab)?.id ?? null;
  const [tf, setTf] = useState<(typeof TIMEFRAMES)[number]>("hour");
  const rrd = useQuery({
    queryKey: ["guest-rrd", id, tf],
    queryFn: () => api.guestRrd(id, tf, search.node, search.type),
    refetchInterval: 15000,
  });
  const ioRrd = useQuery({
    queryKey: ["guest-io-rrd", id, tf],
    queryFn: () => api.guestIoRrd(id, tf, search.node, search.type),
    refetchInterval: 10000,
  });
  const can = useCan();
  const vmPath = `/vms/${id}`;
  const canPower = can(vmPath, "VM.PowerMgmt");
  const canClone = can(vmPath, "VM.Clone");
  const canMigrate = can(vmPath, "VM.Migrate");
  const canConfig = can(vmPath, "VM.Config");
  const canSnapshot = can(vmPath, "VM.Snapshot");
  const canConsole = can(vmPath, "VM.Console");
  const canMonitor = can(vmPath, "VM.Monitor") || canConsole;
  const canBackup = can(vmPath, "VM.Backup");
  const canHA = can("/", "Sys.Modify") || can("/", "Sys.Console");
  const canAgent = can(vmPath, "VM.GuestAgent") || canConfig;
  const canACL = can("/access", "Permissions.Modify") || can("/access", "Sys.Audit") || can("/", "Permissions.Modify");
  const visiblePanels = panels.filter((p) => {
    // Viewing hardware only needs audit rights; the panel hides its edit actions itself.
    if (p.id === "hardware") return canConfig || can(vmPath, "VM.Audit");
    if (p.id === "cloud-init") return canConfig;
    if (p.id === "agent") return canAgent;
    if (p.id === "backup") return canBackup;
    if (p.id === "replication") return can(vmPath, "VM.Replicate") || canBackup || can("/", "Sys.Modify");
    if (p.id === "ha") return canHA;
    if (p.id === "firewall") return can(vmPath, "VM.Config.Network") || can(vmPath, "VM.Audit");
    if (p.id === "snapshots") return canSnapshot || can(vmPath, "VM.Audit");
    if (p.id === "console") return canConsole;
    if (p.id === "history") return can(vmPath, "VM.Audit") || canConfig;
    if (p.id === "permissions") return canACL;
    return true;
  });
  const [confirm, setConfirm] = useState<ConfirmKind | null>(null);
  const [dialog, setDialog] = useState<"clone" | "migrate" | "remote-migrate" | "relocate" | "remove" | null>(null);
  const nav = useNavigate();
  const setPanel = (next: Panel | null) =>
    void nav({ to: "/guests/$vmid", params: { vmid }, search: { node: search.node, type: search.type, tab: next ?? undefined } });
  const [liveStatus, setLiveStatus] = useState<GuestStatus | null>(null);
  const node = guest.data?.node ?? search.node ?? "";
  const type = guest.data?.type ?? search.type ?? "qemu";
  const qemuRunning = (guest.data?.type ?? search.type) === "qemu" && !!node && (liveStatus ?? guest.data?.status)?.status === "running";
  const agentLive = useQuery({
    queryKey: ["guest-agent-live", id, node],
    queryFn: async () => {
      const [fs, nics, os] = await Promise.all([
        api.agentCommand(id, "get-fsinfo", node, type).catch(() => null),
        api.agentCommand(id, "network-get-interfaces", node, type).catch(() => null),
        api.agentCommand(id, "get-osinfo", node, type).catch(() => null),
      ]);
      return { fs, nics, os };
    },
    enabled: qemuRunning,
    refetchInterval: 15000,
    retry: false,
  });
  const haRes = useQuery({
    queryKey: ["ha-resource", haSid(type, id)],
    queryFn: () => api.haResource(haSid(type, id)),
    enabled: Number.isFinite(id),
    retry: false,
  });
  const replJobs = useQuery({
    queryKey: ["replication"],
    queryFn: api.replication,
    retry: false,
  });
  const [editNotes, setEditNotes] = useState(false);
  const [notesDraft, setNotesDraft] = useState("");
  const [notesError, setNotesError] = useState("");
  useEffect(() => {
    setLiveStatus(null);
    if (!node) return;
    const topic = `guest:${node}:${type}:${id}`;
    live.subscribe([topic]);
    const off = live.on((t, data) => {
      if (t === topic) setLiveStatus(data as GuestStatus);
    });
    return () => {
      off();
      live.unsubscribe([topic]);
    };
  }, [node, type, id]);

  useEffect(() => {
    setDialog(null);
    setConfirm(null);
    setEditNotes(false);
    setNotesDraft("");
    setNotesError("");
  }, [id]);

  const power = useMutation({
    mutationFn: (action: string) => api.power(id, action, node, type),
    onSuccess: (res) => {
      attachTask(res.upid, guestKeys(id));
    },
  });

  const status = liveStatus ?? guest.data?.status;
  const cfg = guest.data?.config ?? {};
  const points = rrd.data ?? [];
  const resource = (summary.data?.resources ?? []).find((r: Resource) => r.vmid === id);
  const diskUsed = agentDiskUsed(agentLive.data?.fs) || status?.disk || resource?.disk || 0;
  const diskMax = status?.maxdisk || resource?.maxdisk || 0;
  const diskText = diskUsed > 0 || type !== "qemu" ? `${bytes(diskUsed)} / ${bytes(diskMax)}` : `— / ${bytes(diskMax)}`;
  const ips = (() => {
    const fromAgent = agentHostIPs(agentLive.data?.nics, cfg);
    return fromAgent.length ? fromAgent : configIPs(cfg);
  })();
  const osName = agentOSName(agentLive.data?.os);
  const nodes = (summary.data?.resources ?? []).filter((r: Resource) => r.type === "node");
  const clusterHA = (summary.data?.resources ?? []).find((r: Resource) => r.vmid === id)?.hastate;
  const haEnabled = (!haRes.isError && !!haRes.data) || !!clusterHA;
  const aboutThisGuest = (i: Insight) => !!i.subjects?.some((s) => s.kind === "guest" && s.vmid === id);
  const guestInsights = (insights.data?.insights ?? []).filter(aboutThisGuest);
  const lastBackupAt = insights.data?.lastBackups?.[String(id)];
  const notInJob = guestInsights.some((i) => i.id === "no-backup-job");
  const realSnaps = (snaps.data ?? []).filter((s) => s.name !== "current");
  const oldestSnap = realSnaps.reduce((min, s) => (s.snaptime && (!min || s.snaptime < min) ? s.snaptime : min), 0);
  const backupText = lastBackupAt
    ? formatWhen(lastBackupAt)
    : notInJob
      ? "Not in any backup job"
      : insights.data?.lastBackups || insights.data?.backups
        ? "Not in the last 36 h"
        : "—";
  const snapText = realSnaps.length
    ? `${realSnaps.length}${oldestSnap ? `, oldest ${daysSince(oldestSnap)} d` : ""}`
    : "None";
  const replActive = (replJobs.data ?? []).filter((j) => Number(j.guest) === id && !j.disable);
  const replEnabled = replActive.length > 0;
  const notes = String(cfg.description ?? "");

  const iopsPoints = ioRrd.data?.iops ?? [];
  const delayPoints = hasKey(iopsPoints, "iowait") ? iopsPoints : [];
  const guestName = String(cfg.name || cfg.hostname || status?.name || `Guest ${id}`);
  const isTemplate = Number(cfg.template) === 1;
  const running = status?.status === "running";
  const paused = status?.status === "paused" || status?.status === "suspended";
  const stopped = !running && !paused;
  const canRemove = can(vmPath, "VM.Allocate");
  const pendingItems = appliedPending(guest.data?.pending);
  const moreItems = [
    canPower && !isTemplate && running
      ? { id: "stop", label: "Stop (hard)", danger: true, onClick: () => setConfirm("stop") }
      : null,
    canPower && !isTemplate && running && type === "qemu"
      ? { id: "reset", label: "Reset", danger: true, onClick: () => setConfirm("reset") }
      : null,
    canPower && !isTemplate && running
      ? { id: "suspend", label: "Suspend", onClick: () => setConfirm("suspend") }
      : null,
    canPower && !isTemplate && running && type === "qemu"
      ? { id: "hibernate", label: "Hibernate", onClick: () => setConfirm("hibernate") }
      : null,
    canRemove && !isTemplate
      ? { id: "template", label: "Convert to template", onClick: () => setConfirm("template") }
      : null,
    canMigrate
      ? { id: "remote-migrate", label: "Remote migrate", onClick: () => setDialog("remote-migrate") }
      : null,
    clusterHA && canHA
      ? { id: "relocate", label: "Relocate HA", onClick: () => setDialog("relocate") }
      : null,
    // Destructive and rare, so it lives at the bottom of the menu, not in the bar.
    canRemove ? { id: "remove", label: "Remove…", danger: true, onClick: () => setDialog("remove") } : null,
  ].filter((item): item is { id: string; label: string; danger?: boolean; onClick: () => void } => !!item);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="text-xs text-ink-dim">{type.toUpperCase()} · {node} · {id}</div>
          <h1 className="text-2xl font-semibold">{String(cfg.name || cfg.hostname || status?.name || `Guest ${id}`)}</h1>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <Badge tone={statusTone(status?.status)}>{status?.status || "unknown"}</Badge>
            {pendingItems.length > 0 && <Badge tone="amber">pending restart</Badge>}
            {Number(cfg.template) === 1 && <Badge tone="teal">template</Badge>}
            {clusterHA && <Badge tone={haTone(clusterHA)}>{clusterHA}</Badge>}
            <TagList raw={cfg.tags === undefined ? undefined : String(cfg.tags)} />
            <span className="text-xs text-ink-dim">up {uptime(status?.uptime)}</span>
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          {canPower && !isTemplate && running && (
            <>
              <BarButton disabled={power.isPending} onClick={() => setConfirm("shutdown")}>Shut down</BarButton>
              <BarButton disabled={power.isPending} onClick={() => setConfirm("reboot")}>Reboot</BarButton>
            </>
          )}
          {canPower && !isTemplate && paused && (
            <>
              <BarButton accent disabled={power.isPending} onClick={() => power.mutate("resume")}>Resume</BarButton>
              <BarButton disabled={power.isPending} onClick={() => setConfirm("stop")}>Stop</BarButton>
            </>
          )}
          {canPower && !isTemplate && stopped && (
            <BarButton accent disabled={power.isPending} onClick={() => power.mutate("start")}>Start</BarButton>
          )}
          {(canClone && cloneFeat.data?.hasFeature !== false) || canMigrate || moreItems.length > 0 ? (
            <div className="mx-0.5 hidden h-7 w-px bg-white/15 sm:block" />
          ) : null}
          {canClone && cloneFeat.data?.hasFeature !== false && (
            <BarButton onClick={() => setDialog("clone")}>Clone</BarButton>
          )}
          {canMigrate && <BarButton onClick={() => setDialog("migrate")}>Migrate</BarButton>}
          <MoreMenu items={moreItems} />
          {canConsole && running && !isTemplate && (
            <BarButton accent onClick={() => setPanel("console")}>Open console</BarButton>
          )}
        </div>
      </div>

      {pendingItems.length > 0 && (
        <div className="flex items-start gap-3 rounded-lg border border-warn/30 bg-warn/[0.06] px-3 py-2.5 text-sm">
          <RotateCw className="mt-0.5 h-4 w-4 shrink-0 text-warn" />
          <div className="min-w-0">
            <p className="font-medium text-warn">Pending restart</p>
            <p className="text-ink-dim">
              {pendingItems.map((p) => describePending(p, cfg).summary).join(" · ")} will apply after this guest restarts.
            </p>
          </div>
        </div>
      )}

      <Tabs<Panel | "summary">
        items={["summary", ...visiblePanels.map((p) => p.id)]}
        value={panel ?? "summary"}
        onChange={(t) => setPanel(t === "summary" ? null : t)}
        label={(t) => (t === "summary" ? "Summary" : visiblePanels.find((p) => p.id === t)?.label ?? t)}
      />

      {!panel && (
      <div className="space-y-4">
          {guestInsights.length > 0 && (
            <Card className="overflow-hidden">
              <h2 className="px-5 pt-4 text-sm font-semibold">Needs attention</h2>
              <InsightsList report={insights.data} filter={aboutThisGuest} />
            </Card>
          )}
          <div className="grid gap-4 lg:grid-cols-2">
            <Card className="p-5">
              <p className="text-xs font-medium text-ink-dim">Live</p>
              <div className="mt-4 space-y-4">
                <Meter label={cpuMeterLabel(status?.cpus ?? resource?.maxcpu)} used={cpuPct(status?.cpu)} total={100} text={`${cpuPct(status?.cpu).toFixed(0)}%`} />
                <Meter label="Memory" used={status?.mem} total={status?.maxmem} text={`${bytes(status?.mem)} / ${bytes(status?.maxmem)}`} />
                <Meter label="Disk" used={diskUsed} total={diskMax} text={diskText} />
              </div>
              <div className="mt-5 space-y-2 border-t border-white/[0.06] pt-4">
                <MetaRow label="Operating system" value={osName || "—"} />
                <MetaRow label="IP address" value={ips.length ? ips.join(", ") : "—"} />
                <MetaRow label="HA" value={haEnabled ? "Enabled" : "Disabled"} on={haEnabled} />
                {visiblePanels.some((p) => p.id === "replication") ? (
                  <button type="button" className="w-full text-left hover:text-ink" onClick={() => setPanel("replication")}>
                    <MetaRow
                      label="Replication"
                      value={replEnabled ? `Enabled · ${replActive.map((j) => j.target).filter(Boolean).join(", ") || "—"}` : "Disabled"}
                      on={replEnabled}
                    />
                  </button>
                ) : (
                  <MetaRow
                    label="Replication"
                    value={replEnabled ? `Enabled · ${replActive.map((j) => j.target).filter(Boolean).join(", ") || "—"}` : "Disabled"}
                    on={replEnabled}
                  />
                )}
                <MetaRow label="Last backup" value={backupText} on={!!lastBackupAt} />
                {visiblePanels.some((p) => p.id === "snapshots") ? (
                  <button type="button" className="w-full text-left hover:text-ink" onClick={() => setPanel("snapshots")}>
                    <MetaRow label="Snapshots" value={snapText} on={realSnaps.length > 0} />
                  </button>
                ) : (
                  <MetaRow label="Snapshots" value={snapText} on={realSnaps.length > 0} />
                )}
              </div>
            </Card>
            <Card className="flex min-h-[220px] flex-col p-5">
              <div className="mb-3 flex items-center justify-between gap-2">
                <p className="text-xs font-medium text-ink-dim">Notes</p>
                {canConfig && !editNotes && (
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
                          const trimmed = notesDraft.trim();
                          await api.updateGuest(id, {
                            node,
                            type,
                            digest: configDigest(cfg),
                            fields: trimmed ? { description: notesDraft } : {},
                            delete: trimmed ? [] : ["description"],
                          }, node, type);
                          setEditNotes(false);
                          void qc.invalidateQueries({ queryKey: ["guest", id] });
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
                <h2 className="mt-1 text-sm font-medium">{String(cfg.name || cfg.hostname || status?.name || `Guest ${id}`)}</h2>
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
                  series={[{ key: "mem", label: "Memory", color: CHART_MEM, scale: guestMemToBytes(points) }]}
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
                  points={points}
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
                  points={delayPoints}
                  format="percent"
                  tipSide="left"
                  series={[
                    {
                      key: "iowait",
                      label: "IO delay",
                      color: CHART_CPU,
                      scale: (v) => v * 100,
                    },
                  ]}
                />
              </div>
            </div>
          </Card>
          <PendingChanges
            pending={guest.data?.pending ?? []}
            vmid={id}
            node={node}
            type={type}
            config={cfg}
            onReverted={() => void qc.invalidateQueries({ queryKey: ["guest", id] })}
          />
        </div>
      )}

      {panel && (
        <div className="hv-form-stack space-y-4">
          {panel === "hardware" && guest.data && (
            <HardwareTab
              key={id}
              vmid={id}
              node={node}
              type={type}
              config={cfg}
              pending={guest.data.pending ?? []}
              onSaved={() => void qc.invalidateQueries({ queryKey: ["guest", id] })}
            />
          )}
          {panel === "cloud-init" && (
            <CloudInitPanel
              key={id}
              vmid={id}
              node={node}
              type={type}
              config={cfg}
              onSaved={() => void qc.invalidateQueries({ queryKey: ["guest", id] })}
            />
          )}
          {panel === "agent" && (
            <AgentPanel key={id} vmid={id} node={node} type={type} running={qemuRunning} />
          )}
          {panel === "backup" && node && <BackupPanel key={id} vmid={id} node={node} type={type} />}
          {panel === "replication" && node && <ReplicationPanel key={id} vmid={id} node={node} nodes={nodes} />}
          {panel === "ha" && <HAPanel key={id} vmid={id} type={type} nodes={nodes} />}
          {panel === "firewall" && node && <FirewallPanel key={id} vmid={id} node={node} type={type} />}
          {panel === "snapshots" && (
            <SnapshotsPanel
              key={id}
              vmid={id}
              node={node}
              type={type}
              items={snaps.data ?? []}
              canSnapshot={snapFeat.data?.hasFeature !== false}
              onChange={() => void snaps.refetch()}
            />
          )}
          {panel === "permissions" && <ObjectACL key={vmPath} path={vmPath} />}
          {panel === "history" && node && <ObjectTaskHistory key={id} node={node} guest={String(id)} />}
          {panel === "console" && node && (
            <GuestConsole
              key={id}
              vmid={id}
              node={node}
              type={type}
              canConsole={canConsole}
              canMonitor={canMonitor}
            />
          )}
        </div>
      )}

      {dialog === "clone" && (
        <CloneDialog
          vmid={id}
          node={node}
          type={type}
          name={String(cfg.name || cfg.hostname || "")}
          template={Number(cfg.template) === 1}
          nodes={nodes}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === "migrate" && (
        <MigrateDialog
          vmid={id}
          node={node}
          type={type}
          nodes={nodes}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === "remote-migrate" && (
        <RemoteMigrateDialog
          vmid={id}
          node={node}
          type={type}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === "relocate" && (
        <HARelocateDialog
          vmid={id}
          sid={haSid(type, id)}
          nodes={nodes}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog === "remove" && (
        <RemoveGuestDialog
          vmid={id}
          node={node}
          type={type}
          onClose={() => setDialog(null)}
          onRemoved={() => {
            setDialog(null);
            void nav({ to: "/" });
          }}
        />
      )}

      {confirm && (
        <ConfirmDialog
          title={CONFIRM[confirm].title(guestName)}
          body={CONFIRM[confirm].body}
          danger={confirm === "stop" || confirm === "reset" || confirm === "template"}
          confirmLabel={CONFIRM[confirm].label}
          onClose={() => setConfirm(null)}
          onConfirm={async () => {
            if (confirm === "template") {
              const res = await api.template(id, node, type);
              attachTask(res.upid, guestKeys(id));
            } else {
              power.mutate(confirm);
            }
            setConfirm(null);
          }}
        />
      )}
    </div>
  );
}

function MigrateDialog({
  vmid, node, type, nodes, onClose,
}: {
  vmid: number; node: string; type: string; nodes: Resource[]; onClose: () => void;
}) {
  const targets = nodes.filter((n) => n.node !== node);
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const [target, setTarget] = useState(targets[0]?.node ?? "");
  const [online, setOnline] = useState(true);
  const [withLocalDisks, setWithLocalDisks] = useState(true);
  const [targetStorage, setTargetStorage] = useState("");
  const [migrationNetwork, setMigrationNetwork] = useState("");

  return (
    <Modal
      title={`Migrate ${vmid}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            const res = await api.migrate(vmid, {
              target,
              online,
              withLocalDisks,
              ...(targetStorage ? { targetStorage } : {}),
              ...(migrationNetwork ? { migrationNetwork } : {}),
            }, node, type);
            attachTask(res.upid, guestKeys(vmid));
            onClose();
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
          {storagesForContent(storages.data ?? [], type === "lxc" ? "rootdir" : "images", target).map((s) => (
            <option key={s.storage} value={s.storage}>{s.storage}</option>
          ))}
        </Select>
      </Field>
      <Field label="Migration network">
        <Input value={migrationNetwork} onChange={(e) => setMigrationNetwork(e.target.value)} placeholder="10.0.0.0/24" />
      </Field>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={online} onChange={(e) => setOnline(e.target.checked)} /> Live (online)</label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={withLocalDisks} onChange={(e) => setWithLocalDisks(e.target.checked)} /> With local disks</label>
    </Modal>
  );
}

function targetEndpoint(host: string, apitoken: string, fingerprint: string, port: string) {
  if (!host || !apitoken) return "";
  const parts = [`apitoken=${apitoken}`, `host=${host}`];
  if (fingerprint) parts.push(`fingerprint=${fingerprint}`);
  if (port) parts.push(`port=${port}`);
  return parts.join(",");
}

function RemoteMigrateDialog({
  vmid, node, type, onClose,
}: {
  vmid: number; node: string; type: string; onClose: () => void;
}) {
  const remotes = useQuery({ queryKey: ["cluster-remotes"], queryFn: api.clusterRemotes });
  const [remote, setRemote] = useState("");
  const [host, setHost] = useState("");
  const [tokenUser, setTokenUser] = useState("");
  const [tokenSecret, setTokenSecret] = useState("");
  const [fingerprint, setFingerprint] = useState("");
  const [port, setPort] = useState("");
  const [target, setTarget] = useState("");
  const [targetVmid, setTargetVmid] = useState(String(vmid));
  const [targetStorage, setTargetStorage] = useState("");
  const [targetBridge, setTargetBridge] = useState("");
  const [online, setOnline] = useState(true);
  const [del, setDel] = useState(false);
  const names = (remotes.data?.items ?? []).map((item) => String(item.remote ?? item.name ?? item.id ?? "")).filter(Boolean);
  const apitoken = pveMigrateToken(tokenUser, tokenSecret);
  const endpoint = targetEndpoint(host, apitoken, fingerprint, port);

  return (
    <Modal
      title={`Remote migrate ${vmid}`}
      onClose={onClose}
      footer={
        <Button
          disabled={!remote && !endpoint}
          onClick={async () => {
            const res = await api.remoteMigrate(vmid, {
              ...(remote ? { remote } : {}),
              ...(endpoint ? { "target-endpoint": endpoint } : {}),
              ...(target ? { target } : {}),
              "target-vmid": Number(targetVmid) || vmid,
              ...(targetStorage ? { "target-storage": targetStorage } : {}),
              ...(targetBridge ? { "target-bridge": targetBridge } : {}),
              online,
              delete: del,
            }, node, type);
            attachTask(res.upid, guestKeys(vmid));
            onClose();
          }}
        >
          Remote migrate
        </Button>
      }
    >
      <p className="text-sm text-ink-dim">Migrates this guest to a named remote or a target endpoint. Not a Hypervane fleet switcher.</p>
      {remotes.data?.supported === false && (
        <p className="text-sm text-ink-dim">Remote list is not available. Use a target endpoint, or add remotes under Datacenter if this cluster supports them.</p>
      )}
      <Field label="Remote">
        {names.length ? (
          <Select value={remote} onChange={(e) => setRemote(e.target.value)}>
            <option value="">Select remote</option>
            {names.map((n) => <option key={n}>{n}</option>)}
          </Select>
        ) : (
          <Input value={remote} onChange={(e) => setRemote(e.target.value)} placeholder="optional if using endpoint" />
        )}
      </Field>
      <Field label="Target host"><Input value={host} onChange={(e) => setHost(e.target.value)} placeholder="optional if using a remote" /></Field>
      <Field label="User">
        <Input value={tokenUser} onChange={(e) => setTokenUser(e.target.value)} placeholder="root@pam!hypervane" />
      </Field>
      <Field label="API key">
        <Input type="password" value={tokenSecret} onChange={(e) => setTokenSecret(e.target.value)} placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
      </Field>
      <Field label="Fingerprint"><Input value={fingerprint} onChange={(e) => setFingerprint(e.target.value)} /></Field>
      <Field label="Port"><Input value={port} onChange={(e) => setPort(e.target.value)} placeholder="8006" /></Field>
      <Field label="Target node"><Input value={target} onChange={(e) => setTarget(e.target.value)} placeholder="optional on remote" /></Field>
      <Field label="Target VMID"><Input type="number" value={targetVmid} onChange={(e) => setTargetVmid(e.target.value)} /></Field>
      <Field label="Target storage"><Input value={targetStorage} onChange={(e) => setTargetStorage(e.target.value)} /></Field>
      <Field label="Target bridge"><Input value={targetBridge} onChange={(e) => setTargetBridge(e.target.value)} /></Field>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={online} onChange={(e) => setOnline(e.target.checked)} /> Live (online)</label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={del} onChange={(e) => setDel(e.target.checked)} /> Delete source after migrate</label>
    </Modal>
  );
}

function HARelocateDialog({
  vmid, sid, nodes, onClose,
}: {
  vmid: number; sid: string; nodes: Resource[]; onClose: () => void;
}) {
  const [target, setTarget] = useState(nodes[0]?.node ?? "");
  return (
    <Modal
      title={`Relocate ${sid}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            const res = await api.haRelocate(sid, target);
            if (res.upid) attachTask(res.upid, [["ha-status"], ["ha-resources"], ["ha-resource", sid], ...guestKeys(vmid)]);
            onClose();
          }}
        >
          Relocate
        </Button>
      }
    >
      <p className="text-sm text-ink-dim">Stop on the current node and start on the target through the HA manager.</p>
      <Field label="Target node">
        <Select value={target} onChange={(e) => setTarget(e.target.value)}>
          {nodes.map((n) => <option key={n.id} value={n.node}>{n.node}</option>)}
        </Select>
      </Field>
    </Modal>
  );
}

const sendKeys = [
  { label: "Ctrl-Alt-Del", key: "ctrl-alt-delete" },
  { label: "Ctrl-Alt-F1", key: "ctrl-alt-f1" },
  { label: "Ctrl-Alt-F2", key: "ctrl-alt-f2" },
  { label: "Ctrl-Alt-F7", key: "ctrl-alt-f7" },
];

function GuestConsole({
  vmid, node, type, canConsole, canMonitor,
}: {
  vmid: number; node: string; type: string; canConsole: boolean; canMonitor: boolean;
}) {
  const [kind, setKind] = useState<"vnc" | "term" | "spice">(type === "lxc" ? "term" : "vnc");
  const [customKey, setCustomKey] = useState("");
  const [command, setCommand] = useState("info status");
  const [output, setOutput] = useState("");
  const [error, setError] = useState("");
  const [captureMsg, setCaptureMsg] = useState("");

  return (
    <div className="space-y-3">
      <p className="text-xs text-ink-dim">
        Ticket stays on the API. QEMU can use noVNC, SPICE, or serial; containers use a shell.
      </p>
      <div className="flex flex-wrap gap-2">
        {(type === "lxc" ? (["term"] as const) : (["vnc", "spice", "term"] as const)).map((k) => (
          <Button key={k} size="sm" variant={kind === k ? "selected" : "outline"} onClick={() => setKind(k)}>
            {k === "term" ? "serial / term" : k.toUpperCase()}
          </Button>
        ))}
      </div>
      <ConsoleView node={node} vmid={vmid} type={type} kind={kind} />
      {type === "qemu" && (
        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            disabled={kind !== "vnc"}
            onClick={() => {
              setCaptureMsg("");
              const canvas = document.querySelector(".hv-vnc canvas") as HTMLCanvasElement | null;
              if (!canvas) {
                setCaptureMsg("No VNC frame yet. Wait for the console to connect.");
                return;
              }
              const link = document.createElement("a");
              link.href = canvas.toDataURL("image/png");
              link.download = `vm-${vmid}.png`;
              link.click();
            }}
          >
            Capture
          </Button>
          {kind !== "vnc" && (
            <p className="text-xs text-ink-dim">Capture needs the QEMU VNC console. Serial and SPICE have no framebuffer here.</p>
          )}
          {captureMsg && <p className="text-xs text-rose-400">{captureMsg}</p>}
        </div>
      )}
      {type === "qemu" && canConsole && (
        <Card className="space-y-2 p-4">
          <h3 className="text-sm font-medium">Send key</h3>
          <div className="flex flex-wrap gap-2">
            {sendKeys.map((k) => (
              <Button key={k.key} size="sm" variant="outline" onClick={() => void api.sendKey(vmid, k.key, node, type)}>
                {k.label}
              </Button>
            ))}
            <Input className="max-w-xs" placeholder="free-form key" value={customKey} onChange={(e) => setCustomKey(e.target.value)} />
            <Button size="sm" onClick={() => { if (customKey) void api.sendKey(vmid, customKey, node, type); }}>Send</Button>
          </div>
        </Card>
      )}
      {type === "qemu" && canMonitor && (
        <Card className="space-y-2 p-4">
          <h3 className="text-sm font-medium">QEMU monitor</h3>
          <div className="flex gap-2">
            <Input value={command} onChange={(e) => setCommand(e.target.value)} placeholder="info status" />
            <Button size="sm" onClick={async () => {
              setError("");
              try {
                const res = await api.qemuMonitor(vmid, command, node, type);
                setOutput(res.output || "(empty)");
              } catch (e) {
                setError(e instanceof Error ? e.message : "Monitor failed");
              }
            }}>Run</Button>
          </div>
          {error && <p className="text-sm text-rose-400">{error}</p>}
          {output && <pre className="max-h-48 overflow-auto rounded-md bg-canvas p-3 font-mono text-xs text-ink">{output}</pre>}
        </Card>
      )}
    </div>
  );
}

function RemoveGuestDialog({
  vmid,
  node,
  type,
  onClose,
  onRemoved,
}: {
  vmid: number;
  node: string;
  type: string;
  onClose: () => void;
  onRemoved: () => void;
}) {
  const [purge, setPurge] = useState(true);
  const [destroyDisks, setDestroyDisks] = useState(false);
  const [force, setForce] = useState(false);

  return (
    <Modal
      title={`Remove ${type.toUpperCase()} ${vmid}?`}
      onClose={onClose}
      footer={
        <Button
          variant="danger"
          onClick={async () => {
            const res = await api.deleteGuest(vmid, node, type, {
              purge,
              force: type === "lxc" ? force : false,
              destroyUnreferencedDisks: destroyDisks,
            });
            attachTask(res.upid, guestKeys(vmid));
            onRemoved();
          }}
        >
          Remove
        </Button>
      }
    >
      <p className="text-sm text-ink-dim">Destroys {type.toUpperCase()} {vmid} and its owned volumes. This cannot be undone.</p>
      <label className="flex items-center gap-2 text-sm text-ink-dim">
        <input type="checkbox" checked={purge} onChange={(e) => setPurge(e.target.checked)} />
        Purge from backup, replication, and HA jobs
      </label>
      <label className="flex items-center gap-2 text-sm text-ink-dim">
        <input type="checkbox" checked={destroyDisks} onChange={(e) => setDestroyDisks(e.target.checked)} />
        Destroy unreferenced disks with this VMID
      </label>
      {type === "lxc" && (
        <label className="flex items-center gap-2 text-sm text-ink-dim">
          <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} />
          Force destroy even if running
        </label>
      )}
    </Modal>
  );
}
