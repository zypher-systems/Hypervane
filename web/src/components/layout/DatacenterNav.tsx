import { api } from "@/lib/api";
import { getActiveConnection } from "@/lib/connection";
import { formatWhen } from "@/lib/format";
import { useCan } from "@/lib/perms";
import { cn } from "@/lib/utils";
import { useQuery } from "@tanstack/react-query";
import { Link, useRouterState } from "@tanstack/react-router";

type DatacenterTab = "storage" | "config";

export type ClusterNavItem = {
  id: string;
  label: string;
  to: string;
  tab?: DatacenterTab;
  match: (pathname: string, tab?: string) => boolean;
};

/** Datacenter-scope sections the current user may open, in tab order. */
export function useClusterNav(): ClusterNavItem[] {
  const can = useCan();
  const showBackups = can("/", "VM.Backup") || can("/", "Sys.Modify") || can("/", "Sys.Audit");
  const showHA = can("/", "Sys.Console") || can("/", "Sys.Modify") || can("/", "Sys.Audit");
  const showIdentity = can("/access", "User.Modify") || can("/access", "Permissions.Modify") || can("/", "User.Modify") || can("/", "Sys.Audit");
  const showAdmin = can("/", "Sys.Modify") || can("/", "Sys.Audit");
  const showSDNAcl = can("/sdn", "SDN.Audit") || can("/sdn", "SDN.Allocate") || can("/", "Sys.Audit");
  const sdn = useQuery({ queryKey: ["sdn-status"], queryFn: api.sdnStatus, enabled: showSDNAcl, retry: false });
  // Only offer the Ceph tab once Ceph is installed; installing starts from a node's Ceph tab.
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary, enabled: showAdmin, staleTime: 30_000 });
  const cephNode = (summary.data?.resources ?? []).find((r) => r.type === "node" && r.status === "online")?.node ?? "";
  const ceph = useQuery({
    queryKey: ["ceph-status", cephNode],
    queryFn: () => api.cephStatus(cephNode),
    enabled: showAdmin && !!cephNode,
    retry: false,
    staleTime: 60_000,
  });
  const onDc = (p: string) => p === "/datacenter";
  const items: (ClusterNavItem & { show: boolean })[] = [
    { id: "summary", label: "Overview", to: "/", show: true, match: (p, t) => p === "/" || (onDc(p) && t !== "storage" && t !== "config") },
    { id: "storage", label: "Storage", to: "/datacenter", tab: "storage", show: showAdmin, match: (p, t) => onDc(p) && t === "storage" },
    { id: "backups", label: "Backups", to: "/backups", show: showBackups, match: (p) => p.startsWith("/backups") },
    { id: "ha", label: "HA", to: "/ha", show: showHA, match: (p) => p.startsWith("/ha") },
    { id: "firewall", label: "Firewall", to: "/firewall", show: showAdmin, match: (p) => p.startsWith("/firewall") },
    { id: "access", label: "Access", to: "/identity", show: showIdentity, match: (p) => p.startsWith("/identity") },
    { id: "sdn", label: "SDN", to: "/sdn", show: showSDNAcl && sdn.data?.supported === true, match: (p) => p.startsWith("/sdn") },
    { id: "ceph", label: "Ceph", to: "/ceph", show: showAdmin && ceph.data?.supported === true, match: (p) => p.startsWith("/ceph") },
    { id: "config", label: "Configuration", to: "/datacenter", tab: "config", show: showAdmin, match: (p, t) => onDc(p) && t === "config" },
    { id: "audit", label: "Audit log", to: "/audit", show: true, match: (p) => p.startsWith("/audit") },
    { id: "history", label: "Long history", to: "/history", show: true, match: (p) => p.startsWith("/history") },
  ];
  return items.filter((i) => i.show);
}

export function isDatacenterPath(pathname: string): boolean {
  return (
    pathname === "/" ||
    ["/datacenter", "/backups", "/ha", "/firewall", "/identity", "/sdn", "/ceph", "/audit", "/history"].some((p) => pathname.startsWith(p))
  );
}

