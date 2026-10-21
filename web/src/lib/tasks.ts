export type TaskQueryKey = readonly unknown[];

type Pending = { keys: TaskQueryKey[] };

const pending = new Map<string, Pending>();

export function fleetKeys(): TaskQueryKey[] {
  return [["summary"], ["inventory"]];
}

export function guestKeys(vmid: number): TaskQueryKey[] {
  return [["guest", vmid], ...fleetKeys()];
}

export function snapKeys(vmid: number): TaskQueryKey[] {
  return [["snaps", vmid], ["guest", vmid]];
}

export function nodeKeys(node: string): TaskQueryKey[] {
  return [["node", node], ...fleetKeys()];
}

export function attachTask(upid?: string, keys?: TaskQueryKey[]) {
  if (!upid) return;
  if (keys?.length) {
    const first = !pending.has(upid);
    const cur = pending.get(upid) ?? { keys: [] };
    cur.keys.push(...keys);
    pending.set(upid, cur);
    if (first && typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("hypervane:toast", {
        detail: { id: Date.now(), kind: "info", message: "Task started" },
      }));
    }
  }
  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("hypervane:task", { detail: upid }));
  }
}

export function openTasks(upid?: string) {
  window.dispatchEvent(new CustomEvent("hypervane:tasks-open", { detail: upid ?? "" }));
}

export function hasPendingTask(upid: string) {
  return pending.has(upid);
}

/** Returns registered keys and clears the watcher. Null if this UPID was not ours. */
export function finishTask(upid: string): TaskQueryKey[] | null {
  const cur = pending.get(upid);
  if (!cur) return null;
  pending.delete(upid);
  return cur.keys;
}

export function isTaskFinished(t: { status?: string; endtime?: number; exitstatus?: string }) {
  if (t.exitstatus) return true;
  if (t.endtime) return true;
  const status = (t.status || "").toLowerCase();
  return status !== "" && status !== "running";
}
