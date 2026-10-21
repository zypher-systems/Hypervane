import { LogPanel } from "@/components/firewall/shared";
import { attachTask } from "@/components/layout/AppShell";
import { BarButton } from "@/components/ui/action-bar";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { InventoryCard, InventoryEmpty, InventoryRow, joinSummary } from "@/components/ui/inventory";
import { Advanced, CollapsedRaw, asRecord, asRecords } from "@/components/ui/item-details";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import type { CephCmdSafety, CephOSD, CephPool, Resource } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useSearch } from "@tanstack/react-router";
import { useState } from "react";
import { Tabs } from "@/components/ui/tabs";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

const tabs = ["status", "flags", "monitors", "managers", "osds", "pools", "crush", "cephfs", "rgw", "config"] as const;

export function CephPage() {
  const search = useSearch({ from: "/app/ceph" });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const nodes = (summary.data?.resources ?? []).filter((r: Resource) => r.type === "node");
  const [node, setNode] = useState(typeof search.node === "string" ? search.node : "");
  const current = node || nodes[0]?.node || "";
  const status = useQuery({ queryKey: ["ceph-status", current], queryFn: () => api.cephStatus(current), enabled: !!current });
  const fs = useQuery({
    queryKey: ["ceph-fs", current],
    queryFn: () => api.cephFS(current),
    enabled: !!current && status.data?.supported === true,
  });
  const rgw = useQuery({
    queryKey: ["ceph-rgw", current],
    queryFn: () => api.cephRGW(current),
    enabled: !!current && status.data?.supported === true,
  });
  const [tab, setTab] = useState<(typeof tabs)[number]>("status");
  const hidden = new Set<string>();
  if (fs.data?.supported === false) hidden.add("cephfs");
  if (rgw.data?.supported === false) hidden.add("rgw");
  const visible = tabs.filter((t) => !hidden.has(t));
  const currentTab = hidden.has(tab) ? "status" : tab;

  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold">Ceph</h2>
          <p className="text-sm text-ink-dim">Cluster health, OSDs, pools, and flags. Install and OSD create live on the node Ceph tab.</p>
        </div>
        <Field label="Node">
          <Select className="w-48" value={current} onChange={(e) => setNode(e.target.value)}>
            {nodes.map((n) => (
              <option key={n.node} value={n.node}>
                {n.node}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      {!status.isLoading && !status.data?.supported && (
        <Card className="space-y-2 p-4">
          <p className="text-sm text-ink-dim">Ceph is not installed or not initialized on this cluster.</p>
          {current && (
            <Link className="text-sm text-ink underline decoration-white/30 underline-offset-2 hover:decoration-white" to="/nodes/$node" params={{ node: current }} search={{ tab: "ceph" }}>
              Open {current} Ceph tab to install
            </Link>
          )}
        </Card>
      )}
      {status.data?.supported && (
        <>
          <Tabs items={visible} value={currentTab} onChange={setTab} label={(t) => t === "osds" ? "OSDs" : t === "cephfs" ? "CephFS" : t === "crush" ? "CRUSH" : t === "rgw" ? "RGW" : t} />
          {currentTab === "status" && <StatusTab node={current} />}
          {currentTab === "flags" && <FlagsTab node={current} />}
          {currentTab === "monitors" && (
            <ServiceTab
              title="Monitors"
              node={current}
              listKey="ceph-mons"
              list={() => api.cephMons(current)}
              create={(body) => api.createCephMon(body, current)}
              remove={(id) => api.deleteCephMon(id, current)}
              safety="mon"
            />
          )}
          {currentTab === "managers" && (
            <ServiceTab
              title="Managers"
              node={current}
              listKey="ceph-mgrs"
              list={() => api.cephMgrs(current)}
              create={(body) => api.createCephMgr(body, current)}
              remove={(id) => api.deleteCephMgr(id, current)}
            />
          )}
          {currentTab === "osds" && <OSDsTab node={current} />}
          {currentTab === "pools" && <PoolsTab node={current} />}
          {currentTab === "crush" && <CrushTab node={current} />}
          {currentTab === "cephfs" && <CephFSTab node={current} />}
          {currentTab === "rgw" && <RGWTab node={current} />}
          {currentTab === "config" && <ConfigTab node={current} />}
        </>
      )}
    </div>
  );
}

function StatusTab({ node }: { node: string }) {
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const status = useQuery({ queryKey: ["ceph-status", node], queryFn: () => api.cephStatus(node) });
  const metadata = useQuery({ queryKey: ["ceph-metadata", node], queryFn: () => api.cephMetadata(node) });
  const health = status.data?.data?.health as { status?: string; checks?: Record<string, unknown> } | undefined;
  const pgmap = status.data?.data?.pgmap as Record<string, unknown> | undefined;
  const [service, setService] = useState("");
  const [stop, setStop] = useState(false);
  const [bulkType, setBulkType] = useState<"mon" | "mgr" | "mds" | "osd">("osd");
  const [bulkDry, setBulkDry] = useState(true);
  const [bulkForce, setBulkForce] = useState(false);
  const [bulkOutdated, setBulkOutdated] = useState(false);
  const [bulkLive, setBulkLive] = useState(false);

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="p-4">
          <div className="text-xs text-ink-dim">Health</div>
          <div className="text-2xl font-semibold">{String(health?.status || "—")}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs text-ink-dim">FSID</div>
          <div className="font-mono text-sm">{String(status.data?.data?.fsid || "—")}</div>
        </Card>
        <Card className="p-4">
          <div className="text-xs text-ink-dim">PGs</div>
          <div className="text-2xl font-semibold">{String(pgmap?.num_pgs ?? "—")}</div>
        </Card>
      </div>
      {writable && (
        <div className="flex flex-wrap items-end gap-2">
          <Field label="Service">
            <Input className="w-48" placeholder="ceph.target" value={service} onChange={(e) => setService(e.target.value)} />
          </Field>
          <Button
            size="sm"
            onClick={async () => {
              attachTask((await api.startCeph(node, service || undefined)).upid, [["ceph-status", node], ["ceph-metadata", node]]);
            }}
          >
            Start
          </Button>
          <Button size="sm" variant="outline" onClick={() => setStop(true)}>
            Stop
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={async () => {
              attachTask((await api.restartCeph(node)).upid, [["ceph-status", node], ["ceph-metadata", node]]);
            }}
          >
            Restart
          </Button>
        </div>
      )}
      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Restart-bulk</h2>
          <p className="text-sm text-ink-dim">Cluster-wide rolling restart. Dry-run logs the plan without touching daemons.</p>
          <div className="flex flex-wrap items-end gap-2">
            <Field label="Service">
              <Select className="w-32" value={bulkType} onChange={(e) => setBulkType(e.target.value as typeof bulkType)}>
                <option value="mon">mon</option>
                <option value="mgr">mgr</option>
                <option value="mds">mds</option>
                <option value="osd">osd</option>
              </Select>
            </Field>
            <label className="flex items-center gap-2 text-sm text-ink-dim">
              <input type="checkbox" checked={bulkDry} onChange={(e) => setBulkDry(e.target.checked)} />
              Dry-run
            </label>
            <label className="flex items-center gap-2 text-sm text-ink-dim">
              <input type="checkbox" checked={bulkForce} onChange={(e) => setBulkForce(e.target.checked)} />
              Force past HEALTH_WARN
            </label>
            <label className="flex items-center gap-2 text-sm text-ink-dim">
              <input type="checkbox" checked={bulkOutdated} onChange={(e) => setBulkOutdated(e.target.checked)} />
              Only outdated OSDs
            </label>
            <Button
              size="sm"
              variant={bulkDry ? "outline" : "danger"}
              onClick={() => {
                if (bulkDry) {
                  void (async () => {
                    attachTask((await api.cephRestartBulk({
                      "service-type": bulkType,
                      "dry-run": 1,
                      force: bulkForce ? 1 : 0,
                      "only-outdated": bulkOutdated ? 1 : 0,
                    })).upid, [["ceph-status", node], ["ceph-metadata", node]]);
                  })();
                  return;
                }
                setBulkLive(true);
              }}
            >
              {bulkDry ? "Dry-run" : "Restart"}
            </Button>
          </div>
        </Card>
      )}
      {bulkLive && (
        <ConfirmDialog
          title={`Restart ${bulkType} cluster-wide?`}
          body="Rolling restart of the selected Ceph daemon type. Confirm the cluster can absorb it."
          danger
          confirmLabel="Restart"
          onClose={() => setBulkLive(false)}
          onConfirm={async () => {
            attachTask((await api.cephRestartBulk({
              "service-type": bulkType,
              "dry-run": 0,
              force: bulkForce ? 1 : 0,
              "only-outdated": bulkOutdated ? 1 : 0,
            })).upid, [["ceph-status", node], ["ceph-metadata", node], ["ceph-osds", node]]);
            setBulkLive(false);
          }}
        />
      )}
      {stop && (
        <ConfirmDialog
          title="Stop Ceph services?"
          body={service ? `Stops ${service} on ${node}.` : `Stops ceph.target on ${node}.`}
          danger
          confirmLabel="Stop"
          onClose={() => setStop(false)}
          onConfirm={async () => {
            attachTask((await api.stopCeph(node, service || undefined)).upid, [["ceph-status", node], ["ceph-metadata", node]]);
            setStop(false);
          }}
        />
      )}
      {metadata.data?.supported && (
        <Card className="overflow-hidden">
          <h2 className="px-4 pt-3 font-medium">Metadata</h2>
          <CephMetadataTable data={asRecord(metadata.data.data)} />
          <div className="px-4 pb-3">
            <Advanced label="Advanced: raw metadata">
              <CollapsedRaw data={metadata.data.data ?? {}} />
            </Advanced>
          </div>
        </Card>
      )}
      <CephStatusPanels data={status.data?.data ?? {}} />
    </div>
  );
}

