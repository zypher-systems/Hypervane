import { cn } from "@/lib/utils";
import type { HTMLAttributes, ReactNode } from "react";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn("hv-glass rounded-xl border", className)}
      {...props}
    />
  );
}

export function Badge({
  children,
  tone = "zinc",
}: {
  children: ReactNode;
  tone?: "zinc" | "green" | "amber" | "rose" | "teal";
}) {
  // Status reads from the dot; the pill itself stays neutral so rows don't turn into a color chart.
  const dots = {
    zinc: "",
    green: "bg-ok",
    amber: "bg-warn",
    rose: "bg-danger",
    teal: "",
  };
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-md border border-white/[0.08] px-1.5 py-px text-[11px] font-medium text-ink-dim",
        tone === "amber" && "text-warn",
        tone === "rose" && "text-danger",
      )}
    >
      {dots[tone] && <span className={cn("h-1.5 w-1.5 shrink-0 rounded-full", dots[tone])} />}
      {children}
    </span>
  );
}

export function statusTone(status?: string): "green" | "amber" | "rose" | "zinc" {
  if (status === "running" || status === "online") return "green";
  if (status === "paused" || status === "suspended") return "amber";
  // A stopped guest is a normal state, not an error; an offline node is.
  if (status === "offline") return "rose";
  return "zinc";
}
