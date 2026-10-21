import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

function asRecord(v: unknown): Record<string, unknown> {
  if (v && typeof v === "object" && !Array.isArray(v)) return v as Record<string, unknown>;
  return {};
}

function unwrap(v: unknown): unknown {
  const rec = asRecord(v);
  return rec.result ?? rec.data ?? v;
}

function formatIPs(n: Record<string, unknown>): string {
  const ips = n["ip-addresses"] ?? n.ip_addresses ?? n.ip;
  if (Array.isArray(ips)) {
    return ips.map((ip) => {
      if (typeof ip === "string") return ip;
      const rec = asRecord(ip);
      const addr = String(rec["ip-address"] ?? rec.ip ?? "");
      const prefix = rec.prefix ?? rec["ip-address-prefix"];
      return prefix ? `${addr}/${prefix}` : addr;
    }).filter(Boolean).join(", ") || "—";
  }
  return ips ? String(ips) : "—";
}

function formatUsed(f: Record<string, unknown>): string {
  const used = Number(f.used ?? f["used-bytes"] ?? 0);
  const total = Number(f.total ?? f["total-bytes"] ?? 0);
  if (!total && !used) return "—";
  return `${formatBytes(used)} / ${formatBytes(total)}`;
}

function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return "0";
  if (n >= 1e12) return `${(n / 1e12).toFixed(1)} TiB`;
  if (n >= 1e9) return `${(n / 1e9).toFixed(1)} GiB`;
  if (n >= 1e6) return `${(n / 1e6).toFixed(1)} MiB`;
  return `${Math.round(n)} B`;
}

