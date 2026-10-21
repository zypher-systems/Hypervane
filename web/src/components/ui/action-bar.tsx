import { Button } from "@/components/ui/button";
import { ChevronDown } from "lucide-react";
import { useEffect, useRef, useState, type ReactNode } from "react";

export const BAR_BTN = "border-white/[0.12] bg-transparent text-ink hover:bg-raised";

export function BarButton({
  children,
  accent,
  selected,
  ...props
}: { children: ReactNode; accent?: boolean; selected?: boolean } & Omit<Parameters<typeof Button>[0], "size" | "variant" | "children">) {
  return (
    <Button
      size="sm"
      variant={accent ? "primary" : selected ? "selected" : "outline"}
      aria-pressed={selected}
      className={accent || selected ? undefined : BAR_BTN}
      {...props}
    >
      {children}
    </Button>
  );
}

export type MoreItem =
  | { id: string; label: string; heading: true }
  | { id: string; label: string; danger?: boolean; heading?: false; onClick: () => void };

export function MoreMenu({ items, label = "More" }: { items: MoreItem[]; label?: string }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [open]);
  if (!items.length) return null;
  return (
    <div ref={ref} className="relative">
      <BarButton onClick={() => setOpen((v) => !v)}>
        {label}
        <ChevronDown className="h-3.5 w-3.5" />
      </BarButton>
      {open && (
        <div className="hv-glass absolute right-0 z-30 mt-1 min-w-[12rem] rounded-md border border-white/15 py-1 shadow-lg">
          {items.map((item) =>
            item.heading ? (
              <p key={item.id} className="px-3 pb-1 pt-2 text-xs font-medium text-ink-dim">
                {item.label}
              </p>
            ) : (
              <button
                key={item.id}
                type="button"
                onClick={() => {
                  setOpen(false);
                  item.onClick();
                }}
                className={`flex w-full px-3 py-1.5 text-left text-xs ${
                  item.danger ? "text-rose-300 hover:bg-rose-500/10" : "text-ink hover:bg-white/[0.04]"
                }`}
              >
                {item.label}
              </button>
            ),
          )}
        </div>
      )}
    </div>
  );
}
