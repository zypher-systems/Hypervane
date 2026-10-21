import { attachTask } from "@/components/layout/AppShell";
import { Badge, Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { relTime } from "@/lib/format";
import type { Task } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

export function ObjectTaskHistory({
  node,
  guest,
  storage,
}: {
  node: string;
  guest?: string;
  storage?: string;
}) {
  const [user, setUser] = useState("");
  const [type, setType] = useState("");
  const [applied, setApplied] = useState({ user: "", type: "" });
  useEffect(() => {
    const t = window.setTimeout(() => setApplied({ user: user.trim(), type: type.trim() }), 300);
    return () => window.clearTimeout(t);
  }, [user, type]);
  const tasks = useQuery({
    queryKey: ["object-tasks", node, guest, storage, applied.user, applied.type],
    queryFn: () => api.tasks({
      node,
      guest,
      storage,
      source: "archive",
      user: applied.user || undefined,
      type: applied.type || undefined,
    }),
    enabled: !!node,
    refetchInterval: 15000,
  });

  return (
    <Card className="overflow-hidden">
      <div className="flex items-center justify-between gap-3 border-b border-white/15 px-4 py-3">
        <span className="text-xs font-medium text-ink-dim">History</span>
        <div className="flex items-center gap-2">
          <Input value={user} onChange={(e) => setUser(e.target.value)} placeholder="Filter user" className="h-7 w-32 text-xs" />
          <Input value={type} onChange={(e) => setType(e.target.value)} placeholder="Filter type" className="h-7 w-32 text-xs" />
        </div>
      </div>
      <table className="w-full text-sm">
        <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
          <tr>
            <th className="px-4 py-2">When</th>
            <th>Type</th>
            <th>User</th>
            <th>Status</th>
          </tr>
        </thead>
        <tbody>
          {(tasks.data ?? []).map((t) => (
            <tr
              key={t.upid}
              className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
              onClick={() => attachTask(t.upid)}
            >
              <td className="px-4 py-2 text-ink-dim">{relTime(t.starttime)}</td>
              <td className="font-mono text-xs">{t.type}</td>
              <td className="text-ink-dim">{t.user}</td>
              <td><Badge>{taskStatus(t)}</Badge></td>
            </tr>
          ))}
        </tbody>
      </table>
      {!tasks.data?.length && !tasks.isLoading && (
        <p className="p-4 text-sm text-ink-dim">No archived tasks for this object.</p>
      )}
    </Card>
  );
}

function taskStatus(t: Task) {
  if (t.status) return t.status;
  if (!t.endtime) return "running";
  return "done";
}
