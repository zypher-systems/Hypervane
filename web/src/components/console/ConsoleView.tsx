import { getActiveConnection } from "@/lib/connection";
import { consoleUrl } from "@/lib/consoleUrl";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import { useEffect, useRef, useState } from "react";
import { SpiceView } from "./SpiceView";

export function ConsoleView({
  node,
  vmid,
  type,
  kind,
  cmd,
  grant,
  onReady,
  onClosed,
}: {
  node: string;
  vmid?: number;
  type?: string;
  kind: "vnc" | "term" | "spice";
  cmd?: "upgrade";
  grant?: string;
  /** Terminals: called once connected, with a function that types text into it. */
  onReady?: (type: (text: string) => void) => void;
  /** Terminals: called when the session ends. */
  onClosed?: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [error, setError] = useState("");
  const url = wsAbs(consoleUrl({ node, vmid, type, kind, cmd, grant, connection: getActiveConnection() }));
  // Read through a ref, so a new callback doesn't reconnect the console.
  const hooks = useRef({ onReady, onClosed });
  hooks.current = { onReady, onClosed };

  useEffect(() => {
    if (kind === "spice") return;
    const el = ref.current;
    if (!el) return;
    let alive = true;
    setError("");

    if (kind === "term") {
      const term = new Terminal({
        cursorBlink: true,
        fontFamily: "IBM Plex Mono, monospace",
        fontSize: 13,
        theme: { background: "#09090b", foreground: "#e4e4e7", cursor: "#e85a24" },
      });
      const fit = new FitAddon();
      term.loadAddon(fit);
      term.open(el);
      const ws = new WebSocket(url);
      ws.binaryType = "arraybuffer";
      const sendResize = () => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(`1:${term.cols}:${term.rows}:`);
        }
      };
      const ready = () => {
        if (!alive) return;
        fit.fit();
        sendResize();
        term.focus();
      };
      const typeText = (text: string) => {
        if (ws.readyState !== WebSocket.OPEN) return;
        ws.send(`0:${new TextEncoder().encode(text).length}:${text}`);
        term.focus();
      };
      ws.onopen = () => {
        ready();
        hooks.current.onReady?.(typeText);
      };
      ws.onerror = () => {
        if (alive) setError("Console socket failed.");
      };
      ws.onclose = (ev) => {
        if (!alive) return;
        hooks.current.onClosed?.();
        if (ev.code === 1000 || ev.code === 1001 || ev.code === 1005) return;
        setError("Console disconnected.");
      };
      ws.onmessage = (ev) => {
        if (typeof ev.data === "string") term.write(ev.data);
        else term.write(new Uint8Array(ev.data as ArrayBuffer));
      };
      term.onData((d) => {
        if (ws.readyState !== WebSocket.OPEN) return;
        const n = new TextEncoder().encode(d).length;
        ws.send(`0:${n}:${d}`);
      });
      term.onResize(() => sendResize());
      const ping = window.setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) ws.send("2");
      }, 30000);
      const ro = new ResizeObserver(() => fit.fit());
      ro.observe(el);
      const fitTimer = window.setTimeout(ready, 50);
      return () => {
        alive = false;
        window.clearTimeout(fitTimer);
        window.clearInterval(ping);
        ro.disconnect();
        ws.close();
        term.dispose();
      };
    }

    let cleanup = () => {};
    void import("@novnc/novnc").then((mod) => {
      if (!alive) return;
      const RFB = mod.default;
      const rfb = new RFB(el, url);
      rfb.scaleViewport = true;
      rfb.resizeSession = true;
      rfb.showDotCursor = true;
      rfb.background = "#09090b";
      rfb.addEventListener?.("securityfailure", () => {
        if (alive) setError("VNC authentication failed.");
      });
      rfb.addEventListener?.("credentialsrequired", () => {
        if (alive) setError("Console handshake failed.");
      });
      rfb.addEventListener?.("disconnect", (ev: { detail?: { clean?: boolean } }) => {
        if (alive && !ev.detail?.clean) setError("Console disconnected.");
      });
      cleanup = () => rfb.disconnect();
    }).catch((e: unknown) => {
      if (alive) setError(e instanceof Error ? e.message : "VNC client failed to load");
    });
    return () => {
      alive = false;
      cleanup();
    };
  }, [node, vmid, type, kind, url]);

  if (kind === "spice") {
    return <SpiceView url={url} />;
  }

  return (
    <div className={`flex h-[65vh] min-h-[360px] flex-col overflow-hidden rounded-xl border border-white/15 bg-black ${kind === "vnc" ? "hv-vnc" : ""}`}>
      {error && <p className="shrink-0 px-3 py-2 text-xs text-rose-400">{error}</p>}
      <div ref={ref} className="min-h-0 flex-1" />
    </div>
  );
}

function wsAbs(path: string) {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  return `${proto}://${location.host}${path}`;
}
