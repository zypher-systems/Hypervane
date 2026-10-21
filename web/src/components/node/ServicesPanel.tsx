import { attachTask } from "@/components/layout/AppShell";
import { BarButton } from "@/components/ui/action-bar";
import { Badge, Card } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/dialog";
import { api } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

const ACTIONS = [
  { id: "start", label: "Start" },
  { id: "stop", label: "Stop", confirm: true },
  { id: "restart", label: "Restart", confirm: true },
  { id: "reload", label: "Reload" },
] as const;

export function ServicesPanel({ node, writable }: { node: string; writable: boolean }) {
  const services = useQuery({ queryKey: ["node-svc", node], queryFn: () => api.nodeServices(node) });
  const [pending, setPending] = useState<{ service: string; name: string; action: "stop" | "restart" } | null>(null);

  async function run(service: string, action: "start" | "stop" | "restart" | "reload") {
    attachTask((await api.nodeServiceAction(node, service, action)).upid, [["node-svc", node]]);
  }

  return (
    <>
      <Card className="overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-black/30 text-left text-xs font-medium text-ink-dim">
            <tr><th className="px-4 py-2">Service</th><th>State</th><th></th></tr>
          </thead>
          <tbody>
            {(services.data ?? []).map((s) => (
              <tr key={s.service} className="border-t border-white/10">
                <td className="px-4 py-2">{s.name || s.service}</td>
                <td><Badge tone={s.state === "running" ? "green" : "zinc"}>{s.state || "—"}</Badge></td>
                <td className="space-x-2 px-4 text-right">
                  {writable && ACTIONS.map((a) => (
                    <BarButton
                      key={a.id}
                      onClick={() => {
                        if (a.id === "stop" || a.id === "restart") {
                          setPending({ service: s.service, name: s.name || s.service, action: a.id });
                          return;
                        }
                        void run(s.service, a.id);
                      }}
                    >
                      {a.label}
                    </BarButton>
                  ))}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      {pending && (
        <ConfirmDialog
          title={`${pending.action === "stop" ? "Stop" : "Restart"} ${pending.name}?`}
          body={
            pending.action === "stop"
              ? "Stops this node service. Guests and cluster services that depend on it may fail."
              : "Restarts this node service. There may be a brief interruption."
          }
          danger={pending.action === "stop"}
          confirmLabel={pending.action === "stop" ? "Stop" : "Restart"}
          onClose={() => setPending(null)}
          onConfirm={async () => {
            await run(pending.service, pending.action);
            setPending(null);
          }}
        />
      )}
    </>
  );
}
