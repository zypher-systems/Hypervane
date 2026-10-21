import { Badge, Card } from "@/components/ui/card";
import { api } from "@/lib/api";
import type { Resource } from "@/types";
import { useQueries } from "@tanstack/react-query";

export function SupportPanel({ nodes }: { nodes: Resource[] }) {
  const names = nodes.map((n) => n.node).filter((n): n is string => !!n);
  const rows = useQueries({
    queries: names.map((node) => ({
      queryKey: ["node-sub", node],
      queryFn: () => api.nodeSubscription(node),
      retry: false,
    })),
  });

  return (
    <Card className="overflow-hidden">
      <div className="border-b border-white/15 px-4 py-3 text-xs font-medium text-ink-dim">
        Support
      </div>
      <table className="w-full text-sm">
        <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
          <tr>
            <th className="px-4 py-2">Node</th>
            <th>Status</th>
            <th>Level</th>
            <th>Product</th>
            <th>Next due</th>
            <th>Message</th>
          </tr>
        </thead>
        <tbody>
          {names.map((node, i) => {
            const q = rows[i];
            const sub = q.data;
            return (
              <tr key={node} className="border-t border-line">
                <td className="px-4 py-2 font-mono text-xs">{node}</td>
                <td>
                  {q.isError ? (
                    <Badge tone="rose">offline</Badge>
                  ) : (
                    <Badge>{sub?.status || "…"}</Badge>
                  )}
                </td>
                <td className="text-ink-dim">{sub?.level || "—"}</td>
                <td className="text-ink-dim">{sub?.productname || "—"}</td>
                <td className="font-mono text-xs text-ink-dim">{sub?.nextduedate || "—"}</td>
                <td className="max-w-xs truncate text-ink-dim">{q.isError ? (q.error instanceof Error ? q.error.message : "unavailable") : (sub?.message || "—")}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {!names.length && <p className="p-4 text-sm text-ink-dim">No nodes in the live inventory.</p>}
    </Card>
  );
}
