import { Card } from "@/components/ui/card";
import { bytes } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { FailoverNode } from "@/types";

/**
 * "If a node fails, can the others take its guests?" in memory terms: the
 * configured memory of a node's running guests against the free memory on the
 * rest of the cluster.
 */
export function FailoverCard({ nodes, allNodes }: { nodes?: FailoverNode[]; allNodes: string[] }) {
  if (!nodes?.length) {
    if (allNodes.length === 1) {
      return (
        <Card className="p-5">
          <h2 className="text-sm font-semibold">If a node fails</h2>
          <p className="mt-2 text-[13px] leading-5 text-ink-dim">
            This is a single node, so there is nowhere for its guests to move. Backups are the safety net.
          </p>
        </Card>
      );
    }
    return null;
  }
  const covered = nodes.every((n) => n.fits);
  const haCovered = nodes.every((n) => n.haFits);
  return (
    <Card className="p-5">
      <div className="flex items-center gap-2">
        <h2 className="text-sm font-semibold">If a node fails</h2>
        <span className={cn("ml-auto text-xs", covered ? "text-ok" : haCovered ? "text-warn" : "text-danger")}>
          {covered ? "Covered" : haCovered ? "Partly covered" : "Not covered"}
        </span>
      </div>
      <p className="mt-1.5 text-[13px] leading-5 text-ink-dim">
        Memory the rest of the cluster would need to take over each node's running guests.
      </p>
      <ul className="mt-3 space-y-3">
        {nodes.map((n) => {
          const others = allNodes.filter((x) => x !== n.node);
          const who = others.length === 1 ? `${others[0]} takes over ${n.node}` : `Others take over ${n.node}`;
          const pct = n.available > 0 ? Math.min(100, (n.required / n.available) * 100) : n.required > 0 ? 100 : 0;
          const fill = !n.haFits ? "bg-danger" : !n.fits ? "bg-warn" : "bg-meter";
          return (
            <li key={n.node}>
              <div className="flex items-center justify-between gap-3 text-xs">
                <span className="text-ink">{who}</span>
                <span className="font-mono text-ink-dim">
                  {bytes(n.required)} of {bytes(n.available)} free
                </span>
              </div>
              <div
                className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-white/[0.08]"
                role="meter"
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={Math.round(pct)}
                aria-label={`${who}: needs ${Math.round(pct)}% of the free memory`}
              >
                <div className={cn("h-full rounded-full", fill)} style={{ width: `${pct}%` }} />
              </div>
              {n.haRequired > 0 && (
                <p className="mt-1 text-[11px] text-ink-faint">HA would restart {bytes(n.haRequired)} of it automatically.</p>
              )}
            </li>
          );
        })}
      </ul>
      <p className="mt-3 text-[11px] leading-4 text-ink-faint">
        Counts configured guest memory against free host memory. ZFS cache is reported as used, so real headroom is usually larger.
      </p>
    </Card>
  );
}
