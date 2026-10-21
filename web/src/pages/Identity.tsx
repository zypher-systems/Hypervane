import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard } from "@/components/ui/inventory";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import type { AccessUser, ACLEntry, Resource, TFAEnroll } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Tabs } from "@/components/ui/tabs";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

const tabs = ["users", "groups", "roles", "acl", "realms"] as const;

export function IdentityPage() {
  const [tab, setTab] = useState<(typeof tabs)[number]>("users");
  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <div>
        <h2 className="text-base font-semibold">Identity</h2>
        <p className="text-sm text-ink-dim">Proxmox users, groups, roles, ACL, and realms. Tokens and TFA stay on the cluster.</p>
      </div>
      <Tabs items={tabs} value={tab} onChange={setTab} label={(t) => t === "acl" ? "ACL" : t} />
      {tab === "users" && <UsersTab />}
      {tab === "groups" && <GroupsTab />}
      {tab === "roles" && <RolesTab />}
      {tab === "acl" && <ACLTab />}
      {tab === "realms" && <RealmsTab />}
    </div>
  );
}

function UsersTab() {
  const qc = useQueryClient();
  const can = useCan();
  const users = useQuery({ queryKey: ["access-users"], queryFn: api.accessUsers });
  const [open, setOpen] = useState<AccessUser | "new" | null>(null);
  const [del, setDel] = useState<AccessUser | null>(null);
  const writable = can("/access", "User.Modify") || can("/access", "User.Allocate");

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Users"
        actions={writable ? <BarButton onClick={() => setOpen("new")}>Add</BarButton> : undefined}
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
            {!(users.data ?? []).length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No users</td>
              </tr>
            )}
            {(users.data ?? []).map((u) => {
              const name = [u.firstname, u.lastname].filter(Boolean).join(" ");
              const summary = [name, u.groups, u.enable ? "enabled" : "disabled", u.email].filter(Boolean).join(" · ");
              return (
                <tr
                  key={u.userid}
                  className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
                  onClick={() => setOpen(u)}
                >
                  <td className="px-4 py-2 font-medium">{u.userid}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary || "—"}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    <Button size="sm" variant="outline" onClick={() => setOpen(u)}>Edit</Button>
                    {writable && <Button size="sm" variant="danger" onClick={() => setDel(u)}>Delete</Button>}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {open && (
        <UserDrawer
          user={open === "new" ? null : open}
          writable={writable}
          onClose={() => { setOpen(null); void qc.invalidateQueries({ queryKey: ["access-users"] }); }}
        />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete ${del.userid}?`}
          body="Removes the Proxmox user. Hypervane sessions for this account stop working."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteAccessUser(del.userid);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["access-users"] });
          }}
        />
      )}
    </div>
  );
}

function UserDrawer({ user, writable, onClose }: { user: AccessUser | null; writable: boolean; onClose: () => void }) {
  const qc = useQueryClient();
  const [form, setForm] = useState({
    userid: user?.userid ?? "",
    password: "",
    firstname: user?.firstname ?? "",
    lastname: user?.lastname ?? "",
    email: user?.email ?? "",
    comment: user?.comment ?? "",
    groups: user?.groups ?? "",
    enable: user ? String(user.enable ?? 1) : "1",
  });
  const [error, setError] = useState("");
  const [secret, setSecret] = useState("");
  const [enroll, setEnroll] = useState<TFAEnroll | null>(null);
  const [unlock, setUnlock] = useState(false);
  const tokens = useQuery({
    queryKey: ["access-tokens", user?.userid],
    queryFn: () => api.accessTokens(user!.userid),
    enabled: !!user,
  });
  const tfa = useQuery({
    queryKey: ["access-tfa", user?.userid],
    queryFn: () => api.accessTfa(user!.userid),
    enabled: !!user,
  });

  return (
    <Modal
      title={user ? user.userid : "New user"}
      onClose={onClose}
      footer={
        writable ? (
          <Button
            onClick={async () => {
              setError("");
              try {
                const body = {
                  firstname: form.firstname,
                  lastname: form.lastname,
                  email: form.email,
                  comment: form.comment,
                  groups: form.groups,
                  enable: Number(form.enable),
                  ...(form.password ? { password: form.password } : {}),
                };
                if (!user) {
                  await api.createAccessUser({ ...body, userid: form.userid });
                } else {
                  await api.updateAccessUser(user.userid, body);
                  if (form.password) await api.setAccessPassword(user.userid, form.password);
                }
                onClose();
              } catch (e) {
                setError(e instanceof Error ? e.message : "Save failed");
              }
            }}
          >
            Save
          </Button>
        ) : undefined
      }
    >
      <div className="grid max-h-[70vh] gap-4 overflow-auto pr-1">
        {!user && (
          <Field label="Userid (user@realm)">
            <Input value={form.userid} onChange={(e) => setForm({ ...form, userid: e.target.value })} />
          </Field>
        )}
        <div className="grid gap-3 md:grid-cols-2">
          <Field label="First name"><Input value={form.firstname} onChange={(e) => setForm({ ...form, firstname: e.target.value })} /></Field>
          <Field label="Last name"><Input value={form.lastname} onChange={(e) => setForm({ ...form, lastname: e.target.value })} /></Field>
        </div>
        <Field label="Email"><Input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></Field>
        <Field label="Groups (comma-separated)"><Input value={form.groups} onChange={(e) => setForm({ ...form, groups: e.target.value })} /></Field>
        <Field label="Comment"><Input value={form.comment} onChange={(e) => setForm({ ...form, comment: e.target.value })} /></Field>
        <Field label="Enabled">
          <Select value={form.enable} onChange={(e) => setForm({ ...form, enable: e.target.value })}>
            <option value="1">Yes</option>
            <option value="0">No</option>
          </Select>
        </Field>
        <Field label={user ? "New password" : "Password"}>
          <Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
        </Field>
        {user && (
          <>
            <h3 className="text-sm font-medium">API tokens</h3>
            <div className="flex gap-2">
              <Input id="new-token" placeholder="token id" />
              {writable && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={async () => {
                    const el = document.getElementById("new-token") as HTMLInputElement | null;
                    const tokenid = el?.value.trim();
                    if (!tokenid) return;
                    const created = await api.createAccessToken(user.userid, { tokenid, privsep: 1 });
                    setSecret(created.value || "");
                    el!.value = "";
                    void qc.invalidateQueries({ queryKey: ["access-tokens", user.userid] });
                  }}
                >
                  Create
                </Button>
              )}
            </div>
            {secret && (
              <p className="rounded-md border border-amber-900 bg-amber-950/40 p-2 font-mono text-xs text-amber-200">
                Secret (shown once): {secret}
              </p>
            )}
            {(tokens.data ?? []).map((t) => (
              <div key={t.tokenid} className="flex items-center justify-between text-sm">
                <span className="font-mono text-xs">{t.tokenid}</span>
                {writable && (
                  <Button size="sm" variant="danger" onClick={async () => {
                    await api.deleteAccessToken(user.userid, t.tokenid);
                    void qc.invalidateQueries({ queryKey: ["access-tokens", user.userid] });
                  }}>Delete</Button>
                )}
              </div>
            ))}
            <h3 className="text-sm font-medium">TFA</h3>
            {writable && (
              <div className="flex gap-2">
                <Button size="sm" variant="outline" onClick={async () => {
                  setEnroll(await api.enrollAccessTfa(user.userid, { type: "totp", description: "Hypervane" }));
                  void qc.invalidateQueries({ queryKey: ["access-tfa", user.userid] });
                }}>Enroll TOTP</Button>
                <Button size="sm" variant="outline" onClick={async () => {
                  setEnroll(await api.enrollAccessTfa(user.userid, { type: "recovery" }));
                  void qc.invalidateQueries({ queryKey: ["access-tfa", user.userid] });
                }}>Recovery keys</Button>
                <Button size="sm" variant="outline" onClick={() => setUnlock(true)}>Unlock TFA</Button>
              </div>
            )}
            {enroll?.uri && <p className="break-all font-mono text-xs text-ink-dim">{enroll.uri}</p>}
            {enroll?.recovery && (
              <ul className="font-mono text-xs text-amber-200">
                {enroll.recovery.map((k) => <li key={k}>{k}</li>)}
              </ul>
            )}
            {(tfa.data ?? []).map((t) => (
              <div key={t.id} className="flex items-center justify-between text-sm">
                <span>{t.type} · {t.description || t.id}</span>
                {writable && (
                  <Button size="sm" variant="danger" onClick={async () => {
                    await api.deleteAccessTfa(user.userid, t.id);
                    void qc.invalidateQueries({ queryKey: ["access-tfa", user.userid] });
                  }}>Delete</Button>
                )}
              </div>
            ))}
          </>
        )}
        {error && <p className="text-sm text-rose-400">{error}</p>}
      </div>
      {unlock && user && (
        <ConfirmDialog
          title={`Unlock TFA for ${user.userid}?`}
          body="Clears a TFA lockout so this user can sign in again."
          confirmLabel="Unlock"
          onClose={() => setUnlock(false)}
          onConfirm={async () => {
            await api.unlockAccessTfa(user.userid);
            setUnlock(false);
          }}
        />
      )}
    </Modal>
  );
}

function GroupsTab() {
  const qc = useQueryClient();
  const groups = useQuery({ queryKey: ["access-groups"], queryFn: api.accessGroups });
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [comment, setComment] = useState("");
  const [edit, setEdit] = useState<{ group: string; comment: string } | null>(null);

  function closeAdd() {
    setAdding(false);
    setName("");
    setComment("");
  }

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Groups"
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
            {!(groups.data ?? []).length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No groups</td>
              </tr>
            )}
            {(groups.data ?? []).map((g) => {
              const summary = g.comment || "";
              return (
                <tr
                  key={g.group}
                  className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
                  onClick={() => setEdit({ group: g.group, comment: g.comment ?? "" })}
                >
                  <td className="px-4 py-2 font-medium">{g.group}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary || "—"}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    <Button size="sm" variant="outline" onClick={() => setEdit({ group: g.group, comment: g.comment ?? "" })}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={async () => {
                      await api.deleteAccessGroup(g.group);
                      void qc.invalidateQueries({ queryKey: ["access-groups"] });
                    }}>Delete</Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <Modal
          title="Add group"
          onClose={closeAdd}
          footer={
            <Button onClick={async () => {
              await api.createAccessGroup({ group: name, comment });
              closeAdd();
              void qc.invalidateQueries({ queryKey: ["access-groups"] });
            }}>Save</Button>
          }
        >
          <Field label="Group"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="Comment"><Input value={comment} onChange={(e) => setComment(e.target.value)} /></Field>
        </Modal>
      )}
      {edit && (
        <Modal
          title={`Edit group ${edit.group}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              await api.updateAccessGroup(edit.group, { comment: edit.comment });
              setEdit(null);
              void qc.invalidateQueries({ queryKey: ["access-groups"] });
            }}>Save</Button>
          }
        >
          <Field label="Comment"><Input value={edit.comment} onChange={(e) => setEdit({ ...edit, comment: e.target.value })} /></Field>
        </Modal>
      )}
    </div>
  );
}