function FlagsTab({ node }: { node: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const flags = useQuery({ queryKey: ["ceph-flags", node], queryFn: () => api.cephFlags(node) });
  const [pending, setPending] = useState<{ key: string; next: number } | null>(null);
  const entries = Object.entries(flags.data?.flags ?? {});

  return (
    <div className="space-y-3">
      <p className="text-sm text-ink-dim">Cluster-wide recovery flags. Confirm before toggling.</p>
      <Card className="overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Flag</th>
              <th>Value</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {entries.map(([key, value]) => {
              const on = value === 1 || value === true || value === "1";
              return (
                <tr key={key} className="border-t border-line">
                  <td className="px-4 py-2 font-mono text-xs">{key}</td>
                  <td>{on ? "on" : "off"}</td>
                  <td className="px-4 text-right">
                    {writable && (
                      <Button size="sm" variant="outline" onClick={() => setPending({ key, next: on ? 0 : 1 })}>
                        {on ? "Clear" : "Set"}
                      </Button>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </Card>
      {pending && (
        <ConfirmDialog
          title={`${pending.next ? "Set" : "Clear"} ${pending.key}?`}
          body="Changes cluster recovery behavior immediately."
          confirmLabel={pending.next ? "Set" : "Clear"}
          onClose={() => setPending(null)}
          onConfirm={async () => {
            await api.updateCephFlags({ [pending.key]: pending.next }, node);
            setPending(null);
            void qc.invalidateQueries({ queryKey: ["ceph-flags", node] });
          }}
        />
      )}
    </div>
  );
}

function ServiceTab({
  title,
  node,
  listKey,
  list,
  create,
  remove,
  safety,
}: {
  title: string;
  node: string;
  listKey: string;
  list: () => Promise<{ name: string; host?: string; state?: string; addr?: string }[]>;
  create: (body: Record<string, unknown>) => Promise<{ upid?: string }>;
  remove: (id: string) => Promise<{ upid?: string }>;
  safety?: "osd" | "mon" | "mds";
}) {
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const rows = useQuery({ queryKey: [listKey, node], queryFn: list });
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState(node);
  const [del, setDel] = useState<string | null>(null);
  const check = useQuery({
    queryKey: ["ceph-cmd-safety", node, safety, del, "destroy"],
    queryFn: () => api.cephCmdSafety(node, safety!, del!, "destroy"),
    enabled: !!del && !!safety,
  });

  return (
    <div className="space-y-4">
      <InventoryCard
        title={title}
        actions={writable ? <BarButton onClick={() => { setName(node); setAdding(true); }}>Add</BarButton> : undefined}
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
            {!(rows.data ?? []).length && <InventoryEmpty>None</InventoryEmpty>}
            {(rows.data ?? []).map((row) => (
              <InventoryRow
                key={row.name}
                kind={row.name}
                summary={joinSummary([row.host, row.state, row.addr])}
              >
                {writable && (
                  <Button size="sm" variant="danger" onClick={() => setDel(row.name)}>
                    Destroy
                  </Button>
                )}
              </InventoryRow>
            ))}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <Modal
          title={`Add ${title.toLowerCase()}`}
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              attachTask((await create({ name })).upid, [[listKey, node], ["ceph-status", node]]);
              setAdding(false);
            }}>Save</Button>
          }
        >
          <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
        </Modal>
      )}
      {del && (
        <ConfirmDialog
          title={`Destroy ${del}?`}
          body={safetyHint(check.data, `Removes this ${title.slice(0, -1).toLowerCase()} after the task finishes.`)}
          danger
          confirmLabel="Destroy"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            attachTask((await remove(del)).upid, [[listKey, node], ["ceph-status", node]]);
            setDel(null);
          }}
        />
      )}
    </div>
  );
}

