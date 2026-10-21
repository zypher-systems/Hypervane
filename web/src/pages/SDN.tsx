import { VNetFirewall } from "@/components/firewall/VNetFirewall";
import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { BarButton } from "@/components/ui/action-bar";
import { InventoryCard, InventoryEmpty, InventoryRow, joinSummary } from "@/components/ui/inventory";
import { Advanced, CollapsedRaw, FabricIfaceTable, InferTable, ItemDetails, KeyValueTable, asRecord, asRecords } from "@/components/ui/item-details";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import type { FeatureList, SDNSubnet, SDNVNet, SDNZone } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useState, type ReactNode } from "react";
import { Tabs } from "@/components/ui/tabs";
import { DatacenterHeader } from "@/components/layout/DatacenterNav";

const tabs = ["apply", "zones", "vnets", "controllers", "ipam", "dns", "fabrics", "route-maps", "prefix-lists"] as const;
const zoneTypes = ["simple", "vlan", "qinq", "vxlan", "evpn"] as const;

export function SDNPage() {
  const search = useSearch({ from: "/app/sdn" });
  const nav = useNavigate();
  const status = useQuery({ queryKey: ["sdn-status"], queryFn: api.sdnStatus });
  const fabrics = useQuery({ queryKey: ["sdn-fabrics"], queryFn: api.sdnFabrics, enabled: status.data?.supported === true });
  const maps = useQuery({ queryKey: ["sdn-route-maps"], queryFn: api.sdnRouteMaps, enabled: status.data?.supported === true });
  const prefixes = useQuery({ queryKey: ["sdn-prefix-lists"], queryFn: api.sdnPrefixLists, enabled: status.data?.supported === true });
  const [tab, setTab] = useState<(typeof tabs)[number]>("apply");

  useEffect(() => {
    if (search.tab === "runtime" && search.node) {
      void nav({ to: "/nodes/$node", params: { node: search.node }, search: { tab: "sdn" } });
    }
  }, [nav, search.node, search.tab]);

  if (status.isLoading) {
    return <p className="text-sm text-ink-dim">Loading SDN…</p>;
  }
  if (!status.data?.supported) {
    return (
      <div className="space-y-2">
        <h2 className="text-base font-semibold">SDN</h2>
        <p className="text-sm text-ink-dim">SDN is not installed on this cluster.</p>
      </div>
    );
  }

  const hidden = new Set<string>();
  if (fabrics.data?.supported === false) hidden.add("fabrics");
  if (maps.data?.supported === false) hidden.add("route-maps");
  if (prefixes.data?.supported === false) hidden.add("prefix-lists");
  const visible = tabs.filter((t) => !hidden.has(t));
  const current = hidden.has(tab) ? "apply" : tab;

  return (
    <div className="space-y-5">
      <DatacenterHeader />
      <div>
        <h2 className="text-base font-semibold">SDN</h2>
        <p className="text-sm text-ink-dim">Zones, VNets, controllers, and apply. Host bridges stay on the node Network tab.</p>
      </div>
      <Tabs items={visible} value={current} onChange={setTab} label={(t) => t === "ipam" ? "IPAM" : t === "vnets" ? "VNets" : t === "dns" ? "DNS" : t === "route-maps" ? "Route maps" : t === "prefix-lists" ? "Prefix lists" : t} />
      {current === "apply" && <ApplyTab />}
      {current === "zones" && <ZonesTab />}
      {current === "vnets" && <VNetsTab />}
      {current === "controllers" && (
        <PluginTab
          title="Controllers"
          hint="EVPN and BGP controllers."
          idKey="controller"
          listKey="sdn-controllers"
          list={api.sdnControllers}
          create={api.createSDNController}
          update={api.updateSDNController}
          remove={api.deleteSDNController}
          defaults={{ type: "evpn" }}
          types={["evpn", "bgp"]}
          kind="controller"
        />
      )}
      {current === "ipam" && <IPAMTab />}
      {current === "dns" && (
        <PluginTab
          title="DNS"
          hint="PowerDNS plugin."
          idKey="dns"
          listKey="sdn-dns"
          list={api.sdnDNS}
          create={api.createSDNDNS}
          update={api.updateSDNDNS}
          remove={api.deleteSDNDNS}
          defaults={{ type: "powerdns" }}
          types={["powerdns"]}
          kind="dns"
        />
      )}
      {current === "fabrics" && <FabricsTab />}
      {current === "route-maps" && <RouteMapsTab />}
      {current === "prefix-lists" && <PrefixListsTab />}
    </div>
  );
}

function ApplyTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const status = useQuery({ queryKey: ["sdn-status"], queryFn: api.sdnStatus });
  const zones = useQuery({ queryKey: ["sdn-zones"], queryFn: api.sdnZones });
  const vnets = useQuery({ queryKey: ["sdn-vnets"], queryFn: api.sdnVNets });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const nodes = (summary.data?.resources ?? []).filter((r) => r.type === "node");
  const pendingZones = (zones.data ?? []).filter((z) => z.pending).length;
  const pendingVNets = (vnets.data ?? []).filter((v) => v.pending).length;
  const [rollback, setRollback] = useState(false);
  const [dryNode, setDryNode] = useState("");
  const [lockToken, setLockToken] = useState("");
  const [lockMsg, setLockMsg] = useState("");
  const currentDry = dryNode || nodes[0]?.node || "";
  const canDry = can(`/nodes/${currentDry}`, "Sys.Audit") || can("/", "Sys.Audit");
  const dry = useQuery({
    queryKey: ["sdn-dry-run", currentDry],
    queryFn: () => api.sdnDryRun(currentDry),
    enabled: false,
  });

  const lockBody = lockToken ? { "lock-token": lockToken } : undefined;

  return (
    <div className="space-y-4">
      <Card className="space-y-3 p-4">
        <p className="text-sm text-ink-dim">
          {pendingZones + pendingVNets > 0
            ? `${pendingZones} zone(s) and ${pendingVNets} VNet(s) have pending writes. Apply publishes them cluster-wide.`
            : "No pending zone or VNet writes. Apply still reloads the running SDN config."}
        </p>
        {status.data?.data && <SDNStatusTable data={status.data.data} />}
        {writable && (
          <div className="flex gap-2">
            <Button
              size="sm"
              onClick={async () => {
                attachTask((await api.applySDN(lockBody)).upid, [["sdn-status"], ["sdn-zones"], ["sdn-vnets"]]);
              }}
            >
              Apply
            </Button>
            <Button size="sm" variant="outline" onClick={() => setRollback(true)}>
              Rollback
            </Button>
          </div>
        )}
      </Card>
      <Card className="space-y-3 p-4">
        <h2 className="font-medium">Dry-run</h2>
        <div className="flex flex-wrap items-end gap-2">
          <Field label="Node">
            <Select className="w-48" value={currentDry} onChange={(e) => setDryNode(e.target.value)}>
              {nodes.map((n) => (
                <option key={n.node} value={n.node}>
                  {n.node}
                </option>
              ))}
            </Select>
          </Field>
          {canDry && (
            <Button
              size="sm"
              variant="outline"
              onClick={async () => {
                await dry.refetch();
              }}
            >
              Dry-run
            </Button>
          )}
        </div>
        {dry.data?.supported === false && <p className="text-sm text-ink-dim">Dry-run is not available on this cluster.</p>}
        {dry.data?.supported && <SDNDryRunBlocks data={asRecord(dry.data.item)} />}
      </Card>
      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Lock</h2>
          <p className="text-sm text-ink-dim">SDN config lock-secret stays on this page only. It is not the PVE ticket.</p>
          <div className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={async () => {
                const res = await api.lockSDN();
                if (!res.supported) {
                  setLockMsg("Lock is not available on this cluster.");
                  return;
                }
                setLockToken(res.value ?? "");
                setLockMsg(res.value ? "Lock acquired." : "Lock acquired with an empty token.");
              }}
            >
              Lock
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={async () => {
                const res = await api.unlockSDN(lockToken || undefined, false);
                setLockMsg(res.supported === false ? "Unlock is not available on this cluster." : "Unlocked.");
                if (res.supported) setLockToken("");
              }}
            >
              Unlock
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={async () => {
                const res = await api.unlockSDN(lockToken || undefined, true);
                setLockMsg(res.supported === false ? "Unlock is not available on this cluster." : "Force unlocked.");
                if (res.supported) setLockToken("");
              }}
            >
              Force unlock
            </Button>
          </div>
          {lockToken && <p className="font-mono text-xs text-ink-dim">Token: {lockToken}</p>}
          {lockMsg && <p className="text-sm text-ink-dim">{lockMsg}</p>}
        </Card>
      )}
      {rollback && (
        <ConfirmDialog
          title="Rollback pending SDN?"
          body="Reverts unpublished zone and VNet writes if this cluster supports DELETE /cluster/sdn."
          danger
          confirmLabel="Rollback"
          onClose={() => setRollback(false)}
          onConfirm={async () => {
            await api.rollbackSDN(lockBody);
            setRollback(false);
            void qc.invalidateQueries({ queryKey: ["sdn-status"] });
            void qc.invalidateQueries({ queryKey: ["sdn-zones"] });
            void qc.invalidateQueries({ queryKey: ["sdn-vnets"] });
          }}
        />
      )}
    </div>
  );
}

function ZonesTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const zones = useQuery({ queryKey: ["sdn-zones"], queryFn: api.sdnZones });
  const [adding, setAdding] = useState(false);
  const [type, setType] = useState<(typeof zoneTypes)[number]>("simple");
  const [fields, setFields] = useState<Record<string, string>>({ zone: "" });
  const [edit, setEdit] = useState<SDNZone | null>(null);
  const [del, setDel] = useState<SDNZone | null>(null);

  const set = (key: string, value: string) => setFields((prev) => ({ ...prev, [key]: value }));

  return (
    <div className="space-y-4">
      <FeatureTable
        title="Zones"
        actions={writable ? <BarButton onClick={() => { setFields({ zone: "" }); setType("simple"); setAdding(true); }}>Add</BarButton> : undefined}
        list={{ supported: true, items: (zones.data ?? []) as unknown as Record<string, unknown>[] }}
        idKey="zone"
        writable={writable}
        detailKeys={["type", "pending"]}
        onEdit={(item) => {
          const z = (zones.data ?? []).find((x) => x.zone === String(item.zone));
          if (z) setEdit(z);
        }}
        onDelete={async (id) => {
          const z = (zones.data ?? []).find((x) => x.zone === id);
          if (z) setDel(z);
        }}
      />
      {adding && (
        <Modal
          title="Add zone"
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              await api.createSDNZone(zoneBody(type, fields));
              setFields({ zone: "" });
              setAdding(false);
              void qc.invalidateQueries({ queryKey: ["sdn-zones"] });
            }}>Save</Button>
          }
        >
          <Field label="Zone">
            <Input value={fields.zone ?? ""} onChange={(e) => set("zone", e.target.value)} />
          </Field>
          <Field label="Type">
            <Select value={type} onChange={(e) => setType(e.target.value as (typeof zoneTypes)[number])}>
              {zoneTypes.map((t) => (
                <option key={t}>{t}</option>
              ))}
            </Select>
          </Field>
          {zoneFields(type).map((key) => (
            <Field key={key} label={key}>
              <Input value={fields[key] ?? ""} onChange={(e) => set(key, e.target.value)} />
            </Field>
          ))}
        </Modal>
      )}
      {edit && <ZoneEdit zone={edit} onDone={() => setEdit(null)} />}
      {del && (
        <ConfirmDialog
          title={`Delete zone ${del.zone}?`}
          body="Removes the zone after the next apply. VNets in this zone must be deleted first."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteSDNZone(del.zone);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["sdn-zones"] });
          }}
        />
      )}
    </div>
  );
}

function ZoneEdit({ zone, onDone }: { zone: SDNZone; onDone: () => void }) {
  const qc = useQueryClient();
  const type = zone.type || "simple";
  const [fields, setFields] = useState<Record<string, string>>(() => {
    const rec = zone as unknown as Record<string, unknown>;
    const next: Record<string, string> = {};
    for (const key of zoneFields(type)) {
      if (rec[key] !== undefined) next[key] = String(rec[key] ?? "");
    }
    return next;
  });
  const set = (key: string, value: string) => setFields((prev) => ({ ...prev, [key]: value }));
  return (
    <Modal
      title={`Edit ${zone.zone}`}
      onClose={onDone}
      footer={
        <Button
          onClick={async () => {
            const body = zoneBody(type, fields);
            if (zone.digest) body.digest = zone.digest;
            await api.updateSDNZone(zone.zone, body);
            onDone();
            void qc.invalidateQueries({ queryKey: ["sdn-zones"] });
          }}
        >
          Save
        </Button>
      }
    >
      {zoneFields(type).map((key) => (
        <Field key={key} label={key}>
          <Input value={fields[key] ?? ""} onChange={(e) => set(key, e.target.value)} />
        </Field>
      ))}
    </Modal>
  );
}

function VNetsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const zones = useQuery({ queryKey: ["sdn-zones"], queryFn: api.sdnZones });
  const vnets = useQuery({ queryKey: ["sdn-vnets"], queryFn: api.sdnVNets });
  const [filter, setFilter] = useState("");
  const [adding, setAdding] = useState(false);
  const [vnet, setVnet] = useState("");
  const [zone, setZone] = useState("");
  const [tag, setTag] = useState("");
  const [alias, setAlias] = useState("");
  const [open, setOpen] = useState<string | null>(null);
  const [edit, setEdit] = useState<SDNVNet | null>(null);
  const [editTag, setEditTag] = useState("");
  const [editAlias, setEditAlias] = useState("");
  const [editVlan, setEditVlan] = useState(false);
  const [del, setDel] = useState<SDNVNet | null>(null);
  const rows = (vnets.data ?? []).filter((v) => !filter || v.zone === filter);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        <Select className="max-w-xs" value={filter} onChange={(e) => setFilter(e.target.value)}>
          <option value="">All zones</option>
          {(zones.data ?? []).map((z) => (
            <option key={z.zone} value={z.zone}>
              {z.zone}
            </option>
          ))}
        </Select>
      </div>
      <FeatureTable
        title="VNets"
        actions={writable ? <BarButton onClick={() => { setVnet(""); setZone(filter); setTag(""); setAlias(""); setAdding(true); }}>Add</BarButton> : undefined}
        list={{ supported: true, items: rows as unknown as Record<string, unknown>[] }}
        idKey="vnet"
        writable={writable}
        detailKeys={["zone", "tag", "alias", "pending"]}
        onOpen={setOpen}
        onEdit={(item) => {
          const v = rows.find((x) => x.vnet === String(item.vnet));
          if (!v) return;
          setEdit(v);
          setEditTag(v.tag != null ? String(v.tag) : "");
          setEditAlias(v.alias ?? "");
          setEditVlan(Boolean(v.vlanaware));
        }}
        onDelete={async (id) => {
          const v = rows.find((x) => x.vnet === id);
          if (v) setDel(v);
        }}
      />
      {adding && (
        <Modal
          title="Add VNet"
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              const body: Record<string, unknown> = { vnet, zone };
              if (tag) body.tag = Number(tag);
              if (alias) body.alias = alias;
              await api.createSDNVNet(body);
              setVnet("");
              setTag("");
              setAlias("");
              setAdding(false);
              void qc.invalidateQueries({ queryKey: ["sdn-vnets"] });
            }}>Save</Button>
          }
        >
          <Field label="VNet"><Input value={vnet} onChange={(e) => setVnet(e.target.value)} /></Field>
          <Field label="Zone">
            <Select value={zone} onChange={(e) => setZone(e.target.value)}>
              <option value="">zone</option>
              {(zones.data ?? []).map((z) => (
                <option key={z.zone} value={z.zone}>{z.zone}</option>
              ))}
            </Select>
          </Field>
          <Field label="Tag"><Input value={tag} onChange={(e) => setTag(e.target.value)} /></Field>
          <Field label="Alias"><Input value={alias} onChange={(e) => setAlias(e.target.value)} /></Field>
        </Modal>
      )}
      {open && <VNetDetail vnet={open} onClose={() => setOpen(null)} />}
      {edit && (
        <Modal
          title={`Edit ${edit.vnet}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              const body: Record<string, unknown> = {
                alias: editAlias,
                vlanaware: editVlan ? 1 : 0,
              };
              if (editTag) body.tag = Number(editTag);
              await api.updateSDNVNet(edit.vnet, body);
              setEdit(null);
              void qc.invalidateQueries({ queryKey: ["sdn-vnets"] });
            }}>Save</Button>
          }
        >
          <Field label="Tag"><Input value={editTag} onChange={(e) => setEditTag(e.target.value)} /></Field>
          <Field label="Alias"><Input value={editAlias} onChange={(e) => setEditAlias(e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={editVlan} onChange={(e) => setEditVlan(e.target.checked)} />
            VLAN aware
          </label>
        </Modal>
      )}
      {del && (
        <ConfirmDialog
          title={`Delete VNet ${del.vnet}?`}
          body="Removes the VNet after the next apply."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteSDNVNet(del.vnet);
            if (open === del.vnet) setOpen(null);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["sdn-vnets"] });
          }}
        />
      )}
    </div>
  );
}

function VNetDetail({ vnet, onClose }: { vnet: string; onClose: () => void }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const subnets = useQuery({ queryKey: ["sdn-subnets", vnet], queryFn: () => api.sdnSubnets(vnet) });
  const ips = useQuery({ queryKey: ["sdn-vnet-ips", vnet], queryFn: () => api.sdnVNetIPs(vnet) });
  const [addingSubnet, setAddingSubnet] = useState(false);
  const [addingIp, setAddingIp] = useState(false);
  const [subnet, setSubnet] = useState("");
  const [gateway, setGateway] = useState("");
  const [snat, setSnat] = useState(false);
  const [dhcp, setDhcp] = useState("");
  const [dhcpDns, setDhcpDns] = useState("");
  const [ipAddr, setIpAddr] = useState("");
  const [ipMac, setIpMac] = useState("");
  const [del, setDel] = useState<SDNSubnet | null>(null);
  const [edit, setEdit] = useState<SDNSubnet | null>(null);
  const [editGw, setEditGw] = useState("");
  const [editSnat, setEditSnat] = useState(false);
  const [editDhcp, setEditDhcp] = useState("");
  const [editDns, setEditDns] = useState("");

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="font-medium">{vnet}</h2>
        <Button size="sm" variant="ghost" onClick={onClose}>
          Close
        </Button>
      </div>
      <InventoryCard
        title="Subnets"
        actions={writable ? <BarButton onClick={() => { setSubnet(""); setGateway(""); setDhcp(""); setDhcpDns(""); setSnat(false); setAddingSubnet(true); }}>Add</BarButton> : undefined}
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
            {!(subnets.data ?? []).length && <InventoryEmpty>No subnets</InventoryEmpty>}
            {(subnets.data ?? []).map((s) => (
              <InventoryRow
                key={s.subnet}
                kind={s.subnet}
                summary={joinSummary([
                  s.gateway && `gw ${s.gateway}`,
                  s["dhcp-range"] && `dhcp ${s["dhcp-range"]}`,
                  s["dhcp-dns-server"] && `dns ${s["dhcp-dns-server"]}`,
                  s.snat && "SNAT",
                  !!s.pending && "pending",
                ])}
                onEdit={writable ? () => {
                  setEdit(s);
                  setEditGw(s.gateway ?? "");
                  setEditSnat(!!s.snat);
                  setEditDhcp(String(s["dhcp-range"] ?? ""));
                  setEditDns(s["dhcp-dns-server"] ?? "");
                } : undefined}
              >
                {writable && (
                  <>
                    <Button size="sm" variant="outline" onClick={() => {
                      setEdit(s);
                      setEditGw(s.gateway ?? "");
                      setEditSnat(!!s.snat);
                      setEditDhcp(String(s["dhcp-range"] ?? ""));
                      setEditDns(s["dhcp-dns-server"] ?? "");
                    }}>Edit</Button>
                    <Button size="sm" variant="danger" onClick={() => setDel(s)}>Delete</Button>
                  </>
                )}
              </InventoryRow>
            ))}
          </tbody>
        </table>
      </InventoryCard>
      {ips.data?.supported && (
        <InventoryCard
          title="IP mappings"
          actions={writable ? <BarButton onClick={() => { setIpAddr(""); setIpMac(""); setAddingIp(true); }}>Add</BarButton> : undefined}
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
              {!(ips.data.items ?? []).length && <InventoryEmpty>No IP mappings</InventoryEmpty>}
              {(ips.data.items ?? []).map((item, i) => {
                const ip = String(item.ip || item.v4 || "");
                return (
                  <InventoryRow
                    key={ip || i}
                    kind={ip || "—"}
                    summary={joinSummary([item.mac != null && String(item.mac), item.vmid != null && `VM ${item.vmid}`])}
                  >
                    {writable && ip && (
                      <Button
                        size="sm"
                        variant="danger"
                        onClick={async () => {
                          await api.deleteSDNVNetIP(vnet, ip, item.mac ? String(item.mac) : undefined);
                          void qc.invalidateQueries({ queryKey: ["sdn-vnet-ips", vnet] });
                        }}
                      >
                        Delete
                      </Button>
                    )}
                  </InventoryRow>
                );
              })}
            </tbody>
          </table>
        </InventoryCard>
      )}
      <h3 className="text-sm font-medium">Firewall</h3>
      <VNetFirewall vnet={vnet} />
      {addingSubnet && (
        <Modal
          title="Add subnet"
          onClose={() => setAddingSubnet(false)}
          footer={
            <Button onClick={async () => {
              const body: Record<string, unknown> = { subnet };
              if (gateway) body.gateway = gateway;
              if (snat) body.snat = 1;
              if (dhcp) body["dhcp-range"] = dhcp;
              if (dhcpDns) body["dhcp-dns-server"] = dhcpDns;
              await api.createSDNSubnet(vnet, body);
              setSubnet("");
              setGateway("");
              setDhcp("");
              setDhcpDns("");
              setAddingSubnet(false);
              void qc.invalidateQueries({ queryKey: ["sdn-subnets", vnet] });
            }}>Save</Button>
          }
        >
          <Field label="Subnet"><Input value={subnet} onChange={(e) => setSubnet(e.target.value)} placeholder="10.10.0.0/24" /></Field>
          <Field label="Gateway"><Input value={gateway} onChange={(e) => setGateway(e.target.value)} /></Field>
          <Field label="DHCP range"><Input value={dhcp} onChange={(e) => setDhcp(e.target.value)} placeholder="start,end" /></Field>
          <Field label="DHCP DNS"><Input value={dhcpDns} onChange={(e) => setDhcpDns(e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={snat} onChange={(e) => setSnat(e.target.checked)} />
            SNAT
          </label>
        </Modal>
      )}
      {addingIp && (
        <Modal
          title="Add IP"
          onClose={() => setAddingIp(false)}
          footer={
            <Button onClick={async () => {
              const res = await api.createSDNVNetIP(vnet, { ip: ipAddr, ...(ipMac ? { mac: ipMac } : {}) });
              if (res.supported === false) return;
              setIpAddr("");
              setIpMac("");
              setAddingIp(false);
              void qc.invalidateQueries({ queryKey: ["sdn-vnet-ips", vnet] });
            }}>Save</Button>
          }
        >
          <Field label="IP"><Input value={ipAddr} onChange={(e) => setIpAddr(e.target.value)} /></Field>
          <Field label="MAC"><Input value={ipMac} onChange={(e) => setIpMac(e.target.value)} /></Field>
        </Modal>
      )}
      {edit && (
        <Modal
          title={`Edit subnet ${edit.subnet}`}
          onClose={() => setEdit(null)}
          footer={
            <Button
              onClick={async () => {
                const body: Record<string, unknown> = {
                  gateway: editGw,
                  snat: editSnat ? 1 : 0,
                };
                if (editDhcp) body["dhcp-range"] = editDhcp;
                if (editDns) body["dhcp-dns-server"] = editDns;
                await api.updateSDNSubnet(vnet, edit.subnet, body);
                setEdit(null);
                void qc.invalidateQueries({ queryKey: ["sdn-subnets", vnet] });
              }}
            >
              Save
            </Button>
          }
        >
          <Field label="Gateway"><Input value={editGw} onChange={(e) => setEditGw(e.target.value)} /></Field>
          <Field label="DHCP range"><Input value={editDhcp} onChange={(e) => setEditDhcp(e.target.value)} placeholder="start,end" /></Field>
          <Field label="DHCP DNS"><Input value={editDns} onChange={(e) => setEditDns(e.target.value)} /></Field>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={editSnat} onChange={(e) => setEditSnat(e.target.checked)} />
            SNAT
          </label>
        </Modal>
      )}
      {del && (
        <ConfirmDialog
          title={`Delete subnet ${del.subnet}?`}
          body="Removes the subnet after the next apply."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            await api.deleteSDNSubnet(vnet, del.subnet);
            setDel(null);
            void qc.invalidateQueries({ queryKey: ["sdn-subnets", vnet] });
          }}
        />
      )}
    </div>
  );
}

function IPAMTab() {
  const [open, setOpen] = useState<string | null>(null);
  const status = useQuery({
    queryKey: ["sdn-ipam-status", open],
    queryFn: () => api.sdnIPAMStatus(open!),
    enabled: !!open,
  });
  return (
    <div className="space-y-4">
      <PluginTab
        title="IPAM"
        hint="pve, netbox, or phpipam. Open a plugin for status."
        idKey="ipam"
        listKey="sdn-ipams"
        list={api.sdnIPAMs}
        create={api.createSDNIPAM}
        update={api.updateSDNIPAM}
        remove={api.deleteSDNIPAM}
        defaults={{ type: "pve" }}
        types={["pve", "netbox", "phpipam"]}
        kind="ipam"
        onOpen={setOpen}
      />
      {open && (
        <Card className="space-y-2 p-4">
          <div className="flex items-center justify-between">
            <h2 className="font-medium">{open} status</h2>
            <Button size="sm" variant="ghost" onClick={() => setOpen(null)}>
              Close
            </Button>
          </div>
          {status.data?.supported === false ? (
            <p className="text-sm text-ink-dim">IPAM status is not available on this cluster.</p>
          ) : (
            <KeyValueTable data={asRecord(status.data?.item)} keys={["type", "free", "used", "total"]} />
          )}
        </Card>
      )}
    </div>
  );
}

function pluginFields(kind: "controller" | "ipam" | "dns", type: string): string[] {
  if (kind === "controller") return type === "bgp" ? ["node", "asn", "peers", "ebgp", "loopback"] : ["asn", "peers"];
  if (kind === "ipam") {
    if (type === "netbox") return ["url", "token"];
    if (type === "phpipam") return ["url", "token", "section"];
    return [];
  }
  return ["url", "key", "ttl"];
}

function PluginTab({
  title,
  hint,
  idKey,
  listKey,
  list,
  create,
  update,
  remove,
  defaults,
  types,
  kind,
  onOpen,
}: {
  title: string;
  hint: string;
  idKey: string;
  listKey: string;
  list: () => Promise<FeatureList>;
  create: (body: Record<string, unknown>) => Promise<unknown>;
  update: (id: string, body: Record<string, unknown>) => Promise<unknown>;
  remove: (id: string) => Promise<unknown>;
  defaults: Record<string, string>;
  types: string[];
  kind: "controller" | "ipam" | "dns";
  onOpen?: (id: string) => void;
}) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const data = useQuery({ queryKey: [listKey], queryFn: list });
  const [adding, setAdding] = useState(false);
  const [id, setId] = useState("");
  const [type, setType] = useState(defaults.type ?? types[0] ?? "");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [edit, setEdit] = useState<Record<string, unknown> | null>(null);
  const [editFields, setEditFields] = useState<Record<string, string>>({});
  const [editType, setEditType] = useState("");

  const set = (key: string, value: string) => setFields((prev) => ({ ...prev, [key]: value }));

  function bodyOf(pluginType: string, values: Record<string, string>, includeId?: string): Record<string, unknown> {
    const body: Record<string, unknown> = { type: pluginType };
    if (includeId) body[idKey] = includeId;
    for (const key of pluginFields(kind, pluginType)) {
      if (values[key]) body[key] = key === "ttl" || key === "asn" ? Number(values[key]) : values[key];
    }
    return body;
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">{hint}</p>
      <FeatureTable
        title={title}
        actions={writable ? <BarButton onClick={() => { setId(""); setFields({}); setAdding(true); }}>Add</BarButton> : undefined}
        list={data.data}
        idKey={idKey}
        writable={writable}
        renderDetails={(item) => {
          const pluginType = String(item.type ?? "");
          const keys = ["type", ...pluginFields(kind, pluginType).filter((k) => k !== "token" && k !== "key"), "comment"].slice(0, 4);
          return <ItemDetails item={item} keys={keys} />;
        }}
        onOpen={onOpen}
        onEdit={(item) => {
          const pluginType = String(item.type ?? type);
          setEdit(item);
          setEditType(pluginType);
          const next: Record<string, string> = {};
          for (const key of pluginFields(kind, pluginType)) {
            if (item[key] !== undefined) next[key] = String(item[key] ?? "");
          }
          setEditFields(next);
        }}
        onDelete={async (name) => {
          await remove(name);
          void qc.invalidateQueries({ queryKey: [listKey] });
        }}
      />
      {adding && (
        <Modal
          title={`Add ${title.toLowerCase()}`}
          onClose={() => setAdding(false)}
          footer={
            <Button onClick={async () => {
              await create(bodyOf(type, fields, id));
              setId("");
              setFields({});
              setAdding(false);
              void qc.invalidateQueries({ queryKey: [listKey] });
            }}>Save</Button>
          }
        >
          <Field label={idKey}><Input value={id} onChange={(e) => setId(e.target.value)} /></Field>
          <Field label="Type">
            <Select value={type} onChange={(e) => setType(e.target.value)}>
              {types.map((t) => <option key={t}>{t}</option>)}
            </Select>
          </Field>
          {pluginFields(kind, type).map((key) => (
            <Field key={key} label={key}>
              <Input type={key === "token" || key === "key" ? "password" : "text"} value={fields[key] ?? ""} onChange={(e) => set(key, e.target.value)} />
            </Field>
          ))}
        </Modal>
      )}
      {edit && (
        <Modal
          title={`Edit ${String(edit[idKey] ?? "")}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              await update(String(edit[idKey] ?? ""), bodyOf(editType, editFields));
              setEdit(null);
              void qc.invalidateQueries({ queryKey: [listKey] });
            }}>Save</Button>
          }
        >
          <Field label="Type">
            <Select value={editType} onChange={(e) => setEditType(e.target.value)}>
              {types.map((t) => <option key={t}>{t}</option>)}
            </Select>
          </Field>
          {pluginFields(kind, editType).map((key) => (
            <Field key={key} label={key}>
              <Input
                type={key === "token" || key === "key" ? "password" : "text"}
                value={editFields[key] ?? ""}
                onChange={(e) => setEditFields((prev) => ({ ...prev, [key]: e.target.value }))}
              />
            </Field>
          ))}
        </Modal>
      )}
    </div>
  );
}

