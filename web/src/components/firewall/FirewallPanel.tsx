import { LogPanel, OptionsBar, RuleTable } from "@/components/firewall/shared";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard, InventoryEmpty, InventoryRow, InventoryTable, joinSummary } from "@/components/ui/inventory";
import { Field, Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import type { FWAlias, FWIPSet, FWIPSetCIDR } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

export function FirewallPanel({ vmid, node, type }: { vmid: number; node: string; type: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const path = `/vms/${vmid}`;
  const writable = can(path, "VM.Config.Network");
  const opts = useQuery({
    queryKey: ["fw-guest-opts", vmid, node, type],
    queryFn: () => api.fwGuestOptions(vmid, node, type),
  });
  const rules = useQuery({
    queryKey: ["fw-guest-rules", vmid, node, type],
    queryFn: () => api.fwGuestRules(vmid, node, type),
  });
  const log = useQuery({
    queryKey: ["fw-guest-log", vmid, node, type],
    queryFn: () => api.fwGuestLog(vmid, node, type),
    refetchInterval: 15000,
  });
  const refs = useQuery({
    queryKey: ["fw-guest-refs", vmid, node, type],
    queryFn: () => api.fwGuestRefs(vmid, node, type),
  });
  const macros = useQuery({ queryKey: ["fw-macros"], queryFn: api.fwMacros });
  const groups = useQuery({ queryKey: ["fw-groups"], queryFn: api.fwGroups });
  const aliases = useQuery({
    queryKey: ["fw-guest-aliases", vmid, node, type],
    queryFn: () => api.fwGuestAliases(vmid, node, type),
  });
  const ipsets = useQuery({
    queryKey: ["fw-guest-ipset", vmid, node, type],
    queryFn: () => api.fwGuestIPSets(vmid, node, type),
  });
  const [editAlias, setEditAlias] = useState<FWAlias | "new" | null>(null);
  const [addSet, setAddSet] = useState(false);
  const [openSet, setOpenSet] = useState<string | null>(null);
  const [delAlias, setDelAlias] = useState<FWAlias | null>(null);
  const [delSet, setDelSet] = useState<FWIPSet | null>(null);
  const members = useQuery({
    queryKey: ["fw-guest-ipset-cidrs", vmid, openSet],
    queryFn: () => api.fwGuestIPSetCIDRs(vmid, openSet!, node, type),
    enabled: !!openSet,
  });

  return (
    <div className="space-y-4">
      {opts.data && (
        <OptionsBar
          key={opts.data.digest || "guest"}
          opts={opts.data}
          writable={writable}
          guest
          onSave={async (body) => {
            await api.updateFWGuestOptions(vmid, body, node, type);
            void qc.invalidateQueries({ queryKey: ["fw-guest-opts", vmid] });
          }}
        />
      )}
      <RuleTable
        rules={rules.data ?? []}
        writable={writable}
        refs={refs.data?.supported ? refs.data.refs : undefined}
        macros={macros.data}
        groups={(groups.data ?? []).map((g) => g.group)}
        names={[
          ...(aliases.data ?? []).map((a) => `+${a.name}`),
          ...(ipsets.data ?? []).map((s) => `+${s.name}`),
        ]}
        onCreate={async (body) => {
          await api.createFWGuestRule(vmid, body, node, type);
          void qc.invalidateQueries({ queryKey: ["fw-guest-rules", vmid] });
        }}
        onUpdate={async (pos, body) => {
          await api.updateFWGuestRule(vmid, pos, body, node, type);
          void qc.invalidateQueries({ queryKey: ["fw-guest-rules", vmid] });
        }}
        onDelete={async (pos, digest) => {
          await api.deleteFWGuestRule(vmid, pos, digest, node, type);
          void qc.invalidateQueries({ queryKey: ["fw-guest-rules", vmid] });
        }}
      />
      <InventoryCard
        title="Aliases"
        actions={writable ? <BarButton onClick={() => setEditAlias("new")}>Add</BarButton> : undefined}
      >
        <InventoryTable>
          {!(aliases.data ?? []).length && <InventoryEmpty>No aliases</InventoryEmpty>}
          {(aliases.data ?? []).map((a) => (
            <InventoryRow
              key={a.name}
              kind={a.name}
              summary={joinSummary([a.cidr, a.comment])}
              onEdit={writable ? () => setEditAlias(a) : undefined}
            >
              {writable && (
                <>
                  <Button size="sm" variant="outline" onClick={() => setEditAlias(a)}>Edit</Button>
                  <Button size="sm" variant="danger" onClick={() => setDelAlias(a)}>Delete</Button>
                </>
              )}
            </InventoryRow>
          ))}
        </InventoryTable>
      </InventoryCard>
      <InventoryCard
        title="IP sets"
        actions={writable ? <BarButton onClick={() => setAddSet(true)}>Add</BarButton> : undefined}
      >
        <InventoryTable>
          {!(ipsets.data ?? []).length && <InventoryEmpty>No IP sets</InventoryEmpty>}
          {(ipsets.data ?? []).map((s) => (
            <InventoryRow
              key={s.name}
              kind={s.name}
              summary={joinSummary([s.comment])}
              onEdit={() => setOpenSet(s.name)}
            >
              <Button size="sm" variant="outline" onClick={() => setOpenSet(s.name)}>Members</Button>
              {writable && <Button size="sm" variant="danger" onClick={() => setDelSet(s)}>Delete</Button>}
            </InventoryRow>
          ))}
        </InventoryTable>
      </InventoryCard>
      <LogPanel lines={log.data ?? []} />
      {editAlias && (
        <AliasDialog
          alias={editAlias === "new" ? null : editAlias}
          onClose={() => setEditAlias(null)}
          onSave={async (body) => {
            if (editAlias === "new") await api.createFWGuestAlias(vmid, body, node, type);
            else await api.updateFWGuestAlias(vmid, editAlias.name, { ...body, digest: editAlias.digest }, node, type);
            setEditAlias(null);
            void qc.invalidateQueries({ queryKey: ["fw-guest-aliases", vmid] });
          }}
        />
      )}
      {addSet && (
        <IPSetDialog
          onClose={() => setAddSet(false)}
          onSave={async (body) => {
            await api.createFWGuestIPSet(vmid, body, node, type);
            setAddSet(false);
            void qc.invalidateQueries({ queryKey: ["fw-guest-ipset", vmid] });
          }}
        />
      )}
      {openSet && (
        <IPSetMembersDialog
          name={openSet}
          writable={writable}
          members={members.data ?? []}
          onClose={() => setOpenSet(null)}
          onAdd={async (cidr) => {
            await api.createFWGuestIPSetCIDR(vmid, openSet, { cidr }, node, type);
            void qc.invalidateQueries({ queryKey: ["fw-guest-ipset-cidrs", vmid, openSet] });
          }}
          onDelete={async (m) => {
            await api.deleteFWGuestIPSetCIDR(vmid, openSet, m.cidr, node, type);
            void qc.invalidateQueries({ queryKey: ["fw-guest-ipset-cidrs", vmid, openSet] });
          }}
        />
      )}
      {delAlias && (
        <ConfirmDialog
          title={`Delete alias ${delAlias.name}?`}
          body="Removes this guest alias."
          danger
          confirmLabel="Delete"
          onClose={() => setDelAlias(null)}
          onConfirm={async () => {
            await api.deleteFWGuestAlias(vmid, delAlias.name, node, type);
            setDelAlias(null);
            void qc.invalidateQueries({ queryKey: ["fw-guest-aliases", vmid] });
          }}
        />
      )}
      {delSet && (
        <ConfirmDialog
          title={`Delete IP set ${delSet.name}?`}
          body="Removes this guest IP set."
          danger
          confirmLabel="Delete"
          onClose={() => setDelSet(null)}
          onConfirm={async () => {
            await api.deleteFWGuestIPSet(vmid, delSet.name, node, type);
            if (openSet === delSet.name) setOpenSet(null);
            setDelSet(null);
            void qc.invalidateQueries({ queryKey: ["fw-guest-ipset", vmid] });
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

function IPSetDialog({
  onClose,
  onSave,
}: {
  onClose: () => void;
  onSave: (body: Record<string, unknown>) => Promise<void>;
}) {
  const [name, setName] = useState("");
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
              await onSave({ name });
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
  const [cidr, setCidr] = useState("");
  const [error, setError] = useState("");
  return (
    <>
      <Modal title={`Members of ${name}`} onClose={onClose} size="lg">
        <InventoryCard
          title="CIDRs"
          actions={writable ? <BarButton onClick={() => { setCidr(""); setError(""); setAdding(true); }}>Add</BarButton> : undefined}
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
        <Modal
          title="Add CIDR"
          onClose={() => setAdding(false)}
          footer={
            <Button
              disabled={!cidr}
              onClick={async () => {
                setError("");
                try {
                  await onAdd(cidr);
                  setAdding(false);
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
      )}
    </>
  );
}
