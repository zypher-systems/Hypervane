import { ObjectACL } from "@/components/access/ObjectACL";
import { ObjectTaskHistory } from "@/components/tasks/ObjectTaskHistory";
import { FileRestoreBrowser } from "@/components/backups/FileRestoreBrowser";
import { PruneCard } from "@/components/backups/PruneCard";
import { RestoreDialog } from "@/components/backups/RestoreDialog";
import { MetricChart } from "@/components/charts/MetricChart";
import { attachTask } from "@/components/layout/AppShell";
import { BarButton } from "@/components/ui/action-bar";
import { Badge, Card } from "@/components/ui/card";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Advanced, ImportMetadataView, asRecord } from "@/components/ui/item-details";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { fleetKeys } from "@/lib/tasks";
import { bytes, relTime } from "@/lib/format";
import { useCan } from "@/lib/perms";
import { cn } from "@/lib/utils";
import type { Resource, StorageContent } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams, useSearch } from "@tanstack/react-router";
import { useMemo, useState } from "react";
import { CHART_PRIMARY } from "@/lib/chartColors";
import { meterFill } from "@/components/ui/meter";

const TIMEFRAMES = ["hour", "day", "week", "month", "year"] as const;
const CHART_USED = CHART_PRIMARY;

const CONTENT_LABEL: Record<string, string> = {
  iso: "ISO",
  vztmpl: "CT templates",
  images: "Disk images",
  backup: "Backups",
  rootdir: "Containers",
  snippets: "Snippets",
  import: "Import",
};

const UPLOAD_KINDS = ["iso", "vztmpl", "import"] as const;
const IMPORT_KINDS = ["images", "import"] as const;
const OCI_KINDS = ["images", "rootdir", "import"] as const;

function splitContent(raw?: string) {
  return (raw ?? "").split(",").map((s) => s.trim()).filter(Boolean);
}

function hasAny(types: Set<string>, kinds: readonly string[]) {
  return kinds.some((k) => types.has(k));
}

type SortKey = "volid" | "content" | "notes" | "size" | "ctime";
type SortDir = "asc" | "desc";
type ColFilters = Record<SortKey, string>;

const SORT_DEFAULT: Record<SortKey, SortDir> = {
  volid: "asc",
  content: "asc",
  notes: "asc",
  size: "desc",
  ctime: "desc",
};

const EMPTY_FILTERS: ColFilters = { volid: "", content: "", notes: "", size: "", ctime: "" };

function contentLabel(kind: string) {
  return CONTENT_LABEL[kind] ?? kind;
}

function matchesFilter(hay: string, needle: string) {
  const q = needle.trim().toLowerCase();
  return !q || hay.toLowerCase().includes(q);
}

function compareRows(a: StorageContent, b: StorageContent, key: SortKey) {
  switch (key) {
    case "volid":
      return a.volid.localeCompare(b.volid, undefined, { numeric: true });
    case "content":
      return contentLabel(a.content).localeCompare(contentLabel(b.content));
    case "notes":
      return (a.notes ?? "").localeCompare(b.notes ?? "");
    case "size":
      return (a.size ?? 0) - (b.size ?? 0);
    case "ctime":
      return (a.ctime ?? 0) - (b.ctime ?? 0);
  }
}

function rowMatches(item: StorageContent, filters: ColFilters) {
  return (
    matchesFilter(item.volid, filters.volid)
    && matchesFilter(`${item.content} ${contentLabel(item.content)}`, filters.content)
    && matchesFilter(item.notes ?? "", filters.notes)
    && matchesFilter(`${item.size} ${bytes(item.size)}`, filters.size)
    && matchesFilter(relTime(item.ctime), filters.ctime)
  );
}

function SortTh({
  label,
  col,
  sort,
  onSort,
  className,
}: {
  label: string;
  col: SortKey;
  sort: { key: SortKey; dir: SortDir };
  onSort: (col: SortKey) => void;
  className?: string;
}) {
  const active = sort.key === col;
  return (
    <th aria-sort={active ? (sort.dir === "asc" ? "ascending" : "descending") : "none"} className={className}>
      <button
        type="button"
        className={cn("inline-flex items-center gap-1 hover:text-ink", active ? "text-ink" : "text-ink-faint")}
        onClick={() => onSort(col)}
      >
        {label}
        <span className="w-2 font-mono text-[9px] text-accent">{active ? (sort.dir === "asc" ? "▲" : "▼") : ""}</span>
      </button>
    </th>
  );
}