function FabricsTab() {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const fabrics = useQuery({ queryKey: ["sdn-fabrics"], queryFn: api.sdnFabrics });
  const [id, setId] = useState("");
  const [protocol, setProtocol] = useState("openfabric");
  const [open, setOpen] = useState<string | null>(null);
  const [edit, setEdit] = useState<Record<string, unknown> | null>(null);
  const [editProtocol, setEditProtocol] = useState("openfabric");

  if (fabrics.data && !fabrics.data.supported) {
    return <p className="text-sm text-ink-dim">Fabrics are not available on this cluster.</p>;
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">OpenFabric and OSPF fabrics, nodes, route maps, and prefix lists.</p>
      {writable && (
        <div className="flex flex-wrap gap-2">
          <Input className="max-w-xs" placeholder="fabric id" value={id} onChange={(e) => setId(e.target.value)} />
          <Select className="max-w-[12rem]" value={protocol} onChange={(e) => setProtocol(e.target.value)}>
            <option value="openfabric">openfabric</option>
            <option value="ospf">ospf</option>
          </Select>
          <Button
            size="sm"
            onClick={async () => {
              await api.createSDNFabric({ id, protocol });
              setId("");
              void qc.invalidateQueries({ queryKey: ["sdn-fabrics"] });
            }}
          >
            Add
          </Button>
        </div>
      )}
      <FeatureTable
        list={fabrics.data}
        idKey="id"
        writable={writable}
        detailKeys={["protocol", "ip", "comment"]}
        onOpen={setOpen}
        onEdit={(item) => {
          setEdit(item);
          setEditProtocol(String(item.protocol ?? "openfabric"));
        }}
        onDelete={async (name) => {
          await api.deleteSDNFabric(name);
          if (open === name) setOpen(null);
          void qc.invalidateQueries({ queryKey: ["sdn-fabrics"] });
        }}
      />
      {open && <FabricDetail id={open} />}
      {edit && (
        <Modal
          title={`Edit fabric ${String(edit.id ?? "")}`}
          onClose={() => setEdit(null)}
          footer={
            <Button onClick={async () => {
              await api.updateSDNFabric(String(edit.id ?? ""), { protocol: editProtocol });
              setEdit(null);
              void qc.invalidateQueries({ queryKey: ["sdn-fabrics"] });
            }}>Save</Button>
          }
        >
          <Field label="Protocol">
            <Select value={editProtocol} onChange={(e) => setEditProtocol(e.target.value)}>
              <option value="openfabric">openfabric</option>
              <option value="ospf">ospf</option>
            </Select>
          </Field>
        </Modal>
      )}
    </div>
  );
}

