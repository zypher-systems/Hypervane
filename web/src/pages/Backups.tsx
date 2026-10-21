import { ReplicationPanel } from "@/components/guest/ReplicationPanel";
import { Badge, Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard } from "@/components/ui/inventory";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { relTime } from "@/lib/format";
import { useCan } from "@/lib/perms";
import type {
  BackupJob,
  BackupJobRequest,
  CoverageReport,
  IncludedVolume,
  Resource,
} from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearch } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Tabs } from "@/components/ui/tabs";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

const tabs = ["jobs", "coverage", "replication"] as const;

export function BackupsPage() {
  const search = useSearch({ from: "/app/backups" });
  const initial = search.tab && tabs.includes(search.tab) ? search.tab : "jobs";
  const [tab, setTab] = useState<(typeof tabs)[number]>(initial);

  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <div>
        <h2 className="text-base font-semibold">Data protection</h2>
        <p className="text-sm text-ink-dim">Scheduled backups, coverage, and replication. Mutations return a task UPID.</p>
      </div>
      <Tabs items={tabs} value={tab} onChange={setTab} />
      {tab === "jobs" && <JobsTab />}
      {tab === "coverage" && <CoverageTab />}
      {tab === "replication" && <ReplicationTab />}
    </div>
  );
}

function DefaultsCard({ nodes }: { nodes: string[] }) {
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can("/", "Datastore.Allocate");
  const [node, setNode] = useState("");
  const picked = node || nodes[0] || "";
  const defaults = useQuery({
    queryKey: ["vzdump-defaults", picked],
    queryFn: () => api.vzdumpDefaults(picked),
    enabled: !!picked,
  });
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const [mode, setMode] = useState("");
  const [bandwidth, setBandwidth] = useState("");
  const [fleeceOn, setFleeceOn] = useState(false);
  const [fleeceStore, setFleeceStore] = useState("");
  const [keepLast, setKeepLast] = useState("");
  const [keepDaily, setKeepDaily] = useState("");
  const [keepWeekly, setKeepWeekly] = useState("");
  const [keepMonthly, setKeepMonthly] = useState("");
  const [msg, setMsg] = useState("");
  useEffect(() => {
    const src = defaults.data?.item;
    if (!src) return;
    const fleece = String(src.fleecing ?? "");
    const prune = String(src["prune-backups"] ?? src.pruneBackups ?? "");
    setMode(String(src.mode ?? ""));
    setBandwidth(String(src.bwlimit ?? src.bandwidth ?? ""));
    setFleeceOn(fleece === "1" || /(?:^|,)(?:enabled=)?1(?:$|,)/.test(fleece));
    setFleeceStore(fleece.match(/storage=([^,]+)/)?.[1] ?? "");
    setKeepLast(prune.match(/keep-last=([^,]+)/)?.[1] ?? "");
    setKeepDaily(prune.match(/keep-daily=([^,]+)/)?.[1] ?? "");
    setKeepWeekly(prune.match(/keep-weekly=([^,]+)/)?.[1] ?? "");
    setKeepMonthly(prune.match(/keep-monthly=([^,]+)/)?.[1] ?? "");
  }, [defaults.data]);

  if (!picked || (defaults.data && !defaults.data.supported)) {
    return null;
  }

  return (
    <Card className="space-y-3 p-4">
      <h2 className="font-medium">Defaults</h2>
      <div className="grid gap-3 md:grid-cols-4">
        <Field label="Node">
          <Select value={picked} onChange={(e) => setNode(e.target.value)}>
            {nodes.map((n) => <option key={n}>{n}</option>)}
          </Select>
        </Field>
        <Field label="Mode">
          <Input value={mode} onChange={(e) => setMode(e.target.value)} placeholder="snapshot" disabled={!writable} />
        </Field>
        <Field label="Bandwidth">
          <Input value={bandwidth} onChange={(e) => setBandwidth(e.target.value)} placeholder="kbps" disabled={!writable} />
        </Field>
        <label className="flex items-end gap-2 text-sm">
          <input type="checkbox" checked={fleeceOn} onChange={(e) => setFleeceOn(e.target.checked)} disabled={!writable} />
          Fleecing
        </label>
        <Field label="Fleecing storage">
          <Select value={fleeceStore} onChange={(e) => setFleeceStore(e.target.value)} disabled={!writable}>
            <option value="">Default</option>
            {(storages.data ?? []).map((s) => <option key={s.storage} value={s.storage}>{s.storage}</option>)}
          </Select>
        </Field>
        <Field label="Keep last"><Input value={keepLast} onChange={(e) => setKeepLast(e.target.value)} disabled={!writable} /></Field>
        <Field label="Keep daily"><Input value={keepDaily} onChange={(e) => setKeepDaily(e.target.value)} disabled={!writable} /></Field>
        <Field label="Keep weekly"><Input value={keepWeekly} onChange={(e) => setKeepWeekly(e.target.value)} disabled={!writable} /></Field>
        <Field label="Keep monthly"><Input value={keepMonthly} onChange={(e) => setKeepMonthly(e.target.value)} disabled={!writable} /></Field>
      </div>
      {msg && <p className="text-sm text-ink-dim">{msg}</p>}
      {writable && (
        <Button size="sm" onClick={async () => {
          const body: Record<string, unknown> = {};
          if (mode) body.mode = mode;
          if (bandwidth) body.bwlimit = bandwidth;
          const fleeceBits = [`enabled=${fleeceOn ? 1 : 0}`];
          if (fleeceStore) fleeceBits.push(`storage=${fleeceStore}`);
          body.fleecing = fleeceBits.join(",");
          const pruneBits = [
            keepLast && `keep-last=${keepLast}`,
            keepDaily && `keep-daily=${keepDaily}`,
            keepWeekly && `keep-weekly=${keepWeekly}`,
            keepMonthly && `keep-monthly=${keepMonthly}`,
          ].filter(Boolean);
          if (pruneBits.length) body["prune-backups"] = pruneBits.join(",");
          const res = await api.updateVzdumpDefaults(picked, body);
          setMsg(res.supported === false ? "Saving defaults is not available on this node." : "Saved.");
        }}>Save defaults</Button>
      )}
    </Card>
  );
}

