import { openTasks } from "@/lib/tasks";
import { GuestContextMenu } from "@/components/tree/GuestContextMenu";
import { Badge, statusTone } from "@/components/ui/card";
import { TagList } from "@/components/ui/tags";
import { haTone } from "@/types";
import { Input } from "@/components/ui/input";
import { bytes, pct } from "@/lib/format";
import { guestActivity, guestActivityTask } from "@/lib/guestActivity";
import { getInventorySearch, onInventorySearch, setInventorySearch } from "@/lib/inventorySearch";
import { isSelected, subscribe, toggleGuest } from "@/lib/selection";
import type { InventoryGroup, Resource, Task } from "@/types";
import { Link, useRouterState } from "@tanstack/react-router";
import { ChevronRight, HardDrive, PanelLeftClose, RotateCw, Search, Server } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { meterFill, meterText } from "@/components/ui/meter";
import { isDatacenterPath } from "@/components/layout/DatacenterNav";

const collapsedKey = "hypervane:tree-collapsed";

function loadCollapsed(): string[] {
  try {
    const raw = localStorage.getItem(collapsedKey);
    const parsed = raw ? JSON.parse(raw) : [];
    return Array.isArray(parsed) ? parsed.filter((n) => typeof n === "string") : [];
  } catch {
    return [];
  }
}

type InventoryFilter = "running" | "stopped" | "templates" | "storage";
const filterKey = "hv-tree-filter";

function loadFilter(): InventoryFilter | undefined {
  try {
    const v = localStorage.getItem(filterKey);
    return v === "running" || v === "stopped" || v === "templates" || v === "storage" ? v : undefined;
  } catch {
    return undefined;
  }
}

function matchInventory(r: Resource, inventory?: InventoryFilter) {
  if (!inventory) return true;
  if (inventory === "storage") return r.type === "storage";
  if (r.type !== "qemu" && r.type !== "lxc") return false;
  if (inventory === "templates") return !!r.template;
  if (inventory === "running") return !r.template && r.status === "running";
  if (inventory === "stopped") return !r.template && r.status === "stopped";
  return true;
}