function FabricDetail({ id }: { id: string }) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const nodes = useQuery({ queryKey: ["sdn-fabric-nodes", id], queryFn: () => api.sdnFabricNodes(id) });
  const maps = useQuery({ queryKey: ["sdn-fabric-maps", id], queryFn: () => api.sdnFabricRouteMaps(id) });
  const lists = useQuery({ queryKey: ["sdn-fabric-pfx", id], queryFn: () => api.sdnFabricPrefixLists(id) });
  const [node, setNode] = useState("");
  const [editNode, setEditNode] = useState<Record<string, unknown> | null>(null);
  const [nodeIfaces, setNodeIfaces] = useState("");
  const [nodeIP, setNodeIP] = useState("");
  const [ifaceNode, setIfaceNode] = useState("");
  const [mapName, setMapName] = useState("");
  const [pfx, setPfx] = useState("");
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const clusterNodes = (summary.data?.resources ?? []).filter((r) => r.type === "node");
  const currentIface = ifaceNode || clusterNodes[0]?.node || "";
  const ifaces = useQuery({
    queryKey: ["sdn-fabric-ifaces", id, currentIface],
    queryFn: () => api.sdnFabricInterfaces(currentIface, id),
    enabled: !!currentIface,
  });

  return (
    <Card className="space-y-4 p-4">
      <h2 className="font-medium">{id}</h2>
      {nodes.data?.supported !== false && (
        <NestedList
          title="Nodes"
          list={nodes.data}
          idKey="node"
          writable={writable}
          detailKeys={["node", "interfaces", "ip"]}
          value={node}
          onChange={setNode}
          onAdd={async () => {
            await api.createSDNFabricNode(id, { node });
            setNode("");
            void qc.invalidateQueries({ queryKey: ["sdn-fabric-nodes", id] });
          }}
          onEdit={(item) => {
            setEditNode(item);
            setNodeIfaces(String(item.interfaces ?? item.interface ?? ""));
            setNodeIP(String(item.ip ?? item.ip4 ?? ""));
          }}
          onDelete={async (name) => {
            await api.deleteSDNFabricNode(id, name);
            void qc.invalidateQueries({ queryKey: ["sdn-fabric-nodes", id] });
          }}
        />
      )}
      {maps.data?.supported && (
        <NestedList
          title="Route maps"
          list={maps.data}
          idKey="name"
          writable={writable}
          value={mapName}
          onChange={setMapName}
          onAdd={async () => {
            await api.createSDNFabricRouteMap(id, { name: mapName });
            setMapName("");
            void qc.invalidateQueries({ queryKey: ["sdn-fabric-maps", id] });
          }}
          onDelete={async (name) => {
            await api.deleteSDNFabricRouteMap(id, name);
            void qc.invalidateQueries({ queryKey: ["sdn-fabric-maps", id] });
          }}
        />
      )}
      {ifaces.data && (
        <div className="space-y-2">
          <div className="flex flex-wrap items-end gap-2">
            <Field label="Interfaces node">
              <Select className="w-48" value={currentIface} onChange={(e) => setIfaceNode(e.target.value)}>
                {clusterNodes.map((n) => (
                  <option key={n.node} value={n.node}>
                    {n.node}
                  </option>
                ))}
              </Select>
            </Field>
          </div>
          {ifaces.data.supported === false ? (
            <p className="text-sm text-ink-dim">Fabric interfaces are not available on this node.</p>
          ) : (
            <FabricIfaceTable items={ifaces.data.items ?? []} />
          )}
        </div>
      )}
      {lists.data?.supported && (
        <NestedList
          title="Prefix lists"
          list={lists.data}
          idKey="name"
          writable={writable}
          value={pfx}
          onChange={setPfx}
          onAdd={async () => {
            await api.createSDNFabricPrefixList(id, { name: pfx });
            setPfx("");
            void qc.invalidateQueries({ queryKey: ["sdn-fabric-pfx", id] });
          }}
          onDelete={async (name) => {
            await api.deleteSDNFabricPrefixList(id, name);
            void qc.invalidateQueries({ queryKey: ["sdn-fabric-pfx", id] });
          }}
        />
      )}
      {editNode && (
        <Modal
          title={`Edit node ${String(editNode.node ?? editNode.id ?? "")}`}
          onClose={() => setEditNode(null)}
          footer={
            <Button onClick={async () => {
              const name = String(editNode.node ?? editNode.id ?? "");
              const body: Record<string, unknown> = {};
              if (nodeIfaces) body.interfaces = nodeIfaces;
              if (nodeIP) body.ip = nodeIP;
              await api.updateSDNFabricNode(id, name, body);
              setEditNode(null);
              void qc.invalidateQueries({ queryKey: ["sdn-fabric-nodes", id] });
            }}>Save</Button>
          }
        >
          <Field label="Interfaces"><Input value={nodeIfaces} onChange={(e) => setNodeIfaces(e.target.value)} placeholder="eth0,eth1" /></Field>
          <Field label="IP"><Input value={nodeIP} onChange={(e) => setNodeIP(e.target.value)} placeholder="10.0.0.1" /></Field>
        </Modal>
      )}
    </Card>
  );
}

