import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Field, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import type { ACLEntry } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

export function ObjectACL({ path }: { path: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/access", "Permissions.Modify") || can("/", "Permissions.Modify");
  const acl = useQuery({ queryKey: ["access-acl"], queryFn: api.accessAcl });
  const roles = useQuery({ queryKey: ["access-roles"], queryFn: api.accessRoles });
  const users = useQuery({ queryKey: ["access-users"], queryFn: api.accessUsers });
  const groups = useQuery({ queryKey: ["access-groups"], queryFn: api.accessGroups });
  const [form, setForm] = useState({ roles: "PVEVMUser", users: "", groups: "", propagate: "1" });
  const rows = (acl.data ?? []).filter((row) => matchesPath(row.path, path));

  async function apply(remove: boolean) {
    await api.updateAccessAcl({
      path,
      roles: form.roles,
      users: form.users,
      groups: form.groups,
      propagate: Number(form.propagate),
      delete: remove,
    });
    void qc.invalidateQueries({ queryKey: ["access-acl"] });
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">ACL for <span className="font-mono text-xs">{path}</span>. Writes stay on the cluster.</p>
      {writable && (
        <Card className="grid gap-3 p-4 md:grid-cols-2">
          <Field label="Role">
            <Select value={form.roles} onChange={(e) => setForm({ ...form, roles: e.target.value })}>
              {(roles.data ?? []).map((r) => <option key={r.roleid}>{r.roleid}</option>)}
            </Select>
          </Field>
          <Field label="User">
            <Select value={form.users} onChange={(e) => setForm({ ...form, users: e.target.value })}>
              <option value="">—</option>
              {(users.data ?? []).map((u) => <option key={u.userid}>{u.userid}</option>)}
            </Select>
          </Field>
          <Field label="Group">
            <Select value={form.groups} onChange={(e) => setForm({ ...form, groups: e.target.value })}>
              <option value="">—</option>
              {(groups.data ?? []).map((g) => <option key={g.group}>{g.group}</option>)}
            </Select>
          </Field>
          <Field label="Propagate">
            <Select value={form.propagate} onChange={(e) => setForm({ ...form, propagate: e.target.value })}>
              <option value="1">Yes</option>
              <option value="0">No</option>
            </Select>
          </Field>
          <div className="flex items-end gap-2">
            <Button size="sm" onClick={() => void apply(false)}>Add</Button>
            <Button size="sm" variant="danger" onClick={() => void apply(true)}>Remove</Button>
          </div>
        </Card>
      )}
      <Card className="overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Path</th><th>Who</th><th>Role</th><th>Propagate</th></tr>
          </thead>
          <tbody>
            {rows.map((row: ACLEntry, i) => (
              <tr key={`${row.path}-${row.ugid}-${row.roleid}-${i}`} className="border-t border-line">
                <td className="px-4 py-2 font-mono text-xs">{row.path}</td>
                <td>{row.type}:{row.ugid}</td>
                <td>{row.roleid}</td>
                <td>{row.propagate ? "yes" : "no"}</td>
              </tr>
            ))}
            {!rows.length && (
              <tr><td className="px-4 py-3 text-sm text-ink-dim" colSpan={4}>No ACL entries on this path.</td></tr>
            )}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

function matchesPath(rowPath: string, objectPath: string) {
  return rowPath === objectPath || rowPath.startsWith(`${objectPath}/`);
}