function OSDsTab({ node }: { node: string }) {
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const osds = useQuery({ queryKey: ["ceph-osds", node], queryFn: () => api.cephOSDs(node) });
  const [destroy, setDestroy] = useState<CephOSD | null>(null);
  const [detail, setDetail] = useState<number | null>(null);
  const destroyId = destroy ? destroy.osdid || destroy.id || 0 : 0;
  const safety = useQuery({
    queryKey: ["ceph-cmd-safety", node, "osd", destroyId, "destroy"],
    queryFn: () => api.cephCmdSafety(node, "osd", String(destroyId), "destroy"),
    enabled: destroy != null,
  });
  const meta = useQuery({
    queryKey: ["ceph-osd-meta", node, detail],
    queryFn: () => api.cephOSDMetadata(detail!, node),
    enabled: detail != null,
  });
  const lv = useQuery({
    queryKey: ["ceph-osd-lv", node, detail],
    queryFn: () => api.cephOSDLVInfo(detail!, node),
    enabled: detail != null,
  });

  const idOf = (o: CephOSD) => o.osdid || o.id || 0;

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">
        Create OSDs from unused disks on the{" "}
        <Link className="text-ink underline decoration-white/30 underline-offset-2 hover:decoration-white" to="/nodes/$node" params={{ node }} search={{ tab: "ceph" }}>
          node Ceph tab
        </Link>
        .
      </p>
      <Card className="overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">OSD</th>
              <th>Host</th>
              <th>Status</th>
              <th>In</th>
              <th>Weight</th>
              <th>Class</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(osds.data ?? []).map((o) => (
              <tr key={idOf(o)} className="border-t border-line">
                <td className="px-4 py-2">{o.name || idOf(o)}</td>
                <td>{o.host || "—"}</td>
                <td>{o.status || "—"}</td>
                <td>{o.in ? "in" : "out"}</td>
                <td>{o.weight ?? "—"}</td>
                <td>{o.crush_device_class || "—"}</td>
                <td className="space-x-2 px-4 text-right">
                  <Button size="sm" variant="outline" onClick={() => setDetail(idOf(o))}>
                    Details
                  </Button>
                  {writable && (
                    <>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={async () => {
                          attachTask((await (o.in ? api.cephOSDOut(idOf(o), node) : api.cephOSDIn(idOf(o), node))).upid, [["ceph-osds", node], ["ceph-status", node]]);
                        }}
                      >
                        {o.in ? "Out" : "In"}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={async () => {
                          attachTask((await api.cephOSDScrub(idOf(o), false, node)).upid, [["ceph-osds", node], ["ceph-status", node]]);
                        }}
                      >
                        Scrub
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={async () => {
                          attachTask((await api.cephOSDScrub(idOf(o), true, node)).upid, [["ceph-osds", node], ["ceph-status", node]]);
                        }}
                      >
                        Deep
                      </Button>
                      <Button size="sm" variant="danger" onClick={() => setDestroy(o)}>
                        Destroy
                      </Button>
                    </>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      {destroy && (
        <ConfirmDialog
          title={`Destroy OSD ${idOf(destroy)}?`}
          body={safetyHint(safety.data, "Removes the OSD and cleans up its disk. Data must be replicated elsewhere.")}
          danger
          confirmLabel="Destroy"
          onClose={() => setDestroy(null)}
          onConfirm={async () => {
            attachTask((await api.destroyCephOSD(idOf(destroy), node)).upid, [["ceph-osds", node], ["ceph-status", node], ["node-disks", node]]);
            setDestroy(null);
          }}
        />
      )}
      {detail != null && (
        <Card className="space-y-2 p-4">
          <div className="flex items-center justify-between">
            <h2 className="font-medium">OSD {detail}</h2>
            <Button size="sm" variant="ghost" onClick={() => setDetail(null)}>
              Close
            </Button>
          </div>
          {meta.data?.supported === false && lv.data?.supported === false ? (
            <p className="text-sm text-ink-dim">OSD details are not available on this cluster.</p>
          ) : (
            <OSDDetailTable meta={asRec(meta.data?.item)} lv={asRec(lv.data?.item)} />
          )}
        </Card>
      )}
    </div>
  );
}

function PoolsTab({ node }: { node: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const pools = useQuery({ queryKey: ["ceph-pools", node], queryFn: () => api.cephPools(node) });
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [size, setSize] = useState("3");
  const [minSize, setMinSize] = useState("2");
  const [pg, setPg] = useState("128");
  const [app, setApp] = useState("rbd");
  const [crush, setCrush] = useState("");
  const [addStorage, setAddStorage] = useState(true);
  const [del, setDel] = useState<CephPool | null>(null);
  const [detail, setDetail] = useState<string | null>(null);
  const [ruleFor, setRuleFor] = useState<CephPool | null>(null);
  const [rule, setRule] = useState("");
  const rules = useQuery({ queryKey: ["ceph-rules", node], queryFn: () => api.cephRules(node) });
  const poolStatus = useQuery({
    queryKey: ["ceph-pool-status", node, detail],
    queryFn: () => api.cephPoolStatus(detail!, node),
    enabled: !!detail,
  });
  const ruleNames = (rules.data?.items ?? []).map((item) => String(item.name || item.rule || item.id || "")).filter(Boolean);

  return (
    <div className="space-y-4">
      <InventoryCard
        title="Pools"
        actions={writable ? <BarButton onClick={() => { setName(""); setAdding(true); }}>Add</BarButton> : undefined}
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
            {!(pools.data ?? []).length && <InventoryEmpty>No pools</InventoryEmpty>}
            {(pools.data ?? []).map((p) => (
              <InventoryRow
                key={p.pool_name}
                kind={p.pool_name}
                summary={joinSummary([
                  p.size != null && `${p.size}/${p.min_size}`,
                  p.pg_num != null && `pg ${p.pg_num}`,
                  p.application,
                  p.crush_rule_name || (p.crush_rule != null ? String(p.crush_rule) : ""),
                ])}
              >
                <Button size="sm" variant="outline" onClick={() => setDetail(p.pool_name)}>
                  Details
                </Button>
                {writable && (
                  <>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        setRuleFor(p);
                        setRule(p.crush_rule_name || "");
                      }}
                    >
                      Set rule
                    </Button>
                    <Button size="sm" variant="danger" onClick={() => setDel(p)}>
                      Delete
                    </Button>
                  </>
                )}
              </InventoryRow>
            ))}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <Modal
          title="Add pool"
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              attachTask(
                (
                  await api.createCephPool(
                    {
                      name,
                      size: Number(size),
                      min_size: Number(minSize),
                      pg_num: Number(pg),
                      application: app,
                      add_storages: addStorage ? 1 : 0,
                      ...(crush ? { crush_rule: crush } : {}),
                    },
                    node,
                  )
                ).upid,
                [["ceph-pools", node], ["ceph-status", node], ["storages"]],
              );
              setName("");
              setAdding(false);
            }}>Save</Button>
          }
        >
          <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
          <Field label="Size"><Input value={size} onChange={(e) => setSize(e.target.value)} /></Field>
          <Field label="Min size"><Input value={minSize} onChange={(e) => setMinSize(e.target.value)} /></Field>
          <Field label="PGs"><Input value={pg} onChange={(e) => setPg(e.target.value)} /></Field>
          <Field label="Application">
            <Select value={app} onChange={(e) => setApp(e.target.value)}>
              <option value="rbd">rbd</option>
              <option value="cephfs">cephfs</option>
              <option value="rgw">rgw</option>
            </Select>
          </Field>
          <Field label="CRUSH rule">
            <Input value={crush} onChange={(e) => setCrush(e.target.value)} list="crush-rules" />
            <datalist id="crush-rules">
              {ruleNames.map((ruleName) => (
                <option key={ruleName} value={ruleName} />
              ))}
            </datalist>
          </Field>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={addStorage} onChange={(e) => setAddStorage(e.target.checked)} />
            Add storage
          </label>
        </Modal>
      )}
      {del && (
        <ConfirmDialog
          title={`Delete pool ${del.pool_name}?`}
          body="Destroys the pool. Guests using it will lose that disk."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            attachTask((await api.deleteCephPool(del.pool_name, node)).upid, [["ceph-pools", node], ["ceph-status", node], ["storages"]]);
            setDel(null);
          }}
        />
      )}
      {ruleFor && (
        <Modal
          title={`Set crush rule on ${ruleFor.pool_name}`}
          onClose={() => setRuleFor(null)}
          footer={
            <Button
              onClick={async () => {
                if (!rule) return;
                await api.updateCephPool(ruleFor.pool_name, { crush_rule: rule }, node);
                setRuleFor(null);
                void qc.invalidateQueries({ queryKey: ["ceph-pools", node] });
              }}
            >
              Save
            </Button>
          }
        >
          <p className="text-sm text-ink-dim">Assigns an existing CRUSH rule. This does not edit the crush map.</p>
          <Field label="Rule">
            {ruleNames.length ? (
              <Select value={rule} onChange={(e) => setRule(e.target.value)}>
                <option value="">Select rule</option>
                {ruleNames.map((ruleName) => (
                  <option key={ruleName} value={ruleName}>{ruleName}</option>
                ))}
              </Select>
            ) : (
              <Input value={rule} onChange={(e) => setRule(e.target.value)} placeholder="crush_rule" />
            )}
          </Field>
        </Modal>
      )}
      {detail && (
        <Card className="space-y-2 p-4">
          <div className="flex items-center justify-between">
            <h2 className="font-medium">{detail}</h2>
            <Button size="sm" variant="ghost" onClick={() => setDetail(null)}>
              Close
            </Button>
          </div>
          {poolStatus.data?.supported === false ? (
            <p className="text-sm text-ink-dim">Pool status is not available on this cluster.</p>
          ) : (
            <PoolStatusTable data={asRec(poolStatus.data?.item)} />
          )}
        </Card>
      )}
    </div>
  );
}

