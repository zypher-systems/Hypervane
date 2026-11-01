import { getActiveConnection } from "@/lib/connection";
import { finishTask, hasPendingTask, isTaskFinished } from "@/lib/tasks";
import { toast } from "@/lib/toast";
import { connectionTopic, live, topicDetail } from "@/lib/ws";
import type { Task } from "@/types";
import type { QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef } from "react";

type TaskExit = { status?: string; exitstatus?: string };

export function useTaskCompletion(qc: QueryClient, tasks: Task[]) {
  // UPID to topic. A task stays on the connection it started on, even if the
  // user switches away before it ends.
  const watched = useRef(new Map<string, string>());

  const settle = useCallback(
    (upid: string, info: TaskExit) => {
      const keys = finishTask(upid);
      if (keys == null) return;
      const topic = watched.current.get(upid);
      if (topic) {
        live.unsubscribe([topic]);
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
      const detail = topicDetail(topic);
      if (!detail.startsWith("task:")) return;
      const upid = detail.slice("task:".length);
      const payload = data as { status?: TaskExit };
      if (!isTaskFinished(payload.status ?? {})) return;
      settle(upid, payload.status ?? {});
    });
    const onTask = (ev: Event) => {
      const upid = (ev as CustomEvent<string>).detail;
      const connection = getActiveConnection();
      if (!upid || !connection || !hasPendingTask(upid) || watched.current.has(upid)) return;
      const topic = connectionTopic(connection, `task:${upid}`);
      watched.current.set(upid, topic);
      live.subscribe([topic]);
    };
    window.addEventListener("hypervane:task", onTask);
    return () => {
      off();
      window.removeEventListener("hypervane:task", onTask);
      if (watched.current.size) {
        live.unsubscribe([...watched.current.values()]);
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