export function AgentPanel({ vmid, node, type, running }: { vmid: number; node: string; type: string; running: boolean }) {
  const q = useQuery({
    queryKey: ["agent", vmid, node],
    queryFn: () => api.agent(vmid, node, type),
    enabled: type === "qemu" && running,
    staleTime: 0,
    retry: 1,
  });
  const [user, setUser] = useState("root");
  const [password, setPassword] = useState("");
  const [command, setCommand] = useState("uname -a");
  const [execOut, setExecOut] = useState("");
  const [execStart, setExecStart] = useState<Record<string, unknown> | null>(null);
  const [msg, setMsg] = useState("");
  const [freeze, setFreeze] = useState(false);

  if (type !== "qemu") {
    return <Card className="p-6 text-sm text-ink-dim">Guest agent is a QEMU feature.</Card>;
  }
  if (!running) {
    return <Card className="p-6 text-sm text-ink-dim">Start the VM to talk to the QEMU guest agent.</Card>;
  }
  if (q.isLoading) {
    return <Card className="p-6 text-sm text-ink-dim">Checking guest agent…</Card>;
  }
  if (q.isError) {
    return <Card className="p-6 text-sm text-ink-dim">{q.error instanceof Error ? q.error.message : "Agent request failed."}</Card>;
  }
  const payload = q.data ?? {};
  const hasPayload = !!(payload.osinfo || payload.fsinfo || payload.nics || payload.users || payload.ping || payload.hostname);
  if (payload.available === false && !hasPayload) {
    return (
      <Card className="p-6 text-sm text-ink-dim">
        {typeof payload.error === "string" && payload.error
          ? payload.error
          : "Agent is not responding. Install qemu-guest-agent in the guest and set agent=1."}
      </Card>
    );
  }

  const osinfo = asRecord(unwrap(payload.osinfo));
  const hostname = asRecord(unwrap(payload.hostname));
  const timezone = asRecord(unwrap(payload.timezone));
  const hostName = String(hostname["host-name"] ?? hostname.hostname ?? hostname.name ?? "");
  const tzName = String(timezone.timezone ?? timezone.zone ?? "");
  const nics = unwrap(payload.nics);
  const fsinfo = unwrap(payload.fsinfo);
  const users = unwrap(payload.users);
  const nicList = Array.isArray(asRecord(nics).result) ? asRecord(nics).result as Record<string, unknown>[]
    : Array.isArray(nics) ? nics as Record<string, unknown>[] : [];
  const fsList = Array.isArray(asRecord(fsinfo).result) ? asRecord(fsinfo).result as Record<string, unknown>[]
    : Array.isArray(fsinfo) ? fsinfo as Record<string, unknown>[] : [];
  const userList = Array.isArray(asRecord(users).result) ? asRecord(users).result as Record<string, unknown>[]
    : Array.isArray(users) ? users as Record<string, unknown>[] : [];

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2">
        <Card className="space-y-2 p-5">
          <h3 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">OS</h3>
          <p className="text-sm text-ink">{String(osinfo["pretty-name"] ?? osinfo.name ?? "—")}</p>
          <p className="text-xs text-ink-dim">{String(osinfo.kernel ?? osinfo.version ?? "")}</p>
          {(hostName || tzName) && (
            <p className="text-xs text-ink-dim">
              {hostName && <span className="font-mono">{hostName}</span>}
              {hostName && tzName && " · "}
              {tzName && <span>{tzName}</span>}
            </p>
          )}
        </Card>
        <Card className="space-y-2 p-5">
          <h3 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Users</h3>
          <table className="w-full text-xs">
            <thead className="text-left text-ink-dim">
              <tr><th className="py-1">User</th><th>Domain</th></tr>
            </thead>
            <tbody className="text-ink">
              {userList.length === 0 && <tr><td colSpan={2} className="text-ink-dim">No users reported</td></tr>}
              {userList.map((u, i) => (
                <tr key={i}>
                  <td className="py-1 font-mono">{String(u.user ?? u.username ?? "—")}</td>
                  <td>{String(u.domain ?? "—")}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
        <Card className="space-y-2 p-5">
          <h3 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">NICs</h3>
          <table className="w-full text-xs">
            <thead className="text-left text-ink-dim">
              <tr><th className="py-1">Name</th><th>MAC</th><th>Addresses</th></tr>
            </thead>
            <tbody className="text-ink">
              {nicList.length === 0 && <tr><td colSpan={3} className="text-ink-dim">No interfaces reported</td></tr>}
              {nicList.map((n, i) => (
                <tr key={i}>
                  <td className="py-1 font-mono">{String(n.name ?? "—")}</td>
                  <td className="font-mono">{String(n["hardware-address"] ?? n.hardware_address ?? "—")}</td>
                  <td className="font-mono">{formatIPs(n)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
        <Card className="space-y-2 p-5">
          <h3 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Filesystems</h3>
          <table className="w-full text-xs">
            <thead className="text-left text-ink-dim">
              <tr><th className="py-1">Mount</th><th>Type</th><th>Used / total</th></tr>
            </thead>
            <tbody className="text-ink">
              {fsList.length === 0 && <tr><td colSpan={3} className="text-ink-dim">No mounts reported</td></tr>}
              {fsList.map((f, i) => (
                <tr key={i}>
                  <td className="py-1 font-mono">{String(f.mountpoint ?? f.name ?? "—")}</td>
                  <td>{String(f.type ?? "—")}</td>
                  <td>{formatUsed(f)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      </div>
      <Card className="space-y-3 p-5">
        <h3 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Exec</h3>
        <Field label="Command"><Input value={command} onChange={(e) => setCommand(e.target.value)} /></Field>
        <Button
          size="sm"
          onClick={async () => {
            setExecOut("");
            setMsg("");
            try {
              const started = await api.agentCommandPost(vmid, "exec", { command }, node, type);
              const rec = asRecord(unwrap(started));
              const pid = String(rec.pid ?? asRecord(started).pid ?? "");
              setExecStart({ pid: pid || String(rec.pid ?? "—"), status: rec.status ?? rec["exitcode"] ?? rec.exited ?? "started" });
              if (!pid) {
                setExecOut("");
                return;
              }
              for (let i = 0; i < 10; i++) {
                await new Promise((r) => setTimeout(r, 400));
                const st = await api.agentCommand(vmid, "exec-status", node, type, { pid });
                const rec = asRecord(unwrap(st));
                if (rec.exited === 1 || rec.exited === true) {
                  setExecOut(String(rec["out-data"] ?? rec.out ?? "") + (rec["err-data"] ? `\n${rec["err-data"]}` : ""));
                  return;
                }
              }
              setExecOut("Command is still running.");
            } catch (e) {
              setExecOut(e instanceof Error ? e.message : "Exec failed");
            }
          }}
        >
          Run
        </Button>
        {execStart && (
          <p className="text-sm text-ink-dim">pid {String(execStart.pid)} · {String(execStart.status)}</p>
        )}
        {execOut && <pre className="max-h-48 overflow-auto rounded-md bg-canvas p-3 font-mono text-xs text-ink-dim">{execOut}</pre>}
      </Card>
      <Card className="flex flex-wrap items-end gap-3 p-5">
        <Button
          variant="outline"
          onClick={async () => {
            await api.agentFstrim(vmid, node, type);
            setMsg("fstrim requested");
          }}
        >
          fstrim
        </Button>
        <Button variant="outline" onClick={() => setFreeze(true)}>Freeze filesystems</Button>
        <Button
          variant="outline"
          onClick={async () => {
            await api.agentCommandPost(vmid, "fsfreeze-thaw", {}, node, type);
            setMsg("filesystems thawed");
          }}
        >
          Thaw
        </Button>
        <Field label="Username"><Input value={user} onChange={(e) => setUser(e.target.value)} /></Field>
        <Field label="Password"><Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></Field>
        <Button
          onClick={async () => {
            await api.agentPassword(vmid, user, password, node, type);
            setMsg("password updated");
            setPassword("");
          }}
        >
          Set password
        </Button>
        {msg && <span className="text-xs text-ok">{msg}</span>}
      </Card>
      {freeze && (
        <ConfirmDialog
          title="Freeze guest filesystems?"
          body="I/O will pause until you thaw. Use this only for consistent snapshots or backups."
          danger
          confirmLabel="Freeze"
          onClose={() => setFreeze(false)}
          onConfirm={async () => {
            await api.agentCommandPost(vmid, "fsfreeze-freeze", {}, node, type);
            setMsg("filesystems frozen");
            setFreeze(false);
          }}
        />
      )}
    </div>
  );
}
