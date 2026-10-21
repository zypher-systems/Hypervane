import { useState, type ReactNode } from "react";

function fmt(v: unknown): string {
  if (v == null) return "";
  if (Array.isArray(v)) {
    if (!v.length) return "";
    if (v.every((x) => x && typeof x === "object")) return `${v.length} items`;
    return v.map((x) => (x && typeof x === "object" ? JSON.stringify(x) : String(x))).join(", ");
  }
  if (typeof v === "object") return JSON.stringify(v);
  return String(v);
}

export function fieldLine(item: Record<string, unknown>, keys: string[]): string {
  return keys
    .filter((k) => item[k] != null && item[k] !== "")
    .map((k) => `${k} ${fmt(item[k])}`)
    .join(" · ");
}

export function Advanced({
  label = "Advanced",
  defaultOpen,
  children,
}: {
  label?: string;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(!!defaultOpen);
  return (
    <div>
      <button type="button" className="text-xs text-ink-faint underline" onClick={() => setOpen((v) => !v)}>
        {open ? "Hide advanced" : label}
      </button>
      {open && <div className="mt-2 space-y-3">{children}</div>}
    </div>
  );
}

export function CollapsedRaw({ data }: { data: unknown }) {
  return (
    <pre className="max-h-40 max-w-xl overflow-auto font-mono text-xs text-ink-faint">{JSON.stringify(data, null, 2)}</pre>
  );
}

export function ItemDetails({
  item,
  keys = ["type", "server", "comment", "disable"],
  extra,
}: {
  item: Record<string, unknown>;
  keys?: string[];
  extra?: string;
}) {
  const used = new Set(keys);
  const rest = Object.entries(item)
    .filter(([k, v]) => !used.has(k) && k !== "digest" && v != null && v !== "")
    .map(([k, v]) => `${k} ${fmt(v)}`);
  const line = [extra, fieldLine(item, keys), ...rest].filter(Boolean).join(" · ");
  return <div className="text-xs text-ink">{line || "—"}</div>;
}

export function KeyValueTable({ data, keys }: { data: Record<string, unknown>; keys?: string[] }) {
  const rows = (keys ?? Object.keys(data)).filter((k) => k !== "supported" && data[k] != null);
  if (!rows.length) return <p className="px-4 py-2 text-sm text-ink-dim">None.</p>;
  return (
    <table className="w-full text-sm">
      <tbody>
        {rows.map((k) => (
          <tr key={k} className="border-t border-line">
            <td className="px-4 py-1.5 text-xs uppercase text-ink-faint">{k.replace(/_/g, " ")}</td>
            <td className="px-4 py-1.5 font-mono text-xs text-ink">{fmt(data[k])}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function InferTable({ rows }: { rows: Record<string, unknown>[] }) {
  if (!rows.length) return <p className="p-4 text-sm text-ink-dim">None.</p>;
  const cols = Object.keys(rows[0]).filter((k) => typeof rows[0][k] !== "object").slice(0, 6);
  return (
    <table className="w-full text-sm">
      <thead className="bg-panel text-left text-xs text-ink-faint">
        <tr>
          {cols.map((c) => (
            <th key={c} className="px-4 py-2">{c.replace(/_/g, " ")}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, i) => (
          <tr key={i} className="border-t border-line">
            {cols.map((c) => (
              <td key={c} className="px-4 py-2 font-mono text-xs text-ink-dim">{fmt(row[c])}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function asRecord(v: unknown): Record<string, unknown> {
  return v && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : {};
}

export function FabricIfaceTable({ items }: { items: Record<string, unknown>[] }) {
  if (!items.length) return <p className="text-sm text-ink-dim">None.</p>;
  return (
    <table className="w-full text-sm">
      <thead className="bg-panel text-left text-xs text-ink-faint">
        <tr><th className="px-4 py-2">Name</th><th>State</th><th>Type</th></tr>
      </thead>
      <tbody>
        {items.map((item, i) => (
          <tr key={String(item.name || item.id || i)} className="border-t border-line">
            <td className="px-4 py-2">{String(item.name || item.iface || item.id || "—")}</td>
            <td>{String(item.state ?? item.status ?? "—")}</td>
            <td>{String(item.type ?? item.kind ?? "—")}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

export function ImportMetadataView({ data, message }: { data?: Record<string, unknown> | null; message?: string }) {
  if (message) return <p className="text-sm text-ink-dim">{message}</p>;
  if (!data || !Object.keys(data).length) return null;
  const args = asRecord(data["create-args"]);
  const disks = asRecord(data.disks);
  const nets = asRecord(data.nets ?? data.net);
  return (
    <div className="space-y-2">
      {Object.keys(args).length > 0 && <KeyValueTable data={args} keys={["name", "ostype", "cores", "memory", "scsihw", "bios"]} />}
      {Object.keys(disks).length > 0 && (
        <ul className="font-mono text-xs text-ink-dim">
          {Object.entries(disks).map(([slot, src]) => (
            <li key={slot}>{slot} ← {String(src)}</li>
          ))}
        </ul>
      )}
      {Object.keys(nets).length > 0 && (
        <ul className="font-mono text-xs text-ink-dim">
          {Object.entries(nets).map(([slot, raw]) => (
            <li key={slot}>{slot} {typeof raw === "object" ? JSON.stringify(raw) : String(raw)}</li>
          ))}
        </ul>
      )}
      {!Object.keys(args).length && !Object.keys(disks).length && <KeyValueTable data={data} />}
      <Advanced label="Advanced: raw metadata">
        <CollapsedRaw data={data} />
      </Advanced>
    </div>
  );
}

export function asRecords(v: unknown): Record<string, unknown>[] {
  if (Array.isArray(v)) {
    return v.filter((x) => x && typeof x === "object") as Record<string, unknown>[];
  }
  const rec = asRecord(v);
  return Object.entries(rec).map(([name, value]) => {
    const row = asRecord(value);
    return { name, ...row };
  });
}
