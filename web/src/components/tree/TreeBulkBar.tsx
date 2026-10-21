import { attachTask } from "@/components/layout/AppShell";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { clearSelection, getSelection, subscribe } from "@/lib/selection";
import { fleetKeys, guestKeys } from "@/lib/tasks";
import type { Resource } from "@/types";
import { useEffect, useState } from "react";

export function TreeBulkBar({ nodes }: { nodes: Resource[] }) {
  const [count, setCount] = useState(0);
  const [target, setTarget] = useState(nodes[0]?.node ?? "");
  const [error, setError] = useState("");

  useEffect(() => subscribe(() => setCount(getSelection().length)), []);
  useEffect(() => {
    if (!target && nodes[0]?.node) setTarget(nodes[0].node ?? "");
  }, [nodes, target]);

  if (!count) return null;

  async function run(action: string, extras?: Record<string, unknown>) {
    setError("");
    try {
      const selected = getSelection();
      const keys = [...fleetKeys(), ...selected.flatMap((g) => guestKeys(g.vmid))];
      const res = await api.bulk(action, selected, extras);
      (res.upids ?? []).forEach((upid) => attachTask(upid, keys));
      clearSelection();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Bulk action failed");
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-white/10 bg-black/70 px-4 py-2">
      <span className="text-xs text-ink-dim">{count} selected</span>
      <Button size="sm" variant="outline" onClick={() => void run("start")}>Start</Button>
      <Button size="sm" variant="outline" onClick={() => void run("stop")}>Stop</Button>
      <Button size="sm" variant="outline" onClick={() => void run("suspend")}>Suspend</Button>
      <Select className="h-8 w-36" value={target} onChange={(e) => setTarget(e.target.value)}>
        {nodes.filter((n) => n.node).map((n) => (
          <option key={n.id} value={n.node}>{n.node}</option>
        ))}
      </Select>
      <Button size="sm" variant="outline" disabled={!target} onClick={() => void run("migrate", { target })}>
        Migrate
      </Button>
      <Button size="sm" variant="ghost" onClick={() => clearSelection()}>Clear</Button>
      {error && <span className="text-xs text-rose-400">{error}</span>}
    </div>
  );
}
