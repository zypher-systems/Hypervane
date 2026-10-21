import { attachTask } from "@/components/layout/AppShell";
import { BarButton, MoreMenu } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog, Modal } from "@/components/ui/dialog";
import { Advanced, CollapsedRaw, KeyValueTable, asRecord, asRecords } from "@/components/ui/item-details";
import { InventoryCard, InventoryEmpty, InventoryRow, joinSummary } from "@/components/ui/inventory";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { bytes } from "@/lib/format";
import type { DiskInfo } from "@/types";
import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

const kinds = ["lvm", "lvmthin", "directory", "zfs"] as const;
const kindLabel: Record<(typeof kinds)[number], string> = {
  lvm: "LVM",
  lvmthin: "LVM-thin",
  directory: "directory",
  zfs: "ZFS",
};

export function DisksPanel({ node, writable }: { node: string; writable: boolean }) {
  const qc = useQueryClient();
  const disks = useQuery({ queryKey: ["node-disks", node], queryFn: () => api.nodeDisks(node) });
  const backends = useQueries({
    queries: kinds.map((kind) => ({
      queryKey: ["node-disk-be", node, kind],
      queryFn: () => api.nodeDiskBackends(node, kind),
    })),
  });
  const [create, setCreate] = useState<(typeof kinds)[number] | null>(null);
  const [wipe, setWipe] = useState<DiskInfo | null>(null);
  const [gpt, setGpt] = useState<DiskInfo | null>(null);
  const [smart, setSmart] = useState<Record<string, unknown> | null>(null);
  const [zfsStatus, setZfsStatus] = useState<Record<string, unknown> | null>(null);
  const [zfsImport, setZfsImport] = useState(false);
  const [zfsMsg, setZfsMsg] = useState("");
  const [delBackend, setDelBackend] = useState<{ kind: string; name: string } | null>(null);

  const backendRows = kinds.flatMap((kind, i) =>
    (backends[i].data ?? []).map((b, j) => {
      const name = String(b.lv || b.vg || b.name || b.pool || j);
      const size = Number(b.size ?? b.lv_size);
      const free = Number(b.free);
      const parts = [
        Number.isFinite(size) && size > 0 ? bytes(size) : "",
        Number.isFinite(free) && free > 0 ? `${bytes(free)} free` : "",
        b.health ? String(b.health) : "",
      ];
      return { kind, name, size: parts.filter(Boolean).join(" · ") };
    }),
  );

  return (
    <div className="space-y-4">
      <InventoryCard title="Physical disks">
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {!(disks.data ?? []).length && <InventoryEmpty>No physical disks</InventoryEmpty>}
            {(disks.data ?? []).map((d) => (
              <InventoryRow
                key={d.devpath}
                kind={d.devpath}
                summary={joinSummary([bytes(d.size), d.used, d.gpt ? "GPT" : "no GPT", d.model])}
              >
                <BarButton onClick={async () => setSmart(await api.nodeDiskSMART(node, d.devpath))}>SMART</BarButton>
                {writable && !d.used && !d.gpt && <BarButton onClick={() => setGpt(d)}>Init GPT</BarButton>}
                {writable && <BarButton onClick={() => setWipe(d)}>Wipe</BarButton>}
              </InventoryRow>
            ))}
          </tbody>
        </table>
      </InventoryCard>
      <InventoryCard
        title="Disk backends"
        actions={writable ? (
          <MoreMenu
            label="Add"
            items={[
              ...kinds.map((k) => ({
                id: k,
                label: `Create ${kindLabel[k]}`,
                onClick: () => setCreate(k),
              })),
              { id: "zfs-import", label: "Import ZFS", onClick: () => setZfsImport(true) },
            ]}
          />
        ) : undefined}
        message={zfsMsg || undefined}
      >
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr>
              <th className="px-4 py-2">Kind</th>
              <th>Summary</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {!backendRows.length && <InventoryEmpty>No disk backends</InventoryEmpty>}
            {backendRows.map((row) => (
              <InventoryRow
                key={`${row.kind}:${row.name}`}
                kind={row.name}
                summary={joinSummary([row.kind, row.size])}
              >
                {row.kind === "zfs" && (
                  <BarButton onClick={async () => {
                    const res = await api.zfsStatus(node, row.name);
                    if (!res.supported) {
                      setZfsMsg("ZFS status is not available on this node.");
                      return;
                    }
                    setZfsStatus(res.pool ?? {});
                  }}>Status</BarButton>
                )}
                {writable && row.kind === "zfs" && (
                  <BarButton onClick={async () => {
                    const res = await api.zfsScrub(node, row.name);
                    if (!res.supported) {
                      setZfsMsg("ZFS scrub is not available on this node.");
                      return;
                    }
                    attachTask(res.upid, [["node-disk-be", node], ["node-disks", node]]);
                  }}>Scrub</BarButton>
                )}
                {writable && (
                  <BarButton onClick={() => setDelBackend({ kind: row.kind, name: row.name })}>Delete</BarButton>
                )}
              </InventoryRow>
            ))}
          </tbody>
        </table>
      </InventoryCard>
      {create && (
        <CreateBackend
          node={node}
          kind={create}
          disks={disks.data ?? []}
          onClose={() => setCreate(null)}
        />
      )}
      {delBackend && (
        <ConfirmDialog
          title={`Delete ${delBackend.kind} ${delBackend.name}?`}
          body="Removes this storage backend from the node. Data on it may be lost."
          danger
          confirmLabel="Delete"
          onClose={() => setDelBackend(null)}
          onConfirm={async () => {
            attachTask((await api.deleteNodeDiskBackend(node, delBackend.kind, delBackend.name)).upid, [["node-disk-be", node], ["node-disks", node]]);
            setDelBackend(null);
          }}
        />
      )}
      {wipe && (
        <ConfirmDialog
          title={`Wipe ${wipe.devpath}?`}
          body="Destroys all data on this disk."
          danger
          confirmLabel="Wipe"
          onClose={() => setWipe(null)}
          onConfirm={async () => {
            attachTask((await api.wipeNodeDisk(node, wipe.devpath)).upid, [["node-disks", node], ["node-disk-be", node]]);
            setWipe(null);
          }}
        />
      )}
      {gpt && (
        <ConfirmDialog
          title={`Initialize GPT on ${gpt.devpath}?`}
          body="Writes a GPT partition table on this unused disk."
          confirmLabel="Init GPT"
          onClose={() => setGpt(null)}
          onConfirm={async () => {
            attachTask((await api.initGPT(node, gpt.devpath)).upid, [["node-disks", node]]);
            setGpt(null);
          }}
        />
      )}
      {smart && (
        <Modal title="SMART" onClose={() => setSmart(null)}>
          <SMARTView data={smart} />
        </Modal>
      )}
      {zfsStatus && (
        <Modal title="ZFS status" onClose={() => setZfsStatus(null)}>
          <ZFSStatusView data={zfsStatus} />
        </Modal>
      )}
      {zfsImport && (
        <ZFSImport
          node={node}
          disks={disks.data ?? []}
          onClose={() => setZfsImport(false)}
          onDone={(msg) => {
            setZfsImport(false);
            setZfsMsg(msg);
            void qc.invalidateQueries({ queryKey: ["node-disk-be", node, "zfs"] });
          }}
        />
      )}
    </div>
  );
}

