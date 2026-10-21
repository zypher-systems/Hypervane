import { cn } from "@/lib/utils";
import type { ButtonHTMLAttributes } from "react";

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost" | "danger" | "danger-solid" | "outline" | "selected";
  size?: "sm" | "md";
};

export function Button({ className, variant = "primary", size = "md", ...props }: Props) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-md font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 disabled:opacity-40",
        size === "sm" ? "h-8 px-2.5 text-xs" : "h-10 px-3.5 text-sm",
        variant === "primary" && "bg-accent font-semibold text-accent-ink hover:bg-accent/90",
        variant === "ghost" && "text-ink-dim hover:bg-raised hover:text-ink",
        variant === "outline" && "border border-white/[0.1] text-ink hover:bg-raised",
        // The chosen option in a set of toggle buttons: raised, never the accent.
        variant === "selected" && "border border-white/25 bg-raised text-ink",
        // Row-level destructive actions stay quiet; the confirm step uses danger-solid.
        variant === "danger" && "border border-danger/30 text-danger hover:bg-danger/10",
        variant === "danger-solid" && "bg-[#c2473d] font-semibold text-white hover:bg-[#d0544a]",
        className,
      )}
      {...props}
    />
  );
}
