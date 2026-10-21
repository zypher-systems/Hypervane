import { Card } from "@/components/ui/card";
import { FabricIfaceTable, InferTable } from "@/components/ui/item-details";
import { api } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

export function SDNPanel({ node }: { node: string }) {
  const runtime = useQuery({ queryKey: ["node-sdn", node], queryFn: () => api.nodeSDN(node) });
  const fabrics = useQuery({ queryKey: ["sdn-fabrics"], queryFn: api.sdnFabrics });
  const [fabric, setFabric] = useState("");
  const fabricId = fabric || String(fabrics.data?.items?.[0]?.id || fabrics.data?.items?.[0]?.name || "");
  const ifaces = useQuery({
    queryKey: ["sdn-fabric-ifaces", fabricId, node],
    queryFn: () => api.sdnFabricInterfaces(node, fabricId),
    enabled: !!fabricId,
  });

  if (runtime.isLoading) {
    return <p className="text-sm text-ink-dim">Loading node SDN…</p>;
  }
  if (!runtime.data?.supported) {
    return (
      <p className="text-sm text-ink-dim">
        No SDN runtime on this node.{" "}
        <Link to="/sdn" className="text-accent hover:underline">
          Open cluster SDN
        </Link>
      </p>
    );
  }

  return (
    <div className="space-y-4">
      <RuntimeTable title="Zones" rows={runtime.data.zones} />
      <RuntimeTable title="Neighbors" rows={runtime.data.neighbors} />
      <RuntimeTable title="Routes" rows={runtime.data.routes} />
      {(fabrics.data?.supported !== false || ifaces.data) && (
        <Card className="space-y-3 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Fabric interfaces</h2>
          {(fabrics.data?.items ?? []).length > 0 && (
            <select
              className="rounded-md border border-line bg-canvas px-3 py-1.5 text-sm"
              value={fabricId}
              onChange={(e) => setFabric(e.target.value)}
            >
              {(fabrics.data?.items ?? []).map((item, i) => {
                const id = String(item.id || item.name || i);
                return (
                  <option key={id} value={id}>
                    {id}
                  </option>
                );
              })}
            </select>
          )}
          {ifaces.data?.supported === false ? (
            <p className="text-sm text-ink-dim">Fabric interfaces are not available on this node.</p>
          ) : (
            <FabricIfaceTable items={ifaces.data?.items ?? []} />
          )}
        </Card>
      )}
    </div>
  );
}

function RuntimeTable({ title, rows }: { title: string; rows?: Record<string, unknown>[] }) {
  return (
    <Card className="overflow-hidden">
      <h2 className="border-b border-white/15 px-4 py-3 text-xs font-medium text-ink-dim">{title}</h2>
      {!rows?.length ? (
        <p className="p-4 text-sm text-ink-dim">None.</p>
      ) : (
        <InferTable rows={rows} />
      )}
    </Card>
  );
}
