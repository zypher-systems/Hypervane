import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest, formatHostPCI, nextSlot } from "@/lib/guestHardware";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useState } from "react";

export function PciDialog({
  vmid, node, type, config, onClose, onSaved,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>;
  onClose: () => void; onSaved: () => void;
}) {
  const maps = useQuery({ queryKey: ["maps", "pci"], queryFn: () => api.mappings("pci").catch(() => ({ supported: false, items: [] })) });
  const hw = useQuery({ queryKey: ["hw", node, "pci"], queryFn: () => api.nodeHardware(node, "pci"), enabled: !!node });
  const [source, setSource] = useState<"mapping" | "host">("mapping");
  const [mapping, setMapping] = useState("");
  const [host, setHost] = useState("");
  const [pcie, setPcie] = useState(true);
  const [xvga, setXvga] = useState(false);
  const [rombar, setRombar] = useState(true);
  const [mdev, setMdev] = useState("");
  const [error, setError] = useState("");
  const slot = nextSlot(config, "hostpci", 15);
  const mapItems = maps.data?.items ?? [];
  const hosts = hw.data?.items ?? [];
  const pciID = source === "mapping" ? mapping : host;
  const mdevs = useQuery({
    queryKey: ["mdev", node, pciID],
    queryFn: () => api.nodeMdev(node, pciID),
    enabled: !!node && !!pciID,
  });
  useEffect(() => {
    setMdev("");
  }, [pciID]);
  const mdevTypes = mdevs.data?.supported === true ? (mdevs.data.items ?? []) : [];

  return (
    <Modal
      title={`Add ${slot}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              if (source === "mapping" && !mapping) throw new Error("Pick a mapping or host device");
              if (source === "host" && !host) throw new Error("Pick a mapping or host device");
              const value = formatHostPCI({
                mapping: source === "mapping" ? mapping : undefined,
                host: source === "host" ? host : undefined,
                pcie, xvga, rombar, mdev: mdev || undefined,
              });
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields: { [slot]: value } }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add PCI failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <div className="flex gap-2">
        <Button size="sm" variant={source === "mapping" ? "selected" : "outline"} onClick={() => setSource("mapping")}>Mapping</Button>
        <Button size="sm" variant={source === "host" ? "selected" : "outline"} onClick={() => setSource("host")}>Host device</Button>
      </div>
      {source === "mapping" ? (
        <Field label="PCI mapping">
          <Select value={mapping} onChange={(e) => setMapping(e.target.value)}>
            <option value="">Select…</option>
            {mapItems.map((item) => {
              const id = String(item.id ?? "");
              return <option key={id} value={id}>{id}</option>;
            })}
          </Select>
        </Field>
      ) : (
        <Field label="Host PCI">
          <Select value={host} onChange={(e) => setHost(e.target.value)}>
            <option value="">Select…</option>
            {hosts.map((item) => {
              const id = String(item.id ?? "");
              const name = String(item.device_name ?? item.vendor_name ?? "");
              return <option key={id} value={id}>{id}{name ? ` ${name}` : ""}</option>;
            })}
          </Select>
        </Field>
      )}
      {!hw.data?.supported && source === "host" && (
        <p className="text-xs text-ink-dim">Host PCI inventory is not available on this node.</p>
      )}
      {mdevTypes.length > 0 && (
        <Field label="Mediated device (vGPU)">
          <Select value={mdev} onChange={(e) => setMdev(e.target.value)}>
            <option value="">None (full device)</option>
            {mdevTypes.map((item) => {
              const id = String(item.type ?? item.id ?? "");
              const name = String(item.description ?? item.name ?? "");
              return <option key={id} value={id}>{id}{name ? ` ${name}` : ""}</option>;
            })}
          </Select>
        </Field>
      )}
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={pcie} onChange={(e) => setPcie(e.target.checked)} /> PCIe</label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={xvga} onChange={(e) => setXvga(e.target.checked)} /> Primary GPU (x-vga)</label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={rombar} onChange={(e) => setRombar(e.target.checked)} /> ROM-Bar</label>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
