import { attachTask } from "@/components/layout/AppShell";
import { Badge, Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard } from "@/components/ui/inventory";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import { fleetKeys } from "@/lib/tasks";
import type { HACurrent, HAGroup, HAResource, HARule, Resource } from "@/types";
import { haSid, haTone } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Tabs } from "@/components/ui/tabs";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

const tabs = ["status", "resources", "groups", "rules"] as const;

export function HAPage() {
  const rules = useQuery({ queryKey: ["ha-rules"], queryFn: api.haRules });
  const groups = useQuery({ queryKey: ["ha-groups"], queryFn: api.haGroups, retry: false });
  const supported = rules.data?.supported !== false || rules.isLoading;
  // Proxmox VE 9 migrates HA groups into rules and refuses to list groups.
  const groupsMigrated = groups.error instanceof Error && /migrated to rules/i.test(groups.error.message);
  const visible = tabs.filter((t) => (t !== "rules" || supported) && (t !== "groups" || !groupsMigrated));
  const [tab, setTab] = useState<(typeof tabs)[number]>("status");
  const current = visible.includes(tab) ? tab : "status";

  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <div>
        <h2 className="text-base font-semibold">High availability</h2>
        <p className="text-sm text-ink-dim">HA manager resources, groups, and placement. Relocate goes through the CRM.</p>
      </div>
      <Tabs items={visible} value={current} onChange={setTab} />
      {current === "status" && <StatusTab />}
      {current === "resources" && <ResourcesTab />}
      {current === "groups" && !groupsMigrated && <GroupsTab />}
      {current === "rules" && supported && <RulesTab />}
    </div>
  );
}

function StatusTab() {
  const qc = useQueryClient();
  const can = useCan();
  const canArm = can("/", "Sys.Console");
  const st = useQuery({ queryKey: ["ha-status"], queryFn: api.haStatus, refetchInterval: 10000 });
  const current = st.data?.current ?? [];
  const services = current.filter((c) => c.type === "service" || c.sid);
  const lrms = current.filter((c) => c.type === "lrm" || (c.node && !c.sid && c.type !== "quorum" && c.type !== "service" && c.type !== "master"));
  const quorum = current.find((c) => c.type === "quorum");
  const manager = st.data?.manager as { master_node?: string; master?: string; node_status?: Record<string, string> } | undefined;
  // Proxmox reports the CRM master as a "master" row in the status list.
  const masterRow = current.find((c) => c.type === "master");
  const master = manager?.master_node || manager?.master || masterRow?.node || "";
  const errors = services.filter((s) => /error|recovery|fence/i.test(s.state || s.status || ""));
  const [arm, setArm] = useState(false);
  const [disarm, setDisarm] = useState(false);
  const [mode, setMode] = useState<"freeze" | "ignore">("freeze");

  return (
    <div className="space-y-4">
      {canArm && (
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="outline" onClick={() => setArm(true)}>Arm HA</Button>
          <Button size="sm" variant="danger" onClick={() => setDisarm(true)}>Disarm HA</Button>
        </div>
      )}
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="p-4">
          <div className="text-xs uppercase tracking-wide text-ink-dim">CRM master</div>
          <div className="mt-1 text-2xl font-semibold">{master || "—"}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs uppercase tracking-wide text-ink-dim">Quorum</div>
          <div className="mt-1 text-2xl font-semibold">{quorum ? (quorum.quorate ? "Quorate" : quorum.status || "—") : "—"}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs uppercase tracking-wide text-ink-dim">HA errors</div>
          <div className="mt-1 text-2xl font-semibold">{errors.length}</div>
        </Card>
      </div>
      <Card className="overflow-hidden">
        <h2 className="px-4 pt-4 font-medium">Node LRM</h2>
        <table className="mt-2 w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Node</th><th>Status</th></tr>
          </thead>
          <tbody>
            {lrms.map((n) => (
              <tr key={n.node || n.id} className="border-t border-line">
                <td className="px-4 py-2">{n.node}</td>
                <td><Badge tone={haTone(n.status || n.state)}>{n.status || n.state || "—"}</Badge></td>
              </tr>
            ))}
            {manager?.node_status && lrms.length === 0 && Object.entries(manager.node_status).map(([node, status]) => (
              <tr key={node} className="border-t border-line">
                <td className="px-4 py-2">{node}</td>
                <td><Badge tone={haTone(status)}>{status}</Badge></td>
              </tr>
            ))}
          </tbody>
        </table>
        {!lrms.length && !manager?.node_status && <p className="p-4 text-sm text-ink-dim">No LRM status yet.</p>}
      </Card>
      <Card className="overflow-hidden">
        <h2 className="px-4 pt-4 font-medium">Resources</h2>
        <ServiceTable items={services} />
      </Card>
      {arm && (
        <ConfirmDialog
          title="Arm the HA stack?"
          body="Re-arms cluster HA after it was disarmed. Watchdogs start enforcing again."
          confirmLabel="Arm"
          onClose={() => setArm(false)}
          onConfirm={async () => {
            await api.armHA();
            setArm(false);
            void qc.invalidateQueries({ queryKey: ["ha-status"] });
          }}
        />
      )}
      {disarm && (
        <Modal
          title="Disarm the HA stack?"
          onClose={() => setDisarm(false)}
          footer={
            <Button
              variant="danger"
              onClick={async () => {
                await api.disarmHA(mode);
                setDisarm(false);
                void qc.invalidateQueries({ queryKey: ["ha-status"] });
              }}
            >
              Disarm
            </Button>
          }
        >
          <p className="text-sm text-ink-dim">Releases watchdogs cluster-wide. Current resource state is not changed.</p>
          <Field label="Resource mode">
            <Select value={mode} onChange={(e) => setMode(e.target.value as "freeze" | "ignore")}>
              <option value="freeze">freeze — hold HA commands</option>
              <option value="ignore">ignore — manage guests as if they were not HA</option>
            </Select>
          </Field>
        </Modal>
      )}
    </div>
  );
}

