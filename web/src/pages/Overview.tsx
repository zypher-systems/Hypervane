import { DatacenterNotes } from "@/components/datacenter/DatacenterNotes";
import { FailoverCard } from "@/components/insights/FailoverCard";
import { InsightsList } from "@/components/insights/InsightsList";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";
import { Card } from "@/components/ui/card";
import { meterFill } from "@/components/ui/meter";
import { api, ApiError } from "@/lib/api";
import { getActiveConnection, onActiveConnection } from "@/lib/connection";
import { bytes, cpuPct, uptime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { live } from "@/lib/ws";
import { ClusterHistory, type HistoryNode } from "@/pages/Datacenter";
import type { ClusterStatus, Resource } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { RefreshCw } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

/**
 * The datacenter overview: calm when things are fine, and when they aren't, a
 * short list of what needs a decision.
 */
export function OverviewPage() {
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary, refetchInterval: 15_000 });
  const insights = useQuery({
    queryKey: ["insights", getActiveConnection()],
    queryFn: api.insights,
    refetchInterval: 60_000,
    staleTime: 30_000,
  });
  const [liveResources, setLiveResources] = useState<Resource[] | null>(null);
  const [liveStatus, setLiveStatus] = useState<ClusterStatus[] | null>(null);

  useEffect(() => {
    const off = live.on((topic, data) => {
      if (topic !== "resources") return;
      const payload = data as { connection?: string; resources?: Resource[]; status?: ClusterStatus[] };
      if (payload.connection && payload.connection !== getActiveConnection()) return;
      if (payload.resources) setLiveResources(payload.resources);
      if (payload.status) setLiveStatus(payload.status);
    });
    const unsub = onActiveConnection(() => {
      setLiveResources(null);
      setLiveStatus(null);
    });
    return () => {
      off();
      unsub();
    };
  }, []);

  const resources = liveResources ?? summary.data?.resources ?? [];
  const status = liveStatus ?? summary.data?.status ?? [];
  const nodes = useMemo(
    () =>
      resources
        .filter((r) => r.type === "node")
        .sort((a, b) => (a.node ?? "").localeCompare(b.node ?? "", undefined, { numeric: true })),
    [resources],
  );
  const online = nodes.filter((n) => n.status === "online");
  const clusterName = status.find((c) => c.type === "cluster")?.name || "Datacenter";
  const historyKey = online.map((n) => `${n.node}:${n.maxcpu || 1}`).join("|");
  const historyNodes = useMemo<HistoryNode[]>(
    () =>
      historyKey
        ? historyKey.split("|").map((part) => {
            const [node, maxcpu] = part.split(":");
            return { node, maxcpu: Number(maxcpu) || 1 };
          })
        : [],
    [historyKey],
  );

  if (summary.error instanceof ApiError && summary.error.status === 503) {
    return (
      <div className="mx-auto max-w-lg py-16 text-center">
        <h1 className="text-2xl font-semibold">Connect a Proxmox cluster</h1>
        <p className="mt-2 text-sm text-ink-dim">
          Hypervane is ready. Add a cluster or a standalone node with an API token to start.
        </p>
        <Link to="/settings" className="mt-6 inline-block rounded-md bg-accent px-4 py-2 text-sm font-semibold text-accent-ink hover:bg-accent/90">
          Add a connection
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="min-w-0 space-y-4">
          <Card className="overflow-hidden">
            <div className="flex items-center gap-3 px-5 pt-4">
              <h2 className="text-sm font-semibold">Insights</h2>
              {insights.data && (
                <span className="text-xs text-ink-faint">Checked {ago(insights.data.generatedAt)}</span>
              )}
              <button
                type="button"
                aria-label="Check again"
                title="Check again"
                onClick={() => void insights.refetch()}
                className="ml-auto grid h-7 w-7 place-items-center rounded-md text-ink-dim hover:bg-raised hover:text-ink focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
              >
                <RefreshCw className={cn("h-3.5 w-3.5", insights.isFetching && "animate-spin")} />
              </button>
            </div>
            <InsightsList
              report={insights.data}
              isLoading={insights.isLoading}
              error={insights.error}
              onRetry={() => void insights.refetch()}
            />
          </Card>
          <NodesTable nodes={nodes} resources={resources} />
        </div>
        <div className="space-y-4">
          <FailoverCard nodes={insights.data?.failover?.nodes} allNodes={online.map((n) => n.node ?? n.id)} />
          <DatacenterNotes />
        </div>
      </div>
      {historyNodes.length > 0 && <ClusterHistory nodes={historyNodes} clusterName={clusterName} />}
    </div>
  );
}

function NodesTable({ nodes, resources }: { nodes: Resource[]; resources: Resource[] }) {
  if (!nodes.length) return null;
  const guestsOn = (node?: string) => resources.filter((r) => (r.type === "qemu" || r.type === "lxc") && !r.template && r.node === node);
  return (
    <Card className="overflow-hidden">
      <h2 className="px-5 pb-2 pt-4 text-sm font-semibold">Nodes</h2>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-y border-line text-left text-xs text-ink-dim">
              <th className="px-5 py-2 font-medium">Node</th>
              <th className="px-3 py-2 font-medium">CPU</th>
              <th className="px-3 py-2 font-medium">Memory</th>
              <th className="px-3 py-2 font-medium">Guests</th>
              <th className="px-5 py-2 text-right font-medium">Uptime</th>
            </tr>
          </thead>
          <tbody>
            {nodes.map((n) => {
              const guests = guestsOn(n.node);
              const running = guests.filter((g) => g.status === "running").length;
              const on = n.status === "online";
              const cpu = cpuPct(n.cpu);
              const mem = n.maxmem ? ((n.mem ?? 0) / n.maxmem) * 100 : 0;
              return (
                <tr key={n.id} className="border-b border-line/60 last:border-b-0 hover:bg-white/[0.02]">
                  <td className="px-5 py-2.5">
                    <Link to="/nodes/$node" params={{ node: n.node ?? n.id }} className="inline-flex items-center gap-2 font-medium hover:underline">
                      <span aria-hidden className={cn("h-[7px] w-[7px] rounded-full", on ? "bg-ok" : "bg-danger")} />
                      {n.node}
                      {!on && <span className="text-xs font-normal text-danger">offline</span>}
                    </Link>
                  </td>
                  <td className="px-3 py-2.5">{on ? <MiniMeter pct={cpu} text={`${cpu.toFixed(0)}% of ${n.maxcpu ?? "?"}`} /> : "—"}</td>
                  <td className="px-3 py-2.5">
                    {on ? <MiniMeter pct={mem} text={`${bytes(n.mem)} / ${bytes(n.maxmem)}`} /> : "—"}
                  </td>
                  <td className="px-3 py-2.5 text-ink-dim">
                    <span className="text-ink">{running}</span> of {guests.length} running
                  </td>
                  <td className="px-5 py-2.5 text-right text-ink-dim">{on ? uptime(n.uptime) : "—"}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function MiniMeter({ pct, text }: { pct: number; text: string }) {
  return (
    <div className="flex items-center gap-2.5">
      <div className="h-1 w-16 shrink-0 overflow-hidden rounded-full bg-white/[0.08]">
        <div className={cn("h-full rounded-full", meterFill(pct))} style={{ width: `${Math.min(100, pct)}%` }} />
      </div>
      <span className="whitespace-nowrap font-mono text-xs text-ink-dim">{text}</span>
    </div>
  );
}

function ago(unix: number): string {
  const s = Math.max(0, Math.round(Date.now() / 1000 - unix));
  if (s < 60) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m} min ago`;
  return `${Math.round(m / 60)} h ago`;
}
