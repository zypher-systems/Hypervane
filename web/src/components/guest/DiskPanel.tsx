import { attachTask } from "@/components/layout/AppShell";
import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { ImportMetadataView, asRecord } from "@/components/ui/item-details";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { bytes } from "@/lib/format";
import { guestKeys } from "@/lib/tasks";
import { busMax, configDigest, diskSizeGiB, formatDisk, formatImport, formatKV, formatMount, nextSlot, parseKV } from "@/lib/guestHardware";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

export function EditDiskDialog({
  slot, value, vmid, node, type, onClose, onSaved,
}: {
  slot: string; value: string; vmid: number; node: string; type: string;
  onClose: () => void; onSaved: () => void;
}) {
  const parsed = parseKV(value);
  const [cache, setCache] = useState(parsed.extras.cache ?? "");
  const [discard, setDiscard] = useState(parsed.extras.discard === "on" || parsed.extras.discard === "1");
  const [ssd, setSsd] = useState(parsed.extras.ssd === "1");
  const [iothread, setIothread] = useState(parsed.extras.iothread === "1");
  return (
    <Modal
      title={`Edit ${slot}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            const extras = { ...parsed.extras };
            if (cache) extras.cache = cache;
            else delete extras.cache;
            extras.discard = discard ? "on" : "";
            extras.ssd = ssd ? "1" : "";
            extras.iothread = iothread ? "1" : "";
            await api.updateGuest(vmid, {
              node, type, fields: { [slot]: formatKV(parsed.head, extras) },
            }, node, type);
            onSaved();
          }}
        >
          Save
        </Button>
      }
    >
      <Field label="Cache">
        <Select value={cache} onChange={(e) => setCache(e.target.value)}>
          <option value="">Default</option>
          <option value="none">none</option>
          <option value="writethrough">writethrough</option>
          <option value="writeback">writeback</option>
          <option value="unsafe">unsafe</option>
        </Select>
      </Field>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={discard} onChange={(e) => setDiscard(e.target.checked)} /> Discard</label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={ssd} onChange={(e) => setSsd(e.target.checked)} /> SSD emulation</label>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={iothread} onChange={(e) => setIothread(e.target.checked)} /> IO thread</label>
    </Modal>
  );
}

export function MoveDiskDialog({
  slot, vmid, node, type, storages, digest, onClose, onDone,
}: {
  slot: string;
  vmid: number;
  node: string;
  type: string;
  storages: string[];
  digest: string;
  onClose: () => void;
  onDone: () => void;
}) {
  const qemu = type === "qemu";
  const [storage, setStorage] = useState(storages[0] ?? "");
  const [format, setFormat] = useState("");
  const [deleteSource, setDeleteSource] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  return (
    <Modal
      title={`Move ${slot}`}
      onClose={busy ? () => {} : onClose}
      footer={
        <Button
          disabled={!storage || busy}
          onClick={async () => {
            if (!storage || busy) return;
            setBusy(true);
            setError("");
            try {
              const res = await api.moveDisk(vmid, {
                disk: slot,
                storage,
                ...(qemu && format ? { format } : {}),
                delete: deleteSource,
                digest: digest || undefined,
              }, node, type);
              attachTask(res.upid, [["guest", vmid]]);
              onDone();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Move failed");
              setBusy(false);
            }
          }}
        >
          {busy ? "Moving…" : "Move"}
        </Button>
      }
    >
      <p className="text-sm text-ink-dim">
        Long-running. Can use double the disk space until the source volume is deleted.
      </p>
      <Field label="Destination storage">
        <Select value={storage} onChange={(e) => setStorage(e.target.value)}>
          {storages.map((s) => <option key={s}>{s}</option>)}
        </Select>
      </Field>
      {qemu && (
        <Field label="Format">
          <Select value={format} onChange={(e) => setFormat(e.target.value)}>
            <option value="">Keep current</option>
            <option value="raw">raw</option>
            <option value="qcow2">qcow2</option>
            <option value="vmdk">vmdk</option>
          </Select>
        </Field>
      )}
      <label className="flex items-center gap-2 text-sm">
        <input type="checkbox" checked={deleteSource} onChange={(e) => setDeleteSource(e.target.checked)} disabled={busy} />
        Delete source after move
      </label>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function ResizeDiskDialog({
  slot, current, vmid, node, type, onClose, onResized,
}: {
  slot: string;
  current?: string;
  vmid: number;
  node: string;
  type: string;
  onClose: () => void;
  onResized: (size: string) => void;
}) {
  const currentGiB = diskSizeGiB(current);
  const currentLabel = currentGiB != null ? bytes(currentGiB * 1024 ** 3) : current || "unknown";
  const [targetGiB, setTargetGiB] = useState(currentGiB != null ? String(Math.round(currentGiB + 8)) : "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const next = Number(targetGiB);
  const valid = Number.isFinite(next) && next > 0 && (currentGiB == null || next > currentGiB + 0.001);
  const nextLabel = valid ? bytes(next * 1024 ** 3) : "—";

  return (
    <Modal
      title={`Resize ${slot}`}
      onClose={busy ? () => {} : onClose}
      footer={
        <Button
          disabled={!valid || busy}
          onClick={async () => {
            if (!valid || busy) return;
            setBusy(true);
            setError("");
            try {
              const size = `${next}G`;
              const res = await api.resizeDisk(vmid, { disk: slot, size }, node, type);
              attachTask(res.upid, [["guest", vmid]]);
              onResized(size);
            } catch (e) {
              setError(e instanceof Error ? e.message : "Resize failed");
              setBusy(false);
            }
          }}
        >
          {busy ? "Resizing…" : `Grow to ${nextLabel}`}
        </Button>
      }
    >
      <p className="text-sm text-ink-dim">
        Current size is <span className="font-mono font-semibold text-ink">{currentLabel}</span>.
        Disks can only grow. This cannot be undone from Hypervane.
      </p>
      <Field label="New size (GiB)">
        <Input
          type="number"
          min={currentGiB != null ? String(Math.ceil(currentGiB + 0.1)) : "1"}
          step="1"
          value={targetGiB}
          onChange={(e) => setTargetGiB(e.target.value)}
          disabled={busy}
        />
      </Field>
      <p className="font-mono text-sm tabular-nums">
        <span className="text-ink-dim">{currentLabel}</span>
        <span className="mx-2 text-ink-faint">→</span>
        <span className="font-semibold text-ink">{nextLabel}</span>
      </p>
      {currentGiB != null && Number.isFinite(next) && next <= currentGiB && (
        <p className="text-sm text-rose-400">New size must be larger than {currentLabel}.</p>
      )}
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function AddDiskDialog({
  vmid, node, type, config, storages, unused, onClose, onSaved,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>;
  storages: string[]; unused: [string, string][]; onClose: () => void; onSaved: () => void;
}) {
  const [mode, setMode] = useState<"new" | "unused" | "cloudinit">("new");
  const [bus, setBus] = useState("scsi");
  const [storage, setStorage] = useState(storages[0] ?? "");
  const [sizeGb, setSizeGb] = useState("16");
  const [format, setFormat] = useState("");
  const [cache, setCache] = useState("");
  const [discard, setDiscard] = useState(true);
  const [iothread, setIothread] = useState(false);
  const [backup, setBackup] = useState(true);
  const [unusedKey, setUnusedKey] = useState(unused[0]?.[0] ?? "");
  const [error, setError] = useState("");
  const slot = nextSlot(config, bus, busMax(bus));

  return (
    <Modal
      title="Add disk"
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              let value = "";
              if (mode === "unused") value = unusedKey;
              else value = formatDisk({ storage, sizeGb, format, cache, discard, iothread, backup, cloudinit: mode === "cloudinit" });
              await api.updateGuest(vmid, {
                node, type, digest: configDigest(config), fields: { [slot]: value },
              }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add disk failed");
            }
          }}
        >
          Add {slot}
        </Button>
      }
    >
      <div className="flex flex-wrap gap-2 text-sm">
        {(["new", "unused", "cloudinit"] as const).map((m) => (
          <Button key={m} size="sm" variant={mode === m ? "selected" : "outline"} onClick={() => setMode(m)}>
            {m === "new" ? "New volume" : m === "unused" ? "Unused" : "Cloud-init"}
          </Button>
        ))}
      </div>
      <Field label="Bus">
        <Select value={bus} onChange={(e) => setBus(e.target.value)}>
          <option value="scsi">SCSI</option>
          <option value="virtio">VirtIO Block</option>
          <option value="sata">SATA</option>
          <option value="ide">IDE</option>
        </Select>
      </Field>
      <p className="text-xs text-ink-dim">Next slot: {slot}</p>
      {mode === "unused" ? (
        <Field label="Unused volume">
          <Select value={unusedKey} onChange={(e) => setUnusedKey(e.target.value)}>
            {unused.length === 0 && <option value="">None</option>}
            {unused.map(([k, v]) => <option key={k} value={k}>{k}: {v}</option>)}
          </Select>
        </Field>
      ) : (
        <>
          <Field label="Storage">
            <Select value={storage} onChange={(e) => setStorage(e.target.value)}>
              {storages.map((s) => <option key={s}>{s}</option>)}
            </Select>
          </Field>
          {mode === "new" && (
            <>
              <Field label="Size (GiB)"><Input type="number" value={sizeGb} onChange={(e) => setSizeGb(e.target.value)} /></Field>
              <Field label="Format">
                <Select value={format} onChange={(e) => setFormat(e.target.value)}>
                  <option value="">Default</option>
                  <option value="raw">raw</option>
                  <option value="qcow2">qcow2</option>
                  <option value="vmdk">vmdk</option>
                </Select>
              </Field>
              <Field label="Cache">
                <Select value={cache} onChange={(e) => setCache(e.target.value)}>
                  <option value="">Default</option>
                  <option value="none">none</option>
                  <option value="writethrough">writethrough</option>
                  <option value="writeback">writeback</option>
                  <option value="unsafe">unsafe</option>
                </Select>
              </Field>
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={discard} onChange={(e) => setDiscard(e.target.checked)} /> Discard</label>
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={iothread} onChange={(e) => setIothread(e.target.checked)} /> IO thread</label>
              <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={backup} onChange={(e) => setBackup(e.target.checked)} /> Backup</label>
            </>
          )}
        </>
      )}
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function AddMountDialog({
  vmid, node, type, config, storages, onClose, onSaved,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>;
  storages: string[]; onClose: () => void; onSaved: () => void;
}) {
  const [mode, setMode] = useState<"volume" | "bind">("volume");
  const [storage, setStorage] = useState(storages[0] ?? "");
  const [sizeGb, setSizeGb] = useState("8");
  const [bind, setBind] = useState("");
  const [mp, setMp] = useState("/mnt/data");
  const [error, setError] = useState("");
  const slot = nextSlot(config, "mp", 255);

  return (
    <Modal
      title={`Add ${slot}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              if (mode === "bind" && !bind) throw new Error("Enter a host path");
              if (mode === "volume" && !storage) throw new Error("Pick storage");
              const value = formatMount({
                storage: mode === "volume" ? storage : undefined,
                sizeGb: mode === "volume" ? sizeGb : undefined,
                bind: mode === "bind" ? bind : undefined,
                mp,
              });
              await api.updateGuest(vmid, { node, type, digest: configDigest(config), fields: { [slot]: value } }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Add mount failed");
            }
          }}
        >
          Add
        </Button>
      }
    >
      <div className="flex gap-2">
        <Button size="sm" variant={mode === "volume" ? "selected" : "outline"} onClick={() => setMode("volume")}>Volume</Button>
        <Button size="sm" variant={mode === "bind" ? "selected" : "outline"} onClick={() => setMode("bind")}>Bind mount</Button>
      </div>
      {mode === "volume" ? (
        <>
          <Field label="Storage">
            <Select value={storage} onChange={(e) => setStorage(e.target.value)}>
              {storages.map((s) => <option key={s}>{s}</option>)}
            </Select>
          </Field>
          <Field label="Size (GiB)"><Input type="number" value={sizeGb} onChange={(e) => setSizeGb(e.target.value)} /></Field>
        </>
      ) : (
        <Field label="Host path"><Input value={bind} onChange={(e) => setBind(e.target.value)} placeholder="/mnt/shared" /></Field>
      )}
      <Field label="Mount point"><Input value={mp} onChange={(e) => setMp(e.target.value)} /></Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