function RGWTab({ node }: { node: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const rgw = useQuery({ queryKey: ["ceph-rgw", node], queryFn: () => api.cephRGW(node) });
  const users = useQuery({ queryKey: ["ceph-rgw-users", node], queryFn: () => api.cephRGWUsers(node) });
  const [addingUser, setAddingUser] = useState(false);
  const [uid, setUid] = useState("");
  const [display, setDisplay] = useState("");
  const [email, setEmail] = useState("");
  if (rgw.data && !rgw.data.supported) {
    return <p className="text-sm text-ink-dim">RGW is not available on this cluster.</p>;
  }
  return (
    <div className="space-y-4">
      <ServiceTab
        title="RGW"
        node={node}
        listKey="ceph-rgw-rows"
        list={async () => {
          const data = await api.cephRGW(node);
          return (data.items ?? []).map((item) => ({
            name: String(item.name || item.id || item.host || ""),
            host: item.host != null ? String(item.host) : undefined,
            state: item.state != null ? String(item.state) : undefined,
            addr: item.addr != null ? String(item.addr) : undefined,
          }));
        }}
        create={(body) => api.createCephRGW(body, node)}
        remove={(id) => api.deleteCephRGW(id, node)}
      />
      <InventoryCard
        title="Users"
        actions={writable && users.data?.supported !== false ? <BarButton onClick={() => { setUid(""); setDisplay(""); setEmail(""); setAddingUser(true); }}>Add</BarButton> : undefined}
        message={users.data?.supported === false ? "RGW users are not available on this cluster." : undefined}
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
            {!(users.data?.items ?? []).length && <InventoryEmpty>No RGW users</InventoryEmpty>}
            {(users.data?.items ?? []).map((item, i) => {
              const id = String(item.uid ?? item.user_id ?? item.id ?? i);
              const keys = Array.isArray(item.keys) ? item.keys.length : item.access_key ? 1 : 0;
              return (
                <InventoryRow
                  key={id}
                  kind={id}
                  summary={joinSummary([
                    String(item["display-name"] ?? item.display_name ?? item.name ?? ""),
                    keys ? `${keys} keys` : "",
                  ])}
                >
                  {writable && (
                    <Button size="sm" variant="danger" onClick={async () => {
                      await api.deleteCephRGWUser(id, node);
                      void qc.invalidateQueries({ queryKey: ["ceph-rgw-users", node] });
                    }}>Delete</Button>
                  )}
                </InventoryRow>
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {addingUser && (
        <Modal
          title="Add user"
          onClose={() => setAddingUser(false)}
          footer={
            <Button onClick={async () => {
              await api.createCephRGWUser({ uid, "display-name": display, email }, node);
              setUid("");
              setDisplay("");
              setEmail("");
              setAddingUser(false);
              void qc.invalidateQueries({ queryKey: ["ceph-rgw-users", node] });
            }}>Save</Button>
          }
        >
          <Field label="UID"><Input value={uid} onChange={(e) => setUid(e.target.value)} /></Field>
          <Field label="Display name"><Input value={display} onChange={(e) => setDisplay(e.target.value)} /></Field>
          <Field label="Email"><Input value={email} onChange={(e) => setEmail(e.target.value)} /></Field>
        </Modal>
      )}
    </div>
  );
}

function safetyHint(check: CephCmdSafety | undefined, fallback: string) {
  if (!check || !check.supported) return fallback;
  if (check.safe) return `${fallback} Ceph reports this is safe.`;
  return `${fallback} Ceph reports this may not be safe${check.status ? `: ${check.status}` : "."}`;
}

function CrushTab({ node }: { node: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const crush = useQuery({ queryKey: ["ceph-crush", node], queryFn: () => api.cephCrush(node) });
  const rules = useQuery({ queryKey: ["ceph-rules", node], queryFn: () => api.cephRules(node) });
  const [mapText, setMapText] = useState("");
  const [ruleName, setRuleName] = useState("");
  const [ruleType, setRuleType] = useState("replicated");
  const [ruleRoot, setRuleRoot] = useState("default");
  return (
    <div className="space-y-4">
      <Card className="overflow-hidden">
        <h2 className="px-4 pt-3 font-medium">Rules</h2>
        <CrushRulesTable items={rules.data?.items ?? []} />
        <div className="px-4 pb-3">
          <Advanced label="Advanced: raw rules">
            <CollapsedRaw data={rules.data?.items ?? []} />
          </Advanced>
        </div>
      </Card>
      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Create rule</h2>
          <div className="grid gap-3 md:grid-cols-3">
            <Field label="Name"><Input value={ruleName} onChange={(e) => setRuleName(e.target.value)} /></Field>
            <Field label="Type">
              <Select value={ruleType} onChange={(e) => setRuleType(e.target.value)}>
                <option value="replicated">replicated</option>
                <option value="erasure">erasure</option>
              </Select>
            </Field>
            <Field label="Root"><Input value={ruleRoot} onChange={(e) => setRuleRoot(e.target.value)} /></Field>
          </div>
          <Button size="sm" onClick={async () => {
            await api.createCephRule({ name: ruleName, type: ruleType, root: ruleRoot }, node);
            setRuleName("");
            void qc.invalidateQueries({ queryKey: ["ceph-rules", node] });
          }}>Create</Button>
        </Card>
      )}
      <Card className="overflow-hidden">
        <h2 className="px-4 pt-3 font-medium">CRUSH map</h2>
        <CrushHierarchy data={crush.data?.data ?? {}} />
        <div className="px-4 pb-3">
          <Advanced label="Advanced: raw CRUSH map">
            <CollapsedRaw data={crush.data?.data ?? {}} />
          </Advanced>
        </div>
      </Card>
      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Replace CRUSH map</h2>
          <Textarea className="min-h-32 font-mono text-xs" value={mapText} onChange={(e) => setMapText(e.target.value)} placeholder="# crush map text" />
          <Button size="sm" onClick={async () => {
            await api.updateCephCrush({ crushmap: mapText }, node);
            void qc.invalidateQueries({ queryKey: ["ceph-crush", node] });
          }}>Save map</Button>
        </Card>
      )}
    </div>
  );
}

function CephFSTab({ node }: { node: string }) {
  const can = useCan();
  const writable = can("/", "Sys.Modify") || can(`/nodes/${node}`, "Sys.Modify");
  const fs = useQuery({ queryKey: ["ceph-fs", node], queryFn: () => api.cephFS(node) });
  const mds = useQuery({ queryKey: ["ceph-mds", node], queryFn: () => api.cephMDS(node) });
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");

  if (fs.data && !fs.data.supported) {
    return <p className="text-sm text-ink-dim">CephFS is not available on this cluster.</p>;
  }

  return (
    <div className="space-y-4">
      <InventoryCard
        title="CephFS"
        actions={writable ? <BarButton onClick={() => { setName(""); setAdding(true); }}>Add</BarButton> : undefined}
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
            {!(fs.data?.items ?? []).length && <InventoryEmpty>No filesystems</InventoryEmpty>}
            {(fs.data?.items ?? []).map((item, i) => {
              const fsName = String(item.name || item.id || i);
              const dataPools = item.data_pools ?? item.data_pool;
              return (
                <InventoryRow
                  key={fsName}
                  kind={fsName}
                  summary={joinSummary([
                    item.metadata_pool != null && `meta ${item.metadata_pool}`,
                    item.meta_pool != null && item.metadata_pool == null && `meta ${item.meta_pool}`,
                    Array.isArray(dataPools) ? dataPools.map(String).join(", ") : dataPools != null ? String(dataPools) : "",
                  ])}
                />
              );
            })}
          </tbody>
        </table>
      </InventoryCard>
      {adding && (
        <Modal
          title="Add filesystem"
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              attachTask((await api.createCephFS({ name }, node)).upid, [["ceph-fs", node], ["ceph-status", node]]);
              setName("");
              setAdding(false);
            }}>Save</Button>
          }
        >
          <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
        </Modal>
      )}
      <ServiceTab
        title="MDS"
        node={node}
        listKey="ceph-mds"
        list={() => api.cephMDS(node)}
        create={(body) => api.createCephMDS(body, node)}
        remove={(id) => api.deleteCephMDS(id, node)}
        safety="mds"
      />
      {mds.isError && <p className="text-sm text-ink-dim">No MDS list.</p>}
    </div>
  );
}

