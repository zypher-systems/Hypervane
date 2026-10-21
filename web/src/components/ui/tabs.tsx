import { cn } from "@/lib/utils";
import type { ReactNode } from "react";

/** Underline tabs for switching a page's sections. */
export function Tabs<T extends string>({
  items,
  value,
  onChange,
  label,
  className,
}: {
  items: readonly T[];
  value: T;
  onChange: (value: T) => void;
  label?: (item: T) => ReactNode;
  className?: string;
}) {
  return (
    <div role="tablist" className={cn("flex flex-wrap gap-x-6 border-b border-line text-sm", className)}>
      {items.map((t) => {
        const active = t === value;
        return (
          <button
            key={t}
            type="button"
            role="tab"
            aria-selected={active}
            onClick={() => onChange(t)}
            className={cn(
              "-mb-px border-b-2 py-2.5 capitalize transition focus-visible:outline-none focus-visible:text-ink",
              active ? "border-accent font-medium text-ink" : "border-transparent text-ink-dim hover:text-ink",
            )}
          >
            {label ? label(t) : t}
          </button>
        );
      })}
    </div>
  );
}

/** Compact segmented control for a mode or range inside a panel (timeframes, filters). */
export function Segmented<T extends string>({
  items,
  value,
  onChange,
  label,
  className,
  ariaLabel,
}: {
  items: readonly T[];
  value: T;
  onChange: (value: T) => void;
  label?: (item: T) => ReactNode;
  className?: string;
  ariaLabel?: string;
}) {
  return (
    <div role="radiogroup" aria-label={ariaLabel} className={cn("inline-flex rounded-md bg-white/[0.04] p-0.5", className)}>
      {items.map((t) => {
        const active = t === value;
        return (
          <button
            key={t}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(t)}
            className={cn(
              "rounded px-2.5 py-1 text-xs capitalize transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40",
              active ? "bg-raised text-ink shadow-sm" : "text-ink-dim hover:text-ink",
            )}
          >
            {label ? label(t) : t}
          </button>
        );
      })}
    </div>
  );
}
