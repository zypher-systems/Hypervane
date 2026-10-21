import { Button } from "@/components/ui/button";
import { openTasks } from "@/lib/tasks";
import { cn } from "@/lib/utils";
import type { Insight, InsightsReport, InsightTarget } from "@/types";
import { Link } from "@tanstack/react-router";
import { CircleCheck } from "lucide-react";

const GROUPS = [
  { severity: "action", label: "Needs action", dot: "bg-warn" },
  { severity: "notice", label: "Worth a look", dot: "bg-ink-faint" },
] as const;

/**
 * The cluster's short to-do list: only what needs a decision, each item with
 * the numbers behind it and one next step.
 */
export function InsightsList({
  report,
  isLoading,
  error,
  onRetry,
  filter,
  emptyText = "Nothing needs your attention.",
  emptyHint = "Hypervane checks backups, locks, storage, certificates, snapshots, and failover capacity.",
}: {
  report?: InsightsReport;
  isLoading?: boolean;
  error?: Error | null;
  onRetry?: () => void;
  filter?: (i: Insight) => boolean;
  emptyText?: string;
  emptyHint?: string;
}) {
  if (error && !report) {
    return (
      <div className="flex items-center justify-between gap-3 px-5 py-4 text-sm">
        <span className="text-danger">Couldn't load insights: {error.message}</span>
        {onRetry && (
          <Button size="sm" variant="outline" onClick={onRetry}>
            Retry
          </Button>
        )}
      </div>
    );
  }
  if (isLoading && !report) {
    return (
      <div className="space-y-4 px-5 py-4" aria-busy="true" aria-label="Loading insights">
        {[0, 1, 2].map((i) => (
          <div key={i} className="space-y-2">
            <div className="h-3.5 w-2/3 animate-pulse rounded bg-white/[0.06]" />
            <div className="h-3 w-5/6 animate-pulse rounded bg-white/[0.04]" />
          </div>
        ))}
      </div>
    );
  }
  const items = (report?.insights ?? []).filter((i) => (filter ? filter(i) : true));
  const unavailable = report?.unavailable ?? [];
  if (!items.length) {
    return (
      <div className="px-5 py-5">
        <div className="flex items-center gap-2.5 text-sm">
          <CircleCheck className="h-4 w-4 shrink-0 text-ok" aria-hidden />
          <span>{emptyText}</span>
        </div>
        {emptyHint && <p className="mt-1.5 pl-[26px] text-xs text-ink-dim">{emptyHint}</p>}
        {unavailable.length > 0 && <Unavailable names={unavailable} />}
      </div>
    );
  }
  return (
    <div className="pb-1">
      {GROUPS.map((g) => {
        const rows = items.filter((i) => i.severity === g.severity);
        if (!rows.length) return null;
        return (
          <section key={g.severity} aria-label={g.label}>
            <h3 className="px-5 pb-1 pt-3 text-xs font-medium text-ink-dim">{g.label}</h3>
            <ul>
              {rows.map((i) => (
                <li key={i.id} className="flex items-start gap-3 border-t border-line/70 px-5 py-3">
                  <span aria-hidden className={cn("mt-[7px] h-[7px] w-[7px] shrink-0 rounded-full", g.dot)} />
                  <div className="min-w-0 flex-1">
                    <p className="text-sm text-ink">{i.title}</p>
                    <p className="mt-0.5 text-[13px] leading-5 text-ink-dim">{i.detail}</p>
                  </div>
                  {i.target && <InsightAction target={i.target} label={i.action} />}
                </li>
              ))}
            </ul>
          </section>
        );
      })}
      {unavailable.length > 0 && (
        <div className="px-5 pb-3">
          <Unavailable names={unavailable} />
        </div>
      )}
    </div>
  );
}

function Unavailable({ names }: { names: string[] }) {
  return (
    <p className="mt-3 text-xs text-ink-faint">
      Couldn't check {names.join(", ")}. The connection's token may not be allowed to read them.
    </p>
  );
}

const actionClass =
  "shrink-0 rounded-md border border-white/[0.1] px-2.5 py-1 text-xs text-ink transition hover:bg-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40";

/** The one next step for an insight: a link to where it gets fixed. */
export function InsightAction({ target, label }: { target: InsightTarget; label?: string }) {
  const text = label || "Open";
  switch (target.kind) {
    case "task":
      return (
        <button type="button" className={actionClass} onClick={() => openTasks(target.upid)}>
          {text}
        </button>
      );
    case "guest":
      if (!target.vmid) return null;
      return (
        <Link
          to="/guests/$vmid"
          params={{ vmid: String(target.vmid) }}
          search={{ node: target.node, type: target.type, tab: target.tab || undefined }}
          className={actionClass}
        >
          {text}
        </Link>
      );
    case "node":
      if (!target.node) return null;
      return (
        <Link to="/nodes/$node" params={{ node: target.node }} search={target.tab ? { tab: target.tab } : {}} className={actionClass}>
          {text}
        </Link>
      );
    case "storage":
      if (!target.storage) return null;
      return (
        <Link to="/storage/$storage" params={{ storage: target.storage }} search={{ node: target.node }} className={actionClass}>
          {text}
        </Link>
      );
    case "backups": {
      const tab = target.tab === "coverage" || target.tab === "replication" ? target.tab : "jobs";
      return (
        <Link to="/backups" search={{ tab }} className={actionClass}>
          {text}
        </Link>
      );
    }
    case "ha":
      return (
        <Link to="/ha" className={actionClass}>
          {text}
        </Link>
      );
    default:
      return null;
  }
}
