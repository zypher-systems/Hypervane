import { cn } from "@/lib/utils";

/** Neutral until usage needs attention: amber from 80%, red from 95%. */
export function meterFill(pct: number | null | undefined): string {
  if (pct == null) return "bg-white/20";
  if (pct >= 95) return "bg-danger";
  if (pct >= 80) return "bg-warn";
  return "bg-meter";
}

export function meterText(pct: number | null | undefined): string {
  if (pct == null) return "text-ink-dim";
  if (pct >= 95) return "text-danger";
  if (pct >= 80) return "text-warn";
  return "text-ink";
}

export function Meter({ label, used, total, text }: { label: string; used?: number; total?: number; text: string }) {
  const pctVal = total && total > 0 ? Math.min(100, ((used ?? 0) / total) * 100) : 0;
  return (
    <div>
      <div className="mb-1 flex items-center justify-between gap-3 text-xs">
        <span className="text-ink-dim">{label}</span>
        <span className="font-mono text-ink-dim">{text}</span>
      </div>
      <div className="h-1 overflow-hidden rounded-full bg-white/[0.08]">
        <div className={cn("h-full rounded-full", meterFill(total ? pctVal : null))} style={{ width: `${pctVal}%` }} />
      </div>
    </div>
  );
}
