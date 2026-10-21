import type { Resource, Task } from "@/types";

export type GuestActivity = "backup" | "migrate" | "clone";

const TASK_ACTIVITY: Record<string, GuestActivity> = {
  vzdump: "backup",
  qmigrate: "migrate",
  vzmigrate: "migrate",
  qmclone: "clone",
  vzclone: "clone",
};

const LOCK_ACTIVITY: Record<string, GuestActivity> = {
  backup: "backup",
  migrate: "migrate",
  clone: "clone",
};

export function isRunningTask(t: Task) {
  return !t.endtime && (!t.status || t.status === "running");
}

export function guestActivityTask(guest: Resource, tasks?: Task[]): Task | undefined {
  if (guest.vmid == null) return undefined;
  const id = String(guest.vmid);
  return (tasks ?? []).find((t) => isRunningTask(t) && t.id === id && TASK_ACTIVITY[t.type]);
}

export function guestActivity(guest: Resource, tasks?: Task[]): GuestActivity | undefined {
  const running = guestActivityTask(guest, tasks);
  if (running) return TASK_ACTIVITY[running.type];
  const lock = guest.lock?.toLowerCase();
  return lock ? LOCK_ACTIVITY[lock] : undefined;
}