/**
 * Header shared by every datacenter-scope page: the cluster name, a one-line
 * health summary, and the section tabs, so the cluster reads as one place.
 */
export function DatacenterHeader() {
  return (
    <div className="space-y-4">
      <DatacenterTitle />
      <DatacenterTabs />
    </div>
  );
}

function DatacenterTitle() {
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary, refetchInterval: 15_000 });
  const insights = useQuery({
    queryKey: ["insights", getActiveConnection()],
    queryFn: api.insights,
    refetchInterval: 60_000,
    staleTime: 30_000,
  });
  const resources = summary.data?.resources ?? [];
  const cluster = (summary.data?.status ?? []).find((c) => c.type === "cluster");
  const nodes = resources.filter((r) => r.type === "node");
  const online = nodes.filter((n) => n.status === "online");
  const guests = resources.filter((r) => (r.type === "qemu" || r.type === "lxc") && !r.template);
  const running = guests.filter((g) => g.status === "running").length;
  let cpuUsed = 0;
  let cpuTotal = 0;
  let memUsed = 0;
  let memTotal = 0;
  for (const n of online) {
    cpuUsed += (n.cpu || 0) * (n.maxcpu || 1);
    cpuTotal += n.maxcpu || 1;
    memUsed += n.mem || 0;
    memTotal += n.maxmem || 0;
  }
  const quorumLost = !!cluster && !cluster.quorate;
  const single = nodes.length === 1;
  const backups = insights.data?.backups;
  const name = cluster?.name || (single ? nodes[0]?.node : undefined) || "Datacenter";
  return (
    <header>
      <p className="text-xs text-ink-dim">Datacenter</p>
      <h1 className="mt-0.5 text-2xl font-semibold tracking-tight">{name}</h1>
      {summary.isSuccess && (
        <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-[13px] text-ink-dim">
          <span className={cn("inline-flex items-center gap-1.5", quorumLost ? "text-danger" : "text-ink")}>
            <span aria-hidden className={cn("h-[7px] w-[7px] rounded-full", quorumLost || online.length < nodes.length ? "bg-danger" : "bg-ok")} />
            {quorumLost
              ? "No quorum"
              : single
                ? "Single node"
                : `${cluster ? "Quorate, " : ""}${online.length} of ${nodes.length} nodes online`}
          </span>
          <span>
            {running} of {guests.length} guests running
          </span>
          {cpuTotal > 0 && <span>CPU {Math.round((cpuUsed / cpuTotal) * 100)}%</span>}
          {memTotal > 0 && <span>Memory {Math.round((memUsed / memTotal) * 100)}%</span>}
          {backups?.running && <span>Backup running</span>}
          {backups && !backups.running && backups.lastRun > 0 && (
            <span>
              Last backup {formatWhen(backups.lastRun)}, {backups.ok} ok
              {backups.failed > 0 && <span className="text-warn">, {backups.failed} failed</span>}
            </span>
          )}
        </p>
      )}
    </header>
  );
}

/** The section tabs, one strip across every datacenter-scope page. */
function DatacenterTabs() {
  const items = useClusterNav();
  const location = useRouterState({ select: (s) => s.location });
  const tab = (location.search as { tab?: string }).tab;
  return (
    <nav aria-label="Datacenter" className="flex flex-wrap gap-x-6 border-b border-line text-sm">
      {items.map((item) => {
        const active = item.match(location.pathname, tab);
        return (
          <Link
            key={item.id}
            to={item.to}
            search={item.tab ? { tab: item.tab } : {}}
            aria-current={active ? "page" : undefined}
            className={cn(
              "-mb-px inline-flex items-center gap-1.5 border-b-2 py-2.5 transition",
              active ? "border-accent font-medium text-ink" : "border-transparent text-ink-dim hover:text-ink",
            )}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
