export type StorageLike = {
  storage: string;
  content?: string;
  disable?: number;
  nodes?: string;
};

export const STORAGE_CONTENT_TYPES = [
  { id: "images", label: "Disk images" },
  { id: "rootdir", label: "Container volumes" },
  { id: "iso", label: "ISO images" },
  { id: "vztmpl", label: "Container templates" },
  { id: "backup", label: "Backups" },
  { id: "snippets", label: "Snippets" },
  { id: "import", label: "Import" },
] as const;

export function parseContent(raw?: string): string[] {
  return (raw ?? "")
    .split(",")
    .map((part) => part.trim())
    .filter(Boolean);
}

export function joinContent(ids: string[]): string {
  return STORAGE_CONTENT_TYPES.map((t) => t.id).filter((id) => ids.includes(id)).join(",");
}

export function storageHasContent(st: StorageLike, kind: string): boolean {
  return parseContent(st.content).includes(kind);
}

export function storageOnNode(st: StorageLike, node?: string): boolean {
  if (!node || !st.nodes) return true;
  return parseContent(st.nodes).includes(node);
}

export function storagesForContent<T extends StorageLike>(list: T[], kind: string, node?: string): T[] {
  return list.filter((st) => !st.disable && storageHasContent(st, kind) && storageOnNode(st, node));
}

export function defaultStorage<T extends StorageLike>(list: T[], kind: string, node?: string): string {
  const cands = storagesForContent(list, kind, node);
  if (kind === "import") {
    const local = cands.find((st) => st.storage === "local");
    if (local) return local.storage;
  }
  return cands[0]?.storage ?? "";
}
