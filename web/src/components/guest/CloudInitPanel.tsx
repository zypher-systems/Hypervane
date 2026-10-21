import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Field, Input, Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest } from "@/lib/guestHardware";
import { guestKeys } from "@/lib/tasks";
import { useCan } from "@/lib/perms";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

export function CloudInitPanel({
  vmid, node, type, config, onSaved,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>; onSaved: () => void;
}) {
  const can = useCan();
  const canEdit = can(`/vms/${vmid}`, "VM.Config") || can(`/vms/${vmid}`, "VM.Config.Options");
  const hasCI = Object.keys(config).some((k) => k.startsWith("ci") || k.startsWith("ipconfig") || k === "sshkeys" || String(config[k] ?? "").includes("cloudinit"));
  const [ciuser, setCiuser] = useState(String(config.ciuser ?? ""));
  const [cipassword, setCipassword] = useState("");
  const [sshkeys, setSshkeys] = useState(decodeKeys(String(config.sshkeys ?? "")));
  const [ipconfig0, setIpconfig0] = useState(String(config.ipconfig0 ?? "ip=dhcp"));
  const [nameserver, setNameserver] = useState(String(config.nameserver ?? ""));
  const [searchdomain, setSearchdomain] = useState(String(config.searchdomain ?? ""));
  const [cicustom, setCicustom] = useState(String(config.cicustom ?? ""));
  const [dumpType, setDumpType] = useState<"user" | "network" | "meta">("user");
  const [error, setError] = useState("");
  const [ok, setOk] = useState("");
  const dump = useQuery({
    queryKey: ["ci-dump", vmid, dumpType],
    queryFn: () => api.cloudinitDump(vmid, dumpType, node, type),
    enabled: hasCI,
  });

  if (!hasCI) {
    return (
      <Card className="p-6 text-sm text-ink-dim">
        No cloud-init drive or keys on this guest. Attach a cloud-init disk from Hardware (or create the guest with cloud-init) first.
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card className="grid gap-4 p-5 md:grid-cols-2">
        <h3 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim md:col-span-2">Identity & network</h3>
        <Field label="User"><Input value={ciuser} onChange={(e) => setCiuser(e.target.value)} disabled={!canEdit} /></Field>
        <Field label="Password"><Input type="password" value={cipassword} onChange={(e) => setCipassword(e.target.value)} disabled={!canEdit} /></Field>
        <Field label="ipconfig0"><Input value={ipconfig0} onChange={(e) => setIpconfig0(e.target.value)} disabled={!canEdit} /></Field>
        <Field label="Nameserver"><Input value={nameserver} onChange={(e) => setNameserver(e.target.value)} disabled={!canEdit} /></Field>
        <div className="md:col-span-2">
          <Field label="SSH keys"><Textarea rows={4} value={sshkeys} onChange={(e) => setSshkeys(e.target.value)} disabled={!canEdit} /></Field>
        </div>
        <Field label="Search domain"><Input value={searchdomain} onChange={(e) => setSearchdomain(e.target.value)} disabled={!canEdit} /></Field>
        <Field label="cicustom"><Input value={cicustom} onChange={(e) => setCicustom(e.target.value)} placeholder="user=local:snippets/user.yaml" disabled={!canEdit} /></Field>
        {error && <p className="text-sm text-rose-400 md:col-span-2">{error}</p>}
        {ok && <p className="text-sm text-ok md:col-span-2">{ok}</p>}
        {canEdit && (
          <div className="flex flex-wrap gap-2 md:col-span-2">
            <Button
              onClick={async () => {
                setError("");
                setOk("");
                try {
                  await api.saveCloudinit(vmid, {
                    ciuser, cipassword, sshkeys, ipconfig0, nameserver, searchdomain, cicustom,
                    digest: configDigest(config),
                  }, node, type);
                  setOk("Cloud-init saved.");
                  onSaved();
                } catch (e) {
                  setError(e instanceof Error ? e.message : "Save failed");
                }
              }}
            >
              Save cloud-init
            </Button>
            <Button
              variant="outline"
              onClick={async () => {
                setError("");
                setOk("");
                try {
                  const res = await api.regenerateCloudinit(vmid, node, type);
                  if (!res.supported) {
                    setError("Regenerate is not available on this node");
                    return;
                  }
                  if (res.upid) attachTask(res.upid, [...guestKeys(vmid), ["ci-dump", vmid]]);
                  else void dump.refetch();
                  setOk("Cloud-init image regenerate requested.");
                } catch (e) {
                  setError(e instanceof Error ? e.message : "Regenerate failed");
                }
              }}
            >
              Regenerate image
            </Button>
          </div>
        )}
      </Card>
      <Card className="space-y-3 p-5">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h3 className="text-xs font-medium text-ink-dim">Generated dump</h3>
          <div className="flex gap-2">
            {(["user", "network", "meta"] as const).map((t) => (
              <Button key={t} size="sm" variant={dumpType === t ? "selected" : "outline"} onClick={() => setDumpType(t)}>{t}</Button>
            ))}
          </div>
        </div>
        {!dump.data?.supported && dump.isFetched && (
          <p className="text-xs text-ink-dim">Cloud-init dump is not available on this node.</p>
        )}
        <pre className="max-h-80 overflow-auto rounded-md bg-canvas p-3 font-mono text-xs text-ink-dim">
          {dump.data?.dump || dump.isLoading ? (dump.data?.dump || "Loading…") : "No dump"}
        </pre>
      </Card>
    </div>
  );
}

// Proxmox stores a VM's SSH keys percent-encoded.
function decodeKeys(raw: string) {
  try {
    return decodeURIComponent(raw);
  } catch {
    return raw;
  }
}