function JobsTab() {
  const qc = useQueryClient();
  const jobs = useQuery({ queryKey: ["backup-jobs"], queryFn: api.backupJobs });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const [editing, setEditing] = useState<BackupJob | "new" | null>(null);
  const [del, setDel] = useState<BackupJob | null>(null);
  const can = useCan();
  // Backup jobs live in the cluster config; Proxmox requires Sys.Modify on / to change them.
  const canManage = can("/", "Sys.Modify");
  const guests = (summary.data?.resources ?? []).filter((r) => r.type === "qemu" || r.type === "lxc");
  const backupStores = (storages.data ?? []).filter((s) => s.content?.includes("backup") && !s.disable);

  return (
    <div className="space-y-4">
      <DefaultsCard nodes={[...new Set((summary.data?.resources ?? []).filter((r) => r.type === "node" && r.node).map((r) => r.node as string))]} />
      <InventoryCard
        title="Backup jobs"
        actions={canManage ? <BarButton onClick={() => setEditing("new")}>Add</BarButton> : undefined}
      >
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Job</th>
              <th>Schedule</th>
              <th>Storage</th>
              <th>Mode</th>
              <th>Guests</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(jobs.data ?? []).map((job) => (
              <tr key={job.id} className="border-t border-line">
                <td className="px-4 py-2">
                  <div className="font-mono text-xs">{job.id}</div>
                  <Badge tone={job.enabled ? "green" : "zinc"}>{job.enabled ? "enabled" : "disabled"}</Badge>
                </td>
                <td>{job.schedule || "—"}</td>
                <td>{job.storage || "—"}</td>
                <td>{job.mode || "snapshot"}</td>
                <td className="max-w-xs truncate text-ink-dim">{job.all ? `all${job.exclude ? ` except ${job.exclude}` : ""}` : job.vmid || "—"}</td>
                <td className="space-x-2 px-4 text-right">
                  {canManage && (
                    <>
                      <Button size="sm" variant="outline" onClick={() => setEditing(job)}>Edit</Button>
                      <Button size="sm" variant="danger" onClick={() => setDel(job)}>Delete</Button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {jobs.isError && (
          <p className="p-4 text-sm text-rose-400">{jobs.error instanceof Error ? jobs.error.message : "Failed to load backup jobs."}</p>
        )}
        {!jobs.isError && !jobs.data?.length && <p className="p-4 text-sm text-ink-dim">No backup jobs yet.</p>}
      </InventoryCard>
      {editing && (
        <JobForm
          job={editing === "new" ? null : editing}
          guests={guests}
          storages={backupStores.map((s) => s.storage)}
          fleeceStores={(storages.data ?? []).map((s) => s.storage)}
          nodes={[...new Set((summary.data?.resources ?? []).filter((r) => r.type === "node" && r.node).map((r) => r.node as string))]}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            void qc.invalidateQueries({ queryKey: ["backup-jobs"] });
          }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete job ${del.id}?`}
          body="The schedule is removed. Existing backup volumes are kept."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteBackupJob(del.id);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["backup-jobs"] });
          }}
        />
      )}
    </div>
  );
}

function JobForm({
  job, guests, storages, fleeceStores, nodes, onClose, onSaved,
}: {
  job: BackupJob | null;
  guests: Resource[];
  storages: string[];
  fleeceStores: string[];
  nodes: string[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const fleece = String(job?.fleecing ?? "");
  const [schedule, setSchedule] = useState(job?.schedule ?? "sun 00:00");
  const [storage, setStorage] = useState(job?.storage ?? storages[0] ?? "");
  const [mode, setMode] = useState(job?.mode ?? "snapshot");
  const [compress, setCompress] = useState(job?.compress ?? "zstd");
  const [all, setAll] = useState(!!job?.all);
  const [selected, setSelected] = useState<number[]>(() =>
    (job?.vmid ?? "").split(",").map((s) => Number(s.trim())).filter((n) => n > 0),
  );
  const [exclude, setExclude] = useState<number[]>(() =>
    (job?.exclude ?? "").split(",").map((s) => Number(s.trim())).filter((n) => n > 0),
  );
  const [notes, setNotes] = useState(job?.["notes-template"] ?? "");
  const [mailto, setMailto] = useState(job?.mailto ?? "");
  const [mailNote, setMailNote] = useState(job?.mailnotification ?? "always");
  const [notifyMode, setNotifyMode] = useState(job?.["notification-mode"] ?? "auto");
  const [prune, setPrune] = useState(job?.["prune-backups"] ?? "keep-last=3,keep-daily=7");
  const [comment, setComment] = useState(job?.comment ?? "");
  const [node, setNode] = useState(job?.node ?? "");
  const [bwlimit, setBwlimit] = useState(String(job?.bwlimit ?? ""));
  const [fleeceOn, setFleeceOn] = useState(fleece === "1" || /(?:^|,)(?:enabled=)?1(?:$|,)/.test(fleece));
  const [fleeceStore, setFleeceStore] = useState(fleece.match(/storage=([^,]+)/)?.[1] ?? "");
  const [pbsDetect, setPbsDetect] = useState(job?.["pbs-change-detection-mode"] ?? "");
  const [enabled, setEnabled] = useState(job ? job.enabled === 1 : true);
  const [error, setError] = useState("");
  const volumes = useQuery({
    queryKey: ["job-volumes", job?.id],
    queryFn: () => api.jobVolumes(job!.id),
    enabled: !!job?.id,
  });

  const toggle = (id: number, list: number[], set: (v: number[]) => void) => {
    set(list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);
  };

  return (
    <Modal
      title={job ? `Edit ${job.id}` : "Add job"}
      onClose={onClose}
      size="lg"
      footer={
        <Button
          onClick={async () => {
            setError("");
            const body: BackupJobRequest = {
              schedule, storage, mode, compress, notes, mailto, pruneBackups: prune, enabled, all,
              vmid: all ? "" : selected.join(","),
              exclude: all ? exclude.join(",") : "",
              comment,
              node,
              bwlimit,
              mailnotification: mailNote,
              "notification-mode": notifyMode,
              fleecing: fleeceOn ? `enabled=1${fleeceStore ? `,storage=${fleeceStore}` : ""}` : "enabled=0",
              pbsChangeDetection: pbsDetect,
            };
            try {
              if (job) await api.updateBackupJob(job.id, body);
              else await api.createBackupJob(body);
              onSaved();
            } catch (err) {
              setError(err instanceof Error ? err.message : "Save failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <div className="grid gap-4 md:grid-cols-2">
        <Field label="Schedule"><Input value={schedule} onChange={(e) => setSchedule(e.target.value)} placeholder="sun 00:00" /></Field>
        <Field label="Storage">
          <Select value={storage} onChange={(e) => setStorage(e.target.value)}>
            {storages.map((s) => <option key={s}>{s}</option>)}
          </Select>
        </Field>
        <Field label="Mode">
          <Select value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="snapshot">Snapshot</option>
            <option value="suspend">Suspend</option>
            <option value="stop">Stop</option>
          </Select>
        </Field>
        <Field label="Compress">
          <Select value={compress} onChange={(e) => setCompress(e.target.value)}>
            <option value="zstd">zstd</option>
            <option value="gzip">gzip</option>
            <option value="lzo">lzo</option>
            <option value="0">none</option>
          </Select>
        </Field>
        <Field label="Notes template"><Input value={notes} onChange={(e) => setNotes(e.target.value)} /></Field>
        <Field label="Mail to"><Input value={mailto} onChange={(e) => setMailto(e.target.value)} /></Field>
        <Field label="Mail notification">
          <Select value={mailNote} onChange={(e) => setMailNote(e.target.value)}>
            <option value="always">always</option>
            <option value="failure">failure</option>
          </Select>
        </Field>
        <Field label="Notification mode">
          <Select value={notifyMode} onChange={(e) => setNotifyMode(e.target.value)}>
            <option value="auto">auto</option>
            <option value="legacy-sendmail">legacy-sendmail</option>
            <option value="notification-system">notification-system</option>
          </Select>
        </Field>
        <Field label="Retention"><Input value={prune} onChange={(e) => setPrune(e.target.value)} /></Field>
        <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
        <Field label="Node filter">
          <Select value={node} onChange={(e) => setNode(e.target.value)}>
            <option value="">All nodes</option>
            {nodes.map((n) => <option key={n}>{n}</option>)}
          </Select>
        </Field>
        <Field label="Bandwidth limit"><Input value={bwlimit} onChange={(e) => setBwlimit(e.target.value)} placeholder="kbps" /></Field>
        <label className="flex items-end gap-2 text-sm">
          <input type="checkbox" checked={fleeceOn} onChange={(e) => setFleeceOn(e.target.checked)} />
          Fleecing
        </label>
        <Field label="Fleecing storage">
          <Select value={fleeceStore} onChange={(e) => setFleeceStore(e.target.value)}>
            <option value="">Default</option>
            {fleeceStores.map((s) => <option key={s} value={s}>{s}</option>)}
          </Select>
        </Field>
        <Field label="PBS change detection">
          <Select value={pbsDetect} onChange={(e) => setPbsDetect(e.target.value)}>
            <option value="">Default</option>
            <option value="legacy">legacy</option>
            <option value="data">data</option>
            <option value="metadata">metadata</option>
          </Select>
        </Field>
        <label className="flex items-end gap-2 text-sm">
          <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} />
          Enabled
        </label>
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={all} onChange={(e) => setAll(e.target.checked)} />
        All guests
      </label>
      <div className="max-h-48 overflow-auto rounded-lg border border-line p-3">
        {guests.map((g) => {
          const id = g.vmid ?? 0;
          const checked = all ? exclude.includes(id) : selected.includes(id);
          return (
            <label key={g.id} className="flex items-center gap-2 py-0.5 text-sm text-ink">
              <input
                type="checkbox"
                checked={checked}
                onChange={() => toggle(id, all ? exclude : selected, all ? setExclude : setSelected)}
              />
              <span className="font-mono text-xs">{id}</span>
              <span>{g.name || g.type}</span>
              {all && <span className="text-xs text-ink-dim">exclude</span>}
            </label>
          );
        })}
        {!guests.length && <p className="text-xs text-ink-dim">No guests in inventory.</p>}
      </div>
      {volumes.data && <VolumeTree node={volumes.data} />}
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function VolumeTree({ node, depth = 0 }: { node: IncludedVolume; depth?: number }) {
  const kids = node.children ?? [];
  const label = node.name || String(node.id ?? "");
  if (!label && !kids.length) return null;
  return (
    <div className="text-xs text-ink-dim" style={{ paddingLeft: depth * 12 }}>
      {label && (
        <div className="py-0.5">
          {label}
          {node.included === 0 && <span className="ml-2 text-ink-faint">excluded</span>}
        </div>
      )}
      {kids.map((c, i) => <VolumeTree key={`${c.id}-${i}`} node={c} depth={depth + 1} />)}
    </div>
  );
}

function CoverageTab() {
  const cov = useQuery({ queryKey: ["coverage"], queryFn: api.coverage });
  const data: CoverageReport | undefined = cov.data;
  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-2">
        <Card className="p-4">
          <div className="text-xs uppercase tracking-wide text-ink-dim">Unprotected</div>
          <div className="mt-1 text-2xl font-semibold">{data?.notBackedUp?.length ?? "—"}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs uppercase tracking-wide text-ink-dim">Guests</div>
          <div className="mt-1 text-2xl font-semibold">{data?.guests?.length ?? "—"}</div>
        </Card>
      </div>
      <Card className="overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Guest</th>
              <th>In a job</th>
              <th>Last backup</th>
            </tr>
          </thead>
          <tbody>
            {(data?.guests ?? []).map((g) => (
              <tr key={g.vmid} className="border-t border-line">
                <td className="px-4 py-2">
                  <span className="font-mono text-xs">{g.vmid}</span> {g.name || g.type}
                </td>
                <td><Badge tone={g.inJob ? "green" : "rose"}>{g.inJob ? "covered" : "missing"}</Badge></td>
                <td className="text-ink-dim">{relTime(g.lastBackup)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!data?.guests?.length && <p className="p-4 text-sm text-ink-dim">No guest coverage yet.</p>}
      </Card>
    </div>
  );
}

function ReplicationTab() {
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const resources = summary.data?.resources ?? [];
  return (
    <ReplicationPanel
      nodes={resources.filter((r) => r.type === "node")}
      guests={resources.filter((r) => r.type === "qemu" || r.type === "lxc")}
    />
  );
}
