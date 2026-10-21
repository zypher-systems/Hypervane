import { useEffect, useState } from "react";

export type ToastKind = "success" | "error" | "info";

export type Toast = {
  id: number;
  kind: ToastKind;
  message: string;
};

let nextId = 1;

export function toast(message: string, kind: ToastKind = "success") {
  window.dispatchEvent(new CustomEvent("hypervane:toast", { detail: { id: nextId++, kind, message } }));
}

export function useToasts(): { toasts: Toast[]; dismiss: (id: number) => void } {
  const [toasts, setToasts] = useState<Toast[]>([]);
  useEffect(() => {
    const onToast = (ev: Event) => {
      const t = (ev as CustomEvent<Toast>).detail;
      if (!t?.message) return;
      setToasts((cur) => [...cur.slice(-4), t]);
      window.setTimeout(() => setToasts((cur) => cur.filter((x) => x.id !== t.id)), t.kind === "error" ? 7000 : 4000);
    };
    window.addEventListener("hypervane:toast", onToast);
    return () => window.removeEventListener("hypervane:toast", onToast);
  }, []);
  return { toasts, dismiss: (id) => setToasts((cur) => cur.filter((x) => x.id !== id)) };
}