function ServiceTable({ items }: { items: HACurrent[] }) {
  return (
    <table className="mt-2 w-full text-sm">
      <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
        <tr>
          <th className="px-4 py-2">SID</th>
          <th>State</th>
          <th>Node</th>
          <th>Request</th>
        </tr>
      </thead>
      <tbody>
        {items.map((s) => (
          <tr key={s.sid || s.id} className="border-t border-line">
            <td className="px-4 py-2 font-mono text-xs">{s.sid}</td>
            <td><Badge tone={haTone(s.state || s.crm_state)}>{s.state || s.crm_state || "—"}</Badge></td>
            <td>{s.node || "—"}</td>
            <td className="text-ink-dim">{s.request || "—"}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function ResourcesTab() {
  const qc = useQueryClient();
  const resources = useQuery({ queryKey: ["ha-resources"], queryFn: api.haResources });
  const status = useQuery({ queryKey: ["ha-status"], queryFn: api.haStatus, refetchInterval: 10000 });
  const groups = useQuery({ queryKey: ["ha-groups"], queryFn: api.haGroups });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const [adding, setAdding] = useState(false);
  const [edit, setEdit] = useState<HAResource | null>(null);
  const [del, setDel] = useState<HAResource | null>(null);
  const [purge, setPurge] = useState(true);
  const [move, setMove] = useState<{ sid: string; action: "migrate" | "relocate" } | null>(null);
  const [queued, setQueued] = useState<string | null>(null);
  const guests = (summary.data?.resources ?? []).filter((r) => r.type === "qemu" || r.type === "lxc");
  const nodes = (summary.data?.resources ?? []).filter((r) => r.type === "node");
  const live = Object.fromEntries((status.data?.current ?? []).filter((c) => c.sid).map((c) => [c.sid!, c]));

  const refresh = () => {
    void qc.invalidateQueries({ queryKey: ["ha-resources"] });
    void qc.invalidateQueries({ queryKey: ["ha-status"] });
  };

  return (
    <div className="space-y-4">
      <InventoryCard
        title="HA resources"
        actions={<BarButton onClick={() => setAdding(true)}>Add</BarButton>}
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
            {!(resources.data ?? []).length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No HA resources</td>
              </tr>
            )}
            {(resources.data ?? []).map((res) => {
              const cur = live[res.sid];
              const guest = guests.find((g) => haSid(g.type, g.vmid ?? 0) === res.sid);
              const state = queued === res.sid ? "queued" : cur?.state || res.state || "—";
              const summary = [guest?.name, state, res.group, cur?.node || guest?.node, cur?.request].filter(Boolean).join(" · ");
              return (
                <tr
                  key={res.sid}
                  className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
                  onClick={() => setEdit(res)}
                >
                  <td className="px-4 py-2 font-medium">{res.sid}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>
                    <span className="mr-2"><Badge tone={haTone(state)}>{state}</Badge></span>
                    {summary}
                  </td>
                  <td className="space-x-1 px-2 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    <Button size="sm" variant="outline" onClick={() => void setState(res.sid, "started", refresh)}>Arm</Button>
                    <Button size="sm" variant="outline" onClick={() => void setState(res.sid, "disabled", refresh)}>Disarm</Button>
                    <Button size="sm" variant="outline" onClick={() => setMove({ sid: res.sid, action: "migrate" })}>Migrate</Button>
                    <Button size="sm" variant="outline" onClick={() => setMove({ sid: res.sid, action: "relocate" })}>Relocate</Button>
                    <Button size="sm" variant="outline" onClick={() => setEdit(res)}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={() => setDel(res)}>Delete</Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <ResourceForm
          guests={guests}
          groups={groups.data ?? []}
          onClose={() => setAdding(false)}
          onSaved={() => { setAdding(false); refresh(); }}
        />
      )}
      {edit && (
        <ResourceForm
          existing={edit}
          guests={guests}
          groups={groups.data ?? []}
          onClose={() => setEdit(null)}
          onSaved={() => { setEdit(null); refresh(); }}
        />
      )}
      {move && (
        <MoveDialog
          sid={move.sid}
          action={move.action}
          nodes={nodes}
          onClose={() => setMove(null)}
          onQueued={(upid) => {
            if (upid) attachTask(upid, [["ha-status"], ["ha-resources"], ["ha-resource", move.sid], ...fleetKeys()]);
            else refresh();
            setQueued(move.sid);
            setMove(null);
          }}
        />
      )}
      {del && (
        <Modal
          title={`Unregister ${del.sid}?`}
          onClose={() => setDel(null)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setDel(null)}>Cancel</Button>
              <Button variant="danger" onClick={async () => {
                await api.deleteHAResource(del.sid, { purge });
                setDel(null);
                setPurge(true);
                refresh();
              }}>Remove</Button>
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

async function setState(sid: string, state: string, refresh: () => void) {
  await api.updateHAResource(sid, { state });
  refresh();
}

function ResourceForm({
  existing, guests, groups, onClose, onSaved,
}: {
  existing?: HAResource;
  guests: Resource[];
  groups: HAGroup[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [sid, setSid] = useState(existing?.sid ?? "");
  const [state, setStateVal] = useState(existing?.state ?? "started");
  const [group, setGroup] = useState(existing?.group ?? "");
  const [maxRestart, setMaxRestart] = useState(String(existing?.max_restart ?? 1));
  const [maxRelocate, setMaxRelocate] = useState(String(existing?.max_relocate ?? 1));
  const [comment, setComment] = useState(existing?.comment ?? "");
  const [error, setError] = useState("");

  return (
    <Modal
      title={existing ? `Edit ${existing.sid}` : "Add HA resource"}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            const body = {
              state, group, comment,
              maxRestart: Number(maxRestart),
              maxRelocate: Number(maxRelocate),
            };
            try {
              if (existing) await api.updateHAResource(existing.sid, body);
              else await api.createHAResource({ sid, ...body });
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
      <div className="grid gap-3 md:grid-cols-2">
      <Field label="Guest">
        {existing ? (
          <Input value={existing.sid} disabled />
        ) : (
          <Select value={sid} onChange={(e) => setSid(e.target.value)}>
            <option value="">Select…</option>
            {guests.map((g) => (
              <option key={g.id} value={haSid(g.type, g.vmid ?? 0)}>{g.vmid} {g.name}</option>
            ))}
          </Select>
        )}
      </Field>
      <Field label="State">
        <Select value={state} onChange={(e) => setStateVal(e.target.value)}>
          <option value="started">started (armed)</option>
          <option value="stopped">stopped</option>
          <option value="disabled">disabled (disarmed)</option>
          <option value="ignored">ignored</option>
        </Select>
      </Field>
      <Field label="Group">
        <Select value={group} onChange={(e) => setGroup(e.target.value)}>
          <option value="">None</option>
          {groups.map((g) => <option key={g.group}>{g.group}</option>)}
        </Select>
      </Field>
      <Field label="Max restart"><Input type="number" value={maxRestart} onChange={(e) => setMaxRestart(e.target.value)} /></Field>
      <Field label="Max relocate"><Input type="number" value={maxRelocate} onChange={(e) => setMaxRelocate(e.target.value)} /></Field>
      <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function MoveDialog({
  sid, action, nodes, onClose, onQueued,
}: {
  sid: string;
  action: "migrate" | "relocate";
  nodes: Resource[];
  onClose: () => void;
  onQueued: (upid?: string) => void;
}) {
  const [node, setNode] = useState(nodes[0]?.node ?? "");
  const [error, setError] = useState("");
  return (
    <Modal
      title={`${action === "migrate" ? "HA migrate" : "Relocate"} ${sid}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              const res = action === "migrate" ? await api.haMigrate(sid, node) : await api.haRelocate(sid, node);
              onQueued(res.upid);
            } catch (err) {
              setError(err instanceof Error ? err.message : "Move failed");
            }
          }}
        >
          {action === "migrate" ? "Migrate" : "Relocate"}
        </Button>
      }
    >
      <p className="text-sm text-ink-dim">
        {action === "migrate" ? "Live move through the HA manager." : "Stop on the current node and start on the target."}
      </p>
      <Field label="Target node">
        <Select value={node} onChange={(e) => setNode(e.target.value)}>
          {nodes.map((n) => <option key={n.id} value={n.node}>{n.node}</option>)}
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function GroupsTab() {
  const qc = useQueryClient();
  const groups = useQuery({ queryKey: ["ha-groups"], queryFn: api.haGroups });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const nodes = (summary.data?.resources ?? []).filter((r) => r.type === "node");
  const [editing, setEditing] = useState<HAGroup | "new" | null>(null);
  const [del, setDel] = useState<HAGroup | null>(null);

  return (
    <div className="space-y-4">
      <InventoryCard
        title="HA groups"
        actions={<BarButton onClick={() => setEditing("new")}>Add</BarButton>}
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
            {!(groups.data ?? []).length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No HA groups</td>
              </tr>
            )}
            {(groups.data ?? []).map((g) => {
              const summary = [g.nodes, g.restricted ? "restricted" : "", g.nofailback ? "no failback" : "", g.comment].filter(Boolean).join(" · ");
              return (
                <tr
                  key={g.group}
                  className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
                  onClick={() => setEditing(g)}
                >
                  <td className="px-4 py-2 font-medium">{g.group}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary || "—"}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    <Button size="sm" variant="outline" onClick={() => setEditing(g)}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={() => setDel(g)}>Delete</Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {editing && (
        <GroupForm
          group={editing === "new" ? null : editing}
          nodes={nodes}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            void qc.invalidateQueries({ queryKey: ["ha-groups"] });
          }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete group ${del.group}?`}
          body="Resources in this group keep running but lose the group constraint."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteHAGroup(del.group);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["ha-groups"] });
          }}
        />
      )}
    </div>
  );
}

function GroupForm({
  group, nodes, onClose, onSaved,
}: {
  group: HAGroup | null;
  nodes: Resource[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const parsed = Object.fromEntries(
    (group?.nodes ?? "").split(",").map((p) => {
      const [n, pr] = p.split(":");
      return [n, pr || "1"];
    }).filter(([n]) => n),
  );
  const [name, setName] = useState(group?.group ?? "");
  const [prios, setPrios] = useState<Record<string, string>>(() => {
    const init: Record<string, string> = {};
    for (const n of nodes) init[n.node ?? ""] = parsed[n.node ?? ""] ?? "";
    return init;
  });
  const [restricted, setRestricted] = useState(!!group?.restricted);
  const [nofailback, setNofailback] = useState(!!group?.nofailback);
  const [comment, setComment] = useState(group?.comment ?? "");
  const [error, setError] = useState("");

  const nodeStr = Object.entries(prios)
    .filter(([, p]) => p !== "")
    .map(([n, p]) => `${n}:${p || "1"}`)
    .join(",");

  return (
    <Modal
      title={group ? `Edit ${group.group}` : "Add HA group"}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              const body = { group: name, nodes: nodeStr, restricted, nofailback, comment };
              if (group) await api.updateHAGroup(group.group, body);
              else await api.createHAGroup(body);
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
      <Field label="Name"><Input value={name} disabled={!!group} onChange={(e) => setName(e.target.value)} /></Field>
      <div className="space-y-2">
        <div className="text-xs font-medium uppercase tracking-wide text-ink-dim">Nodes and priority</div>
        {nodes.map((n) => (
          <div key={n.id} className="flex items-center gap-2">
            <span className="w-32 text-sm">{n.node}</span>
            <Input
              className="max-w-[6rem]"
              placeholder="prio"
              value={prios[n.node ?? ""] ?? ""}
              onChange={(e) => setPrios({ ...prios, [n.node ?? ""]: e.target.value })}
            />
          </div>
        ))}
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={restricted} onChange={(e) => setRestricted(e.target.checked)} /> Restricted
      </label>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={nofailback} onChange={(e) => setNofailback(e.target.checked)} /> No failback
      </label>
      <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function RulesTab() {
  const qc = useQueryClient();
  const rules = useQuery({ queryKey: ["ha-rules"], queryFn: api.haRules });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const guests = (summary.data?.resources ?? []).filter((r) => r.type === "qemu" || r.type === "lxc");
  const nodes = (summary.data?.resources ?? []).filter((r) => r.type === "node");
  const [editing, setEditing] = useState<HARule | "new" | null>(null);
  const [del, setDel] = useState<HARule | null>(null);

  if (rules.data && !rules.data.supported) {
    return <p className="text-sm text-ink-dim">This cluster does not expose HA rules.</p>;
  }

  return (
    <div className="space-y-4">
      <InventoryCard
        title="HA rules"
        actions={<BarButton onClick={() => setEditing("new")}>Add</BarButton>}
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
            {!(rules.data?.rules ?? []).length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No placement rules</td>
              </tr>
            )}
            {(rules.data?.rules ?? []).map((rule) => {
              const constraint = rule.type === "resource-affinity" ? rule.affinity : rule.nodes;
              const summary = [rule.type, rule.resources, constraint].filter(Boolean).join(" · ");
              return (
                <tr
                  key={rule.rule}
                  className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
                  onClick={() => setEditing(rule)}
                >
                  <td className="px-4 py-2 font-medium">{rule.rule}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    <Button size="sm" variant="outline" onClick={() => setEditing(rule)}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={() => setDel(rule)}>Delete</Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {editing && (
        <RuleForm
          rule={editing === "new" ? null : editing}
          guests={guests}
          nodes={nodes}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            void qc.invalidateQueries({ queryKey: ["ha-rules"] });
          }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete rule ${del.rule}?`}
          body="The placement constraint is removed."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteHARule(del.rule);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["ha-rules"] });
          }}
        />
      )}
    </div>
  );
}

function RuleForm({
  rule, guests, nodes, onClose, onSaved,
}: {
  rule: HARule | null;
  guests: Resource[];
  nodes: Resource[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const [name, setName] = useState(rule?.rule ?? "");
  const [type, setType] = useState(rule?.type ?? "node-affinity");
  const [selected, setSelected] = useState<string[]>(() => (rule?.resources ?? "").split(",").map((s) => s.trim()).filter(Boolean));
  const [nodeList, setNodeList] = useState<string[]>(() => (rule?.nodes ?? "").split(",").map((s) => s.trim()).filter(Boolean));
  const [affinity, setAffinity] = useState(rule?.affinity ?? "positive");
  const [strict, setStrict] = useState(!!rule?.strict);
  const [comment, setComment] = useState(rule?.comment ?? "");
  const [disable, setDisable] = useState(!!rule?.disable);
  const [error, setError] = useState("");

  const toggle = (id: string, list: string[], set: (v: string[]) => void) => {
    set(list.includes(id) ? list.filter((x) => x !== id) : [...list, id]);
  };

  return (
    <Modal
      title={rule ? `Edit ${rule.rule}` : "Add HA rule"}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            const body = {
              rule: name, type, resources: selected.join(","),
              nodes: type === "node-affinity" ? nodeList.join(",") : "",
              affinity: type === "resource-affinity" ? affinity : "",
              strict, comment, disable,
            };
            try {
              if (rule) await api.updateHARule(rule.rule, body);
              else await api.createHARule(body);
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
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Name"><Input value={name} disabled={!!rule} onChange={(e) => setName(e.target.value)} /></Field>
        <Field label="Type">
          <Select value={type} onChange={(e) => setType(e.target.value)}>
            <option value="node-affinity">Node affinity</option>
            <option value="resource-affinity">Resource affinity</option>
          </Select>
        </Field>
      </div>
      <div className="max-h-40 overflow-auto rounded-lg border border-line p-3">
        {guests.map((g) => {
          const sid = haSid(g.type, g.vmid ?? 0);
          return (
            <label key={g.id} className="flex items-center gap-2 py-0.5 text-sm">
              <input type="checkbox" checked={selected.includes(sid)} onChange={() => toggle(sid, selected, setSelected)} />
              {g.vmid} {g.name}
            </label>
          );
        })}
      </div>
      {type === "node-affinity" ? (
        <div className="rounded-lg border border-line p-3">
          {nodes.map((n) => (
            <label key={n.id} className="flex items-center gap-2 py-0.5 text-sm">
              <input
                type="checkbox"
                checked={nodeList.includes(n.node ?? "")}
                onChange={() => toggle(n.node ?? "", nodeList, setNodeList)}
              />
              {n.node}
            </label>
          ))}
          <label className="mt-2 flex items-center gap-2 text-sm">
            <input type="checkbox" checked={strict} onChange={(e) => setStrict(e.target.checked)} /> Strict
          </label>
        </div>
      ) : (
        <Field label="Affinity">
          <Select value={affinity} onChange={(e) => setAffinity(e.target.value)}>
            <option value="positive">Keep together</option>
            <option value="negative">Keep apart</option>
          </Select>
        </Field>
      )}
      <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={disable} onChange={(e) => setDisable(e.target.checked)} />
        Disable rule
      </label>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
