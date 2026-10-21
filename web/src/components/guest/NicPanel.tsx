import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest, formatNet, type NicForm } from "@/lib/guestHardware";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

export function NicDialog({
  vmid, node, type, config, nic, isNew, onClose, onSaved,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>;
  nic: NicForm; isNew: boolean; onClose: () => void; onSaved: () => void;
}) {
  const ifaces = useQuery({ queryKey: ["node-net", node], queryFn: () => api.nodeNetwork(node), enabled: !!node });
  const vnets = useQuery({ queryKey: ["sdn-vnets"], queryFn: () => api.sdnVNets().catch(() => []) });
  const [form, setForm] = useState(nic);
  const [error, setError] = useState("");
  const bridges = useMemo(() => {
    const fromNode = (ifaces.data ?? [])
      .filter((n) => n.type === "bridge" || n.type === "OVSBridge")
      .map((n) => n.iface);
    const fromSdn = (vnets.data ?? []).map((v) => v.vnet).filter(Boolean);
    return [...new Set([...fromNode, ...fromSdn, form.bridge].filter(Boolean))];
  }, [ifaces.data, vnets.data, form.bridge]);

  return (
    <Modal
      title={isNew ? `Add ${form.slot}` : `Edit ${form.slot}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              await api.updateGuest(vmid, {
                node, type, digest: configDigest(config), fields: { [form.slot]: formatNet(form, type) },
              }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save NIC failed");
            }
          }}
        >
          Save
        </Button>
      }
    >
      <div className="grid gap-3 md:grid-cols-2">
        {type === "lxc" ? (
          <Field label="Name"><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} /></Field>
        ) : (
          <Field label="Model">
            <Select value={form.model} onChange={(e) => setForm({ ...form, model: e.target.value })}>
              <option value="virtio">virtio</option>
              <option value="e1000">e1000</option>
              <option value="e1000e">e1000e</option>
              <option value="rtl8139">rtl8139</option>
              <option value="vmxnet3">vmxnet3</option>
            </Select>
          </Field>
        )}
        <Field label="Bridge">
          <Select value={form.bridge} onChange={(e) => setForm({ ...form, bridge: e.target.value })}>
            {bridges.map((b) => <option key={b}>{b}</option>)}
          </Select>
        </Field>
        <Field label="VLAN tag"><Input value={form.tag} onChange={(e) => setForm({ ...form, tag: e.target.value })} /></Field>
        <Field label="Trunks"><Input value={form.trunks} onChange={(e) => setForm({ ...form, trunks: e.target.value })} placeholder="10;20" /></Field>
        <Field label="Rate (MB/s)"><Input value={form.rate} onChange={(e) => setForm({ ...form, rate: e.target.value })} /></Field>
        <Field label="MTU"><Input value={form.mtu} onChange={(e) => setForm({ ...form, mtu: e.target.value })} /></Field>
        <Field label={type === "lxc" ? "MAC" : "MAC address"}><Input value={form.mac} onChange={(e) => setForm({ ...form, mac: e.target.value })} /></Field>
        {type === "lxc" && (
          <>
            <Field label="IPv4"><Input value={form.ip} onChange={(e) => setForm({ ...form, ip: e.target.value })} placeholder="dhcp or 10.0.0.2/24" /></Field>
            <Field label="Gateway"><Input value={form.gw} onChange={(e) => setForm({ ...form, gw: e.target.value })} /></Field>
            <Field label="IPv6"><Input value={form.ip6} onChange={(e) => setForm({ ...form, ip6: e.target.value })} /></Field>
            <Field label="IPv6 gateway"><Input value={form.gw6} onChange={(e) => setForm({ ...form, gw6: e.target.value })} /></Field>
          </>
        )}
      </div>
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={form.firewall} onChange={(e) => setForm({ ...form, firewall: e.target.checked })} />
        Firewall
      </label>
      {type === "qemu" && (
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={form.linkDown} onChange={(e) => setForm({ ...form, linkDown: e.target.checked })} />
          Link down
        </label>
      )}
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
