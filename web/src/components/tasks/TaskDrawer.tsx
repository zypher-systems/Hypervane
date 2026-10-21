import { Badge } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { api } from "@/lib/api";
import { live } from "@/lib/ws";
import { relTime } from "@/lib/format";
import type { Task, TaskLogLine } from "@/types";
import { ChevronDown, ChevronUp, ListTodo } from "lucide-react";
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

function isRunning(t: Task) {
  return !t.endtime && (!t.status || t.status === "running");
}

const typeLabels: Record<string, string> = {
  vzdump: "Backup",
  qmstart: "Start",
  qmstop: "Stop",
  qmshutdown: "Shutdown",
  qmreboot: "Reboot",
  qmreset: "Reset",
  qmsuspend: "Suspend",
  qmresume: "Resume",
  qmigrate: "Migrate",
  qmclone: "Clone",
  qmdestroy: "Remove",
  qmcreate: "Create",
  qmsnapshot: "Snapshot",
  vzstart: "Start",
  vzstop: "Stop",
  vzshutdown: "Shutdown",
  vzreboot: "Reboot",
  vzsuspend: "Suspend",
  vzresume: "Resume",
  vzmigrate: "Migrate",
  vzclone: "Clone",
  vzdestroy: "Remove",
  vzcreate: "Create",
  vzsnapshot: "Snapshot",
  startall: "Start all",
  stopall: "Stop all",
  suspendall: "Suspend all",
  migrateall: "Migrate all",
};

function typeLabel(type: string) {
  return typeLabels[type] ?? type;
}

export function TaskDrawer({
  tasks,
  activeUpid,
  onSelect,
}: {
  tasks: Task[];
  activeUpid: string | null;
  onSelect: (upid: string | null) => void;
}) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("all");
  const [log, setLog] = useState<TaskLogLine[]>([]);
  const [logTotal, setLogTotal] = useState(0);
  const [status, setStatus] = useState<string>("");
  const [stop, setStop] = useState(false);
  const logPane = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onOpen = (ev: Event) => {
      const upid = (ev as CustomEvent<string>).detail;
      setOpen(true);
      if (upid) onSelect(upid);
    };
    window.addEventListener("hypervane:tasks-open", onOpen);
    return () => window.removeEventListener("hypervane:tasks-open", onOpen);
  }, [onSelect]);

  useEffect(() => {
    if (!activeUpid) {
      setLog([]);
      setLogTotal(0);
      setStatus("");
      return;
    }
    live.subscribe([`task:${activeUpid}`]);
    const off = live.on((topic, data) => {
      if (topic !== `task:${activeUpid}`) return;
      const payload = data as { status?: { status?: string; exitstatus?: string }; log?: TaskLogLine[]; total?: number };
      setStatus(payload.status?.exitstatus || payload.status?.status || "");
      if (payload.log) setLog(payload.log);
      setLogTotal(payload.total ?? 0);
    });
    return () => {
      off();
      live.unsubscribe([`task:${activeUpid}`]);
    };
  }, [activeUpid]);

  const ordered = useMemo(
    () => [...tasks].sort((a, b) => (b.starttime ?? 0) - (a.starttime ?? 0) || a.upid.localeCompare(b.upid)),
    [tasks],
  );
  const types = useMemo(
    () => [...new Set(ordered.map((t) => t.type).filter(Boolean))].sort((a, b) => typeLabel(a).localeCompare(typeLabel(b))),
    [ordered],
  );
  const visible = useMemo(() => {
    if (filter === "all") return ordered;
    if (filter === "running") return ordered.filter(isRunning);
    return ordered.filter((t) => t.type === filter);
  }, [ordered, filter]);
  const running = ordered.filter(isRunning).length;
  const active = tasks.find((t) => t.upid === activeUpid);
  const canStop = !!active && isRunning(active);
  const filters = [
    { id: "all", label: "All" },
    { id: "running", label: "Running" },
    ...types.map((type) => ({ id: type, label: typeLabel(type) })),
  ];

  useLayoutEffect(() => {
    const el = logPane.current;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  }, [log, activeUpid, status]);

  return (
    <div className="hv-glass border-t">
      <button
        className="flex w-full items-center justify-between px-4 py-2 text-xs text-ink-dim hover:bg-white/[0.04]"
        onClick={() => setOpen((v) => !v)}
      >
        <span className="inline-flex items-center gap-2">
          <ListTodo className="h-3.5 w-3.5 text-ink-dim" />
          Tasks {running ? <Badge tone="teal">{running} running</Badge> : null}
        </span>
        {open ? <ChevronDown className="h-4 w-4" /> : <ChevronUp className="h-4 w-4" />}
      </button>
      {open && (
        <>
          <div className="flex flex-wrap gap-1 border-t border-white/[0.06] px-4 py-2">
            {filters.map((f) => (
              <button
                key={f.id}
                type="button"
                onClick={() => setFilter(f.id)}
                className={`rounded-full px-2.5 py-1 font-mono text-[11px] ${
                  filter === f.id ? "bg-raised text-ink" : "text-ink-dim hover:bg-white/[0.04] hover:text-ink"
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>
          <div className="grid h-48 grid-cols-2 gap-0 border-t border-white/[0.06]">
            <div className="overflow-auto">
              {visible.slice(0, 40).map((t) => (
                <button
                  key={t.upid}
                  onClick={() => onSelect(t.upid)}
                  className={`flex w-full items-center gap-2 px-4 py-1.5 text-left text-xs hover:bg-white/[0.04] ${
                    activeUpid === t.upid ? "bg-white/[0.06] text-ink" : "text-ink"
                  }`}
                >
                  <span className="w-20 truncate font-mono text-ink-dim">{t.node}</span>
                  <span className="w-24 truncate">{typeLabel(t.type)}</span>
                  <span className="flex-1 truncate">{t.id}</span>
                  <span className="w-32 truncate text-ink-dim">{relTime(t.starttime)}</span>
                </button>
              ))}
              {!visible.length && <p className="p-4 text-xs text-ink-dim">No tasks in this filter.</p>}
            </div>
            <div ref={logPane} className="overflow-auto border-l border-white/[0.06] bg-black/30">
              {activeUpid && canStop && (
                <div className="sticky top-0 flex justify-end bg-black/50 px-3 pt-2">
                  <Button size="sm" variant="danger" onClick={() => setStop(true)}>Stop</Button>
                </div>
              )}
              <pre className="p-3 font-mono text-[11px] leading-5 text-ink">
                {activeUpid ? (
                  <>
                    <div className="mb-2 text-ink-dim">{status || "following log…"}</div>
                    {logTotal > log.length && (
                      <div className="mb-2 text-ink-faint">Showing the last {log.length} of {logTotal} lines.</div>
                    )}
                    {log.map((l) => (
                      <div key={l.n}>{l.t}</div>
                    ))}
                  </>
                ) : (
                  <span className="text-ink-dim">Select a task to stream its log.</span>
                )}
              </pre>
            </div>
          </div>
        </>
      )}
      {stop && activeUpid && (
        <ConfirmDialog
          title="Stop this task?"
          body="Asks PVE to abort the running task. In-flight work may be left incomplete."
          danger
          confirmLabel="Stop"
          onClose={() => setStop(false)}
          onConfirm={async () => {
            await api.stopTask(activeUpid, active?.node);
            setStop(false);
          }}
        />
      )}
    </div>
  );
}