function NestedList({
  title,
  list,
  idKey,
  writable,
  value,
  onChange,
  onAdd,
  onDelete,
  onEdit,
  detailKeys,
}: {
  title: string;
  list?: FeatureList;
  idKey: string;
  writable: boolean;
  value: string;
  onChange: (v: string) => void;
  onAdd: () => Promise<void>;
  onDelete: (id: string) => Promise<void>;
  onEdit?: (item: Record<string, unknown>) => void;
  detailKeys?: string[];
}) {
  return (
    <div className="space-y-2">
      <h3 className="text-sm font-medium">{title}</h3>
      {writable && (
        <div className="flex gap-2">
          <Input className="max-w-xs" value={value} onChange={(e) => onChange(e.target.value)} />
          <Button size="sm" onClick={() => void onAdd()}>
            Add
          </Button>
        </div>
      )}
      {(list?.items ?? []).map((item, i) => {
        const name = String(item[idKey] || item.name || item.id || i);
        return (
          <div key={name} className="flex items-center justify-between text-sm">
            <ItemDetails item={item} keys={detailKeys ?? [idKey, "interfaces", "ip", "comment"]} extra={name} />
            <span className="space-x-2">
              {writable && onEdit && (
                <Button size="sm" variant="outline" onClick={() => onEdit(item)}>
                  Open
                </Button>
              )}
              {writable && (
                <Button size="sm" variant="danger" onClick={() => void onDelete(name)}>
                  Delete
                </Button>
              )}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function FeatureTable({
  title,
  actions,
  list,
  idKey,
  writable,
  onDelete,
  onOpen,
  onEdit,
  detailKeys,
  renderDetails,
}: {
  title?: string;
  actions?: ReactNode;
  list?: FeatureList;
  idKey: string;
  writable: boolean;
  onDelete: (id: string) => Promise<void>;
  onOpen?: (id: string) => void;
  onEdit?: (item: Record<string, unknown>) => void;
  detailKeys?: string[];
  renderDetails?: (item: Record<string, unknown>) => ReactNode;
}) {
  return (
    <InventoryCard title={title || "Items"} actions={actions}>
      <table className="w-full text-sm">
        <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
          <tr>
            <th className="px-4 py-2">Kind</th>
            <th>Summary</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {!(list?.items ?? []).length && (
            <tr className="border-t border-line">
              <td className="px-4 py-6 text-sm text-ink-dim" colSpan={3}>None</td>
            </tr>
          )}
          {(list?.items ?? []).map((item, i) => {
            const id = String(item[idKey] || item.name || item.id || i);
            return (
              <tr
                key={id}
                className={`border-t border-line ${onEdit ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
                onClick={onEdit ? () => onEdit(item) : undefined}
              >
                <td className="px-4 py-2 font-medium">{id}</td>
                <td className="max-w-xl px-0 py-2 text-ink-dim">
                  {renderDetails ? renderDetails(item) : <ItemDetails item={item} keys={detailKeys} />}
                </td>
                <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
                  {onOpen && (
                    <Button size="sm" variant="outline" onClick={() => onOpen(id)}>
                      Open
                    </Button>
                  )}
                  {writable && onEdit && (
                    <Button size="sm" variant="outline" onClick={() => onEdit(item)}>
                      Edit
                    </Button>
                  )}
                  {writable && (
                    <Button size="sm" variant="danger" onClick={() => void onDelete(id)}>
                      Delete
                    </Button>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </InventoryCard>
  );
}

function SDNStatusTable({ data }: { data: Record<string, unknown> }) {
  const rows = Array.isArray(data.index) ? asRecords(data.index) : asRecords(data).filter((r) => r.zone || r.vnet || r.status);
  const zoned = rows.length > 0 && rows.some((r) => r.zone || r.vnet);
  if (zoned) {
    return (
      <div className="space-y-2">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Zone</th><th>VNet</th><th>Status</th><th>Detail</th></tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={String(row.zone || row.vnet || i)} className="border-t border-line">
                <td className="px-4 py-2">{String(row.zone ?? "—")}</td>
                <td>{String(row.vnet ?? "—")}</td>
                <td>{String(row.status ?? row.state ?? "—")}</td>
                <td className="text-ink-dim">{String(row.statusmsg ?? row.message ?? row.reason ?? "—")}</td>
              </tr>
            ))}
          </tbody>
        </table>
        <Advanced label="Advanced: raw status">
          <CollapsedRaw data={data} />
        </Advanced>
      </div>
    );
  }
  if (rows.length) {
    return (
      <div className="space-y-2">
        <InferTable rows={rows} />
        <Advanced label="Advanced: raw status">
          <CollapsedRaw data={data} />
        </Advanced>
      </div>
    );
  }
  return (
    <div className="space-y-2">
      <KeyValueTable data={data} />
      <Advanced label="Advanced: raw status">
        <CollapsedRaw data={data} />
      </Advanced>
    </div>
  );
}

function SDNDryRunBlocks({ data }: { data: Record<string, unknown> }) {
  const labels = ["frr-diff", "interfaces-diff", ...Object.keys(data).filter((k) => k.endsWith("-diff") && k !== "frr-diff" && k !== "interfaces-diff")];
  const shown = labels.filter((k) => data[k] != null);
  if (!shown.length) return <KeyValueTable data={data} />;
  return (
    <div className="space-y-3">
      {shown.map((k) => (
        <div key={k}>
          <div className="text-xs uppercase tracking-wide text-ink-dim">{k}</div>
          <pre className="max-h-48 overflow-auto font-mono text-xs text-ink-dim">{String(data[k])}</pre>
        </div>
      ))}
      <Advanced label="Advanced: raw dry-run">
        <CollapsedRaw data={data} />
      </Advanced>
    </div>
  );
}

function zoneFields(type: string): string[] {
  const common = ["mtu", "nodes", "ipam", "dns"];
  if (type === "simple" || type === "vlan") return ["bridge", "vlan-protocol", ...common];
  if (type === "qinq") return ["bridge", "vlan-protocol", ...common];
  if (type === "vxlan") return ["peers", ...common];
  return ["controller", "exitnodes", ...common];
}

function zoneBody(type: string, fields: Record<string, string>): Record<string, unknown> {
  const body: Record<string, unknown> = { type };
  for (const [k, v] of Object.entries(fields)) {
    if (v === "") continue;
    body[k] = k === "mtu" ? Number(v) : v;
  }
  return body;
}

function RouteMapsTab() {
  return (
    <OrderedListTab
      title="Route maps"
      hint="Global SDN route maps. Add ordered entries, then Apply."
      listKey="sdn-route-maps"
      list={api.sdnRouteMaps}
      create={api.createSDNRouteMap}
      remove={api.deleteSDNRouteMap}
      entriesKey="sdn-route-map-entries"
      entries={api.sdnRouteMapEntries}
      createEntry={api.createSDNRouteMapEntry}
      updateEntry={api.updateSDNRouteMapEntry}
      removeEntry={api.deleteSDNRouteMapEntry}
      entryFields={[
        { key: "order", label: "Order", placeholder: "10" },
        { key: "action", label: "Action", placeholder: "permit" },
      ]}
    />
  );
}

function PrefixListsTab() {
  return (
    <OrderedListTab
      title="Prefix lists"
      hint="Global SDN prefix lists. Add ordered entries, then Apply."
      listKey="sdn-prefix-lists"
      list={api.sdnPrefixLists}
      create={api.createSDNPrefixList}
      update={api.updateSDNPrefixList}
      remove={api.deleteSDNPrefixList}
      entriesKey="sdn-prefix-list-entries"
      entries={api.sdnPrefixListEntries}
      createEntry={api.createSDNPrefixListEntry}
      updateEntry={api.updateSDNPrefixListEntry}
      removeEntry={api.deleteSDNPrefixListEntry}
      entryFields={[
        { key: "seq", label: "Seq", placeholder: "10" },
        { key: "action", label: "Action", placeholder: "permit" },
        { key: "prefix", label: "Prefix", placeholder: "10.0.0.0/8" },
      ]}
    />
  );
}

function OrderedListTab({
  title,
  hint,
  listKey,
  list,
  create,
  update,
  remove,
  entriesKey,
  entries,
  createEntry,
  updateEntry,
  removeEntry,
  entryFields,
}: {
  title: string;
  hint: string;
  listKey: string;
  list: () => Promise<FeatureList>;
  create: (body: Record<string, unknown>) => Promise<unknown>;
  update?: (id: string, body: Record<string, unknown>) => Promise<unknown>;
  remove: (id: string) => Promise<unknown>;
  entriesKey: string;
  entries: (id: string) => Promise<FeatureList>;
  createEntry: (id: string, body: Record<string, unknown>) => Promise<unknown>;
  updateEntry?: (id: string, key: string, body: Record<string, unknown>) => Promise<unknown>;
  removeEntry: (id: string, key: string) => Promise<unknown>;
  entryFields: { key: string; label: string; placeholder: string }[];
}) {
  const qc = useQueryClient();
  const can = useCan();
  const writable = can("/sdn", "SDN.Allocate");
  const data = useQuery({ queryKey: [listKey], queryFn: list });
  const [id, setId] = useState("");
  const [open, setOpen] = useState<string | null>(null);
  const [entry, setEntry] = useState<Record<string, string>>({});
  const [editList, setEditList] = useState<Record<string, unknown> | null>(null);
  const [editComment, setEditComment] = useState("");
  const [editEntry, setEditEntry] = useState<string | null>(null);
  const rows = useQuery({
    queryKey: [entriesKey, open],
    queryFn: () => entries(open!),
    enabled: !!open,
  });

  if (data.data && !data.data.supported) {
    return <p className="text-sm text-ink-dim">{title} are not available on this cluster.</p>;
  }

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">{hint}</p>
      {writable && (
        <div className="flex flex-wrap gap-2">
          <Input className="max-w-xs" placeholder="id" value={id} onChange={(e) => setId(e.target.value)} />
          <Button
            size="sm"
            onClick={async () => {
              await create({ id });
              setId("");
              void qc.invalidateQueries({ queryKey: [listKey] });
            }}
          >
            Add
          </Button>
        </div>
      )}
      <FeatureTable
        list={data.data}
        idKey="id"
        writable={writable}
        onOpen={setOpen}
        onEdit={update ? (item) => {
          setEditList(item);
          setEditComment(String(item.comment ?? ""));
        } : undefined}
        onDelete={async (name) => {
          await remove(name);
          if (open === name) setOpen(null);
          void qc.invalidateQueries({ queryKey: [listKey] });
        }}
      />
      {editList && update && (
        <Modal
          title={`Edit ${String(editList.id ?? "")}`}
          onClose={() => setEditList(null)}
          footer={
            <Button onClick={async () => {
              await update(String(editList.id ?? ""), { comment: editComment });
              setEditList(null);
              void qc.invalidateQueries({ queryKey: [listKey] });
            }}>Save</Button>
          }
        >
          <Field label="Comment"><Input value={editComment} onChange={(e) => setEditComment(e.target.value)} /></Field>
        </Modal>
      )}
      {open && (
        <Card className="space-y-3 p-4">
          <h2 className="font-medium">Entries for {open}</h2>
          {writable && (
            <div className="flex flex-wrap gap-2">
              {entryFields.map((f) => (
                <Field key={f.key} label={f.label}>
                  <Input
                    className="max-w-xs"
                    placeholder={f.placeholder}
                    value={entry[f.key] ?? ""}
                    onChange={(e) => setEntry({ ...entry, [f.key]: e.target.value })}
                  />
                </Field>
              ))}
              <Button
                size="sm"
                onClick={async () => {
                  const body: Record<string, unknown> = {};
                  for (const f of entryFields) {
                    if (entry[f.key]) body[f.key] = f.key === "order" || f.key === "seq" ? Number(entry[f.key]) : entry[f.key];
                  }
                  await createEntry(open, body);
                  setEntry({});
                  void qc.invalidateQueries({ queryKey: [entriesKey, open] });
                }}
              >
                Add entry
              </Button>
            </div>
          )}
          <table className="w-full text-sm">
            <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
              <tr><th className="px-2 py-1">Order</th><th>Action</th><th>Prefix</th><th></th></tr>
            </thead>
            <tbody>
              {(rows.data?.items ?? []).map((item, i) => {
                const key = String(item.order ?? item.seq ?? item.url_seq ?? i);
                return (
                  <tr key={key} className="border-t border-line">
                    <td className="px-2 py-1 font-mono text-xs">{key}</td>
                    <td>{String(item.action ?? item.match ?? "—")}</td>
                    <td className="font-mono text-xs">{String(item.prefix ?? item.cidr ?? item.ip ?? "—")}</td>
                    <td className="space-x-2 text-right">
                      {writable && updateEntry && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            const next: Record<string, string> = {};
                            for (const f of entryFields) {
                              next[f.key] = String(item[f.key] ?? item.prefix ?? "");
                            }
                            setEntry(next);
                            setEditEntry(key);
                          }}
                        >
                          Edit
                        </Button>
                      )}
                      {writable && (
                        <Button
                          size="sm"
                          variant="danger"
                          onClick={async () => {
                            await removeEntry(open, key);
                            void qc.invalidateQueries({ queryKey: [entriesKey, open] });
                          }}
                        >
                          Delete
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          {rows.data?.supported === false && <p className="text-sm text-ink-dim">Entries are not available.</p>}
        </Card>
      )}
      {editEntry && open && updateEntry && (
        <Modal
          title={`Edit entry ${editEntry}`}
          onClose={() => setEditEntry(null)}
          footer={
            <Button onClick={async () => {
              const body: Record<string, unknown> = {};
              for (const f of entryFields) {
                if (f.key === "order" || f.key === "seq") continue;
                if (entry[f.key]) body[f.key] = entry[f.key];
              }
              await updateEntry(open, editEntry, body);
              setEditEntry(null);
              setEntry({});
              void qc.invalidateQueries({ queryKey: [entriesKey, open] });
            }}>Save</Button>
          }
        >
          {entryFields.filter((f) => f.key !== "order" && f.key !== "seq").map((f) => (
            <Field key={f.key} label={f.label}>
              <Input
                value={entry[f.key] ?? ""}
                onChange={(e) => setEntry({ ...entry, [f.key]: e.target.value })}
                placeholder={f.placeholder}
              />
            </Field>
          ))}
        </Modal>
      )}
    </div>
  );
}
