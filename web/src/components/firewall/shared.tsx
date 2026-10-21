import { Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard } from "@/components/ui/inventory";
import { Field, Input, Select } from "@/components/ui/input";
import type { FWLogLine, FWMacro, FWOptions, FWRef, FWRule } from "@/types";
import { useState } from "react";

export function OptionsBar({
  opts,
  writable,
  guest,
  onSave,
}: {
  opts?: FWOptions;
  writable: boolean;
  guest?: boolean;
  onSave: (body: Record<string, unknown>) => Promise<void>;
}) {
  const [enable, setEnable] = useState(String(opts?.enable ?? 0));
  const [policyIn, setPolicyIn] = useState(opts?.policy_in || "DROP");
  const [policyOut, setPolicyOut] = useState(opts?.policy_out || "ACCEPT");
  const [logIn, setLogIn] = useState(opts?.log_level_in || "nolog");
  const [logOut, setLogOut] = useState(opts?.log_level_out || "nolog");
  const [ebtables, setEbtables] = useState(String(opts?.ebtables ?? 1));
  const [dhcp, setDhcp] = useState(String(opts?.dhcp ?? 0));
  const [ndp, setNdp] = useState(String(opts?.ndp ?? 0));
  const [radv, setRadv] = useState(String(opts?.radv ?? 0));
  const [macfilter, setMacfilter] = useState(String(opts?.macfilter ?? 0));
  const [ipfilter, setIpfilter] = useState(String(opts?.ipfilter ?? 0));
  const [error, setError] = useState("");

  return (
    <Card className="grid gap-3 p-4 md:grid-cols-4">
      <Field label="Enabled">
        <Select value={enable} onChange={(e) => setEnable(e.target.value)} disabled={!writable}>
          <option value="1">Yes</option>
          <option value="0">No</option>
        </Select>
      </Field>
      <Field label="Input policy">
        <Select value={policyIn} onChange={(e) => setPolicyIn(e.target.value)} disabled={!writable}>
          <option>ACCEPT</option>
          <option>DROP</option>
          <option>REJECT</option>
        </Select>
      </Field>
      <Field label="Output policy">
        <Select value={policyOut} onChange={(e) => setPolicyOut(e.target.value)} disabled={!writable}>
          <option>ACCEPT</option>
          <option>DROP</option>
          <option>REJECT</option>
        </Select>
      </Field>
      <Field label="Log in">
        <Select value={logIn} onChange={(e) => setLogIn(e.target.value)} disabled={!writable}>
          <option value="nolog">nolog</option>
          <option value="info">info</option>
          <option value="notice">notice</option>
          <option value="warning">warning</option>
          <option value="err">err</option>
          <option value="crit">crit</option>
          <option value="alert">alert</option>
          <option value="emerg">emerg</option>
        </Select>
      </Field>
      <Field label="Log out">
        <Select value={logOut} onChange={(e) => setLogOut(e.target.value)} disabled={!writable}>
          <option value="nolog">nolog</option>
          <option value="info">info</option>
          <option value="notice">notice</option>
          <option value="warning">warning</option>
          <option value="err">err</option>
          <option value="crit">crit</option>
          <option value="alert">alert</option>
          <option value="emerg">emerg</option>
        </Select>
      </Field>
      {!guest && (
        <Field label="ebtables">
          <Select value={ebtables} onChange={(e) => setEbtables(e.target.value)} disabled={!writable}>
            <option value="1">Yes</option>
            <option value="0">No</option>
          </Select>
        </Field>
      )}
      <Field label="NDP">
        <Select value={ndp} onChange={(e) => setNdp(e.target.value)} disabled={!writable}>
          <option value="1">Yes</option>
          <option value="0">No</option>
        </Select>
      </Field>
      {guest && (
        <>
          <Field label="DHCP">
            <Select value={dhcp} onChange={(e) => setDhcp(e.target.value)} disabled={!writable}>
              <option value="1">Yes</option>
              <option value="0">No</option>
            </Select>
          </Field>
          <Field label="RADV">
            <Select value={radv} onChange={(e) => setRadv(e.target.value)} disabled={!writable}>
              <option value="1">Yes</option>
              <option value="0">No</option>
            </Select>
          </Field>
          <Field label="MAC filter">
            <Select value={macfilter} onChange={(e) => setMacfilter(e.target.value)} disabled={!writable}>
              <option value="1">Yes</option>
              <option value="0">No</option>
            </Select>
          </Field>
          <Field label="IP filter">
            <Select value={ipfilter} onChange={(e) => setIpfilter(e.target.value)} disabled={!writable}>
              <option value="1">Yes</option>
              <option value="0">No</option>
            </Select>
          </Field>
        </>
      )}
      {writable && (
        <div className="flex items-end">
          <Button
            size="sm"
            onClick={async () => {
              setError("");
              try {
                await onSave({
                  enable: Number(enable),
                  policy_in: policyIn,
                  policy_out: policyOut,
                  log_level_in: logIn,
                  log_level_out: logOut,
                  ndp: Number(ndp),
                  digest: opts?.digest,
                  ...(guest
                    ? { dhcp: Number(dhcp), radv: Number(radv), macfilter: Number(macfilter), ipfilter: Number(ipfilter) }
                    : { ebtables: Number(ebtables) }),
                });
              } catch (e) {
                setError(e instanceof Error ? e.message : "Save failed");
              }
            }}
          >
            Save options
          </Button>
        </div>
      )}
      {error && <p className="text-sm text-rose-400 md:col-span-4">{error}</p>}
    </Card>
  );
}

