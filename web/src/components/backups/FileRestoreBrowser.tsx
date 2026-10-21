import { Modal } from "@/components/ui/dialog";
import { BarButton } from "@/components/ui/action-bar";
import { api } from "@/lib/api";
import { bytes, relTime } from "@/lib/format";
import type { FileRestoreEntry } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

type Crumb = { label: string; filepath: string };

function isDir(item: FileRestoreEntry): boolean {
  if (item.leaf === true || item.leaf === 1) return false;
  if (item.leaf === false || item.leaf === 0) return true;
  return item.type === "d" || item.type === "directory" || item.type === "dir";
}

function entryName(item: FileRestoreEntry): string {
  if (item.text) return item.text;
  return item.filepath || "item";
}

export function FileRestoreBrowser({
  storage,
  node,
  volume,
  onClose,
}: {
  storage: string;
  node: string;
  volume: string;
  onClose: () => void;
}) {
  const [filepath, setFilepath] = useState("/");
  const [stack, setStack] = useState<Crumb[]>([]);
  const list = useQuery({
    queryKey: ["file-restore", storage, node, volume, filepath],
    queryFn: () => api.fileRestore(storage, node, volume, filepath),
    retry: false,
  });

  function openRoot() {
    setStack([]);
    setFilepath("/");
  }

  function openCrumb(index: number) {
    const next = stack[index];
    setStack(stack.slice(0, index + 1));
    setFilepath(next.filepath);
  }

  function openDir(item: FileRestoreEntry) {
    setStack([...stack, { label: entryName(item), filepath: item.filepath }]);
    setFilepath(item.filepath);
  }

  return (
    <Modal title="Browse backup files" onClose={onClose} size="lg">
      <p className="text-sm text-ink-dim">
        File restore works on Proxmox Backup Server snapshots only. Regular vzdump archives cannot be browsed.
      </p>
      <p className="break-all font-mono text-xs text-ink-faint">{volume}</p>
      <div className="flex flex-wrap gap-1 text-xs text-ink-dim">
        <button type="button" className="hover:text-accent" onClick={openRoot}>/</button>
        {stack.map((c, i) => (
          <span key={`${c.filepath}-${i}`}>
            <span className="text-ink-faint">/</span>
            <button type="button" className="hover:text-accent" onClick={() => openCrumb(i)}>{c.label}</button>
          </span>
        ))}
      </div>
      <div className="max-h-72 overflow-auto rounded-lg border border-white/10">
        {list.isLoading && <p className="p-3 text-sm text-ink-dim">Loading…</p>}
        {(list.data ?? []).map((item: FileRestoreEntry) => {
          const name = entryName(item);
          const dir = isDir(item);
          return (
            <div key={item.filepath || name} className="flex items-center justify-between border-t border-white/10 px-3 py-1.5 text-sm first:border-t-0">
              <button
                type="button"
                className="truncate text-left hover:text-accent"
                onClick={() => {
                  if (dir) openDir(item);
                }}
              >
                {dir ? "▸ " : ""}{name}
              </button>
              <div className="flex items-center gap-3 text-xs text-ink-faint">
                <span>{dir ? "dir" : bytes(item.size)}</span>
                <span>{relTime(item.mtime)}</span>
                {!dir && (
                  <a
                    className="text-accent hover:underline"
                    href={api.fileRestoreDownloadUrl(storage, node, volume, item.filepath)}
                  >
                    Download
                  </a>
                )}
              </div>
            </div>
          );
        })}
        {list.isError && (
          <p className="p-3 text-sm text-rose-400">
            {(list.error as Error).message || "This snapshot does not support file restore."}
          </p>
        )}
        {!list.data?.length && !list.isLoading && !list.isError && (
          <p className="p-3 text-sm text-ink-dim">Empty directory.</p>
        )}
      </div>
      <BarButton onClick={onClose}>Close</BarButton>
    </Modal>
  );
}
