import { InventoryAside } from "@/components/tree/InventoryAside";
import { CommandPalette, openCommandPalette } from "@/components/layout/CommandPalette";
import { isDatacenterPath } from "@/components/layout/DatacenterNav";
import { TreeBulkBar } from "@/components/tree/TreeBulkBar";
import { TaskDrawer } from "@/components/tasks/TaskDrawer";
import { Button } from "@/components/ui/button";
import { ToastViewport } from "@/components/ui/toast";
import { api } from "@/lib/api";
import { getActiveConnection, onActiveConnection, setActiveConnection } from "@/lib/connection";
import { clearSelection } from "@/lib/selection";
import { focusInventorySearch } from "@/lib/inventorySearch";
import { useTaskCompletion } from "@/lib/taskWatch";
import { live } from "@/lib/ws";
import type { InventoryGroup, Task } from "@/types";
import { Link, Outlet, useNavigate, useRouter, useRouterState } from "@tanstack/react-router";
import { useCan, hasWritePrivilege } from "@/lib/perms";
import type { Permissions } from "@/lib/perms";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Plus, Search } from "lucide-react";
import { useEffect, useState } from "react";

export function AppShell({
  username,
  kind,
  permissions,
  tokenScoped,
  clusterConfigured,
}: {
  username: string;
  kind: string;
  permissions: Permissions;
  tokenScoped: boolean;
  clusterConfigured: boolean;
}) {
  const nav = useNavigate();
  const router = useRouter();
  const readOnly = tokenScoped && !hasWritePrivilege(permissions);
  const qc = useQueryClient();
  const can = useCan();
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const [activeConnection, setActive] = useState(getActiveConnection);
  const showCreate = can("/", "VM.Allocate");
  const [groups, setGroups] = useState<InventoryGroup[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [activeUpid, setActiveUpid] = useState<string | null>(null);
  useTaskCompletion(qc, tasks);
  const connections = useQuery({ queryKey: ["connections"], queryFn: api.connections });
  const inventory = useQuery({ queryKey: ["inventory"], queryFn: api.inventory, refetchInterval: 15000 });

  useEffect(() => {
    if (kind === "local" && !clusterConfigured && pathname !== "/settings") {
      void nav({ to: "/settings" });
    }
  }, [kind, clusterConfigured, pathname, nav]);

  useEffect(() => {
    const list = connections.data ?? [];
    if (!list.length) return;
    const current = getActiveConnection();
    if (!current || !list.some((c) => c.id === current)) {
      setActiveConnection(list[0].id);
      setActive(list[0].id);
    }
  }, [connections.data]);

  useEffect(() => {
    return onActiveConnection((id) => {
      setActive(id);
      clearSelection();
      void qc.invalidateQueries();
      // Each connection has its own token, so re-read what this one may do.
      void router.invalidate();
    });
  }, [qc, router]);

  useEffect(() => {
    live.start();
    live.subscribe(["resources", "tasks"]);
    const off = live.on((topic, data) => {
      if (topic === "resources") {
        const payload = data as { connection?: string; resources?: InventoryGroup["resources"]; status?: InventoryGroup["status"] };
        if (!payload.resources || !payload.connection) return;
        setGroups((cur) =>
          cur.map((g) => (g.id === payload.connection ? { ...g, resources: payload.resources ?? [], status: payload.status } : g)),
        );
      }
      if (topic === "tasks") {
        const payload = Array.isArray(data) ? { tasks: data as Task[] } : (data as { connection?: string; tasks?: Task[] });
        if (payload.connection && payload.connection !== getActiveConnection()) return;
        if (payload.tasks) setTasks(payload.tasks);
      }
    });
    void api.tasks().then(setTasks).catch(() => {});
    const onTask = (ev: Event) => {
      const upid = (ev as CustomEvent<string>).detail;
      if (upid) setActiveUpid(upid);
    };
    window.addEventListener("hypervane:task", onTask);
    return () => {
      off();
      live.unsubscribe(["resources", "tasks"]);
      window.removeEventListener("hypervane:task", onTask);
    };
  }, []);

  useEffect(() => {
    void api.tasks().then(setTasks).catch(() => {});
  }, [activeConnection]);

  useEffect(() => {
    if (inventory.data?.connections) setGroups(inventory.data.connections);
  }, [inventory.data]);

  const onDatacenter = isDatacenterPath(pathname);
  const onTop = pathname === "/top";
  const onRecipes = pathname === "/recipes" || pathname === "/scripts";
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "/" || e.ctrlKey || e.metaKey || e.altKey) return;
      const tag = (e.target as HTMLElement | null)?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || (e.target as HTMLElement | null)?.isContentEditable) return;
      e.preventDefault();
      focusInventorySearch();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className="flex h-full flex-col bg-canvas">
      <header className="hv-glass flex h-12 items-center justify-between border-b px-4">
        <div className="flex items-center gap-4">
          <Link to="/" className="flex items-center gap-2.5">
            <img src="/hypervane.svg" alt="" className="h-8 w-8" />
            <span className="text-sm font-semibold tracking-tight">Hypervane</span>
          </Link>
          <nav className="flex items-center gap-1">
            <Link
              to="/"
              className={`rounded-md px-2.5 py-1 text-sm ${
                onDatacenter ? "bg-white/[0.06] text-ink" : "text-ink-dim hover:bg-white/[0.04] hover:text-ink"
              }`}
            >
              Datacenter
            </Link>
            <Link
              to="/top"
              className={`rounded-md px-2.5 py-1 text-sm ${
                onTop ? "bg-white/[0.06] text-ink" : "text-ink-dim hover:bg-white/[0.04] hover:text-ink"
              }`}
            >
              Top 10
            </Link>
            <Link
              to="/recipes"
              className={`rounded-md px-2.5 py-1 text-sm ${
                onRecipes ? "bg-white/[0.06] text-ink" : "text-ink-dim hover:bg-white/[0.04] hover:text-ink"
              }`}
            >
              Recipes
            </Link>
            {kind === "local" && (
              <Link
                to="/settings"
                className={`rounded-md px-2.5 py-1 text-sm ${
                  pathname === "/settings" ? "bg-white/[0.06] text-ink" : "text-ink-dim hover:bg-white/[0.04] hover:text-ink"
                }`}
              >
                Settings
              </Link>
            )}
          </nav>
        </div>
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={openCommandPalette}
            className="hidden h-8 w-52 items-center gap-2 rounded-md border border-white/[0.1] px-2.5 text-xs text-ink-dim transition hover:bg-raised hover:text-ink sm:flex"
          >
            <Search className="h-3.5 w-3.5" />
            Search or jump to…
            <kbd className="ml-auto rounded border border-white/[0.12] px-1 font-mono text-[10px]">⌘K</kbd>
          </button>
          {showCreate && (
            <Button size="sm" onClick={() => nav({ to: "/create", search: {} })}>
              <Plus className="h-3.5 w-3.5" />
              Create guest
            </Button>
          )}
          {(connections.data ?? []).length > 0 && (
            <select
              aria-label="Active connection"
              value={activeConnection}
              disabled={kind === "pve"}
              onChange={(e) => setActiveConnection(e.target.value)}
              className="h-8 max-w-[12rem] rounded-md border border-white/15 bg-black/30 px-2 text-xs text-ink"
            >
              {(connections.data ?? []).map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          )}
          <span className="font-mono text-[11px] text-ink-dim">{username}</span>
          <Button
            size="sm"
            variant="ghost"
            onClick={async () => {
              await api.logout();
              nav({ to: "/login" });
            }}
          >
            Sign out
          </Button>
        </div>
      </header>
      {readOnly && (
        <div className="border-b border-line bg-white/[0.02] px-4 py-1.5 text-xs text-ink-dim">
          Read-only connection: this API token can view the cluster but not change it, so actions are hidden.
          {kind === "local" && (
            <>
              {" "}
              <Link to="/settings" className="text-ink underline decoration-white/30 underline-offset-2 hover:decoration-white">
                Connection settings
              </Link>
            </>
          )}
        </div>
      )}
      <div className="flex min-h-0 flex-1">
        <InventoryAside groups={groups} tasks={tasks} taskConnection={activeConnection} onSelectConnection={setActiveConnection} />
        <main className="min-w-0 flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
      <TreeBulkBar nodes={groups.flatMap((g) => (g.resources ?? []).filter((r) => r.type === "node"))} />
      <ToastViewport />
      <CommandPalette groups={groups} />
      <TaskDrawer
        tasks={tasks}
        activeUpid={activeUpid}
        onSelect={(upid) => {
          setActiveUpid(upid);
          window.dispatchEvent(new CustomEvent("hypervane:task", { detail: upid }));
        }}
      />
    </div>
  );
}


export { attachTask, openTasks } from "@/lib/tasks";