export function StorageBrowserPage() {
  const qc = useQueryClient();
  const nav = useNavigate();
  const { storage } = useParams({ from: "/app/storage/$storage" });
  const search = useSearch({ from: "/app/storage/$storage" });
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const node = search.node || summary.data?.resources.find((r) => r.type === "node")?.node || "";
  const [content, setContent] = useState("");
  const [sort, setSort] = useState<{ key: SortKey; dir: SortDir }>({ key: "ctime", dir: "desc" });
  const [colFilter, setColFilter] = useState<ColFilters>(EMPTY_FILTERS);
  const [tf, setTf] = useState<(typeof TIMEFRAMES)[number]>("hour");
  const items = useQuery({
    queryKey: ["content", storage, node, content],
    queryFn: () => api.storageContent(storage, node, content || undefined),
    enabled: !!node,
  });
  const rrd = useQuery({
    queryKey: ["storage-rrd", storage, node, tf],
    queryFn: () => api.storageRrd(storage, node, tf),
    enabled: !!node,
    refetchInterval: 15000,
  });
  const [del, setDel] = useState<StorageContent | null>(null);
  const [restore, setRestore] = useState<StorageContent | null>(null);
  const [browse, setBrowse] = useState<StorageContent | null>(null);
  const storeMeta = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const meta = storeMeta.data?.find((s) => s.storage === storage);
  const types = useMemo(() => new Set(splitContent(meta?.content)), [meta?.content]);
  const contentKinds = useMemo(
    () => (types.size ? [...types] : Object.keys(CONTENT_LABEL)),
    [types],
  );
  const rows = useMemo(() => {
    const filtered = (items.data ?? []).filter((item) => rowMatches(item, colFilter));
    const mul = sort.dir === "asc" ? 1 : -1;
    return filtered.sort((a, b) => compareRows(a, b, sort.key) * mul);
  }, [items.data, colFilter, sort]);
  const filterOn = Object.values(colFilter).some(Boolean);
  function toggleSort(key: SortKey) {
    setSort((cur) => (cur.key === key ? { key, dir: cur.dir === "asc" ? "desc" : "asc" } : { key, dir: SORT_DEFAULT[key] }));
  }
  const uploadKinds = UPLOAD_KINDS.filter((k) => types.has(k));
  const canUpload = uploadKinds.length > 0;
  const canImportGuest = hasAny(types, IMPORT_KINDS);
  const canOci = hasAny(types, OCI_KINDS);
  const isPbs = meta?.type === "pbs";
  const canBackup = types.has("backup");
  const [uploadKind, setUploadKind] = useState("");
  const [progress, setProgress] = useState<number | null>(null);
  const [urlForm, setUrlForm] = useState({ url: "", filename: "", content: "" });
  const [ingestError, setIngestError] = useState("");
  const can = useCan();
  const storePath = `/storage/${storage}`;
  const canAllocate = can(storePath, "Datastore.AllocateSpace") || can(storePath, "Datastore.Allocate");
  const canDelete = can(storePath, "Datastore.Allocate") || can(storePath, "Datastore.AllocateTemplate");
  const canRestore = can("/", "VM.Allocate") || can("/", "VM.Backup");
  const canAudit = can(storePath, "Datastore.Audit") || canAllocate;
  // Proxmox filters backups out of a content listing unless the caller has
  // Datastore.AllocateSpace here and VM.Backup on the guest, so an audit-only
  // token sees an empty backup store rather than an error.
  const backupsHidden = canBackup && !canAllocate;
  const [noteVol, setNoteVol] = useState<StorageContent | null>(null);
  const [notes, setNotes] = useState("");
  const [noteMsg, setNoteMsg] = useState("");
  const [impVol, setImpVol] = useState("");
  const [impUrl, setImpUrl] = useState("");
  const [impDump, setImpDump] = useState<Record<string, unknown> | null>(null);
  const [impMsg, setImpMsg] = useState("");
  const [ociUrl, setOciUrl] = useState("");
  const [ociOn, setOciOn] = useState(true);
  const [ociMsg, setOciMsg] = useState("");
  const cfg = useQuery({ queryKey: ["storage-cfg", storage], queryFn: () => api.storageConfig(storage) });
  const ident = useQuery({
    queryKey: ["storage-id", storage, node],
    queryFn: () => api.storageIdentity(storage, node),
    enabled: !!node,
  });
  const [fp, setFp] = useState("");
  const [encKey, setEncKey] = useState("");
  const [cryptMode, setCryptMode] = useState("");
  const [identExtra, setIdentExtra] = useState("");
  const [identMsg, setIdentMsg] = useState("");
  const [panel, setPanel] = useState<"upload" | "import" | "identity" | "prune" | "permissions" | "history" | null>(null);
  const showIdentity = isPbs || ident.data?.supported === true || Boolean(cfg.data?.fingerprint || cfg.data?.encryption || cfg.data?.["crypt-mode"]);
  const showImport = canAudit && (canImportGuest || (canAllocate && canOci && ociOn));
  const canACL = can("/access", "Permissions.Modify") || can("/access", "Sys.Audit") || can("/", "Permissions.Modify");
  const storeRes = (summary.data?.resources ?? []).find((r: Resource) => {
    if (r.type !== "storage") return false;
    if ((r.storage ?? r.id.split("/").pop()) !== storage) return false;
    return !node || r.node === node;
  });
  const used = storeRes?.disk;
  const total = storeRes?.maxdisk;
  const usedPct = total && total > 0 ? Math.min(100, ((used ?? 0) / total) * 100) : 0;
  const activeUpload = uploadKinds.includes(uploadKind as (typeof UPLOAD_KINDS)[number]) ? uploadKind : (uploadKinds[0] ?? "");
  const activeUrlKind = uploadKinds.includes(urlForm.content as (typeof UPLOAD_KINDS)[number]) ? urlForm.content : (uploadKinds[0] ?? "");

  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold">{storage}</h1>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-ink-dim">
            <span>Node {node || "…"}</span>
            {meta?.type && <Badge>{meta.type}</Badge>}
            {contentKinds.map((k) => (
              <Badge key={k} tone="teal">{CONTENT_LABEL[k] ?? k}</Badge>
            ))}
          </div>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          {canAllocate && canUpload && <BarButton onClick={() => setPanel("upload")}>Upload</BarButton>}
          {showImport && <BarButton onClick={() => setPanel("import")}>Import</BarButton>}
          {showIdentity && <BarButton onClick={() => setPanel("identity")}>Identity</BarButton>}
          {canBackup && canDelete && node && <BarButton onClick={() => setPanel("prune")}>Prune</BarButton>}
          {node && <BarButton onClick={() => setPanel("history")}>History</BarButton>}
          {canACL && <BarButton onClick={() => setPanel("permissions")}>Permissions</BarButton>}
        </div>
      </div>

      <Card className="p-5">
        <div className="mb-4 flex items-center justify-between gap-3">
          <h2 className="text-xs font-medium text-ink-dim">Usage</h2>
          <Select className="w-28" value={tf} onChange={(e) => setTf(e.target.value as (typeof TIMEFRAMES)[number])}>
            {TIMEFRAMES.map((t) => <option key={t}>{t}</option>)}
          </Select>
        </div>
        <div className="mb-5">
          <div className="mb-1 flex items-center justify-between gap-3 text-[11px]">
            <span className="text-ink-faint">Used</span>
            <span className="font-mono text-ink-dim">
              {total ? `${bytes(used)} / ${bytes(total)}` : bytes(used)}
              {total ? ` · ${usedPct.toFixed(0)}%` : ""}
            </span>
          </div>
          <div className="h-1 overflow-hidden rounded-full bg-white/10">
            <div className={`h-full rounded-full ${meterFill(usedPct)}`} style={{ width: `${usedPct}%` }} />
          </div>
        </div>
        <MetricChart
          points={rrd.data ?? []}
          format="bytes"
          series={[{ key: "used", label: "Used", color: CHART_USED }]}
        />
      </Card>

      {panel === "import" && (
        <Modal title="Import" onClose={() => setPanel(null)} size="lg">
          {canImportGuest && (
            <>
              <div className="grid gap-3 md:grid-cols-2">
                <Field label="Volume">
                  <Input value={impVol} onChange={(e) => setImpVol(e.target.value)} placeholder={`${storage}:import/disk.ova`} />
                </Field>
                <Field label="URL">
                  <Input value={impUrl} onChange={(e) => setImpUrl(e.target.value)} placeholder="https://…" />
                </Field>
              </div>
              <div className="flex flex-wrap gap-2">
                <BarButton
                  onClick={async () => {
                    if (!node || !impVol) return;
                    const res = await api.importMetadata(storage, node, impVol);
                    if (res.supported === false) {
                      setImpMsg("Import metadata is not available.");
                      setImpDump(null);
                      return;
                    }
                    setImpMsg("");
                    setImpDump(asRecord(res.data));
                  }}
                >
                  Show volume metadata
                </BarButton>
                {canUpload && (
                  <BarButton
                    onClick={async () => {
                      if (!node || !impUrl) return;
                      const res = await api.queryUrlMetadata(storage, node, impUrl);
                      if (res.supported === false) {
                        setImpMsg("URL metadata is not available.");
                        setImpDump(null);
                        return;
                      }
                      setImpMsg("");
                      setImpDump(asRecord(res.data));
                    }}
                  >
                    Show URL metadata
                  </BarButton>
                )}
                <BarButton
                  disabled={!impVol}
                  onClick={() => nav({ to: "/create", search: { import: "1", storage, node, volume: impVol } })}
                >
                  Import as guest
                </BarButton>
              </div>
              {(impDump || impMsg) && <ImportMetadataView data={impDump} message={impMsg} />}
            </>
          )}
          {canAllocate && canOci && ociOn && (
            <div className="flex flex-wrap items-end gap-2">
              <Field label="OCI image">
                <Input value={ociUrl} onChange={(e) => setOciUrl(e.target.value)} placeholder="oci://registry/image:tag" />
              </Field>
              <BarButton
                accent
                onClick={async () => {
                  if (!node || !ociUrl) return;
                  const res = await api.ociPull(storage, node, { url: ociUrl });
                  if (res.supported === false) {
                    setOciOn(false);
                    setOciMsg("OCI pull is not available on this storage.");
                    return;
                  }
                  if (res.upid) attachTask(res.upid, [["content", storage, node], ["storages"]]);
                  setOciMsg("Pull started.");
                }}
              >
                Pull OCI
              </BarButton>
            </div>
          )}
          {ociMsg && <p className="text-sm text-ink-dim">{ociMsg}</p>}
        </Modal>
      )}

      {panel === "identity" && (
        <Modal title="Identity" onClose={() => setPanel(null)} size="lg">
          <p className="text-sm text-ink-dim">PBS fingerprint and encryption keys for this storage.</p>
          <div className="grid gap-3 md:grid-cols-2">
            <Field label="Fingerprint">
              <Input
                value={fp || String(ident.data?.item?.fingerprint ?? cfg.data?.fingerprint ?? "")}
                onChange={(e) => setFp(e.target.value)}
                disabled={!canAllocate}
              />
            </Field>
            <Field label="Encryption key">
              <Input
                value={encKey || String(ident.data?.item?.["encryption-key"] ?? cfg.data?.["encryption-key"] ?? "")}
                onChange={(e) => setEncKey(e.target.value)}
                disabled={!canAllocate}
                placeholder="storage-key or path"
              />
            </Field>
            <Field label="Crypt mode">
              <Select
                value={cryptMode || String(ident.data?.item?.["crypt-mode"] ?? cfg.data?.["crypt-mode"] ?? "")}
                onChange={(e) => setCryptMode(e.target.value)}
                disabled={!canAllocate}
              >
                <option value="">unchanged</option>
                <option value="none">none</option>
                <option value="default">default</option>
                <option value="encrypt">encrypt</option>
              </Select>
            </Field>
          </div>
          <Advanced label="Advanced: extra keys" defaultOpen={!!identExtra}>
            <Field label="Extra keys">
              <Input value={identExtra} onChange={(e) => setIdentExtra(e.target.value)} placeholder="leftover key=value" disabled={!canAllocate} />
            </Field>
          </Advanced>
          {identMsg && <p className="text-sm text-ink-dim">{identMsg}</p>}
          {canAllocate && (
            <BarButton
              accent
              onClick={async () => {
                if (!node) return;
                const body: Record<string, unknown> = {};
                if (fp) body.fingerprint = fp;
                if (encKey) body["encryption-key"] = encKey;
                if (cryptMode) body["crypt-mode"] = cryptMode;
                for (const part of identExtra.split(/\s+/)) {
                  const [k, v] = part.split("=");
                  if (k && v) body[k] = v;
                }
                const res = await api.updateStorageIdentity(storage, node, body);
                setIdentMsg(res.supported === false ? "Saving identity is not available." : "Saved.");
                void qc.invalidateQueries({ queryKey: ["storage-id", storage, node] });
              }}
            >
              Save identity
            </BarButton>
          )}
        </Modal>
      )}

      {panel === "upload" && (
        <Modal title="Upload" onClose={() => setPanel(null)} size="lg">
          <div className="grid gap-6 md:grid-cols-2">
          <div className="space-y-3">
            <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">File</h2>
            <Field label="Content">
              <Select value={activeUpload} onChange={(e) => setUploadKind(e.target.value)}>
                {uploadKinds.map((k) => (
                  <option key={k} value={k}>{CONTENT_LABEL[k] ?? k}</option>
                ))}
              </Select>
            </Field>
            <input
              type="file"
              className="text-sm text-ink-dim"
              onChange={async (e) => {
                const file = e.target.files?.[0];
                if (!file || !node || !activeUpload) return;
                setIngestError("");
                setProgress(0);
                try {
                  await api.upload(storage, node, file, activeUpload, setProgress);
                  void items.refetch();
                } catch (err) {
                  setIngestError(err instanceof Error ? err.message : "Upload failed");
                } finally {
                  setProgress(null);
                  e.target.value = "";
                }
              }}
            />
            {progress != null && <p className="text-xs text-accent">{progress}%</p>}
          </div>
          <div className="space-y-3">
            <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Download from URL</h2>
            <Field label="URL"><Input value={urlForm.url} onChange={(e) => setUrlForm({ ...urlForm, url: e.target.value })} /></Field>
            <Field label="Filename"><Input value={urlForm.filename} onChange={(e) => setUrlForm({ ...urlForm, filename: e.target.value })} /></Field>
            <Field label="Content">
              <Select value={activeUrlKind} onChange={(e) => setUrlForm({ ...urlForm, content: e.target.value })}>
                {uploadKinds.map((k) => (
                  <option key={k} value={k}>{CONTENT_LABEL[k] ?? k}</option>
                ))}
              </Select>
            </Field>
            <BarButton
              accent
              onClick={async () => {
                setIngestError("");
                try {
                  const res = await api.downloadUrl(storage, node, { ...urlForm, content: activeUrlKind });
                  attachTask(res.upid, [["content", storage, node], ["storages"]]);
                } catch (err) {
                  setIngestError(err instanceof Error ? err.message : "Download failed");
                }
              }}
            >
              Start download
            </BarButton>
          </div>
          {ingestError && <p className="text-sm text-rose-400 md:col-span-2">{ingestError}</p>}
          </div>
        </Modal>
      )}

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-white/15 px-4 py-3">
          <div className="flex items-baseline gap-3">
            <h2 className="text-xs font-medium text-ink-dim">Content</h2>
            {!!items.data?.length && (
              <span className="text-xs text-ink-faint">
                {filterOn ? `${rows.length} / ${items.data.length}` : items.data.length}
              </span>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {filterOn && (
              <BarButton onClick={() => setColFilter(EMPTY_FILTERS)}>Clear filters</BarButton>
            )}
            <Select value={content} onChange={(e) => setContent(e.target.value)} className="w-48">
              <option value="">All content</option>
              {contentKinds.map((k) => (
                <option key={k} value={k}>{CONTENT_LABEL[k] ?? k}</option>
              ))}
            </Select>
          </div>
        </div>
        <table className="w-full text-sm">
          <thead className="bg-black/30 text-left text-xs font-medium text-ink-dim">
            <tr>
              <SortTh label="Volume" col="volid" sort={sort} onSort={toggleSort} className="px-4 py-2" />
              <SortTh label="Content" col="content" sort={sort} onSort={toggleSort} />
              <SortTh label="Notes" col="notes" sort={sort} onSort={toggleSort} />
              <SortTh label="Size" col="size" sort={sort} onSort={toggleSort} />
              <SortTh label="Created" col="ctime" sort={sort} onSort={toggleSort} />
              <th></th>
            </tr>
            <tr className="bg-black/20 normal-case tracking-normal">
              {(["volid", "content", "notes", "size", "ctime"] as const).map((col) => (
                <th key={col} className={cn("py-1.5 font-normal", col === "volid" ? "px-4" : "pr-3")}>
                  <Input
                    className="h-7 px-2 text-xs"
                    value={colFilter[col]}
                    placeholder="Filter"
                    onChange={(e) => setColFilter((f) => ({ ...f, [col]: e.target.value }))}
                  />
                </th>
              ))}
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((item) => (
              <tr key={item.volid} className="border-t border-white/10">
                <td className="px-4 py-2 font-mono text-xs">{item.volid}</td>
                <td><Badge>{contentLabel(item.content)}</Badge></td>
                <td className="max-w-xs truncate text-ink-dim">{item.notes || "—"}</td>
                <td className="font-mono text-ink-dim">{bytes(item.size)}</td>
                <td className="text-ink-faint">{relTime(item.ctime)}</td>
                <td className="space-x-2 px-4 text-right">
                  {canAllocate && (
                    <BarButton onClick={() => { setNoteVol(item); setNotes(item.notes ?? ""); setNoteMsg(""); }}>Notes</BarButton>
                  )}
                  {item.content === "backup" && canRestore && (
                    <BarButton onClick={() => setRestore(item)}>Restore</BarButton>
                  )}
                  {item.content === "backup" && isPbs && canRestore && (
                    <BarButton onClick={() => setBrowse(item)}>Files</BarButton>
                  )}
                  {item.content === "backup" && !isPbs && node && (
                    <BarButton onClick={() => { window.location.href = api.volumeDownloadUrl(storage, node, item.volid); }}>Download</BarButton>
                  )}
                  {canDelete && <BarButton onClick={() => setDel(item)}>Delete</BarButton>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!rows.length && !items.isLoading && !backupsHidden && (
          <p className="p-4 text-sm text-ink-dim">
            {items.data?.length ? "No rows match the filters." : "No content on this storage."}
          </p>
        )}
        {backupsHidden && (
          <p className="border-t border-line p-4 text-sm text-ink-dim">
            Backups on this storage are hidden from this connection. Proxmox only lists a backup when the API token has{" "}
            <span className="font-mono text-xs text-ink">Datastore.AllocateSpace</span> on this storage and{" "}
            <span className="font-mono text-xs text-ink">VM.Backup</span> on the guest it belongs to.
          </p>
        )}
      </Card>

      {panel === "permissions" && (
        <Modal title="Permissions" onClose={() => setPanel(null)} size="lg">
          <ObjectACL path={storePath} />
        </Modal>
      )}
      {panel === "history" && node && (
        <Modal title="History" onClose={() => setPanel(null)} size="lg">
          <ObjectTaskHistory node={node} storage={storage} />
        </Modal>
      )}
      {panel === "prune" && node && (
        <Modal title="Prune backups" onClose={() => setPanel(null)} size="lg">
          <PruneCard storage={storage} node={node} />
        </Modal>
      )}
      {noteVol && canAllocate && (
        <Modal
          title={`Notes · ${noteVol.volid}`}
          onClose={() => setNoteVol(null)}
          footer={
            <BarButton
              accent
              onClick={async () => {
                if (!node) return;
                const res = await api.updateVolumeNotes(storage, node, noteVol.volid, notes);
                setNoteMsg(res.supported === false ? "Saving notes is not available." : "Saved.");
                void items.refetch();
                if (res.supported !== false) setNoteVol(null);
              }}
            >
              Save notes
            </BarButton>
          }
        >
          <Textarea className="min-h-24" value={notes} onChange={(e) => setNotes(e.target.value)} />
          {noteMsg && <p className="text-sm text-ink-dim">{noteMsg}</p>}
        </Modal>
      )}
      {restore && node && (
        <RestoreDialog
          node={node}
          archive={restore.volid}
          storage={storage}
          defaultVmid={restore.vmid}
          onClose={() => setRestore(null)}
        />
      )}
      {browse && node && (
        <FileRestoreBrowser storage={storage} node={node} volume={browse.volid} onClose={() => setBrowse(null)} />
      )}
      {del && (
        <ConfirmDialog
          title={`Delete ${del.volid}?`}
          body="This removes the volume from storage. Guests using it may fail."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(null)}
          onConfirm={async () => {
            const res = await api.deleteVolume(storage, node, del.volid);
            attachTask(res.upid, [["content", storage, node], ...fleetKeys()]);
            setDel(null);
          }}
        />
      )}
    </div>
  );
}
