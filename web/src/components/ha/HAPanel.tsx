import { attachTask } from "@/components/layout/AppShell";
import { Badge, Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { guestKeys } from "@/lib/tasks";
import type { Resource } from "@/types";
import { haSid, haTone } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useCan } from "@/lib/perms";

export function HAPanel({ vmid, type, nodes }: { vmid: number; type: string; nodes: Resource[] }) {
  const sid = haSid(type, vmid);
  const qc = useQueryClient();
  const can = useCan();
  // Proxmox requires Sys.Console on / to change HA resources.
  const canManage = can("/", "Sys.Console");
  const resource = useQuery({
    queryKey: ["ha-resource", sid],
    queryFn: () => api.haResource(sid),
    retry: false,
  });
  const groups = useQuery({ queryKey: ["ha-groups"], queryFn: api.haGroups });
  const status = useQuery({ queryKey: ["ha-status"], queryFn: api.haStatus, refetchInterval: 10000 });
  const [state, setState] = useState("");
  const [group, setGroup] = useState("");
  const [maxRestart, setMaxRestart] = useState("1");
  const [maxRelocate, setMaxRelocate] = useState("1");
  const [comment, setComment] = useState("");
  const [error, setError] = useState("");
  const [move, setMove] = useState(false);
  const [target, setTarget] = useState(nodes[0]?.node ?? "");
  const [remove, setRemove] = useState(false);
  const [purge, setPurge] = useState(true);
  const managed = !!resource.data && !resource.isError;
  const live = (status.data?.current ?? []).find((c) => c.sid === sid);

  useEffect(() => {
    if (!resource.data) return;
    setState(resource.data.state || "started");
    setGroup(resource.data.group || "");
    setMaxRestart(String(resource.data.max_restart ?? 1));
    setMaxRelocate(String(resource.data.max_relocate ?? 1));
    setComment(resource.data.comment || "");
  }, [resource.data]);

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ["ha-resource", sid] });
    void qc.invalidateQueries({ queryKey: ["ha-status"] });
    void qc.invalidateQueries({ queryKey: ["ha-resources"] });
  };

  return (
    <div className="space-y-4">
      <Card className="space-y-4 p-5">
        <div className="flex flex-wrap items-center gap-2">
          <Badge tone={managed ? haTone(live?.state || resource.data?.state) : "zinc"}>
            {managed ? live?.state || resource.data?.state || "managed" : "not managed"}
          </Badge>
          {live?.node && <span className="text-xs text-ink-dim">on {live.node}</span>}
          {live?.request && <span className="text-xs text-ink-dim">{live.request}</span>}
        </div>
        <div className="grid gap-3 md:grid-cols-2">
          <Field label="Requested state">
            <Select value={state || "started"} onChange={(e) => setState(e.target.value)}>
              <option value="started">started (armed)</option>
              <option value="stopped">stopped</option>
              <option value="disabled">disabled (disarmed)</option>
              <option value="ignored">ignored</option>
            </Select>
          </Field>
          <Field label="Group">
            <Select value={group} onChange={(e) => setGroup(e.target.value)}>
              <option value="">None</option>
              {(groups.data ?? []).map((g) => <option key={g.group}>{g.group}</option>)}
            </Select>
          </Field>
          <Field label="Max restart"><Input type="number" value={maxRestart} onChange={(e) => setMaxRestart(e.target.value)} /></Field>
          <Field label="Max relocate"><Input type="number" value={maxRelocate} onChange={(e) => setMaxRelocate(e.target.value)} /></Field>
          <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
        </div>
        {error && <p className="text-sm text-danger">{error}</p>}
        {!canManage && <p className="text-xs text-ink-dim">Read-only: changing HA needs the Sys.Console privilege.</p>}
        {canManage && (
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            onClick={async () => {
              setError("");
              const body = { sid, state: state || "started", group, comment, maxRestart: Number(maxRestart), maxRelocate: Number(maxRelocate) };
              try {
                if (managed) await api.updateHAResource(sid, body);
                else await api.createHAResource(body);
                refresh();
              } catch (err) {
                setError(err instanceof Error ? err.message : "Save failed");
              }
            }}
          >
            {managed ? "Save HA" : "Register in HA"}
          </Button>
          {managed && (
            <>
              <Button size="sm" variant="outline" onClick={() => setMove(true)}>Relocate</Button>
              <Button size="sm" variant="danger" onClick={() => setRemove(true)}>Unregister</Button>
            </>
          )}
        </div>
        )}
      </Card>
      {move && (
        <Modal
          title={`Relocate ${sid}`}
          onClose={() => setMove(false)}
          footer={
            <Button
              onClick={async () => {
                try {
                  const res = await api.haRelocate(sid, target);
                  if (res.upid) attachTask(res.upid, [["ha-resource", sid], ["ha-status"], ["ha-resources"], ...guestKeys(vmid)]);
                  else refresh();
                  setMove(false);
                } catch (err) {
                  setError(err instanceof Error ? err.message : "Relocate failed");
                }
              }}
            >
              Relocate
            </Button>
          }
        >
          <Field label="Target node">
            <Select value={target} onChange={(e) => setTarget(e.target.value)}>
              {nodes.map((n) => <option key={n.id} value={n.node}>{n.node}</option>)}
            </Select>
          </Field>
        </Modal>
      )}
      {remove && (
        <Modal
          title={`Unregister ${sid}?`}
          onClose={() => setRemove(false)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setRemove(false)}>Cancel</Button>
              <Button variant="danger" onClick={async () => {
                await api.deleteHAResource(sid, { purge });
                setRemove(false);
                setPurge(true);
                setState("");
                refresh();
              }}>Unregister</Button>
            </>
          }
        >
          <p className="text-sm text-ink-dim">The guest leaves HA management. It is not deleted.</p>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={purge} onChange={(e) => setPurge(e.target.checked)} />
            Purge from HA rules
          </label>
        </Modal>
      )}
    </div>
  );
}