function RolesTab() {
  const qc = useQueryClient();
  const roles = useQuery({ queryKey: ["access-roles"], queryFn: api.accessRoles });
  const [adding, setAdding] = useState(false);
  const [roleid, setRoleid] = useState("");
  const [addPrivs, setAddPrivs] = useState("");
  const [edit, setEdit] = useState<string | null>(null);
  const [privs, setPrivs] = useState("");

  function closeAdd() {
    setAdding(false);
    setRoleid("");
    setAddPrivs("");
  }

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Roles"
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
            {!(roles.data ?? []).length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No roles</td>
              </tr>
            )}
            {(roles.data ?? []).map((r) => {
              const summary = r.privs || "";
              return (
                <tr
                  key={r.roleid}
                  className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
                  onClick={() => { setEdit(r.roleid); setPrivs(r.privs ?? ""); }}
                >
                  <td className="px-4 py-2 font-medium">{r.roleid}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary || "—"}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    <Button size="sm" variant="outline" onClick={() => { setEdit(r.roleid); setPrivs(r.privs ?? ""); }}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={async () => {
                      await api.deleteAccessRole(r.roleid);
                      void qc.invalidateQueries({ queryKey: ["access-roles"] });
                    }}>Delete</Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <Modal
          title="Add role"
          onClose={closeAdd}
          footer={
            <Button onClick={async () => {
              await api.createAccessRole({ roleid, privs: addPrivs });
              closeAdd();
              void qc.invalidateQueries({ queryKey: ["access-roles"] });
            }}>Save</Button>
          }
        >
          <Field label="Role"><Input value={roleid} onChange={(e) => setRoleid(e.target.value)} /></Field>
          <Field label="Privileges (comma-separated)"><Input value={addPrivs} onChange={(e) => setAddPrivs(e.target.value)} /></Field>
        </Modal>
      )}
      {edit && (
        <Modal
          title={`Edit ${edit}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              await api.updateAccessRole(edit, { privs });
              setEdit(null);
              setPrivs("");
              void qc.invalidateQueries({ queryKey: ["access-roles"] });
            }}>Save</Button>
          }
        >
          <Field label="Privileges"><Input value={privs} onChange={(e) => setPrivs(e.target.value)} /></Field>
        </Modal>
      )}
    </div>
  );
}

function ACLTab() {
  const qc = useQueryClient();
  const acl = useQuery({ queryKey: ["access-acl"], queryFn: api.accessAcl });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const roles = useQuery({ queryKey: ["access-roles"], queryFn: api.accessRoles });
  const users = useQuery({ queryKey: ["access-users"], queryFn: api.accessUsers });
  const groups = useQuery({ queryKey: ["access-groups"], queryFn: api.accessGroups });
  const [form, setForm] = useState({ path: "/", roles: "PVEVMUser", users: "", groups: "", propagate: "1" });
  const paths = resourcePaths(summary.data?.resources ?? []);

  async function apply(remove: boolean) {
    await api.updateAccessAcl({
      path: form.path,
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
      <Card className="grid gap-3 p-4 md:grid-cols-2">
        <Field label="Path">
          <Input list="acl-paths" value={form.path} onChange={(e) => setForm({ ...form, path: e.target.value })} />
          <datalist id="acl-paths">
            {paths.map((p) => <option key={p} value={p} />)}
          </datalist>
        </Field>
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
      <Card className="overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Path</th><th>Who</th><th>Role</th><th>Propagate</th></tr>
          </thead>
          <tbody>
            {(acl.data ?? []).map((row: ACLEntry, i) => (
              <tr key={`${row.path}-${row.ugid}-${row.roleid}-${i}`} className="border-t border-line">
                <td className="px-4 py-2 font-mono text-xs">{row.path}</td>
                <td>{row.type}:{row.ugid}</td>
                <td>{row.roleid}</td>
                <td>{row.propagate ? "yes" : "no"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

function resourcePaths(resources: Resource[]) {
  const paths = new Set(["/", "/vms", "/nodes", "/storage", "/access"]);
  for (const r of resources) {
    if ((r.type === "qemu" || r.type === "lxc") && r.vmid) paths.add(`/vms/${r.vmid}`);
    if (r.type === "node" && r.node) paths.add(`/nodes/${r.node}`);
    if (r.type === "storage" && r.storage) paths.add(`/storage/${r.storage}`);
    if (r.type === "pool" && (r.pool || r.id)) paths.add(`/pool/${r.pool || String(r.id).replace(/^pool\//, "")}`);
  }
  return [...paths];
}

const emptyRealm = {
  realm: "", type: "pam", comment: "", server1: "", base_dn: "", user_attr: "uid", bind_dn: "",
  bind_password: "", port: "", secure: false, default: false,
  issuer: "", client_id: "", client_key: "", username_claim: "email", scopes: "openid profile email", autocreate: false,
};

function realmBody(form: typeof emptyRealm, includeRealm = true): Record<string, unknown> {
  const body: Record<string, unknown> = { type: form.type, comment: form.comment };
  if (includeRealm) body.realm = form.realm;
  if (form.default) body.default = 1;
  if (form.type === "ldap" || form.type === "ad") {
    body.server1 = form.server1;
    body.base_dn = form.base_dn;
    body.user_attr = form.user_attr;
    body.bind_dn = form.bind_dn;
    if (form.bind_password) body.password = form.bind_password;
    if (form.port) body.port = form.port;
    if (form.secure) body.secure = 1;
  }
  if (form.type === "openid") {
    if (form.issuer) body["issuer-url"] = form.issuer;
    if (form.client_id) body["client-id"] = form.client_id;
    if (form.client_key) body["client-key"] = form.client_key;
    if (form.username_claim) body["username-claim"] = form.username_claim;
    if (form.scopes) body.scopes = form.scopes;
    if (form.autocreate) body.autocreate = 1;
  }
  return body;
}

function RealmFields({
  form, setForm, openidOn, editing,
}: {
  form: typeof emptyRealm;
  setForm: (next: typeof emptyRealm) => void;
  openidOn: boolean;
  editing?: boolean;
}) {
  const set = (key: keyof typeof emptyRealm, value: string | boolean) => setForm({ ...form, [key]: value });
  return (
    <>
      <Field label="Realm"><Input value={form.realm} onChange={(e) => set("realm", e.target.value)} disabled={editing} /></Field>
      <Field label="Type">
        <Select value={form.type} onChange={(e) => set("type", e.target.value)} disabled={editing}>
          <option value="pam">PAM</option>
          <option value="ldap">LDAP</option>
          <option value="ad">Active Directory</option>
          <option value="pve">PVE</option>
          {openidOn && <option value="openid">OpenID</option>}
        </Select>
      </Field>
      <Field label="Comment"><Input value={form.comment} onChange={(e) => set("comment", e.target.value)} /></Field>
      <label className="flex items-center gap-2 text-sm text-ink-dim">
        <input type="checkbox" checked={form.default} onChange={(e) => set("default", e.target.checked)} />
        Default realm
      </label>
      {(form.type === "ldap" || form.type === "ad") && (
        <>
          <Field label="Server"><Input value={form.server1} onChange={(e) => set("server1", e.target.value)} /></Field>
          <Field label="Port"><Input value={form.port} onChange={(e) => set("port", e.target.value)} placeholder="389" /></Field>
          <Field label="Base DN"><Input value={form.base_dn} onChange={(e) => set("base_dn", e.target.value)} /></Field>
          <Field label="User attribute"><Input value={form.user_attr} onChange={(e) => set("user_attr", e.target.value)} /></Field>
          <Field label="Bind DN"><Input value={form.bind_dn} onChange={(e) => set("bind_dn", e.target.value)} /></Field>
          <Field label="Bind password"><Input type="password" value={form.bind_password} onChange={(e) => set("bind_password", e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={form.secure} onChange={(e) => set("secure", e.target.checked)} />
            Secure (LDAPS)
          </label>
        </>
      )}
      {form.type === "openid" && openidOn && (
        <>
          <Field label="Issuer"><Input value={form.issuer} onChange={(e) => set("issuer", e.target.value)} /></Field>
          <Field label="Client ID"><Input value={form.client_id} onChange={(e) => set("client_id", e.target.value)} /></Field>
          <Field label="Client key"><Input type="password" value={form.client_key} onChange={(e) => set("client_key", e.target.value)} /></Field>
          <Field label="Username claim"><Input value={form.username_claim} onChange={(e) => set("username_claim", e.target.value)} /></Field>
          <Field label="Scopes"><Input value={form.scopes} onChange={(e) => set("scopes", e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={form.autocreate} onChange={(e) => set("autocreate", e.target.checked)} />
            Autocreate users
          </label>
        </>
      )}
    </>
  );
}

function RealmsTab() {
  const qc = useQueryClient();
  const realms = useQuery({ queryKey: ["access-realms"], queryFn: api.accessRealms });
  const openid = useQuery({ queryKey: ["access-openid"], queryFn: api.openid });
  const openidOn = openid.data?.supported !== false || openid.isLoading;
  const [adding, setAdding] = useState(false);
  const [form, setForm] = useState(emptyRealm);
  const [edit, setEdit] = useState<typeof emptyRealm | null>(null);
  const [syncMissing, setSyncMissing] = useState(false);

  function closeAdd() {
    setAdding(false);
    setForm(emptyRealm);
  }

  async function openEdit(r: { realm: string; type: string; comment?: string }) {
    const cfg = await api.accessRealm(r.realm);
    setEdit({
      ...emptyRealm,
      realm: r.realm,
      type: String(cfg.type ?? r.type),
      comment: String(cfg.comment ?? r.comment ?? ""),
      server1: String(cfg.server1 ?? ""),
      base_dn: String(cfg.base_dn ?? ""),
      user_attr: String(cfg.user_attr ?? "uid"),
      bind_dn: String(cfg.bind_dn ?? ""),
      port: String(cfg.port ?? ""),
      secure: cfg.secure === 1 || cfg.secure === true || cfg.secure === "1",
      default: cfg.default === 1 || cfg.default === true,
      issuer: String(cfg["issuer-url"] ?? ""),
      client_id: String(cfg["client-id"] ?? ""),
      username_claim: String(cfg["username-claim"] ?? "email"),
      scopes: String(cfg.scopes ?? "openid profile email"),
      autocreate: cfg.autocreate === 1 || cfg.autocreate === true,
    });
  }

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Realms"
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
            {!(realms.data ?? []).length && (
              <tr className="border-t border-line">
                <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>No realms</td>
              </tr>
            )}
            {(realms.data ?? []).map((r) => {
              const summary = [r.type, r.comment].filter(Boolean).join(" · ");
              return (
                <tr
                  key={r.realm}
                  className="cursor-pointer border-t border-line hover:bg-white/[0.04]"
                  onClick={() => void openEdit(r)}
                >
                  <td className="px-4 py-2 font-medium">{r.realm}</td>
                  <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary || "—"}</td>
                  <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                    <Button size="sm" variant="outline" onClick={() => void openEdit(r)}>Edit</Button>
                    {!syncMissing && r.type !== "pam" && r.type !== "pve" && (
                      <Button size="sm" variant="outline" onClick={async () => {
                        try {
                          const res = await api.syncAccessRealm(r.realm);
                          attachTask(res.upid, [["access-users"], ["access-groups"], ["access-realms"]]);
                        } catch (e) {
                          if (e instanceof Error && /not supported|404/i.test(e.message)) setSyncMissing(true);
                        }
                      }}>Sync</Button>
                    )}
                    <Button size="sm" variant="danger" onClick={async () => {
                      await api.deleteAccessRealm(r.realm);
                      void qc.invalidateQueries({ queryKey: ["access-realms"] });
                    }}>Delete</Button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <Modal
          title="Add realm"
          onClose={closeAdd}
          footer={
            <Button onClick={async () => {
              await api.createAccessRealm(realmBody(form));
              closeAdd();
              void qc.invalidateQueries({ queryKey: ["access-realms"] });
            }}>Save</Button>
          }
        >
          <div className="grid gap-3 md:grid-cols-2">
            <RealmFields form={form} setForm={setForm} openidOn={openidOn} />
          </div>
        </Modal>
      )}
      {edit && (
        <Modal
          title={`Edit realm ${edit.realm}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              await api.updateAccessRealm(edit.realm, realmBody(edit, false));
              setEdit(null);
              void qc.invalidateQueries({ queryKey: ["access-realms"] });
            }}>Save</Button>
          }
        >
          <div className="grid gap-3 md:grid-cols-2">
            <RealmFields form={edit} setForm={setEdit} openidOn={openidOn} editing />
          </div>
        </Modal>
      )}
    </div>
  );
}
