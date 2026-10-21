import { ResourceTree } from "@/components/tree/ResourceTree";
import type { InventoryGroup, Task } from "@/types";
import { PanelLeft } from "lucide-react";
import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";

const WIDTH_KEY = "hypervane:tree-width";
const OPEN_KEY = "hypervane:tree-open";
const MIN = 240;
const MAX = 560;
const DEFAULT = 320;
const RAIL = 40;

function clampWidth(n: number) {
  return Math.min(MAX, Math.max(MIN, Math.round(n)));
}

function loadWidth() {
  const n = Number(localStorage.getItem(WIDTH_KEY));
  return Number.isFinite(n) ? clampWidth(n) : DEFAULT;
}

function loadOpen() {
  return localStorage.getItem(OPEN_KEY) !== "0";
}

export function InventoryAside({
  groups,
  tasks,
  taskConnection,
  onSelectConnection,
}: {
  groups: InventoryGroup[];
  tasks?: Task[];
  taskConnection?: string;
  onSelectConnection?: (id: string) => void;
}) {
  const [open, setOpen] = useState(loadOpen);
  const [width, setWidth] = useState(loadWidth);
  const widthRef = useRef(width);
  widthRef.current = width;

  useEffect(() => {
    localStorage.setItem(OPEN_KEY, open ? "1" : "0");
  }, [open]);
  useEffect(() => {
    const onSearch = () => setOpen(true);
    window.addEventListener("hypervane:search", onSearch);
    return () => window.removeEventListener("hypervane:search", onSearch);
  }, []);
  useEffect(() => {
    localStorage.setItem(WIDTH_KEY, String(width));
  }, [width]);

  function startResize(e: ReactPointerEvent<HTMLButtonElement>) {
    if (!open) return;
    e.preventDefault();
    const startX = e.clientX;
    const startW = widthRef.current;
    const prevCursor = document.body.style.cursor;
    const prevSelect = document.body.style.userSelect;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    const onMove = (ev: PointerEvent) => {
      setWidth(clampWidth(startW + ev.clientX - startX));
    };
    const onUp = () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      document.body.style.cursor = prevCursor;
      document.body.style.userSelect = prevSelect;
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  }

  if (!open) {
    return (
      <aside className="flex shrink-0 flex-col border-r border-white/[0.06] bg-black/55" style={{ width: RAIL }}>
        <button
          type="button"
          title="Show inventory"
          aria-label="Show inventory"
          onClick={() => setOpen(true)}
          className="grid h-10 place-items-center text-ink-dim hover:text-ink"
        >
          <PanelLeft className="h-4 w-4" />
        </button>
      </aside>
    );
  }

  return (
    <aside
      className="relative flex shrink-0 flex-col border-r border-white/[0.06] bg-black/55"
      style={{ width }}
    >
      <div className="min-h-0 flex-1">
        <ResourceTree groups={groups} tasks={tasks} taskConnection={taskConnection} onSelectConnection={onSelectConnection} onCollapse={() => setOpen(false)} />
      </div>
      <button
        type="button"
        aria-label="Resize inventory"
        title="Drag to resize · double-click to reset"
        onPointerDown={startResize}
        onDoubleClick={() => setWidth(DEFAULT)}
        className="absolute inset-y-0 -right-px z-10 w-2 cursor-col-resize border-0 bg-transparent p-0 hover:bg-accent/35"
      />
    </aside>
  );
}
