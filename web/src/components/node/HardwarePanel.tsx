import { InventoryCard, InventoryEmpty, InventoryRow, joinSummary } from "@/components/ui/inventory";
import { api } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";

function field(item: Record<string, unknown>, ...keys: string[]): string {
  for (const key of keys) {
    const value = item[key];
    if (value !== undefined && value !== null && String(value) !== "") return String(value);
  }
  return "";
}

function vendorDevice(item: Record<string, unknown>, vendorKeys: string[], deviceKeys: string[]) {
  const vendor = field(item, ...vendorKeys);
  const device = field(item, ...deviceKeys);
  if (vendor && device) return `${vendor}/${device}`;
  return vendor || device;
}

export function HardwarePanel({ node }: { node: string }) {
  const pci = useQuery({ queryKey: ["node-hw-pci", node], queryFn: () => api.nodeHardware(node, "pci") });
  const usb = useQuery({ queryKey: ["node-hw-usb", node], queryFn: () => api.nodeHardware(node, "usb") });
  const pciOn = pci.data?.supported !== false;
  const usbOn = usb.data?.supported !== false;

  if (pci.data && usb.data && !pciOn && !usbOn) {
    return <p className="text-sm text-ink-dim">Hardware inventory is not available on this node.</p>;
  }

  const pciRows = pciOn ? (pci.data?.items ?? []).map((item, i) => {
    const id = field(item, "id", "pci", "map") || String(i);
    return {
      key: `pci:${id}`,
      kind: "PCI",
      summary: joinSummary([id, vendorDevice(item, ["vendor_name", "vendor", "subsystem_vendor"], ["device_name", "device", "class"])]),
    };
  }) : [];
  const usbRows = usbOn ? (usb.data?.items ?? []).map((item, i) => {
    const vend = field(item, "vendid", "vendor");
    const prod = field(item, "prodid", "product_id");
    const id = vend && prod ? `${vend}:${prod}` : field(item, "id") || String(i);
    return {
      key: `usb:${id}`,
      kind: "USB",
      summary: joinSummary([id, vendorDevice(item, ["manufacturer", "vendor_name"], ["product", "name"])]),
    };
  }) : [];
  const rows = [...pciRows, ...usbRows];
  const message = [
    pci.data && !pciOn ? "PCI inventory is not available." : "",
    usb.data && !usbOn ? "USB inventory is not available." : "",
  ].filter(Boolean).join(" ");

  return (
    <InventoryCard title="Hardware" message={message || undefined}>
      <table className="w-full text-sm">
        <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
          <tr>
            <th className="px-4 py-2">Kind</th>
            <th>Summary</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {!rows.length && <InventoryEmpty>No PCI or USB devices</InventoryEmpty>}
          {rows.map((row) => (
            <InventoryRow key={row.key} kind={row.kind} summary={row.summary} />
          ))}
        </tbody>
      </table>
    </InventoryCard>
  );
}
