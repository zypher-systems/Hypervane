import { useToasts } from "@/lib/toast";
import { CheckCircle2, CircleAlert, Info, X } from "lucide-react";

const tone: Record<string, string> = {
  success: "border-emerald-500/30 text-emerald-300",
  error: "border-rose-500/30 text-rose-300",
  info: "border-sky-500/30 text-sky-300",
};

const Icon = { success: CheckCircle2, error: CircleAlert, info: Info };

export function ToastViewport() {
  const { toasts, dismiss } = useToasts();
  return (
    <div aria-live="polite" className="pointer-events-none fixed bottom-4 right-4 z-[70] flex w-80 flex-col gap-2">
      {toasts.map((t) => {
        const I = Icon[t.kind];
        return (
          <div
            key={t.id}
            role="status"
            className={`hv-glass pointer-events-none flex items-start gap-2.5 rounded-lg border p-3 shadow-lg ${tone[t.kind]}`}
            style={{ pointerEvents: "auto" }}
          >
            <I className="mt-0.5 h-4 w-4 shrink-0" />
            <p className="min-w-0 flex-1 text-sm text-ink">{t.message}</p>
            <button
              type="button"
              aria-label="Dismiss"
              onClick={() => dismiss(t.id)}
              className="shrink-0 rounded p-0.5 text-ink-dim hover:text-ink"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        );
      })}
    </div>
  );
}
