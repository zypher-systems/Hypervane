import { attachTask } from "@/components/layout/AppShell";
import { Badge } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { relTime } from "@/lib/format";
import { fleetKeys } from "@/lib/tasks";
import type { PrunePreview } from "@/types";
import { useState } from "react";
import { useCan } from "@/lib/perms";

export function PruneCard({ storage, node }: { storage: string; node: string }) {
  const [prune, setPrune] = useState("keep-last=3,keep-daily=7,keep-weekly=4");
  const [preview, setPreview] = useState<PrunePreview[] | null>(null);
  const [error, setError] = useState("");
  const [confirm, setConfirm] = useState(false);
  const can = useCan();
  const canPrune = can(`/storage/${storage}`, "Datastore.Allocate");

  return (
    <div className="hv-form-stack space-y-3">
      <Field label="Retention">
        <Input value={prune} onChange={(e) => setPrune(e.target.value)} />
      </Field>
      <div className="flex gap-2">
        <Button
          size="sm"
          variant="outline"
          onClick={async () => {
            setError("");
            try {
              setPreview(await api.prunePreview(storage, node, prune));
            } catch (err) {
              setError(err instanceof Error ? err.message : "Preview failed");
            }
          }}
        >
          Dry run
        </Button>
        {canPrune && (
        <Button size="sm" variant="danger" disabled={!preview} onClick={() => setConfirm(true)}>
          Apply prune
        </Button>
        )}
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
      {preview && (
        <table className="w-full text-sm">
          <thead className="text-left text-xs font-medium text-ink-dim">
            <tr><th className="py-1">Volume</th><th>Mark</th><th>Created</th></tr>
          </thead>
          <tbody>
            {preview.map((p) => (
              <tr key={p.volid} className="border-t border-white/10">
                <td className="py-1 font-mono text-xs">{p.volid}</td>
                <td>
                  <Badge tone={p.mark === "remove" ? "rose" : p.mark === "protected" ? "amber" : "green"}>
                    {p.mark || "keep"}
                  </Badge>
                </td>
                <td className="text-ink-faint">{relTime(p.ctime)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {confirm && (
        <ConfirmDialog
          title="Apply prune?"
          body="Volumes marked remove will be deleted according to this retention policy."
          danger
          confirmLabel="Prune"
          onClose={() => setConfirm(false)}
          onConfirm={async () => {
            const res = await api.prune(storage, node, prune);
            attachTask(res.upid, [["content", storage, node], ["guest-backups"], ...fleetKeys()]);
            setConfirm(false);
            setPreview(null);
          }}
        />
      )}
    </div>
  );
}
