import { FileRestoreBrowser } from "@/components/backups/FileRestoreBrowser";
import { RestoreDialog } from "@/components/backups/RestoreDialog";
import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { bytes, relTime } from "@/lib/format";
import { guestKeys } from "@/lib/tasks";
import type { StorageContent } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { useMemo, useState } from "react";

type GuestBackup = StorageContent & { store: string };

export function BackupPanel({
  vmid, node, type,
}: {
  vmid: number; node: string; type: string;
}) {
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const backupStores = (storages.data ?? []).filter((s) => s.content?.includes("backup") && !s.disable);
  const fleeceStores = (storages.data ?? []).map((s) => s.storage);
  const [storage, setStorage] = useState("");
  const [mode, setMode] = useState("snapshot");
  const [notes, setNotes] = useState("");
  const [compress, setCompress] = useState("zstd");
  const [bwlimit, setBwlimit] = useState("");
  const [mailto, setMailto] = useState("");
  const [mailNote, setMailNote] = useState("always");
  const [notifyMode, setNotifyMode] = useState("auto");
  const [fleeceOn, setFleeceOn] = useState(false);
  const [fleeceStore, setFleeceStore] = useState("");
  const [pbsDetect, setPbsDetect] = useState("");
  const [error, setError] = useState("");
  const [restore, setRestore] = useState<GuestBackup | null>(null);
  const [browse, setBrowse] = useState<GuestBackup | null>(null);

  const dest = storage || backupStores[0]?.storage || "";
  const pbs = useMemo(() => new Set((storages.data ?? []).filter((s) => s.type === "pbs").map((s) => s.storage)), [storages.data]);

  const volumes = useQuery({
    queryKey: ["guest-backups", dest, node, vmid],
    queryFn: async () => {
      const items: GuestBackup[] = [];
      for (const st of backupStores) {
        try {
          const list = await api.storageContent(st.storage, node, "backup");
          items.push(...list.filter((it) => it.vmid === vmid).map((it) => ({ ...it, store: st.storage })));
        } catch {
          /* storage may not be reachable on this node */
        }
      }
      return items.sort((a, b) => (b.ctime ?? 0) - (a.ctime ?? 0));
    },
    enabled: !!node && backupStores.length > 0,
  });

  return (
    <div className="space-y-4">
      <Card className="grid gap-3 p-5 md:grid-cols-3">
        <Field label="Storage">
          <Select value={dest} onChange={(e) => setStorage(e.target.value)}>
            {backupStores.map((s) => <option key={s.storage}>{s.storage}</option>)}
          </Select>
        </Field>
        <Field label="Mode">
          <Select value={mode} onChange={(e) => setMode(e.target.value)}>
            <option value="snapshot">Snapshot</option>
            <option value="suspend">Suspend</option>
            <option value="stop">Stop</option>
          </Select>
        </Field>
        <Field label="Compress">
          <Select value={compress} onChange={(e) => setCompress(e.target.value)}>
            <option value="zstd">zstd</option>
            <option value="gzip">gzip</option>
            <option value="lzo">lzo</option>
            <option value="0">none</option>
          </Select>
        </Field>
        <Field label="Notes"><Input value={notes} onChange={(e) => setNotes(e.target.value)} /></Field>
        <Field label="Bandwidth limit"><Input value={bwlimit} onChange={(e) => setBwlimit(e.target.value)} placeholder="kbps" /></Field>
        <Field label="Mail to"><Input value={mailto} onChange={(e) => setMailto(e.target.value)} /></Field>
        <Field label="Mail notification">
          <Select value={mailNote} onChange={(e) => setMailNote(e.target.value)}>
            <option value="always">always</option>
            <option value="failure">failure</option>
          </Select>
        </Field>
        <Field label="Notification mode">
          <Select value={notifyMode} onChange={(e) => setNotifyMode(e.target.value)}>
            <option value="auto">auto</option>
            <option value="legacy-sendmail">legacy-sendmail</option>
            <option value="notification-system">notification-system</option>
          </Select>
        </Field>
        <label className="flex items-end gap-2 text-sm">
          <input type="checkbox" checked={fleeceOn} onChange={(e) => setFleeceOn(e.target.checked)} />
          Fleecing
        </label>
        <Field label="Fleecing storage">
          <Select value={fleeceStore} onChange={(e) => setFleeceStore(e.target.value)}>
            <option value="">Default</option>
            {fleeceStores.map((s) => <option key={s} value={s}>{s}</option>)}
          </Select>
        </Field>
        <Field label="PBS change detection">
          <Select value={pbsDetect} onChange={(e) => setPbsDetect(e.target.value)}>
            <option value="">Default</option>
            <option value="legacy">legacy</option>
            <option value="data">data</option>
            <option value="metadata">metadata</option>
          </Select>
        </Field>
        <div className="md:col-span-3">
          <Button
            onClick={async () => {
              setError("");
              try {
                const res = await api.guestBackup(vmid, {
                  storage: dest,
                  mode,
                  notes,
                  compress,
                  bwlimit,
                  fleecing: fleeceOn ? `enabled=1${fleeceStore ? `,storage=${fleeceStore}` : ""}` : "enabled=0",
                  ...(mailto ? { mailto, mailnotification: mailNote, "notification-mode": notifyMode } : {}),
                  ...(pbsDetect ? { pbsChangeDetection: pbsDetect } : {}),
                }, node, type);
                attachTask(res.upid, [...guestKeys(vmid), ["guest-backups", dest, node, vmid]]);
              } catch (err) {
                setError(err instanceof Error ? err.message : "Backup failed");
              }
            }}
          >
            Backup now
          </Button>
        </div>
        {error && <p className="text-sm text-rose-400 md:col-span-3">{error}</p>}
      </Card>
      <Card className="overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Volume</th>
              <th>Size</th>
              <th>Created</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(volumes.data ?? []).map((item) => (
                <tr key={item.volid} className="border-t border-line">
                  <td className="px-4 py-2 font-mono text-xs">{item.volid}</td>
                  <td>{bytes(item.size)}</td>
                  <td className="text-ink-dim">{relTime(item.ctime)}</td>
                  <td className="space-x-2 px-4 text-right">
                    <Button size="sm" variant="outline" onClick={() => setRestore(item)}>Restore</Button>
                    {pbs.has(item.store) && (
                      <Button size="sm" variant="outline" onClick={() => setBrowse(item)}>
                        Files
                      </Button>
                    )}
                  </td>
                </tr>
            ))}
          </tbody>
        </table>
        {!volumes.data?.length && <p className="p-4 text-sm text-ink-dim">No backups found for this guest.</p>}
      </Card>
      {restore && (
        <RestoreDialog
          node={node}
          archive={restore.volid}
          defaultType={type}
          defaultVmid={vmid}
          onClose={() => setRestore(null)}
        />
      )}
      {browse && (
        <FileRestoreBrowser
          storage={browse.store}
          node={node}
          volume={browse.volid}
          onClose={() => setBrowse(null)}
        />
      )}
    </div>
  );
}