function ConfigTab({ node }: { node: string }) {
  const cfg = useQuery({ queryKey: ["ceph-config", node], queryFn: () => api.cephConfig(node) });
  const log = useQuery({ queryKey: ["ceph-log", node], queryFn: () => api.cephLog(node), refetchInterval: 15000 });
  return (
    <div className="space-y-4">
      <Card className="overflow-hidden">
        <h2 className="px-4 pt-3 font-medium">ceph.conf</h2>
        <CephConfigTable data={cfg.data?.data ?? {}} />
        <div className="px-4 pb-3">
          <Advanced label="Advanced: raw ceph.conf">
            <CollapsedRaw data={cfg.data?.data ?? {}} />
          </Advanced>
        </div>
      </Card>
      <LogPanel lines={log.data ?? []} />
    </div>
  );
}

function asRec(v: unknown): Record<string, unknown> {
  return v && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : {};
}

function pickStr(data: Record<string, unknown>, keys: string[]): string {
  for (const k of keys) {
    if (data[k] != null && data[k] !== "") return String(data[k]);
  }
  return "—";
}

function OSDDetailTable({ meta, lv }: { meta: Record<string, unknown>; lv: Record<string, unknown> }) {
  return (
    <div className="space-y-2">
      <table className="w-full text-sm">
        <tbody>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">UUID</td><td className="px-4 py-1.5 font-mono text-xs">{pickStr(meta, ["uuid", "osd_uuid"])}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">Host</td><td className="px-4 py-1.5">{pickStr(meta, ["hostname", "host"])}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">Device</td><td className="px-4 py-1.5 font-mono text-xs">{pickStr(meta, ["devices", "device", "backend_dev_name"])}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">Class</td><td className="px-4 py-1.5">{pickStr(meta, ["device_class", "crush_device_class", "class"])}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">Status</td><td className="px-4 py-1.5">{pickStr(meta, ["status", "state"])}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">LV</td><td className="px-4 py-1.5 font-mono text-xs">{pickStr(lv, ["lv_name", "lv_path", "name"])}</td></tr>
        </tbody>
      </table>
      <Advanced label="Advanced: raw OSD details">
        <CollapsedRaw data={{ metadata: meta, lv }} />
      </Advanced>
    </div>
  );
}

