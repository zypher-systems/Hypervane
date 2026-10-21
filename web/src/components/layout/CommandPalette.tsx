import { setActiveConnection } from "@/lib/connection";
import type { InventoryGroup } from "@/types";
import { useClusterNav } from "@/components/layout/DatacenterNav";
import { useNavigate } from "@tanstack/react-router";
import { ArrowRight, Box, HardDrive, Monitor, Server } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

type Item = {
  id: string;
  label: string;
  hint?: string;
  icon: "qemu" | "lxc" | "node" | "storage" | "page";
  run: () => void;
};

const OPEN_EVENT = "hypervane:palette-open";

/** Opens the command palette from anywhere (header button, empty states). */
export function openCommandPalette() {
  window.dispatchEvent(new CustomEvent(OPEN_EVENT));
}

// Datacenter › Configuration sections, so settings are one search away.
const CONFIG_SECTIONS: [string, string][] = [
  ["options", "Options"],
  ["cluster", "Cluster"],
  ["remotes", "Remotes"],
  ["pools", "Pools"],
  ["jobs", "Jobs"],
  ["metrics", "Metric servers"],
  ["notifications", "Notifications"],
  ["mappings", "Mappings"],
  ["acme", "ACME"],
  ["log", "Cluster log"],
  ["support", "Support"],
];

export function CommandPalette({ groups }: { groups: InventoryGroup[] }) {
  const [open, setOpen] = useState(false);
  const [q, setQ] = useState("");
  const [sel, setSel] = useState(0);
  const nav = useNavigate();
  const clusterNav = useClusterNav();
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      }
    };
    const onOpen = () => setOpen(true);
    window.addEventListener("keydown", onKey);
    window.addEventListener(OPEN_EVENT, onOpen);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener(OPEN_EVENT, onOpen);
    };
  }, []);

  useEffect(() => {
    if (open) {
      setQ("");
      setSel(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  const items = useMemo<Item[]>(() => {
    const query = q.trim().toLowerCase();
    const out: Item[] = [];
    if (query) {
      const pages: Item[] = [
        { id: "p-dashboard", label: "Dashboard", icon: "page", run: () => void nav({ to: "/" }) },
        { id: "p-top", label: "Top 10", hint: "busiest guests", icon: "page", run: () => void nav({ to: "/top" }) },
        { id: "p-recipes", label: "Recipes", hint: "templates and scripts", icon: "page", run: () => void nav({ to: "/recipes" }) },
        { id: "p-create", label: "Create guest", hint: "new VM or container", icon: "page", run: () => void nav({ to: "/create", search: {} }) },
        ...clusterNav
          .filter((c) => c.id !== "config")
          .map<Item>((c) => ({
            id: `p-dc-${c.id}`,
            label: c.id === "summary" ? "Datacenter" : `Datacenter › ${c.label}`,
            icon: "page",
            run: () => void nav({ to: c.to, search: c.tab ? { tab: c.tab } : {} }),
          })),
        ...(clusterNav.some((c) => c.id === "config")
          ? CONFIG_SECTIONS.map<Item>(([section, label]) => ({
              id: `p-cfg-${section}`,
              label: `Configuration › ${label}`,
              icon: "page",
              run: () => void nav({ to: "/datacenter", search: { tab: "config", section } }),
            }))
          : []),
      ];
      out.push(...pages.filter((p) => `${p.label} ${p.hint ?? ""}`.toLowerCase().includes(query)));
    }
    for (const g of groups) {
      for (const r of g.resources ?? []) {
        const haystack = `${r.name ?? ""} ${r.vmid ?? ""} ${r.node ?? ""} ${r.storage ?? ""} ${r.tags ?? ""}`.toLowerCase();
        if (query && !haystack.includes(query)) continue;
        const conn = g.id;
        if (r.type === "qemu" || r.type === "lxc") {
          out.push({
            id: `g-${conn}-${r.id}`,
            label: r.name || `${r.type} ${r.vmid}`,
            hint: `${r.type} · ${r.node} · ${r.vmid}`,
            icon: r.type,
            run: () => {
              setActiveConnection(conn);
              void nav({ to: "/guests/$vmid", params: { vmid: String(r.vmid) }, search: { node: r.node, type: r.type } });
            },
          });
        } else if (r.type === "node") {
          out.push({
            id: `n-${conn}-${r.id}`,
            label: r.node ?? r.id,
            hint: "node",
            icon: "node",
            run: () => {
              setActiveConnection(conn);
              void nav({ to: "/nodes/$node", params: { node: r.node ?? r.id } });
            },
          });
        } else if (r.type === "storage") {
          out.push({
            id: `s-${conn}-${r.id}`,
            label: r.storage ?? r.id,
            hint: `storage · ${r.node}`,
            icon: "storage",
            run: () => {
              setActiveConnection(conn);
              void nav({ to: "/storage/$storage", params: { storage: r.storage ?? r.id }, search: { node: r.node } });
            },
          });
        }
      }
    }
    return out.slice(0, 60);
  }, [groups, q, nav, clusterNav]);

  useEffect(() => setSel(0), [q]);

  if (!open) return null;

  const active = items[Math.min(sel, items.length - 1)];

  function icon(kind: Item["icon"]) {
    const cls = "h-3.5 w-3.5 shrink-0 text-ink-dim";
    if (kind === "page") return <ArrowRight className={cls} />;
    if (kind === "node") return <Server className={cls} />;
    if (kind === "storage") return <HardDrive className={cls} />;
    if (kind === "lxc") return <Box className={cls} />;
    return <Monitor className={cls} />;
  }

  return (
    <div
      className="fixed inset-0 z-[60] flex items-start justify-center bg-black/60 pt-[12vh]"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) setOpen(false);
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        className="hv-glass w-full max-w-xl overflow-hidden rounded-xl border"
      >
        <input
          ref={inputRef}
          value={q}
          onChange={(e) => setQ(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Escape") setOpen(false);
            else if (e.key === "ArrowDown") {
              e.preventDefault();
              setSel((s) => Math.min(s + 1, items.length - 1));
            } else if (e.key === "ArrowUp") {
              e.preventDefault();
              setSel((s) => Math.max(s - 1, 0));
            } else if (e.key === "Enter" && active) {
              setOpen(false);
              active.run();
            }
          }}
          placeholder="Jump to a guest, node, storage, or page…"
          className="h-12 w-full border-b border-white/10 bg-transparent px-4 text-sm text-ink outline-none placeholder:text-ink-faint"
        />
        <div role="listbox" className="max-h-[50vh] overflow-y-auto py-1">
          {!items.length && <p className="px-4 py-6 text-center text-sm text-ink-dim">No matches.</p>}
          {items.map((it, i) => (
            <button
              key={it.id}
              type="button"
              role="option"
              aria-selected={i === sel}
              onMouseEnter={() => setSel(i)}
              onClick={() => {
                setOpen(false);
                it.run();
              }}
              className={`flex w-full items-center gap-3 px-4 py-2 text-left text-sm ${
                i === sel ? "bg-white/[0.07] text-ink" : "text-ink-dim"
              }`}
            >
              {icon(it.icon)}
              <span className="min-w-0 flex-1 truncate">{it.label}</span>
              {it.hint && <span className="shrink-0 font-mono text-[10px] text-ink-faint">{it.hint}</span>}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-4 border-t border-white/10 px-4 py-2 font-mono text-[10px] text-ink-faint">
          <span>↑↓ navigate</span>
          <span>↵ open</span>
          <span>esc close</span>
        </div>
      </div>
    </div>
  );
}