export function ResourceTree({
  groups,
  tasks,
  taskConnection,
  onSelectConnection,
  onCollapse,
}: {
  groups: InventoryGroup[];
  tasks?: Task[];
  taskConnection?: string;
  onSelectConnection?: (id: string) => void;
  onCollapse?: () => void;
}) {
  const [q, setQ] = useState(getInventorySearch);
  const [, setSelectedTick] = useState(0);
  const [menu, setMenu] = useState<{ guest: Resource; x: number; y: number; connection: string } | null>(null);
  const [collapsed, setCollapsed] = useState<string[]>(() => loadCollapsed());
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const [inventory, setInventory] = useState<InventoryFilter | undefined>(loadFilter);
  useEffect(() => {
    try {
      if (inventory) localStorage.setItem(filterKey, inventory);
      else localStorage.removeItem(filterKey);
    } catch {
      /* storage unavailable: the filter just won't persist */
    }
  }, [inventory]);
  useEffect(() => {
    localStorage.setItem(collapsedKey, JSON.stringify(collapsed));
  }, [collapsed]);
  useEffect(() => onInventorySearch(setQ), []);
  const searchInput = useRef<HTMLInputElement>(null);
  useEffect(() => {
    // "/" anywhere focuses the tree search (the rail opens itself on the same event).
    const onFocus = () => requestAnimationFrame(() => searchInput.current?.focus());
    window.addEventListener("hypervane:search", onFocus);
    return () => window.removeEventListener("hypervane:search", onFocus);
  }, []);
  useEffect(() => subscribe(() => setSelectedTick((n) => n + 1)), []);

  function toggleNode(id: string) {
    setCollapsed((cur) => (cur.includes(id) ? cur.filter((n) => n !== id) : [...cur, id]));
  }

  const tree = useMemo(() => {
    const query = q.trim().toLowerCase();
    return groups.map((group) => {
      const resources = group.resources ?? [];
      const nodes = resources.filter((r) => r.type === "node").sort((a, b) => (a.node ?? "").localeCompare(b.node ?? "", undefined, { numeric: true }));
      const branches = nodes.map((node) => {
        const children = resources.filter((r) => {
          if (r.node !== node.node) return false;
          if (r.type !== "qemu" && r.type !== "lxc" && r.type !== "storage") return false;
          if (!matchInventory(r, inventory)) return false;
          if (!query) return true;
          return `${r.name ?? ""} ${r.vmid ?? ""} ${r.storage ?? ""} ${r.type}`.toLowerCase().includes(query);
        }).sort((a, b) => {
          if (a.type === "storage" && b.type !== "storage") return 1;
          if (a.type !== "storage" && b.type === "storage") return -1;
          return (a.vmid ?? a.storage ?? "").toString().localeCompare((b.vmid ?? b.storage ?? "").toString(), undefined, { numeric: true });
        });
        return { node, children };
      }).filter((g) => {
        if (inventory) return g.children.length > 0;
        if (q) return g.children.length > 0 || (g.node.node ?? "").toLowerCase().includes(q.toLowerCase());
        return true;
      });
      return { group, branches };
    }).filter((entry) => {
      if (!q && !inventory) return true;
      return entry.branches.length > 0 || entry.group.name.toLowerCase().includes(query);
    });
  }, [groups, q, inventory]);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-1 p-3">
        <div className="relative min-w-0 flex-1">
          <Search className="absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-ink-dim" />
          <Input
            ref={searchInput}
            aria-label="Search the inventory"
            value={q}
            onChange={(e) => setInventorySearch(e.target.value)}
            placeholder="Search guests, nodes…"
            className="h-8 border-white/25 bg-black/20 pl-8 text-xs placeholder:text-ink-dim focus:border-accent/60"
          />
        </div>
        {onCollapse && (
          <button
            type="button"
            title="Hide inventory"
            aria-label="Hide inventory"
            onClick={onCollapse}
            className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-ink-dim hover:bg-white/[0.04] hover:text-ink"
          >
            <PanelLeftClose className="h-3.5 w-3.5" />
          </button>
        )}
      </div>
      <div className="min-h-0 flex-1 overflow-auto px-2 pb-3">
        <div className="flex items-center justify-between gap-2 px-2 pb-1.5">
          <span className="text-xs text-ink-dim">Inventory</span>
          <select
            aria-label="Filter the inventory"
            value={inventory ?? "all"}
            onChange={(e) => setInventory(e.target.value === "all" ? undefined : (e.target.value as InventoryFilter))}
            className={`h-6 rounded border bg-transparent px-1 text-xs ${
              inventory ? "border-accent/50 text-ink" : "border-white/[0.1] text-ink-dim"
            }`}
          >
            <option value="all">All</option>
            <option value="running">Running</option>
            <option value="stopped">Stopped</option>
            <option value="templates">Templates</option>
            <option value="storage">Storage</option>
          </select>
        </div>
        <Link
          to="/"
          className={`mb-2 flex h-8 items-center gap-2 rounded-md px-2 text-sm font-medium ${
            isDatacenterPath(pathname)
              ? "bg-white/[0.06] text-ink shadow-[inset_2px_0_0_rgb(var(--hv-accent))]"
              : "text-ink hover:bg-white/[0.04]"
          }`}
        >
          <Server className="h-3.5 w-3.5 shrink-0 text-ink-dim" />
          Datacenter
        </Link>
        {tree.map(({ group, branches }) => {
          const groupOpen = !!q || !collapsed.includes(`conn:${group.id}`);
          return (
          <div key={group.id} className="mb-3">
            <button
              type="button"
              onClick={() => {
                onSelectConnection?.(group.id);
                toggleNode(`conn:${group.id}`);
              }}
              className="mb-1 flex w-full items-center gap-1 rounded-md px-1.5 py-1 text-left hover:bg-white/[0.04]"
            >
              <ChevronRight className={`h-3.5 w-3.5 shrink-0 text-ink-dim transition ${groupOpen ? "rotate-90" : ""}`} />
              <span className="min-w-0 truncate text-xs font-medium text-ink-dim">{group.name}</span>
            </button>
            {groupOpen && branches.map(({ node, children }) => {
          const id = `${group.id}:${node.node ?? node.id}`;
          const open = !!q || !collapsed.includes(id);
          return (
          <div key={id} className="mb-3">
            <div className="flex items-center gap-0.5">
              <button
                type="button"
                aria-expanded={open}
                aria-label={open ? `Collapse ${node.node}` : `Expand ${node.node}`}
                onClick={() => toggleNode(id)}
                className="grid h-6 w-5 shrink-0 place-items-center text-ink-dim hover:text-ink"
              >
                <ChevronRight className={`h-3.5 w-3.5 transition ${open ? "rotate-90" : ""}`} />
              </button>
              <Link
                to="/nodes/$node"
                params={{ node: node.node ?? node.id }}
                onClick={() => onSelectConnection?.(group.id)}
                className={`flex min-w-0 flex-1 items-center gap-2 rounded-md px-1.5 py-1.5 text-sm hover:bg-white/[0.04] ${
                  pathname === `/nodes/${node.node}` ? "bg-white/[0.06] text-ink" : "text-ink"
                }`}
              >
                <Server className="h-3.5 w-3.5 shrink-0 text-ink-dim" />
                <span className="min-w-0 shrink truncate font-medium">{node.node}</span>
                <Badge tone={statusTone(node.status)}>{node.status || "unknown"}</Badge>
              </Link>
            </div>
            {open && (
            <div className="ml-5 border-l border-line pl-2">
              {children.map((c) => {
                if (c.type === "storage") {
                  const href = `/storage/${c.storage ?? c.id}`;
                  const used = c.maxdisk ? pct(c.disk, c.maxdisk) : null;
                  const fill = meterFill(used);
                  const label = meterText(used);
                  return (
                    <Link
                      key={c.id}
                      to="/storage/$storage"
                      params={{ storage: c.storage ?? c.id }}
                      search={{ node: c.node }}
                      onClick={() => onSelectConnection?.(group.id)}
                      className={`mt-px flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs hover:bg-white/[0.04] ${
                        pathname === href ? "bg-white/[0.06] text-ink" : "text-ink"
                      }`}
                    >
                      <HardDrive className="h-3 w-3 shrink-0 text-ink-dim" />
                      <span className="min-w-0 truncate">{c.storage}</span>
                      {used != null && (
                        <>
                          <div className="ml-auto h-1 w-10 shrink-0 overflow-hidden rounded-full bg-white/10">
                            <div className={`h-full rounded-full ${fill}`} style={{ width: `${used}%` }} />
                          </div>
                          <span className={`shrink-0 text-right font-mono text-[10px] tabular-nums ${label}`}>{bytes(c.disk)}</span>
                        </>
                      )}
                    </Link>
                  );
                }
                const scopedTasks = group.id === taskConnection ? tasks : undefined;
                const activity = guestActivity(c, scopedTasks);
                const activityTask = guestActivityTask(c, scopedTasks);
                return (
                  <div
                    key={c.id}
                    className="mt-0.5 flex items-center gap-1"
                    onContextMenu={(e) => {
                      e.preventDefault();
                      onSelectConnection?.(group.id);
                      setMenu({ guest: c, x: e.clientX, y: e.clientY, connection: group.id });
                    }}
                  >
                    <Link
                      to="/guests/$vmid"
                      params={{ vmid: String(c.vmid) }}
                      search={{ node: c.node, type: c.type }}
                      onClick={(e) => {
                        onSelectConnection?.(group.id);
                        if (e.metaKey || e.ctrlKey) {
                          e.preventDefault();
                          if (c.vmid && c.node) {
                            toggleGuest({ vmid: c.vmid, node: c.node, type: c.type, name: c.name });
                          }
                        }
                      }}
                      title={`${c.type === "lxc" ? "Container" : "VM"} ${c.vmid} · ${c.template ? "template" : c.status}${c.hastate ? ` · HA ${c.hastate}` : ""}`}
                      className={`flex h-7 min-w-0 flex-1 items-center gap-2 rounded-md px-2 text-[13px] hover:bg-white/[0.04] ${
                        pathname === `/guests/${c.vmid}` || (c.vmid && c.node && isSelected({ vmid: c.vmid, node: c.node, type: c.type }))
                          ? "bg-white/[0.06] shadow-[inset_2px_0_0_rgb(var(--hv-accent))]"
                          : ""
                      }`}
                    >
                      <StatusDot status={c.template ? "template" : c.status} />
                      <span className="w-8 shrink-0 font-mono text-xs text-ink-faint">{c.vmid}</span>
                      <span
                        className={`min-w-0 flex-1 truncate ${c.status === "running" ? "text-ink" : "text-ink-dim"}`}
                      >
                        {c.name || c.type}
                      </span>
                      {c.type === "lxc" && <span className="shrink-0 font-mono text-[10px] text-ink-faint">CT</span>}
                      <TagList raw={c.tags} compact />
                      {c.pending && (
                        <span title="Pending restart" className="shrink-0">
                          <RotateCw className="h-3 w-3 text-warn" aria-label="Pending restart" />
                        </span>
                      )}
                      {activity && (
                        <span
                          role="button"
                          tabIndex={0}
                          title={`${activity} in progress — open tasks`}
                          className="shrink-0 cursor-pointer text-[11px] text-warn hover:underline"
                          onClick={(e) => {
                            e.preventDefault();
                            e.stopPropagation();
                            openTasks(activityTask?.upid);
                          }}
                          onKeyDown={(e) => {
                            if (e.key !== "Enter" && e.key !== " ") return;
                            e.preventDefault();
                            e.stopPropagation();
                            openTasks(activityTask?.upid);
                          }}
                        >
                          {activity}
                        </span>
                      )}
                      {c.hastate && haTone(c.hastate) !== "green" && (
                        <span className={`shrink-0 text-[11px] ${haTone(c.hastate) === "rose" ? "text-danger" : "text-warn"}`}>
                          HA {c.hastate}
                        </span>
                      )}
                    </Link>
                  </div>
                );
              })}
            </div>
            )}
          </div>
          );
            })}
          </div>
          );
        })}
        {!tree.length && (
          <p className="px-3 text-xs text-ink-dim">
            {inventory ? `No ${inventory} in the live inventory.` : "No resources yet. Add a connection in Settings to populate the tree."}
          </p>
        )}
      </div>
      {menu && (
        <GuestContextMenu
          guest={menu.guest}
          nodes={(groups.find((g) => g.id === menu.connection)?.resources ?? []).filter((r) => r.type === "node")}
          pos={{ x: menu.x, y: menu.y }}
          onClose={() => setMenu(null)}
        />
      )}
    </div>
  );
}

/** Filled green when running, hollow when stopped: a stopped guest is not an error. */
function StatusDot({ status }: { status?: string }) {
  const cls =
    status === "running" || status === "online"
      ? "bg-ok"
      : status === "paused" || status === "suspended"
        ? "bg-warn"
        : status === "offline" || status === "unknown"
          ? "bg-danger"
          : status === "template"
            ? "rounded-[2px] bg-ink-faint"
            : "shadow-[inset_0_0_0_1.5px_rgb(var(--hv-ink-faint))]";
  return <span aria-hidden className={`h-[7px] w-[7px] shrink-0 rounded-full ${cls}`} />;
}
