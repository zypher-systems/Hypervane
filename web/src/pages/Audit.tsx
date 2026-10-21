import { InventoryCard } from "@/components/ui/inventory";
import { api } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

export function AuditPage() {
  const q = useQuery({ queryKey: ["audit"], queryFn: api.audit, refetchInterval: 15000 });
  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <header>
        <h2 className="text-base font-semibold">Audit log</h2>
        <p className="text-sm text-ink-dim">Every change made through Hypervane, newest first.</p>
      </header>
      <InventoryCard title="Events">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-3 py-2">When</th>
              <th className="px-3 py-2">User</th>
              <th className="px-3 py-2">Method</th>
              <th className="px-3 py-2">Path</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2">UPID</th>
            </tr>
          </thead>
          <tbody>
            {(q.data?.events ?? []).map((ev) => (
              <tr key={ev.id} className="border-t border-line">
                <td className="px-3 py-2 font-mono text-xs text-ink-dim">{new Date(ev.at).toLocaleString()}</td>
                <td className="px-3 py-2">{ev.username || "—"}</td>
                <td className="px-3 py-2 font-mono text-xs">{ev.method}</td>
                <td className="truncate px-3 py-2 font-mono text-xs">{ev.path}</td>
                <td className="px-3 py-2">{ev.status}</td>
                <td className="truncate px-3 py-2 font-mono text-[11px] text-ink-faint">{ev.upid || "—"}</td>
              </tr>
            ))}
            {!q.data?.events?.length && (
              <tr className="border-t border-line">
                <td className="px-3 py-6 text-ink-dim" colSpan={6}>
                  No audit events yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </InventoryCard>
    </div>
  );
}
