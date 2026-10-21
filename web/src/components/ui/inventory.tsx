import { Card } from "@/components/ui/card";
import type { ReactNode } from "react";

export function InventoryCard({
  title,
  actions,
  message,
  children,
}: {
  title: string;
  actions?: ReactNode;
  message?: string;
  children: ReactNode;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-white/15 px-4 py-3">
        <div className="text-xs font-medium text-ink-dim">{title}</div>
        {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
      </div>
      {message && <p className="px-4 pt-3 text-xs text-ink-dim">{message}</p>}
      {children}
    </Card>
  );
}

export function InventoryTable({
  children,
  columns = ["Kind", "Summary"],
}: {
  children: ReactNode;
  columns?: string[];
}) {
  return (
    <table className="w-full text-sm">
      <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
        <tr>
          {columns.map((col, i) => (
            <th key={col} className={i === 0 ? "px-4 py-2" : undefined}>{col}</th>
          ))}
          <th></th>
        </tr>
      </thead>
      <tbody>{children}</tbody>
    </table>
  );
}

export function InventoryEmpty({ children, colSpan = 3 }: { children: ReactNode; colSpan?: number }) {
  return (
    <tr className="border-t border-line">
      <td className="px-4 py-6 text-sm text-ink-dim" colSpan={colSpan}>{children}</td>
    </tr>
  );
}

export function InventoryRow({
  kind,
  summary,
  onEdit,
  children,
}: {
  kind: ReactNode;
  summary: string;
  onEdit?: () => void;
  children?: ReactNode;
}) {
  return (
    <tr
      className={`border-t border-line ${onEdit ? "cursor-pointer hover:bg-white/[0.04]" : ""}`}
      onClick={onEdit}
    >
      <td className="px-4 py-2 font-medium">{kind}</td>
      <td className="max-w-xl truncate font-mono text-xs text-ink-dim" title={summary}>{summary || "—"}</td>
      <td className="space-x-2 px-4 py-2 text-right" onClick={(e) => e.stopPropagation()}>
        {children}
      </td>
    </tr>
  );
}

export function joinSummary(parts: (string | false | number | undefined)[]) {
  return parts.filter((p) => p !== false && p !== undefined && p !== "").map(String).join(" · ");
}
