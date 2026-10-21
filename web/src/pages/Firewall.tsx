import { LogPanel, OptionsBar, RuleTable } from "@/components/firewall/shared";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard, InventoryEmpty, InventoryRow, InventoryTable, joinSummary } from "@/components/ui/inventory";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import type { FWAlias, FWGroup, FWIPSet, FWIPSetCIDR, Resource } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { Tabs } from "@/components/ui/tabs";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

const tabs = ["datacenter", "node", "groups", "aliases", "ipsets"] as const;

export function FirewallPage() {
  const search = useSearch({ from: "/app/firewall" });
  const [tab, setTab] = useState<(typeof tabs)[number]>(search.tab === "node" ? "node" : "datacenter");

  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <div>
        <h2 className="text-base font-semibold">Firewall</h2>
        <p className="text-sm text-ink-dim">Datacenter, node, and shared objects. Guest rules live on the guest page. VNet rules live on SDN.</p>
      </div>
      <Tabs items={tabs} value={tab} onChange={setTab} label={(t) => t === "ipsets" ? "IP sets" : t} />
      {tab === "datacenter" && <DatacenterTab />}
      {tab === "node" && <NodeTab initial={typeof search.node === "string" ? search.node : ""} />}
      {tab === "groups" && <GroupsTab />}
      {tab === "aliases" && <AliasesTab />}
      {tab === "ipsets" && <IPSetsTab />}
    </div>
  );
}

function DatacenterTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const opts = useQuery({ queryKey: ["fw-dc-opts"], queryFn: api.fwDCOptions });
  const rules = useQuery({ queryKey: ["fw-dc-rules"], queryFn: api.fwDCRules });
  const refs = useQuery({ queryKey: ["fw-dc-refs"], queryFn: api.fwDCRefs });
  const macros = useQuery({ queryKey: ["fw-macros"], queryFn: api.fwMacros });
  const groups = useQuery({ queryKey: ["fw-groups"], queryFn: api.fwGroups });
  const aliases = useQuery({ queryKey: ["fw-dc-aliases"], queryFn: api.fwDCAliases });
  const ipsets = useQuery({ queryKey: ["fw-dc-ipset"], queryFn: api.fwDCIPSets });
  const names = [
    ...(aliases.data ?? []).map((a) => `+${a.name}`),
    ...(ipsets.data ?? []).map((s) => `+${s.name}`),
  ];

  return (
    <div className="space-y-4">
      {opts.data && (
        <OptionsBar
          key={opts.data.digest || "dc"}
          opts={opts.data}
          writable={writable}
          onSave={async (body) => {
            await api.updateFWDCOptions(body);
            void qc.invalidateQueries({ queryKey: ["fw-dc-opts"] });
          }}
        />
      )}
      <RuleTable
        rules={rules.data ?? []}
        writable={writable}
        refs={refs.data?.supported ? refs.data.refs : undefined}
        macros={macros.data}
        groups={(groups.data ?? []).map((g) => g.group)}
        names={names}
        onCreate={async (body) => {
          await api.createFWDCRule(body);
          void qc.invalidateQueries({ queryKey: ["fw-dc-rules"] });
        }}
        onUpdate={async (pos, body) => {
          await api.updateFWDCRule(pos, body);
          void qc.invalidateQueries({ queryKey: ["fw-dc-rules"] });
        }}
        onDelete={async (pos, digest) => {
          await api.deleteFWDCRule(pos, digest);
          void qc.invalidateQueries({ queryKey: ["fw-dc-rules"] });
        }}
      />
      <p className="text-xs text-ink-dim">Firewall logs are kept per node. Open the Node tab to read one.</p>
    </div>
  );
}