function CreateBackend({
  node, kind, disks, onClose,
}: {
  node: string;
  kind: string;
  disks: DiskInfo[];
  onClose: () => void;
}) {
  const [name, setName] = useState("");
  const [device, setDevice] = useState(disks[0]?.devpath || "");
  const [error, setError] = useState("");
  return (
    <Modal
      title={`Create ${kind}`}
      onClose={onClose}
      footer={
        <Button onClick={async () => {
          setError("");
          try {
            const body: Record<string, unknown> = { name, add_storage: 1, storage: name };
            if (kind === "zfs") {
              body.devices = device;
              body.raidlevel = "single";
            } else {
              body.device = device;
            }
            attachTask((await api.createNodeDiskBackend(node, kind, body)).upid, [["node-disk-be", node], ["node-disks", node]]);
            onClose();
          } catch (e) {
            setError(e instanceof Error ? e.message : "Create failed");
          }
        }}>
          Save
        </Button>
      }
    >
      <Field label="Name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
      <Field label="Device">
        <Select value={device} onChange={(e) => setDevice(e.target.value)}>
          {disks.map((d) => <option key={d.devpath}>{d.devpath}</option>)}
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function ZFSImport({
  node, disks, onClose, onDone,
}: {
  node: string;
  disks: DiskInfo[];
  onClose: () => void;
  onDone: (msg: string) => void;
}) {
  const [name, setName] = useState("");
  const [device, setDevice] = useState("");
  const [error, setError] = useState("");
  return (
    <Modal
      title="Import ZFS pool"
      onClose={onClose}
      footer={
        <Button onClick={async () => {
          setError("");
          try {
            const res = await api.zfsImport(node, { name, device });
            if (!res.supported) {
              onDone("ZFS import is not available on this node.");
              return;
            }
            attachTask(res.upid, [["node-disk-be", node], ["node-disks", node]]);
            onClose();
          } catch (e) {
            setError(e instanceof Error ? e.message : "Import failed");
          }
        }}>
          Save
        </Button>
      }
    >
      <Field label="Pool name"><Input value={name} onChange={(e) => setName(e.target.value)} /></Field>
      <Field label="Device (optional)">
        <Select value={device} onChange={(e) => setDevice(e.target.value)}>
          <option value="">None</option>
          {disks.map((d) => <option key={d.devpath}>{d.devpath}</option>)}
        </Select>
      </Field>
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function SMARTView({ data }: { data: Record<string, unknown> }) {
  const rec = asRecord(data);
  const attrs = asRecords(rec.attributes ?? rec.ata_smart_attributes ?? rec.Attributes);
  return (
    <div className="space-y-3">
      <KeyValueTable data={rec} keys={["health", "type", "model", "serial", "wearout", "temperature"]} />
      {!!attrs.length && (
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-2 py-1">ID</th><th>Name</th><th>Value</th><th>Raw</th></tr>
          </thead>
          <tbody>
            {attrs.map((a, i) => (
              <tr key={String(a.id ?? a.name ?? i)} className="border-t border-line">
                <td className="px-2 py-1 font-mono text-xs">{String(a.id ?? a.num ?? "—")}</td>
                <td>{String(a.name ?? a.attr ?? "—")}</td>
                <td className="font-mono text-xs">{String(a.value ?? a.normalized ?? "—")}</td>
                <td className="font-mono text-xs text-ink-dim">{String(a.raw ?? a.raw_value ?? "—")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <Advanced label="Advanced: raw SMART">
        <CollapsedRaw data={data} />
      </Advanced>
    </div>
  );
}

function ZFSStatusView({ data }: { data: Record<string, unknown> }) {
  const rec = asRecord(data);
  const scan = asRecord(rec.scan ?? rec.scan_status);
  return (
    <div className="space-y-3">
      <KeyValueTable data={rec} keys={["name", "state", "status", "errors", "scan"]} />
      {Object.keys(scan).length > 0 && (
        <KeyValueTable data={scan} keys={["function", "state", "errors", "start_time", "end_time"]} />
      )}
      <Advanced label="Advanced: raw ZFS status">
        <CollapsedRaw data={data} />
      </Advanced>
    </div>
  );
}
