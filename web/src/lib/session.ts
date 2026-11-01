import { clearSelection } from "@/lib/selection";
import { clearPendingTasks } from "@/lib/tasks";
import type { QueryClient } from "@tanstack/react-query";

/** Drops everything held for the signed-in account. */
export function resetClientState(qc: QueryClient) {
  qc.clear();
  clearSelection();
  clearPendingTasks();
}