function NodeTab({ initial }: { initial: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const nodes = (summary.data?.resources ?? []).filter((r: Resource) => r.type === "node");
  const [node, setNode] = useState(initial || nodes[0]?.node || "");
  const current = node || nodes[0]?.node || "";
  const opts = useQuery({ queryKey: ["fw-node-opts", current], queryFn: () => api.fwNodeOptions(current), enabled: !!current });
  const rules = useQuery({ queryKey: ["fw-node-rules", current], queryFn: () => api.fwNodeRules(current), enabled: !!current });
  const log = useQuery({
    queryKey: ["fw-node-log", current],
    queryFn: () => api.fwNodeLog(current),
    enabled: !!current,
    refetchInterval: 15000,
  });
  const refs = useQuery({ queryKey: ["fw-dc-refs"], queryFn: api.fwDCRefs });
  const macros = useQuery({ queryKey: ["fw-macros"], queryFn: api.fwMacros });
  const groups = useQuery({ queryKey: ["fw-groups"], queryFn: api.fwGroups });
  const aliases = useQuery({ queryKey: ["fw-dc-aliases"], queryFn: api.fwDCAliases });
  const ipsets = useQuery({ queryKey: ["fw-dc-ipset"], queryFn: api.fwDCIPSets });
  const names = [
    ...(aliases.data ?? []).map((a) => `+${a.name}`),
    ...(ipsets.data ?? []).map((s) => `+${s.name}`),
  ];

  return (
    <div className="space-y-4">
      <Field label="Node">
        <Select value={current} onChange={(e) => setNode(e.target.value)} className="max-w-xs">
          {nodes.map((n) => <option key={n.id} value={n.node}>{n.node}</option>)}
        </Select>
      </Field>
      {current && opts.data && (
        <OptionsBar
          key={`${current}-${opts.data.digest || ""}`}
          opts={opts.data}
          writable={writable}
          onSave={async (body) => {
            await api.updateFWNodeOptions(current, body);
            void qc.invalidateQueries({ queryKey: ["fw-node-opts", current] });
          }}
        />
      )}
      {current && (
        <RuleTable
          rules={rules.data ?? []}
          writable={writable}
          refs={refs.data?.supported ? refs.data.refs : undefined}
          macros={macros.data}
          groups={(groups.data ?? []).map((g) => g.group)}
          names={names}
          onCreate={async (body) => {
            await api.createFWNodeRule(current, body);
            void qc.invalidateQueries({ queryKey: ["fw-node-rules", current] });
          }}
          onUpdate={async (pos, body) => {
            await api.updateFWNodeRule(current, pos, body);
            void qc.invalidateQueries({ queryKey: ["fw-node-rules", current] });
          }}
          onDelete={async (pos, digest) => {
            await api.deleteFWNodeRule(current, pos, digest);
            void qc.invalidateQueries({ queryKey: ["fw-node-rules", current] });
          }}
        />
      )}
      {current && <LogPanel lines={log.data ?? []} />}
    </div>
  );
}

function GroupsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const groups = useQuery({ queryKey: ["fw-groups"], queryFn: api.fwGroups });
  const [adding, setAdding] = useState(false);
  const [open, setOpen] = useState<string | null>(null);
  const [del, setDel] = useState<FWGroup | null>(null);
  const rules = useQuery({
    queryKey: ["fw-group-rules", open],
    queryFn: () => api.fwGroupRules(open!),
    enabled: !!open,
  });
  const macros = useQuery({ queryKey: ["fw-macros"], queryFn: api.fwMacros });
  const refs = useQuery({ queryKey: ["fw-dc-refs"], queryFn: api.fwDCRefs });
  const aliases = useQuery({ queryKey: ["fw-dc-aliases"], queryFn: api.fwDCAliases });
  const ipsets = useQuery({ queryKey: ["fw-dc-ipset"], queryFn: api.fwDCIPSets });
  const names = [
    ...(aliases.data ?? []).map((a) => `+${a.name}`),
    ...(ipsets.data ?? []).map((s) => `+${s.name}`),
  ];

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Groups"
        actions={writable ? <BarButton onClick={() => setAdding(true)}>Add</BarButton> : undefined}
      >
        <InventoryTable>
          {!(groups.data ?? []).length && <InventoryEmpty>No groups</InventoryEmpty>}
          {(groups.data ?? []).map((g) => (
            <InventoryRow
              key={g.group}
              kind={g.group}
              summary={joinSummary([g.comment])}
              onEdit={() => setOpen(g.group)}
            >
              <Button size="sm" variant="outline" onClick={() => setOpen(g.group)}>Rules</Button>
              {writable && <Button size="sm" variant="danger" onClick={() => setDel(g)}>Delete</Button>}
            </InventoryRow>
          ))}
        </InventoryTable>
      </InventoryCard>
      {open && (
        <div className="space-y-3">
          <h2 className="font-medium">Rules in {open}</h2>
          <RuleTable
            rules={rules.data ?? []}
            writable={writable}
            refs={refs.data?.supported ? refs.data.refs : undefined}
            macros={macros.data}
            names={names}
            onCreate={async (body) => {
              await api.createFWGroupRule(open, body);
              void qc.invalidateQueries({ queryKey: ["fw-group-rules", open] });
            }}
            onUpdate={async (pos, body) => {
              await api.updateFWGroupRule(open, pos, body);
              void qc.invalidateQueries({ queryKey: ["fw-group-rules", open] });
            }}
            onDelete={async (pos, digest) => {
              await api.deleteFWGroupRule(open, pos, digest);
              void qc.invalidateQueries({ queryKey: ["fw-group-rules", open] });
            }}
          />
        </div>
      )}
      {adding && (
        <GroupDialog
          onClose={() => setAdding(false)}
          onSave={async (body) => {
            await api.createFWGroup(body);
            setAdding(false);
            void qc.invalidateQueries({ queryKey: ["fw-groups"] });
          }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete group ${del.group}?`}
          body="Removes the security group and its rules."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteFWGroup(del.group);
            if (open === del.group) setOpen(null);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["fw-groups"] });
          }}
        />
      )}
    </div>
  );
}

function GroupDialog({
  onClose,
  onSave,
}: {
  onClose: () => void;
  onSave: (body: Record<string, unknown>) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [comment, setComment] = useState("");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Add group"
      onClose={onClose}
      footer={
        <Button
          disabled={!name}
          onClick={async () => {
            setError("");
            try {
              await onSave({ group: name, comment });
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
      <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function AliasesTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const aliases = useQuery({ queryKey: ["fw-dc-aliases"], queryFn: api.fwDCAliases });
  const [edit, setEdit] = useState<FWAlias | "new" | null>(null);
  const [del, setDel] = useState<FWAlias | null>(null);

  return (
    <div className="space-y-3">
      <InventoryCard
        title="Aliases"
        actions={writable ? <BarButton onClick={() => setEdit("new")}>Add</BarButton> : undefined}
      >
        <InventoryTable>
          {!(aliases.data ?? []).length && <InventoryEmpty>No aliases</InventoryEmpty>}
          {(aliases.data ?? []).map((a) => {
            const summary = joinSummary([a.cidr, a.comment]);
            return (
              <InventoryRow
                key={a.name}
                kind={a.name}
                summary={summary}
                onEdit={writable ? () => setEdit(a) : undefined}
              >
                {writable && (
                  <>
                    <Button size="sm" variant="outline" onClick={() => setEdit(a)}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={() => setDel(a)}>Delete</Button>
                  </>
                )}
              </InventoryRow>
            );
          })}
        </InventoryTable>
      </InventoryCard>
      {edit && (
        <AliasDialog
          alias={edit === "new" ? null : edit}
          onClose={() => setEdit(null)}
          onSave={async (body) => {
            if (edit === "new") await api.createFWDCAlias(body);
            else await api.updateFWDCAlias(edit.name, { ...body, digest: edit.digest });
            setEdit(null);
            void qc.invalidateQueries({ queryKey: ["fw-dc-aliases"] });
          }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete alias ${del.name}?`}
          body="Rules that reference this alias will fail until updated."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteFWDCAlias(del.name, del.digest);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["fw-dc-aliases"] });
          }}
        />
      )}
    </div>
  );
}

