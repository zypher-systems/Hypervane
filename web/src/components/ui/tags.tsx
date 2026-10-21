import { cn } from "@/lib/utils";

/** PVE stores tags as a semicolon-separated string. */
export function parseTags(raw?: string): string[] {
  if (!raw) return [];
  return raw
    .split(";")
    .map((t) => t.trim())
    .filter(Boolean);
}

export function formatTags(tags: string[]): string {
  return tags.map((t) => t.trim()).filter(Boolean).join(";");
}

export function TagChip({ tag, className }: { tag: string; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded border border-white/[0.1] px-1.5 py-px font-mono text-[10px] leading-4 text-ink-dim",
        className,
      )}
    >
      {tag}
    </span>
  );
}

export function TagList({
  raw,
  className,
  compact,
}: {
  raw?: string;
  className?: string;
  compact?: boolean;
}) {
  const tags = parseTags(raw);
  if (!tags.length) return null;
  const shown = compact ? tags.slice(0, 2) : tags;
  const extra = tags.length - shown.length;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1",
        compact ? "max-w-[7.5rem] shrink-0 overflow-hidden" : "flex-wrap",
        className,
      )}
      title={compact ? tags.join(", ") : undefined}
    >
      {shown.map((t) => (
        <TagChip key={t} tag={t} className={compact ? "max-w-[3.25rem] truncate" : undefined} />
      ))}
      {extra > 0 && <span className="font-mono text-[10px] text-ink-faint">+{extra}</span>}
    </span>
  );
}