export function RuleTable({
  rules,
  writable,
  refs,
  macros,
  groups,
  names,
  onCreate,
  onUpdate,
  onDelete,
}: {
  rules: FWRule[];
  writable: boolean;
  refs?: FWRef[];
  macros?: FWMacro[];
  groups?: string[];
  names?: string[];
  onCreate: (body: Record<string, unknown>) => Promise<void>;
  onUpdate: (pos: number, body: Record<string, unknown>) => Promise<void>;
  onDelete: (pos: number, digest?: string) => Promise<void>;
}) {
  const [edit, setEdit] = useState<FWRule | "new" | null>(null);
  const [del, setDel] = useState<FWRule | null>(null);

  return (
    <>
      <InventoryCard
        title="Rules"
        actions={writable ? <BarButton onClick={() => setEdit("new")}>Add</BarButton> : undefined}
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
            {!rules.length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No rules</td>
              </tr>
            )}
            {rules.map((rule) => {
              const match = rule.macro || [rule.proto, rule.dport].filter(Boolean).join("/") || "";
              const summary = [rule.enable ? "" : "off", rule.source && `src ${rule.source}`, rule.dest && `dst ${rule.dest}`, match, rule.comment].filter(Boolean).join(" · ") || "—";
              return (
                <tr
                  key={`${rule.pos}-${rule.digest}`}
                  className={`border-t border-line ${writable ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
                  onClick={writable ? () => setEdit(rule) : undefined}
                >
                  <td className="px-4 py-2 font-medium">{rule.type} {rule.action}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    {writable && <Button size="sm" variant="outline" onClick={() => setEdit(rule)}>Edit</Button>}
                    {writable && <Button size="sm" variant="danger" onClick={() => setDel(rule)}>Delete</Button>}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {edit && (
        <RuleDialog
          rule={edit === "new" ? null : edit}
          refs={refs}
          macros={macros}
          groups={groups}
          names={names}
          onClose={() => setEdit(null)}
          onSave={async (body) => {
            if (edit === "new") await onCreate(body);
            else await onUpdate(edit.pos, { ...body, digest: edit.digest });
            setEdit(null);
          }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete rule ${del.pos}?`}
          body="Removes this firewall rule."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await onDelete(del.pos, del.digest);
            setDel(null);
          }}
        />
      )}
    </>
  );
}