export function ImportDiskDialog({
  vmid, node, type, config, storages, onClose, onSaved,
}: {
  vmid: number; node: string; type: string; config: Record<string, unknown>;
  storages: string[]; onClose: () => void; onSaved: () => void;
}) {
  const [mode, setMode] = useState<"volume" | "url">("volume");
  const [srcStore, setSrcStore] = useState(storages[0] ?? "");
  const [destStore, setDestStore] = useState(storages[0] ?? "");
  const [volume, setVolume] = useState("");
  const [url, setUrl] = useState("");
  const [filename, setFilename] = useState("disk.ova");
  const [meta, setMeta] = useState<Record<string, unknown> | null>(null);
  const [metaMsg, setMetaMsg] = useState("");
  const [error, setError] = useState("");
  const slot = nextSlot(config, "scsi", busMax("scsi"));
  const volumes = useQuery({
    queryKey: ["content", srcStore, node, "import"],
    queryFn: () => api.storageContent(srcStore, node).catch(() => []),
    enabled: mode === "volume" && !!srcStore && !!node,
  });

  return (
    <Modal
      title={`Import ${slot}`}
      onClose={onClose}
      footer={
        <Button
          onClick={async () => {
            setError("");
            try {
              if (mode === "url") {
                if (!url) throw new Error("Enter a URL");
                const res = await api.downloadUrl(srcStore, node, { url, filename, content: "import" });
                if (res.upid) attachTask(res.upid, [["content", srcStore, node], ...guestKeys(vmid)]);
                const volid = `${srcStore}:import/${filename}`;
                await api.updateGuest(vmid, {
                  node, type, digest: configDigest(config),
                  fields: { [slot]: formatImport({ storage: destStore, importFrom: volid }) },
                }, node, type);
              } else {
                if (!volume) throw new Error("Pick a volume");
                await api.updateGuest(vmid, {
                  node, type, digest: configDigest(config),
                  fields: { [slot]: formatImport({ storage: destStore, importFrom: volume }) },
                }, node, type);
              }
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Import failed");
            }
          }}
        >
          Import
        </Button>
      }
    >
      <div className="flex gap-2">
        <Button size="sm" variant={mode === "volume" ? "selected" : "outline"} onClick={() => setMode("volume")}>Volume</Button>
        <Button size="sm" variant={mode === "url" ? "selected" : "outline"} onClick={() => setMode("url")}>URL</Button>
      </div>
      <Field label="Source storage">
        <Select value={srcStore} onChange={(e) => setSrcStore(e.target.value)}>
          {storages.map((s) => <option key={s}>{s}</option>)}
        </Select>
      </Field>
      <Field label="Destination storage">
        <Select value={destStore} onChange={(e) => setDestStore(e.target.value)}>
          {storages.map((s) => <option key={s}>{s}</option>)}
        </Select>
      </Field>
      {mode === "volume" ? (
        <Field label="Volume">
          <Select value={volume} onChange={(e) => setVolume(e.target.value)}>
            <option value="">Select…</option>
            {(volumes.data ?? []).map((v) => (
              <option key={v.volid} value={v.volid}>{v.volid}</option>
            ))}
          </Select>
        </Field>
      ) : (
        <>
          <Field label="URL"><Input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://…" /></Field>
          <Field label="Filename"><Input value={filename} onChange={(e) => setFilename(e.target.value)} /></Field>
        </>
      )}
      <Button
        size="sm"
        variant="outline"
        onClick={async () => {
          setError("");
          try {
            const res = mode === "url"
              ? await api.queryUrlMetadata(srcStore, node, url)
              : await api.importMetadata(srcStore, node, volume);
            if (!res.supported) {
              setMeta(null);
              setMetaMsg("Metadata is not available on this node.");
              return;
            }
            setMetaMsg("");
            setMeta(asRecord(res.data));
            const name = String(res.data?.filename ?? "");
            if (name) setFilename(name);
          } catch (e) {
            setError(e instanceof Error ? e.message : "Metadata failed");
          }
        }}
      >
        Show metadata
      </Button>
      {(meta || metaMsg) && <ImportMetadataView data={meta} message={metaMsg} />}
      <p className="text-xs text-ink-dim">Attaches {slot} with import-from. URL mode starts a download first.</p>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}