function PoolStatusTable({ data }: { data: Record<string, unknown> }) {
  const stats = asRec(data.stats);
  const pg = asRec(data.pgmap);
  return (
    <div className="space-y-2">
      <table className="w-full text-sm">
        <tbody>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">PGs</td><td className="px-4 py-1.5">{pickStr({ ...pg, ...data, ...stats }, ["pg_num", "pg_stat", "num_pgs", "pg"])}</td></tr>
          <tr className="border-t border-line"><td className="px-4 py-1.5 text-xs uppercase text-ink-dim">Objects</td><td className="px-4 py-1.5">{pickStr({ ...stats, ...data }, ["objects", "num_objects", "stored_objects"])}</td></tr>
        </tbody>
      </table>
      <Advanced label="Advanced: raw pool status">
        <CollapsedRaw data={data} />
      </Advanced>
    </div>
  );
}

function CephStatusPanels({ data }: { data: Record<string, unknown> }) {
  const health = asRec(data.health);
  const checks = asRec(health.checks);
  const monmap = asRec(data.monmap);
  const osdmap = asRec(data.osdmap);
  const pgmap = asRec(data.pgmap);
  return (
    <div className="space-y-4">
      <Card className="overflow-hidden">
        <h2 className="px-4 pt-3 font-medium">Health checks</h2>
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Check</th><th>Severity</th><th>Summary</th></tr>
          </thead>
          <tbody>
            {Object.entries(checks).map(([id, raw]) => {
              const check = asRec(raw);
              const summary = asRec(check.summary);
              return (
                <tr key={id} className="border-t border-line">
                  <td className="px-4 py-2 font-mono text-xs">{id}</td>
                  <td>{String(check.severity ?? "—")}</td>
                  <td className="text-ink-dim">{String(summary.message ?? check.summary ?? "—")}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {!Object.keys(checks).length && <p className="px-4 py-3 text-sm text-ink-dim">No health checks.</p>}
      </Card>
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="space-y-1 p-4 text-sm">
          <div className="text-xs text-ink-dim">Monitors</div>
          <div>Count {String(monmap.num_mons ?? (Array.isArray(monmap.mons) ? monmap.mons.length : "—"))}</div>
        </Card>
        <Card className="space-y-1 p-4 text-sm">
          <div className="text-xs text-ink-dim">OSDs</div>
          <div>Total {String(osdmap.num_osds ?? "—")}</div>
          <div>Up {String(osdmap.num_up_osds ?? "—")} / in {String(osdmap.num_in_osds ?? "—")}</div>
        </Card>
        <Card className="space-y-1 p-4 text-sm">
          <div className="text-xs text-ink-dim">PGs</div>
          <div>PGs {String(pgmap.num_pgs ?? "—")}</div>
          <div>Used {String(pgmap.bytes_used ?? "—")} / {String(pgmap.bytes_total ?? "—")}</div>
        </Card>
      </div>
      <Advanced label="Advanced: raw status">
        <CollapsedRaw data={data} />
      </Advanced>
    </div>
  );
}

function CephConfigTable({ data }: { data: Record<string, unknown> }) {
  const rows: { section: string; key: string; value: string }[] = [];
  if (typeof data.value === "string") {
    let section = "global";
    for (const line of data.value.split("\n")) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith("#")) continue;
      const sec = trimmed.match(/^\[(.+)]$/);
      if (sec) {
        section = sec[1];
        continue;
      }
      const eq = trimmed.indexOf("=");
      if (eq > 0) rows.push({ section, key: trimmed.slice(0, eq).trim(), value: trimmed.slice(eq + 1).trim() });
    }
  } else if (Array.isArray(data.index)) {
    for (const item of data.index) {
      const rec = asRec(item);
      rows.push({
        section: String(rec.section ?? rec.who ?? "global"),
        key: String(rec.name ?? rec.key ?? ""),
        value: String(rec.value ?? rec.val ?? ""),
      });
    }
  } else {
    for (const [section, raw] of Object.entries(data)) {
      const rec = asRec(raw);
      if (Object.keys(rec).length) {
        for (const [key, value] of Object.entries(rec)) {
          rows.push({ section, key, value: String(value ?? "") });
        }
      } else {
        rows.push({ section: "global", key: section, value: String(raw ?? "") });
      }
    }
  }
  if (!rows.length) return <p className="px-4 py-3 text-sm text-ink-dim">No config keys.</p>;
  return (
    <table className="w-full text-sm">
      <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
        <tr><th className="px-4 py-2">Section</th><th>Key</th><th>Value</th></tr>
      </thead>
      <tbody>
        {rows.map((r, i) => (
          <tr key={`${r.section}-${r.key}-${i}`} className="border-t border-line">
            <td className="px-4 py-2">{r.section}</td>
            <td className="font-mono text-xs">{r.key}</td>
            <td className="text-ink-dim">{r.value}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function crushSteps(raw: unknown): string {
  const steps = asRecords(raw);
  if (!steps.length) return "—";
  return steps.map((s) => {
    const op = String(s.op ?? s.operation ?? "—");
    const item = String(s.item_name ?? s.item ?? s.name ?? "");
    const typ = String(s.type_name ?? s.type ?? "");
    return [op, item, typ].filter(Boolean).join(" ");
  }).join(" → ");
}

function CephMetadataTable({ data }: { data: Record<string, unknown> }) {
  const rows: { kind: string; name: string; version: string }[] = [];
  for (const kind of ["mgr", "mon", "osd"]) {
    const group = asRecord(data[kind]);
    const items = Object.keys(group).length ? Object.entries(group) : asRecords(data[kind]).map((item, i) => [String(item.id ?? item.name ?? i), item] as const);
    for (const [name, raw] of items) {
      const rec = asRecord(raw);
      if (!Object.keys(rec).length) continue;
      rows.push({
        kind,
        name: String(rec.id ?? rec.name ?? rec.hostname ?? name),
        version: String(rec.ceph_version_short ?? rec.ceph_version ?? rec.version ?? "—"),
      });
    }
  }
  if (!rows.length) return <p className="px-4 py-3 text-sm text-ink-dim">No mgr/mon/osd version rows.</p>;
  return (
    <table className="w-full text-sm">
      <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
        <tr><th className="px-4 py-2">Kind</th><th>Name</th><th>Version</th></tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={`${r.kind}-${r.name}`} className="border-t border-line">
            <td className="px-4 py-2">{r.kind}</td>
            <td>{r.name}</td>
            <td className="font-mono text-xs text-ink-dim">{r.version}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function CrushRulesTable({ items }: { items: Record<string, unknown>[] }) {
  if (!items.length) return <p className="px-4 py-3 text-sm text-ink-dim">No rules.</p>;
  return (
    <table className="w-full text-sm">
      <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
        <tr><th className="px-4 py-2">Name</th><th>Type</th><th>Steps</th></tr>
      </thead>
      <tbody>
        {items.map((item, i) => (
          <tr key={String(item.rule_name ?? item.name ?? i)} className="border-t border-line">
            <td className="px-4 py-2">{String(item.rule_name ?? item.name ?? "—")}</td>
            <td>{String(item.type ?? item.rule_type ?? "—")}</td>
            <td className="font-mono text-xs text-ink-dim">
              {crushSteps(item.steps)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function CrushHierarchy({ data }: { data: Record<string, unknown> }) {
  const buckets = Array.isArray(data.buckets) ? data.buckets as Record<string, unknown>[]
    : Array.isArray(asRec(data.value).buckets) ? asRec(data.value).buckets as Record<string, unknown>[]
    : [];
  if (!buckets.length) return <p className="px-4 py-3 text-sm text-ink-dim">No named buckets.</p>;
  return (
    <ul className="space-y-1 px-4 py-3 text-sm">
      {buckets.map((b, i) => (
        <li key={String(b.name ?? b.id ?? i)}>
          <span className="font-medium">{String(b.name ?? "—")}</span>
          <span className="text-ink-dim"> {String(b.type_name ?? b.type ?? "")}</span>
        </li>
      ))}
    </ul>
  );
}
