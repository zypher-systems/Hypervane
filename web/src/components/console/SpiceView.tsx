import { useEffect, useRef } from "react";

type SpiceMainConn = {
  stop?: () => void;
};

export function SpiceView({ url }: { url: string }) {
  const host = useRef<HTMLDivElement>(null);
  const screenId = useRef(`spice-${Math.random().toString(36).slice(2)}`);

  useEffect(() => {
    const el = host.current;
    if (!el) return;
    el.innerHTML = "";
    const screen = document.createElement("div");
    screen.id = screenId.current;
    screen.className = "h-full min-h-[360px] w-full";
    el.appendChild(screen);
    let conn: SpiceMainConn | undefined;
    let cancelled = false;
    void import("@spice-project/spice-html5").then((mod) => {
      if (cancelled) return;
      conn = new mod.SpiceMainConn({
        uri: url,
        screen_id: screenId.current,
        password: "",
      });
    }).catch((err: unknown) => {
      if (!cancelled) {
        screen.textContent = err instanceof Error ? err.message : "SPICE client failed to load";
        screen.className = "p-4 text-sm text-rose-400";
      }
    });
    return () => {
      cancelled = true;
      conn?.stop?.();
    };
  }, [url]);

  return <div ref={host} className="h-full min-h-[360px] overflow-hidden rounded-xl border border-line bg-black" />;
}