function RuleDialog({
  rule,
  refs,
  macros,
  groups,
  names,
  onClose,
  onSave,
}: {
  rule: FWRule | null;
  refs?: FWRef[];
  macros?: FWMacro[];
  groups?: string[];
  names?: string[];
  onClose: () => void;
  onSave: (body: Record<string, unknown>) => Promise<void>;
}) {
  const [form, setForm] = useState({
    type: rule?.type || "in",
    action: rule?.action || "ACCEPT",
    enable: String(rule?.enable ?? 1),
    source: rule?.source || "",
    dest: rule?.dest || "",
    proto: rule?.proto || "",
    dport: rule?.dport || "",
    sport: rule?.sport || "",
    macro: rule?.macro || "",
    iface: rule?.iface || "",
    comment: rule?.comment || "",
    log: rule?.log || "",
  });
  const [error, setError] = useState("");
  const suggestions = [
    ...new Set([
      ...(refs ?? []).map((r) => r.ref || r.name).filter(Boolean) as string[],
      ...(names ?? []),
    ]),
  ];

  return (
    <Modal
      title={rule ? `Edit rule ${rule.pos}` : "Add rule"}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              await onSave({
                type: form.type,
                action: form.action,
                enable: Number(form.enable),
                source: form.source,
                dest: form.dest,
                proto: form.proto,
                dport: form.dport,
                sport: form.sport,
                macro: form.macro,
                iface: form.iface,
                comment: form.comment,
                log: form.log,
              });
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <div className="grid gap-3 md:grid-cols-2">
        <Field label="Direction">
          <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
            <option value="in">in</option>
            <option value="out">out</option>
            <option value="group">group</option>
          </Select>
        </Field>
        <Field label={form.type === "group" ? "Security group" : "Action"}>
          {form.type === "group" && groups?.length ? (
            <Select value={form.action} onChange={(e) => setForm({ ...form, action: e.target.value })}>
              {groups.map((g) => <option key={g}>{g}</option>)}
            </Select>
          ) : (
            <Select value={form.action} onChange={(e) => setForm({ ...form, action: e.target.value })}>
              <option>ACCEPT</option>
              <option>DROP</option>
              <option>REJECT</option>
            </Select>
          )}
        </Field>
        <Field label="Enabled">
          <Select value={form.enable} onChange={(e) => setForm({ ...form, enable: e.target.value })}>
            <option value="1">Yes</option>
            <option value="0">No</option>
          </Select>
        </Field>
        <Field label="Macro">
          <Input list="fw-macros" value={form.macro} onChange={(e) => setForm({ ...form, macro: e.target.value })} />
          <datalist id="fw-macros">
            {(macros ?? []).map((m) => <option key={m.macro} value={m.macro}>{m.descr}</option>)}
          </datalist>
        </Field>
        <Field label="Source">
          <Input list="fw-refs" value={form.source} onChange={(e) => setForm({ ...form, source: e.target.value })} />
        </Field>
        <Field label="Dest">
          <Input list="fw-refs" value={form.dest} onChange={(e) => setForm({ ...form, dest: e.target.value })} />
          <datalist id="fw-refs">
            {suggestions.map((s) => <option key={s} value={s} />)}
          </datalist>
        </Field>
        <Field label="Proto"><Input value={form.proto} onChange={(e) => setForm({ ...form, proto: e.target.value })} /></Field>
        <Field label="Dest port"><Input value={form.dport} onChange={(e) => setForm({ ...form, dport: e.target.value })} /></Field>
        <Field label="Source port"><Input value={form.sport} onChange={(e) => setForm({ ...form, sport: e.target.value })} /></Field>
        <Field label="Interface"><Input value={form.iface} onChange={(e) => setForm({ ...form, iface: e.target.value })} /></Field>
        <Field label="Log">
          <Select value={form.log} onChange={(e) => setForm({ ...form, log: e.target.value })}>
            <option value="">nolog</option>
            <option value="info">info</option>
            <option value="notice">notice</option>
            <option value="warning">warning</option>
            <option value="err">err</option>
            <option value="crit">crit</option>
            <option value="alert">alert</option>
            <option value="emerg">emerg</option>
          </Select>
        </Field>
        <Field label="Comment"><Input value={form.comment} onChange={(e) => setForm({ ...form, comment: e.target.value })} /></Field>
      </div>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function LogPanel({ lines }: { lines: FWLogLine[] }) {
  return (
    <Card className="overflow-hidden">
      <h2 className="px-4 pt-4 font-medium">Log</h2>
      <pre className="max-h-72 overflow-auto p-4 font-mono text-xs text-ink-dim">
        {(lines ?? []).map((l) => `${l.n}  ${l.t}`).join("\n") || "No log lines."}
      </pre>
    </Card>
  );
}