function AliasDialog({
  alias,
  onClose,
  onSave,
}: {
  alias: FWAlias | null;
  onClose: () => void;
  onSave: (body: Record<string, unknown>) => Promise<void>;
}) {
  const [name, setName] = useState(alias?.name ?? "");
  const [cidr, setCidr] = useState(alias?.cidr ?? "");
  const [comment, setComment] = useState(alias?.comment ?? "");
  const [error, setError] = useState("");
  return (
    <Modal
      title={alias ? `Edit alias ${alias.name}` : "Add alias"}
      onClose={onClose}
      footer={
        <Button
          disabled={!cidr || (!alias && !name)}
          onClick={async () => {
            setError("");
            try {
              await onSave(alias ? { cidr, comment } : { name, cidr, comment });
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      {!alias && <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>}
      <Field label="CIDR"><Input value={cidr} onChange={(e) => setCidr(e.target.value)} /></Field>
      <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function IPSetsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify");
  const sets = useQuery({ queryKey: ["fw-dc-ipset"], queryFn: api.fwDCIPSets });
  const [adding, setAdding] = useState(false);
  const [open, setOpen] = useState<string | null>(null);
  const [del, setDel] = useState<FWIPSet | null>(null);
  const members = useQuery({
    queryKey: ["fw-dc-ipset-cidrs", open],
    queryFn: () => api.fwDCIPSetCIDRs(open!),
    enabled: !!open,
  });

  return (
    <div className="space-y-4">
      <InventoryCard
        title="IP sets"
        actions={writable ? <BarButton onClick={() => setAdding(true)}>Add</BarButton> : undefined}
      >
        <InventoryTable>
          {!(sets.data ?? []).length && <InventoryEmpty>No IP sets</InventoryEmpty>}
          {(sets.data ?? []).map((s) => (
            <InventoryRow
              key={s.name}
              kind={s.name}
              summary={joinSummary([s.comment])}
              onEdit={() => setOpen(s.name)}
            >
              <Button size="sm" variant="outline" onClick={() => setOpen(s.name)}>Members</Button>
              {writable && <Button size="sm" variant="danger" onClick={() => setDel(s)}>Delete</Button>}
            </InventoryRow>
          ))}
        </InventoryTable>
      </InventoryCard>
      {open && (
        <IPSetMembersDialog
          name={open}
          writable={writable}
          members={members.data ?? []}
          onClose={() => setOpen(null)}
          onAdd={async (cidr) => {
            await api.createFWDCIPSetCIDR(open, { cidr });
            void qc.invalidateQueries({ queryKey: ["fw-dc-ipset-cidrs", open] });
          }}
          onDelete={async (m) => {
            await api.deleteFWDCIPSetCIDR(open, m.cidr, m.digest);
            void qc.invalidateQueries({ queryKey: ["fw-dc-ipset-cidrs", open] });
          }}
        />
      )}
      {adding && (
        <IPSetDialog
          onClose={() => setAdding(false)}
          onSave={async (body) => {
            await api.createFWDCIPSet(body);
            setAdding(false);
            void qc.invalidateQueries({ queryKey: ["fw-dc-ipset"] });
          }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete IP set ${del.name}?`}
          body="Removes the set and its members."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteFWDCIPSet(del.name);
            if (open === del.name) setOpen(null);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["fw-dc-ipset"] });
          }}
        />
      )}
    </div>
  );
}

function IPSetDialog({
  onClose,
  onSave,
}: {
  onClose: () => void;
  onSave: (body: Record<string, unknown>) => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [comment, setComment] = useState("");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Add IP set"
      onClose={onClose}
      footer={
        <Button
          disabled={!name}
          onClick={async () => {
            setError("");
            try {
              await onSave({ name, comment });
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
      <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function IPSetMembersDialog({
  name,
  writable,
  members,
  onClose,
  onAdd,
  onDelete,
}: {
  name: string;
  writable: boolean;
  members: FWIPSetCIDR[];
  onClose: () => void;
  onAdd: (cidr: string) => Promise<void>;
  onDelete: (m: FWIPSetCIDR) => Promise<void>;
}) {
  const [adding, setAdding] = useState(false);
  return (
    <>
      <Modal title={`Members of ${name}`} onClose={onClose} size="lg">
        <InventoryCard
          title="CIDRs"
          actions={writable ? <BarButton onClick={() => setAdding(true)}>Add</BarButton> : undefined}
        >
          <InventoryTable>
            {!members.length && <InventoryEmpty>No CIDRs</InventoryEmpty>}
            {members.map((m) => (
              <InventoryRow key={m.cidr} kind={m.cidr} summary={joinSummary([m.comment])}>
                {writable && <Button size="sm" variant="danger" onClick={() => void onDelete(m)}>Delete</Button>}
              </InventoryRow>
            ))}
          </InventoryTable>
        </InventoryCard>
      </Modal>
      {adding && (
        <CidrDialog
          onClose={() => setAdding(false)}
          onSave={async (cidr) => {
            await onAdd(cidr);
            setAdding(false);
          }}
        />
      )}
    </>
  );
}

function CidrDialog({
  onClose,
  onSave,
}: {
  onClose: () => void;
  onSave: (cidr: string) => Promise<void>;
}) {
  const [cidr, setCidr] = useState("");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Add CIDR"
      onClose={onClose}
      footer={
        <Button
          disabled={!cidr}
          onClick={async () => {
            setError("");
            try {
              await onSave(cidr);
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <Field label="CIDR"><Input value={cidr} onChange={(e) => setCidr(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
