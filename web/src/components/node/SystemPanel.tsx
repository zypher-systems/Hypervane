import { Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Advanced, CollapsedRaw, asRecords } from "@/components/ui/item-details";
import { Field, Input, Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

export function SystemPanel({ node, writable }: { node: string; writable: boolean }) {
  const qc = useQueryClient();
  const can = useCan();
  const canJournal = can(`/nodes/${node}`, "Sys.Syslog") || can(`/nodes/${node}`, "Sys.Audit") || writable;
  const dns = useQuery({ queryKey: ["node-dns", node], queryFn: () => api.nodeDNS(node) });
  const time = useQuery({ queryKey: ["node-time", node], queryFn: () => api.nodeTime(node) });
  const hosts = useQuery({ queryKey: ["node-hosts", node], queryFn: () => api.nodeHosts(node) });
  const sub = useQuery({ queryKey: ["node-sub", node], queryFn: () => api.nodeSubscription(node) });
  const journal = useQuery({
    queryKey: ["node-journal", node],
    queryFn: () => api.nodeJournal(node),
    enabled: canJournal,
    refetchInterval: 15000,
  });
  const [search, setSearch] = useState("");
  const [dns1, setDns1] = useState("");
  const [tz, setTz] = useState("");
  const [hostsData, setHostsData] = useState("");
  const [key, setKey] = useState("");
  const [execRows, setExecRows] = useState<Record<string, unknown>[]>([]);
  const config = useQuery({ queryKey: ["node-config", node], queryFn: () => api.nodeConfig(node) });
  const [notes, setNotes] = useState("");
  const [commands, setCommands] = useState('[\n  {"method":"GET","path":"/version"}\n]');
  const [execOut, setExecOut] = useState<unknown>(null);
  const [execConfirm, setExecConfirm] = useState(false);
  const [execError, setExecError] = useState("");

  return (
    <div className="space-y-4">
      {config.data && (
        <Card className="space-y-2 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Notes</h2>
          <Textarea
            className="min-h-24"
            defaultValue={String(config.data.description ?? "")}
            onChange={(e) => setNotes(e.target.value)}
            disabled={!writable}
          />
          {writable && (
            <BarButton onClick={async () => {
              await api.updateNodeConfig(node, { description: notes || String(config.data?.description ?? "") });
              void qc.invalidateQueries({ queryKey: ["node-config", node] });
            }}>Save notes</BarButton>
          )}
        </Card>
      )}
      {dns.data && (
        <Card className="grid gap-3 p-4 md:grid-cols-3">
          <Field label="Search domain">
            <Input defaultValue={dns.data.search} onChange={(e) => setSearch(e.target.value)} disabled={!writable} />
          </Field>
          <Field label="DNS 1">
            <Input defaultValue={dns.data.dns1} onChange={(e) => setDns1(e.target.value)} disabled={!writable} />
          </Field>
          <Field label="DNS 2">
            <Input defaultValue={dns.data.dns2} disabled={!writable} id="dns2" />
          </Field>
          {writable && (
            <BarButton onClick={async () => {
              const dns2 = (document.getElementById("dns2") as HTMLInputElement | null)?.value;
              await api.updateNodeDNS(node, {
                search: search || dns.data?.search,
                dns1: dns1 || dns.data?.dns1,
                dns2,
                dns3: dns.data?.dns3,
              });
              void qc.invalidateQueries({ queryKey: ["node-dns", node] });
            }}>Save DNS</BarButton>
          )}
        </Card>
      )}
      {time.data && (
        <Card className="flex flex-wrap items-end gap-3 p-4">
          <Field label="Timezone">
            <Input defaultValue={time.data.timezone} onChange={(e) => setTz(e.target.value)} disabled={!writable} />
          </Field>
          {writable && (
            <BarButton onClick={async () => {
              await api.updateNodeTime(node, tz || time.data?.timezone || "UTC");
              void qc.invalidateQueries({ queryKey: ["node-time", node] });
            }}>Save timezone</BarButton>
          )}
        </Card>
      )}
      {hosts.data && (
        <Card className="space-y-2 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Hosts</h2>
          <Textarea
            className="min-h-32 font-mono text-xs"
            defaultValue={hosts.data.data}
            onChange={(e) => setHostsData(e.target.value)}
            disabled={!writable}
          />
          {writable && (
            <BarButton onClick={async () => {
              await api.updateNodeHosts(node, { data: hostsData || hosts.data?.data, digest: hosts.data?.digest });
              void qc.invalidateQueries({ queryKey: ["node-hosts", node] });
            }}>Save hosts</BarButton>
          )}
        </Card>
      )}
      {sub.data && (
        <Card className="space-y-2 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Subscription</h2>
          <p className="text-sm text-ink-dim">{sub.data.status || "unknown"} · {sub.data.level || "—"} · {sub.data.message || ""}</p>
          {writable && (
            <div className="flex gap-2">
              <Input placeholder="subscription key" value={key} onChange={(e) => setKey(e.target.value)} />
              <BarButton onClick={async () => {
                await api.setNodeSubscription(node, key);
                setKey("");
                void qc.invalidateQueries({ queryKey: ["node-sub", node] });
              }}>Set key</BarButton>
            </div>
          )}
        </Card>
      )}
      {writable && (
        <Card className="space-y-2 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Execute</h2>
          <p className="text-sm text-ink-dim">JSON batch of node-local API calls. Root-only on PVE. Not a shell.</p>
          <Textarea className="min-h-32 font-mono text-xs" value={commands} onChange={(e) => setCommands(e.target.value)} />
          <BarButton onClick={() => { setExecError(""); setExecConfirm(true); }}>Run batch</BarButton>
          {execError && <p className="text-sm text-rose-400">{execError}</p>}
          {execRows.length > 0 && (
            <table className="w-full text-sm">
              <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
                <tr><th className="px-2 py-1">Path</th><th>Status</th><th>Data</th></tr>
              </thead>
              <tbody>
                {execRows.map((row, i) => (
                  <tr key={i} className="border-t border-line">
                    <td className="px-2 py-1 font-mono text-xs">{String(row.path ?? row.uri ?? "—")}</td>
                    <td>{String(row.status ?? row.code ?? "—")}</td>
                    <td className="font-mono text-xs text-ink-dim">{typeof row.data === "object" ? JSON.stringify(row.data) : String(row.data ?? row.message ?? "—")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
          {execOut != null && (
            <Advanced label="Advanced: raw execute output">
              <CollapsedRaw data={execOut} />
            </Advanced>
          )}
        </Card>
      )}
      {execConfirm && (
        <ConfirmDialog
          title={`Execute on ${node}?`}
          body="Sends this command batch to PVE as the cluster token. Confirm the JSON is what you intend."
          confirmLabel="Execute"
          danger
          onClose={() => setExecConfirm(false)}
          onConfirm={async () => {
            try {
              const parsed = JSON.parse(commands) as unknown;
              const out = await api.nodeExecute(node, parsed);
              setExecRows(asRecords(out));
              setExecOut(out);
              setExecError("");
            } catch (e) {
              setExecError(e instanceof Error ? e.message : "Execute failed");
            }
            setExecConfirm(false);
          }}
        />
      )}
      {canJournal && (
        <Card className="overflow-hidden">
          <h2 className="px-4 pt-4 font-medium">Journal</h2>
          <pre className="max-h-72 overflow-auto p-4 font-mono text-xs text-ink-dim">
            {(journal.data ?? []).map((l) => `${l.n}  ${l.t}`).join("\n") || "No journal lines."}
          </pre>
        </Card>
      )}
    </div>
  );
}
