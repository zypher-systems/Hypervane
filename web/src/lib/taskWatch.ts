import { finishTask, hasPendingTask, isTaskFinished } from "@/lib/tasks";
import { toast } from "@/lib/toast";
import { live } from "@/lib/ws";
import type { Task } from "@/types";
import type { QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef } from "react";

type TaskExit = { status?: string; exitstatus?: string };

export function useTaskCompletion(qc: QueryClient, tasks: Task[]) {
  const watched = useRef(new Set<string>());

  const settle = useCallback(
    (upid: string, info: TaskExit) => {
      const keys = finishTask(upid);
      if (keys == null) return;
      if (watched.current.has(upid)) {
        live.unsubscribe([`task:${upid}`]);
        watched.current.delete(upid);
      }
      for (const key of keys) {
        void qc.invalidateQueries({ queryKey: [...key] });
      }
      const exit = (info.exitstatus || info.status || "").trim();
      if (exit && exit !== "OK" && exit.toLowerCase() !== "stopped") {
        toast(exit, "error");
        return;
      }
      toast("Task finished", "success");
    },
    [qc],
  );

  useEffect(() => {
    const off = live.on((topic, data) => {
      if (!topic.startsWith("task:")) return;
      const upid = topic.slice("task:".length);
      const payload = data as { status?: TaskExit };
      if (!isTaskFinished(payload.status ?? {})) return;
      settle(upid, payload.status ?? {});
    });
    const onTask = (ev: Event) => {
      const upid = (ev as CustomEvent<string>).detail;
      if (!upid || !hasPendingTask(upid) || watched.current.has(upid)) return;
      watched.current.add(upid);
      live.subscribe([`task:${upid}`]);
    };
    window.addEventListener("hypervane:task", onTask);
    return () => {
      off();
      window.removeEventListener("hypervane:task", onTask);
      if (watched.current.size) {
        live.unsubscribe([...watched.current].map((u) => `task:${u}`));
        watched.current.clear();
      }
    };
  }, [settle]);

  useEffect(() => {
    for (const t of tasks) {
      if (isTaskFinished(t)) settle(t.upid, t);
    }
  }, [tasks, settle]);
}
