import { LogPanel } from "@/components/firewall/shared";
import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { bytes } from "@/lib/format";
import type { DiskInfo } from "@/types";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { useState } from "react";

export function CephPanel({ node, writable }: { node: string; writable: boolean }) {
  const status = useQuery({ queryKey: ["ceph-status", node], queryFn: () => api.nodeCeph(node) });
  const disks = useQuery({ queryKey: ["node-disks", node], queryFn: () => api.nodeDisks(node) });
  const log = useQuery({ queryKey: ["node-ceph-log", node], queryFn: () => api.nodeCephLog(node), refetchInterval: 15000 });
  const releases = useQuery({ queryKey: ["ceph-releases", node], queryFn: () => api.cephReleases(node) });
  const releaseNames = (releases.data?.items ?? [])
    .map((item) => String(item.name || item.release || item.version || ""))
    .filter(Boolean);
  const [version, setVersion] = useState("squid");
  const [repo, setRepo] = useState("no-subscription");
  const [resumeBulk, setResumeBulk] = useState(false);
  const [bulk, setBulk] = useState(false);
  const [network, setNetwork] = useState("");
  const [clusterNet, setClusterNet] = useState("");
  const [monName, setMonName] = useState(node);
  const [mgrName, setMgrName] = useState(node);
  const [osdDisk, setOsdDisk] = useState<DiskInfo | null>(null);
  const [encrypted, setEncrypted] = useState(false);
  const [klass, setKlass] = useState("");
  const [monAddress, setMonAddress] = useState("");
  const [dbDev, setDbDev] = useState("");
  const [walDev, setWalDev] = useState("");
  const [osdsPerDevice, setOsdsPerDevice] = useState("");

  const unused = (disks.data ?? []).filter((d) => !d.used || d.used === "unused");

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-dim">
        Cluster view is on{" "}
        <Link className="text-accent hover:underline" to="/ceph" search={{ node }}>
          /ceph
        </Link>
        . This tab installs packages, inits the cluster, and creates OSDs on unused disks.
      </p>
      {!status.data?.supported && (
        <Card className="p-4 text-sm text-ink-dim">Ceph is not running on this node yet. Install packages, then init.</Card>
      )}
      {status.data?.supported && (
        <Card className="p-4">
          <div className="text-xs text-ink-dim">Health</div>
          <div className="font-medium">{String((status.data.data?.health as { status?: string } | undefined)?.status || "ok")}</div>
        </Card>
      )}

      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Install packages</h2>
          <div className="flex flex-wrap gap-2">
            <Field label="Release">
              <Select value={version} onChange={(e) => setVersion(e.target.value)}>
                {(releaseNames.length ? releaseNames : ["squid", "reef", "quincy"]).map((name) => (
                  <option key={name}>{name}</option>
                ))}
              </Select>
            </Field>
            <Field label="Repository">
              <Select value={repo} onChange={(e) => setRepo(e.target.value)}>
                <option value="no-subscription">no-subscription</option>
                <option value="enterprise">enterprise</option>
                <option value="test">test</option>
              </Select>
            </Field>
          </div>
          <Button
            size="sm"
            onClick={async () => {
              attachTask((await api.installCeph(node, { version, repository: repo })).upid, [["ceph-status", node], ["ceph-releases", node]]);
            }}
          >
            Install
          </Button>
        </Card>
      )}

      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Init cluster</h2>
          <div className="flex flex-wrap gap-2">
            <Input className="max-w-xs" placeholder="public network 10.0.0.0/24" value={network} onChange={(e) => setNetwork(e.target.value)} />
            <Input className="max-w-xs" placeholder="cluster network (optional)" value={clusterNet} onChange={(e) => setClusterNet(e.target.value)} />
            <Button
              size="sm"
              onClick={async () => {
                const body: Record<string, unknown> = {};
                if (network) body.network = network;
                if (clusterNet) body["cluster-network"] = clusterNet;
                attachTask((await api.initCeph(node, body)).upid, [["ceph-status", node]]);
              }}
            >
              Init
            </Button>
          </div>
        </Card>
      )}

      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Monitor / manager</h2>
          <div className="flex flex-wrap gap-2">
            <Input className="max-w-xs" value={monName} onChange={(e) => setMonName(e.target.value)} />
            <Input className="max-w-xs" placeholder="mon-address (optional)" value={monAddress} onChange={(e) => setMonAddress(e.target.value)} />
            <Button
              size="sm"
              onClick={async () => {
                const body: Record<string, unknown> = { name: monName };
                if (monAddress) body["mon-address"] = monAddress;
                attachTask((await api.createCephMon(body, node)).upid, [["ceph-status", node], ["ceph-metadata", node]]);
              }}
            >
              Create mon
            </Button>
            <Input className="max-w-xs" value={mgrName} onChange={(e) => setMgrName(e.target.value)} />
            <Button
              size="sm"
              onClick={async () => {
                attachTask((await api.createCephMgr({ name: mgrName }, node)).upid, [["ceph-status", node], ["ceph-metadata", node]]);
              }}
            >
              Create mgr
            </Button>
          </div>
        </Card>
      )}

      <Card className="space-y-3 p-4">
        <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Unused disks</h2>
        {writable && (
          <div className="flex flex-wrap gap-2">
            <label className="flex items-center gap-2 text-sm text-ink-dim">
              <input type="checkbox" checked={encrypted} onChange={(e) => setEncrypted(e.target.checked)} />
              Encrypted
            </label>
            <Input className="max-w-xs" placeholder="crush-device-class" value={klass} onChange={(e) => setKlass(e.target.value)} />
            <Input className="max-w-xs" placeholder="db_dev" value={dbDev} onChange={(e) => setDbDev(e.target.value)} />
            <Input className="max-w-xs" placeholder="wal_dev" value={walDev} onChange={(e) => setWalDev(e.target.value)} />
            <Input className="max-w-xs" placeholder="osds-per-device" value={osdsPerDevice} onChange={(e) => setOsdsPerDevice(e.target.value)} />
          </div>
        )}
        {!unused.length && <p className="text-sm text-ink-dim">No unused disks. Wipe a disk on the Disks tab first.</p>}
        {unused.map((d) => (
          <div key={d.devpath} className="flex items-center justify-between text-sm">
            <span className="font-mono text-xs">
              {d.devpath} · {bytes(d.size)} {d.model ? `· ${d.model}` : ""}
            </span>
            {writable && (
              <Button size="sm" onClick={() => setOsdDisk(d)}>
                Create OSD
              </Button>
            )}
          </div>
        ))}
      </Card>

      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">OSD restart-bulk</h2>
          <p className="text-sm text-ink-dim">Rolling restart of OSDs on this node. Dry-run first.</p>
          <label className="flex items-center gap-2 text-sm text-ink-dim">
            <input type="checkbox" checked={resumeBulk} onChange={(e) => setResumeBulk(e.target.checked)} />
            Resume an aborted run
          </label>
          <BarButton onClick={() => setBulk(true)}>Restart OSDs</BarButton>
        </Card>
      )}

      <LogPanel lines={log.data ?? []} />

      {bulk && (
        <ConfirmDialog
          title={`Restart OSDs on ${node}?`}
          body="Runs a rolling OSD restart on this node. Confirm the cluster can absorb it."
          confirmLabel="Restart OSDs"
          danger
          onClose={() => setBulk(false)}
          onConfirm={async () => {
            attachTask((await api.nodeCephRestartBulk(node, { resume: resumeBulk ? 1 : 0 })).upid, [["ceph-status", node], ["node-ceph-log", node]]);
            setBulk(false);
          }}
        />
      )}
      {osdDisk && (
        <ConfirmDialog
          title={`Create OSD on ${osdDisk.devpath}?`}
          body="The disk will be consumed by Ceph. Confirm the device is unused."
          confirmLabel="Create OSD"
          onClose={() => setOsdDisk(null)}
          onConfirm={async () => {
            const body: Record<string, unknown> = { dev: osdDisk.devpath };
            if (encrypted) body.encrypted = 1;
            if (klass) body["crush-device-class"] = klass;
            if (dbDev) body.db_dev = dbDev;
            if (walDev) body.wal_dev = walDev;
            if (osdsPerDevice) body["osds-per-device"] = Number(osdsPerDevice);
            attachTask((await api.createNodeCephOSD(node, body)).upid, [["node-disks", node], ["ceph-osds"], ["ceph-status", node]]);
            setOsdDisk(null);
          }}
        />
      )}
    </div>
  );
}
