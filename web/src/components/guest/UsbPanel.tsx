import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Field, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest, formatUSB, nextSlot } from "@/lib/guestHardware";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

export function UsbDialog({
  vmid, node, type, config, onClose, onSaved,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>;
  onClose: () => void; onSaved: () => void;
}) {
  const maps = useQuery({ queryKey: ["maps", "usb"], queryFn: () => api.mappings("usb").catch(() => ({ supported: false, items: [] })) });
  const hw = useQuery({ queryKey: ["hw", node, "usb"], queryFn: () => api.nodeHardware(node, "usb"), enabled: !!node });
  const [source, setSource] = useState<"mapping" | "host">("mapping");
  const [mapping, setMapping] = useState("");
  const [host, setHost] = useState("");
  const [usb3, setUsb3] = useState(true);
  const [error, setError] = useState("");
  const slot = nextSlot(config, "usb", 14);
  const mapItems = maps.data?.items ?? [];
  const hosts = hw.data?.items ?? [];

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
              const value = formatUSB({
                mapping: source === "mapping" ? mapping : undefined,
                host: source === "host" ? host : undefined,
                usb3,
              });
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields: { [slot]: value } }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add USB failed");
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
        <Field label="USB mapping">
          <Select value={mapping} onChange={(e) => setMapping(e.target.value)}>
            <option value="">Select…</option>
            {mapItems.map((item) => {
              const id = String(item.id ?? "");
              return <option key={id} value={id}>{id}</option>;
            })}
          </Select>
        </Field>
      ) : (
        <Field label="Host USB">
          <Select value={host} onChange={(e) => setHost(e.target.value)}>
            <option value="">Select…</option>
            {hosts.map((item, i) => {
              const vend = String(item.vendid ?? "");
              const prod = String(item.prodid ?? "");
              const id = vend && prod ? `${vend}:${prod}` : String(item.usbpath ?? item.id ?? i);
              const name = String(item.product ?? item.manufacturer ?? "");
              return <option key={`${id}-${i}`} value={id}>{id}{name ? ` ${name}` : ""}</option>;
            })}
          </Select>
        </Field>
      )}
      {!hw.data?.supported && source === "host" && (
        <p className="text-xs text-ink-dim">Host USB inventory is not available on this node.</p>
      )}
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={usb3} onChange={(e) => setUsb3(e.target.checked)} /> USB 3</label>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
