import { attachTask } from "@/components/layout/AppShell";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { InventoryCard, InventoryEmpty, InventoryRow, joinSummary } from "@/components/ui/inventory";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { relTime } from "@/lib/format";
import { guestKeys } from "@/lib/tasks";
import type { ReplicationJob, ReplicationStatus, Resource, TaskLogLine } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useCan } from "@/lib/perms";

export function ReplicationPanel({
  vmid,
  node,
  nodes,
  sourceNode,
  guests,
}: {
  vmid?: number;
  node?: string;
  nodes: Resource[];
  sourceNode?: string;
  guests?: Resource[];
}) {
  const qc = useQueryClient();
  const jobs = useQuery({
    queryKey: ["replication"],
    queryFn: api.replication,
    refetchInterval: 15000,
  });
  const mine = (jobs.data ?? []).filter((j) => {
    if (sourceNode) return j.source === sourceNode;
    if (vmid != null) return Number(j.guest) === vmid;
    return true;
  });
  const targets = nodes.filter((n) => n.node && (!node || n.node !== node)).map((n) => n.node as string);
  const can = useCan();
  // Proxmox allows replication changes with VM.Replicate on the guest, or Sys.Modify.
  const canManage = (guest?: number) =>
    can("/", "Sys.Modify") || (guest != null ? can(`/vms/${guest}`, "VM.Replicate") : false);
  const canCreate = !sourceNode && (vmid != null || guests !== undefined) && (vmid != null ? canManage(vmid) : can("/", "Sys.Modify"));
  const [dialog, setDialog] = useState<
    | { kind: "add" }
    | { kind: "edit"; job: ReplicationJob }
    | { kind: "status"; job: ReplicationJob }
    | { kind: "delete"; job: ReplicationJob }
    | null
  >(null);

  function refresh() {
    void qc.invalidateQueries({ queryKey: ["replication"] });
  }

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Replication"
        actions={canCreate ? <BarButton onClick={() => setDialog({ kind: "add" })}>Add</BarButton> : undefined}
        message={sourceNode && !mine.length ? "No replication jobs sourced from this node." : undefined}
      >
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {!mine.length && <InventoryEmpty>{canCreate ? "No replication jobs" : "No replication jobs for this view"}</InventoryEmpty>}
            {mine.map((job) => {
              const summary = joinSummary([
                vmid == null && job.guest != null && `guest ${job.guest}`,
                job.target && `→ ${job.target}`,
                job.schedule || "*/15",
                job.disable ? "disabled" : "enabled",
                job.comment,
              ]);
              return (
                <InventoryRow
                  key={job.id}
                  kind={job.id}
                  summary={summary}
                  onEdit={canManage(job.guest ?? vmid) ? () => setDialog({ kind: "edit", job }) : undefined}
                >
                  {canManage(job.guest ?? vmid) && (
                    <Button size="sm" variant="outline" onClick={() => setDialog({ kind: "edit", job })}>Edit</Button>
                  )}
                  <Button size="sm" variant="outline" onClick={() => setDialog({ kind: "status", job })}>Status</Button>
                  {canManage(job.guest ?? vmid) && (
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={async () => {
                      const res = await api.replicationNow(job.id, job.source || node);
                      attachTask(res.upid, [["replication"], ["repl-status", job.id], ...(job.guest != null ? guestKeys(job.guest) : vmid != null ? guestKeys(vmid) : [])]);
                    }}
                  >
                    Schedule now
                  </Button>
                  )}
                  {canManage(job.guest ?? vmid) && (
                    <Button size="sm" variant="danger" onClick={() => setDialog({ kind: "delete", job })}>Delete</Button>
                  )}
                </InventoryRow>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>

      {dialog?.kind === "add" && (vmid != null || guests !== undefined) && (
        <ReplicationDialog
          title="Add replication"
          targets={targets}
          guests={vmid == null ? guests : undefined}
          guestVmid={vmid}
          onClose={() => setDialog(null)}
          onSave={async (body) => {
            const guest = vmid ?? Number(body.guest);
            await api.createReplication({ type: "local", ...body, guest });
            setDialog(null);
            refresh();
          }}
        />
      )}
      {dialog?.kind === "edit" && (
        <ReplicationDialog
          title={`Edit ${dialog.job.id}`}
          job={dialog.job}
          onClose={() => setDialog(null)}
          onSave={async (body) => {
            await api.updateReplication(dialog.job.id, body);
            setDialog(null);
            refresh();
          }}
        />
      )}
      {dialog?.kind === "status" && (
        <Modal title={`Status ${dialog.job.id}`} onClose={() => setDialog(null)} size="lg">
          <JobStatus id={dialog.job.id} node={dialog.job.source || node} />
        </Modal>
      )}
      {dialog?.kind === "delete" && (
        <DeleteReplicationDialog
          job={dialog.job}
          onClose={() => setDialog(null)}
          onDeleted={() => {
            setDialog(null);
            refresh();
          }}
        />
      )}
    </div>
  );
}

function ReplicationDialog({
  title,
  job,
  targets,
  guests,
  guestVmid,
  onClose,
  onSave,
}: {
  title: string;
  job?: ReplicationJob;
  targets?: string[];
  guests?: Resource[];
  guestVmid?: number;
  onClose: () => void;
  onSave: (body: Record<string, unknown>) => Promise<void>;
}) {
  const [guest, setGuest] = useState(guestVmid != null ? String(guestVmid) : "");
  const [target, setTarget] = useState(job?.target || (guests ? "" : targets?.[0] || ""));
  const [schedule, setSchedule] = useState(job?.schedule ?? "*/15");
  const [comment, setComment] = useState(job?.comment ?? "");
  const [rate, setRate] = useState(job?.rate != null ? String(job.rate) : "");
  const [disabled, setDisabled] = useState(!!job?.disable);
  const [error, setError] = useState("");
  const selectedGuest = guests?.find((g) => String(g.vmid) === guest);
  const targetOptions = (targets ?? []).filter((n) => n !== selectedGuest?.node);
  return (
    <Modal
      title={title}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            if (!job && guests && !guest) {
              setError("Choose a guest");
              return;
            }
            if (!job && !target) {
              setError("Choose a target node");
              return;
            }
            try {
              await onSave(job
                ? { schedule, comment, rate, disable: disabled }
                : { guest: guestVmid ?? Number(guest), target, schedule, comment, rate: rate || undefined });
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            }
          }}
        >
          {job ? "Save" : "Create"}
        </Button>
      }
    >
      <div className="grid gap-3 md:grid-cols-2">
        {!job && guests && (
          <Field label="Guest">
            <Select value={guest} onChange={(e) => setGuest(e.target.value)}>
              <option value="">Select…</option>
              {guests.map((g) => (
                <option key={g.id} value={String(g.vmid)}>{g.vmid} {g.name}</option>
              ))}
            </Select>
          </Field>
        )}
        {!job && (
          <Field label="Target node">
            <Select value={target} onChange={(e) => setTarget(e.target.value)}>
              <option value="">Select…</option>
              {targetOptions.map((n) => <option key={n}>{n}</option>)}
            </Select>
          </Field>
        )}
        <Field label="Schedule"><Input value={schedule} onChange={(e) => setSchedule(e.target.value)} placeholder="*/15" /></Field>
        <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
        <Field label="Rate (MB/s)"><Input value={rate} onChange={(e) => setRate(e.target.value)} placeholder="unlimited" /></Field>
        {job && (
          <label className="flex items-end gap-2 text-sm">
            <input type="checkbox" checked={disabled} onChange={(e) => setDisabled(e.target.checked)} />
            Disabled
          </label>
        )}
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function DeleteReplicationDialog({
  job, onClose, onDeleted,
}: {
  job: ReplicationJob; onClose: () => void; onDeleted: () => void;
}) {
  const [keep, setKeep] = useState(false);
  const [force, setForce] = useState(false);
  const [removeJob, setRemoveJob] = useState<"" | "local" | "full">("");
  return (
    <Modal
      title={`Delete replication ${job.id}?`}
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button
            variant="danger"
            onClick={async () => {
              if (removeJob) await api.updateReplication(job.id, { remove_job: removeJob });
              await api.deleteReplication(job.id, { keep, force });
              onDeleted();
            }}
          >
            Delete
          </Button>
        </>
      }
    >
      <p className="text-sm text-ink-dim">The job is removed. Replicated volumes on the target stay unless you choose full remove.</p>
      <label className="mt-3 flex items-center gap-2 text-sm text-ink-dim">
        <input type="checkbox" checked={keep} onChange={(e) => setKeep(e.target.checked)} />
        Keep replicated data
      </label>
      <label className="mt-2 flex items-center gap-2 text-sm text-ink-dim">
        <input type="checkbox" checked={force} onChange={(e) => setForce(e.target.checked)} />
        Force (no cleanup)
      </label>
      <Field label="Remove job">
        <Select value={removeJob} onChange={(e) => setRemoveJob(e.target.value as "" | "local" | "full")}>
          <option value="">Default</option>
          <option value="local">local</option>
          <option value="full">full</option>
        </Select>
      </Field>
    </Modal>
  );
}

function JobStatus({ id, node }: { id: string; node?: string }) {
  const status = useQuery({ queryKey: ["repl-status", id], queryFn: () => api.replicationStatus(id, node) });
  const log = useQuery({ queryKey: ["repl-log", id], queryFn: () => api.replicationLog(id, node) });
  const st = status.data as ReplicationStatus | undefined;
  return (
    <div className="space-y-2">
      <div className="grid gap-2 text-xs text-ink-dim md:grid-cols-3">
        <div>Last sync {relTime(st?.last_sync)}</div>
        <div>Duration {st?.duration != null ? `${st.duration}s` : "—"}</div>
        <div>Failures {st?.fail_count ?? 0}</div>
      </div>
      {st?.error && <p className="text-sm text-rose-400">{st.error}</p>}
      <pre className="max-h-40 overflow-auto rounded-lg bg-black/40 p-3 text-xs text-ink-dim">
        {(log.data as TaskLogLine[] | undefined)?.map((l) => l.t).join("\n") || "No log yet."}
      </pre>
    </div>
  );
}
